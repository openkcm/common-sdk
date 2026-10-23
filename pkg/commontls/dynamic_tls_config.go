// Package commontls provides [DynamicTLSConfig], a *tls.Config for mutual TLS
// clients whose credentials follow certificate rotation on disk.
//
// A DynamicTLSConfig embeds the *tls.Config to use. It loads a client
// certificate, its private key and, optionally, the certificate authorities
// that verify the server, all described by a [commoncfg.MTLS] value. When any
// of them come from files, it watches the directories holding those files and
// reloads the credentials after a change. The embedded configuration reads the
// credentials at handshake time, so it can be handed once to an
// http.Transport or a gRPC transport credential and picks up a rotation on
// the next handshake, without being rebuilt.
//
// Typical usage, with the files of a Kubernetes TLS secret mounted at
// /etc/tls:
//
//	mtls := &commoncfg.MTLS{
//	    Cert: commoncfg.SourceRef{
//	        Source: commoncfg.FileSourceValue,
//	        File:   commoncfg.CredentialFile{Path: "/etc/tls/tls.crt"},
//	    },
//	    CertKey: commoncfg.SourceRef{
//	        Source: commoncfg.FileSourceValue,
//	        File:   commoncfg.CredentialFile{Path: "/etc/tls/tls.key"},
//	    },
//	    ServerCA: &commoncfg.SourceRef{
//	        Source: commoncfg.FileSourceValue,
//	        File:   commoncfg.CredentialFile{Path: "/etc/tls/ca.crt"},
//	    },
//	}
//
//	dyn, err := commontls.NewDynamicTLSConfig(2*time.Second, mtls)
//	if err != nil {
//	    return err
//	}
//	defer dyn.Close()
//
//	httpClient := &http.Client{
//	    Transport: &http.Transport{TLSClientConfig: dyn.Config},
//	}
//	grpcCreds := credentials.NewTLS(dyn.Config)
package commontls

import (
	"crypto/fips140"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commonfs/notifier"
)

// recheckInterval is how often the credential files are read again without a
// file system event, to catch changes the watcher missed.
const recheckInterval = time.Minute

var (
	errNoPeerCertificate = errors.New("tls: server sent no certificate")
	errNoServerName      = errors.New("tls: no server name to verify the server certificate against; " +
		"connect to the server by host name or set attributes.serverName")
)

// DynamicTLSConfig is a TLS client configuration for mutual TLS whose client
// certificate and certificate authorities are reloaded when their files
// change.
//
// It embeds *tls.Config, so dyn.Config can be passed wherever a *tls.Config
// is expected, and its fields and methods can be used directly. The embedded
// pointer stays the same for the life of the DynamicTLSConfig: a reload does
// not replace it, it changes what the configuration serves through two
// callbacks:
//   - GetClientCertificate presents the most recently loaded client
//     certificate and key (MTLS.Cert and MTLS.CertKey), choosing the same way
//     crypto/tls does for Certificates, which is therefore empty.
//   - VerifyConnection verifies the server against the most recently loaded
//     certificate authorities (MTLS.ServerCA and MTLS.RootCAs), when at least
//     one of them comes from a file. See below.
//
// The rules of tls.Config apply: do not modify the embedded configuration once
// it is in use, and use Clone to get a copy to change, for example to set
// NextProtos. A clone keeps the callbacks, so it follows reloads as well. Do
// not set Certificates or RootCAs or replace the callbacks: reloads would no
// longer take effect, and replacing VerifyConnection turns server
// verification off.
//
// The embedded configuration requires TLS 1.2 or later and applies
// MTLS.Attributes: ServerName, InsecureSkipVerify, SessionTicketsDisabled and
// AllowTLSRenegotiationOnce. The attributes are read once, when the
// DynamicTLSConfig is created.
//
// How the server is verified depends on where the certificate authorities
// come from:
//   - None configured, or none from a file: standard crypto/tls verification,
//     against those authorities in RootCAs or, if there are none, the system
//     pool. RootCAs does not change after creation.
//   - At least one from a file: crypto/tls reads RootCAs from the
//     configuration in use, and that field cannot be changed safely once the
//     configuration has been handed out. InsecureSkipVerify is therefore set,
//     RootCAs is nil, and VerifyConnection verifies the server against the
//     most recently loaded authorities: the chain must lead to one of them,
//     and the certificate must be valid for server authentication and for the
//     server name. In FIPS 140-3 mode crypto/tls also rejects chains with keys
//     that are not FIPS-approved; here such a chain is accepted and a warning
//     is logged on every handshake with it.
//
// In the second case the server name comes from SNI: the host the client
// connects to, or ServerName. SNI never carries an IP address, so when
// connecting to a server by IP address set ServerName (through
// MTLS.Attributes.ServerName) to a name or IP address the server certificate
// is valid for; otherwise the handshake fails instead of skipping the check.
//
// If MTLS.Attributes.InsecureSkipVerify is true, the server is not verified
// at all.
//
// Longer chains: MTLS.Cert may hold the client certificate followed by any
// number of intermediates, the way cert-manager writes tls.crt. The file is
// reloaded and sent as a whole, so rotating any of the intermediates, or
// several at once, is a reload like any other; without the intermediates in
// the file, a server that names its acceptable authorities gets no client
// certificate. The intermediates of the server are taken from the handshake,
// so a server moving to new intermediates under a trusted root needs no change
// here. Every certificate in the certificate
// authority sources is a trust anchor; if those sources hold an intermediate,
// rotating it means updating them.
//
// Only sources of type commoncfg.FileSourceValue are watched. Embedded and
// environment sources are read again on every reload but never trigger one.
// If no source is a file, nothing is watched and the credentials never
// change.
//
// Reloads happen after file changes and, as a safety net for changes the
// watcher misses, every minute. The client identity (certificate chain and
// key) and the certificate authorities are reloaded and activated
// independently, so a broken file of one does not hold back a rotation of the
// other. Unchanged files are detected by their contents and cause nothing.
//
// Reloaded credentials are validated before they are activated:
//   - Client identity: the certificate and key must parse and match, as
//     commoncfg requires. Then the whole chain is checked, at every level: the
//     client certificate and every intermediate must be within their validity
//     period (allowing five minutes of clock skew), every intermediate must
//     be a certificate authority that signed the certificate below it (in any
//     order in the file), no certificate may lie outside that chain, no key
//     may be weaker than RSA 2048 bits or ECDSA P-256, and the client
//     certificate must allow client authentication.
//   - Certificate authorities: every PEM block must be a complete, parsable
//     certificate, with nothing but blank and "#" comment lines between
//     blocks, so a file read while it is being written is caught; at least
//     one authority must be currently valid; and no key may be weak. When the
//     authorities come from files, the trusted set is exactly what they hold,
//     possibly nothing: it never falls back to the system authorities.
//
// A defect does not always break the credentials, so defects never prevent
// creation; they are logged. A reload is activated only if it brings no
// defect the active credentials do not already have, so a rotation can never
// make a working configuration worse, or if it repairs active credentials
// that no longer work, such as an expired client certificate. A rejected
// reload is logged with its defects and the active credentials stay in use;
// it is evaluated again on every later reload, so a certificate rejected for
// not being valid yet is activated once it is. When the active credentials
// develop a defect over time, typically when the client certificate expires,
// that is logged as well.
//
// Changing the files requires write access to them, which nothing here can
// defend against: whoever can write the certificate authorities can make the
// client trust any server.
//
// Established connections are not affected by a reload; only new handshakes
// use the new credentials. To move a long-lived client to a rotated
// certificate, its connections have to be reopened, for example by closing
// the idle connections of an http.Transport.
//
// A DynamicTLSConfig is safe for concurrent use. Call
// [DynamicTLSConfig.Close] when it is no longer needed to stop watching the
// files.
type DynamicTLSConfig struct {
	*tls.Config

	mtls     *commoncfg.MTLS
	notifier *notifier.Notifier
	now      func() time.Time

	// reloadAuthorities is set when the certificate authorities come from
	// files and verify the server in VerifyConnection.
	reloadAuthorities bool

	closeOnce sync.Once
	closed    atomic.Bool
	stop      chan struct{} // closed by Close, stops the periodic check

	// refreshMu serializes reloads, so the last one to finish read the
	// newest files, and guards logState.
	refreshMu sync.Mutex
	// logState holds, per log subject, the key of the last message logged,
	// so that a condition that persists is logged once.
	logState map[string]string

	// active holds the credentials in use. The callbacks of the embedded
	// configuration read them at handshake time; the embedded configuration
	// itself cannot hold them, since crypto/tls reads it without locking and
	// clones copy its fields but share its callbacks.
	active atomic.Pointer[credentials]
}

// credentials are the active client identity and certificate authorities.
// They are replaced as a whole, never modified.
type credentials struct {
	identity *identity
	// authorities is nil unless they are reloaded (reloadAuthorities).
	authorities *authorities
}

// component names a part of the credentials in log messages and log state.
type component string

const (
	componentIdentity    component = "client certificate"
	componentAuthorities component = "certificate authorities"
)

// NewDynamicTLSConfig loads the credentials described by mtls and starts
// watching the directories that hold its file sources.
//
// mtls is kept and read again on every reload, so it must not be modified
// afterwards.
//
// Directories are watched rather than the files themselves, so a reload also
// follows files that are replaced by a rename, such as the atomic symlink swap
// Kubernetes uses to update mounted secrets. This relies on inotify (Linux).
// On macOS and BSD the watcher can miss a rename over an existing file, so a
// rotation done that way may not be picked up there.
//
// throttleInterval groups changes: a reload runs once no further change has
// been seen for throttleInterval. Writing a certificate and its key in quick
// succession therefore causes a single reload. With zero or less, every change
// causes a reload.
//
// It returns an error if mtls is nil (commoncfg.ErrMTLSIsNil), if a directory
// to watch does not exist, or if the credentials cannot be loaded. Nothing is
// left running when it returns an error.
func NewDynamicTLSConfig(throttleInterval time.Duration, mtls *commoncfg.MTLS) (*DynamicTLSConfig, error) {
	return newDynamicTLSConfig(throttleInterval, recheckInterval, mtls, time.Now)
}

// newDynamicTLSConfig is NewDynamicTLSConfig with the interval of the
// periodic check (none if zero or less) and the clock as parameters.
func newDynamicTLSConfig(throttleInterval, recheck time.Duration, mtls *commoncfg.MTLS, now func() time.Time) (*DynamicTLSConfig, error) {
	if mtls == nil {
		return nil, commoncfg.ErrMTLSIsNil
	}

	// The configuration commoncfg builds gives the attributes, and fails
	// exactly where static credentials would.
	base, err := commoncfg.LoadMTLSConfig(mtls)
	if err != nil {
		return nil, err
	}

	d := &DynamicTLSConfig{
		mtls:              mtls,
		now:               now,
		reloadAuthorities: !base.InsecureSkipVerify && slices.ContainsFunc(caRefs(mtls), isFileSource),
		stop:              make(chan struct{}),
		logState:          make(map[string]string),
	}

	err = d.start(throttleInterval)
	if err != nil {
		_ = d.Close()
		return nil, err
	}

	d.Config = d.newClientConfig(base)

	if d.notifier != nil && recheck > 0 {
		go d.recheck(recheck)
	}

	return d, nil
}

// start starts watching the files and loads the first credentials. It holds
// refreshMu throughout, so a reload caused by an early file change waits for
// the first credentials and then reads the files again.
func (d *DynamicTLSConfig) start(throttleInterval time.Duration) error {
	d.refreshMu.Lock()
	defer d.refreshMu.Unlock()

	dirs := watchedDirs(d.mtls)
	if len(dirs) > 0 {
		nt, err := notifier.Create(
			notifier.OnPaths(dirs...),
			notifier.WithEventHandler(d.eventHandler),
			notifier.WithThrottleInterval(throttleInterval),
			notifier.WithBurstNumber(0),
		)
		if err != nil {
			return err
		}

		err = nt.Start()
		if err != nil {
			return err
		}

		d.notifier = nt
	}

	creds := &credentials{}

	id, err := loadIdentity(d.mtls)
	if err != nil {
		return err
	}

	creds.identity = id

	if d.reloadAuthorities {
		a, err := loadAuthorities(caRefs(d.mtls))
		if err != nil {
			return err
		}

		creds.authorities = a
	}

	d.active.Store(creds)
	d.reportActive(creds, d.now())

	return nil
}

// Close stops watching the credential files and the periodic check, and waits
// for a reload in progress to finish: once Close returns, the credentials no
// longer change. The embedded configuration, and clones of it, keep working
// with the credentials loaded last. Calling Close more than once is safe.
func (d *DynamicTLSConfig) Close() error {
	d.closeOnce.Do(func() {
		d.closed.Store(true)
		close(d.stop)
	})

	var err error
	if d.notifier != nil {
		err = d.notifier.Close()
	}

	// A reload that started before Close holds refreshMu; later ones see
	// closed and do nothing.
	d.refreshMu.Lock()
	defer d.refreshMu.Unlock()

	return err
}

// newClientConfig derives the embedded configuration from the configuration
// commoncfg built for the initial credentials.
func (d *DynamicTLSConfig) newClientConfig(loaded *tls.Config) *tls.Config {
	cfg := loaded.Clone()

	// Read the client certificate from d at handshake time.
	cfg.Certificates = nil
	cfg.GetClientCertificate = d.getClientCertificate

	if d.reloadAuthorities {
		cfg.RootCAs = nil
		cfg.InsecureSkipVerify = true // the server is verified by VerifyConnection
		cfg.VerifyConnection = d.verifyConnection
	}

	return cfg
}

// refresh reads the credentials again and activates each part that changed
// and passes accept. See DynamicTLSConfig for the rules.
func (d *DynamicTLSConfig) refresh() {
	d.refreshMu.Lock()
	defer d.refreshMu.Unlock()

	if d.closed.Load() {
		return
	}

	active := d.active.Load()
	next := *active
	now := d.now()

	identityFiles := filePaths(d.mtls.Cert, d.mtls.CertKey)

	id, err := loadIdentity(d.mtls)
	next.identity = reload(d, componentIdentity, identityFiles, active.identity, id, err, now, func(id *identity) {
		leaf := id.leaf()

		slog.Info("activated reloaded TLS credentials", "part", componentIdentity, "files", identityFiles,
			"subject", leaf.Subject.String(), "serial", leaf.SerialNumber.String(), "not-after", leaf.NotAfter,
			"chain-length", len(id.chain), "problems", id.problems(now))
	})

	if d.reloadAuthorities {
		authorityFiles := filePaths(caRefs(d.mtls)...)

		a, err := loadAuthorities(caRefs(d.mtls))
		next.authorities = reload(d, componentAuthorities, authorityFiles, active.authorities, a, err, now, func(a *authorities) {
			slog.Info("activated reloaded TLS credentials", "part", componentAuthorities, "files", authorityFiles,
				"count", len(a.certs), "problems", a.problems(now))
		})
	}

	if next != *active {
		d.active.Store(&next)
	}

	d.reportActive(&next, now)
}

// reloadable is a part of the credentials: the identity or the authorities.
type reloadable interface {
	problems(now time.Time) []problem
	usable(now time.Time) bool
	sum() [sha256.Size]byte
}

// reload decides whether loaded, the part c read again from files (or err if
// it could not be), replaces active, and logs the decision. It returns the
// part to keep active, calling activated if that is loaded. The caller must
// hold refreshMu.
func reload[T reloadable](d *DynamicTLSConfig, c component, files []string, active, loaded T, err error, now time.Time, activated func(T)) T {
	subject := string(c)

	switch {
	case err != nil:
		d.logOnce(subject, "error:"+err.Error(), func() {
			slog.Error("reloading TLS credentials failed, keeping the active ones", "part", c, "files", files, "error", err)
		})

		return active

	case loaded.sum() == active.sum():
		d.logState[subject] = ""
		return active

	case accept(loaded.problems(now), active.problems(now), loaded.usable(now), active.usable(now)):
		d.logState[subject] = ""

		activated(loaded)

		return loaded

	default:
		sum := loaded.sum()

		d.logOnce(subject, "rejected:"+hex.EncodeToString(sum[:]), func() {
			slog.Error("rejected reloaded TLS credentials, keeping the active ones", "part", c, "files", files,
				"problems", loaded.problems(now), "active-problems", active.problems(now))
		})

		return active
	}
}

// logOnce runs log unless the last message logged for subject had the same
// key, so that a reload failing the same way every minute is logged once.
// The caller must hold refreshMu.
func (d *DynamicTLSConfig) logOnce(subject, key string, log func()) {
	if d.logState[subject] == key {
		return
	}

	d.logState[subject] = key

	log()
}

// reportActive logs the defects of the active credentials when they change,
// for example when the client certificate expires. The caller must hold
// refreshMu.
func (d *DynamicTLSConfig) reportActive(creds *credentials, now time.Time) {
	report := func(c component, problems []problem, usable bool) {
		d.logOnce("active "+string(c), fmt.Sprint(problems), func() {
			switch {
			case len(problems) == 0:
			case usable:
				slog.Warn("the active TLS credentials have defects", "part", c, "problems", problems)
			default:
				slog.Error("the active TLS credentials no longer work", "part", c, "problems", problems)
			}
		})
	}

	report(componentIdentity, creds.identity.problems(now), creds.identity.usable(now))

	if creds.authorities != nil {
		report(componentAuthorities, creds.authorities.problems(now), creds.authorities.usable(now))
	}
}

// eventHandler is called by the notifier after changes in a watched
// directory.
func (d *DynamicTLSConfig) eventHandler(string, []fsnotify.Event) {
	d.refresh()
}

// recheck reloads the credentials every interval until Close.
func (d *DynamicTLSConfig) recheck(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-d.stop:
			return
		case <-ticker.C:
			d.refresh()
		}
	}
}

// getClientCertificate implements tls.Config.GetClientCertificate. Like
// crypto/tls does for tls.Config.Certificates, it sends the certificate only
// if the server can accept it, and otherwise sends none.
func (d *DynamicTLSConfig) getClientCertificate(cri *tls.CertificateRequestInfo) (*tls.Certificate, error) {
	cert := d.active.Load().identity.certificate

	err := cri.SupportsCertificate(cert)
	if err != nil {
		return &tls.Certificate{}, nil //nolint:nilerr // no acceptable certificate: send none, as crypto/tls does
	}

	return cert, nil
}

// verifyConnection implements tls.Config.VerifyConnection when the
// certificate authorities come from files. It verifies the server certificate
// chain and host name against the most recently loaded authorities.
func (d *DynamicTLSConfig) verifyConnection(cs tls.ConnectionState) error {
	if len(cs.PeerCertificates) == 0 {
		return errNoPeerCertificate
	}

	// cs.ServerName is the SNI value, which omits IP addresses. Fall back to
	// the configured ServerName rather than skipping the host name check.
	serverName := cs.ServerName
	if serverName == "" {
		serverName = d.ServerName
	}

	if serverName == "" {
		return errNoServerName
	}

	opts := x509.VerifyOptions{
		Roots:         d.active.Load().authorities.pool,
		DNSName:       serverName,
		Intermediates: x509.NewCertPool(),
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	for _, cert := range cs.PeerCertificates[1:] {
		opts.Intermediates.AddCert(cert)
	}

	chains, err := cs.PeerCertificates[0].Verify(opts)
	if err != nil {
		return &tls.CertificateVerificationError{UnverifiedCertificates: cs.PeerCertificates, Err: err}
	}

	if fips140.Enabled() && !slices.ContainsFunc(chains, isFIPSAllowedChain) {
		slog.Warn("server certificate chain has keys that are not FIPS-approved, accepting it anyway",
			"server-name", serverName)
	}

	return nil
}

// isFIPSAllowedChain reports whether every certificate in chain has a key
// that crypto/tls accepts in FIPS 140-3 mode (see isWeakKey). It mirrors the
// unexported check of crypto/tls (Go 1.27, native FIPS module) and is only
// used to log a warning.
func isFIPSAllowedChain(chain []*x509.Certificate) bool {
	return len(chain) > 0 && !slices.ContainsFunc(chain, func(c *x509.Certificate) bool { return isWeakKey(c.PublicKey) })
}

// watchedDirs returns the directories that hold the file sources of mtls,
// without duplicates.
func watchedDirs(mtls *commoncfg.MTLS) []string {
	refs := append([]commoncfg.SourceRef{mtls.Cert, mtls.CertKey}, caRefs(mtls)...)

	dirs := make([]string, 0, len(refs))
	for _, ref := range refs {
		if !isFileSource(ref) {
			continue
		}

		dir := filepath.Dir(strings.TrimSpace(ref.File.Path))
		if !slices.Contains(dirs, dir) {
			dirs = append(dirs, dir)
		}
	}

	return dirs
}

// caRefs returns the certificate authority sources of mtls.
func caRefs(mtls *commoncfg.MTLS) []commoncfg.SourceRef {
	refs := make([]commoncfg.SourceRef, 0, len(mtls.RootCAs)+1)
	if mtls.ServerCA != nil {
		refs = append(refs, *mtls.ServerCA)
	}

	return append(refs, mtls.RootCAs...)
}

// filePaths returns the paths of the file sources among refs.
func filePaths(refs ...commoncfg.SourceRef) []string {
	paths := make([]string, 0, len(refs))

	for _, ref := range refs {
		if isFileSource(ref) {
			paths = append(paths, strings.TrimSpace(ref.File.Path))
		}
	}

	return paths
}

func isFileSource(ref commoncfg.SourceRef) bool {
	return ref.Source == commoncfg.FileSourceValue && strings.TrimSpace(ref.File.Path) != ""
}
