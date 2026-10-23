// Package tls provides [DynamicTLSConfig], a *tls.Config for mutual TLS
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
//	dyn, err := tls.NewDynamicTLSConfig(2*time.Second, mtls)
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
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/fips140"
	"crypto/mldsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"errors"
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
// A reload loads the certificate, the key and the certificate authorities
// together. If any of them cannot be read or parsed, for example because the
// certificate has been replaced but its key not yet, the previous credentials
// stay in use and the error is logged; the next change triggers another
// attempt. A certificate authority file without any valid PEM certificate is
// not treated as an error by commoncfg: it yields no authority, and servers
// are then rejected until the file is fixed.
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
	closed   atomic.Bool

	refreshMu sync.Mutex // serializes refreshes, so the last one to finish read the newest files

	// loaded is the configuration commoncfg built from the most recently
	// loaded credentials. It is never handed out or modified; the callbacks
	// of the embedded configuration read the client certificate and the
	// certificate authorities from it. The embedded configuration itself
	// cannot hold them: crypto/tls reads it without locking during
	// handshakes, and clones copy its fields but share its callbacks.
	loaded atomic.Pointer[tls.Config]
}

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
	if mtls == nil {
		return nil, commoncfg.ErrMTLSIsNil
	}

	d := &DynamicTLSConfig{mtls: mtls}

	// Start watching before the first load, so that a change made while
	// loading triggers another reload instead of being missed.
	dirs := watchedDirs(mtls)
	if len(dirs) > 0 {
		nt, err := notifier.Create(
			notifier.OnPaths(dirs...),
			notifier.WithEventHandler(d.eventHandler),
			notifier.WithThrottleInterval(throttleInterval),
			notifier.WithBurstNumber(0),
		)
		if err != nil {
			return nil, err
		}

		err = nt.Start()
		if err != nil {
			return nil, err
		}

		d.notifier = nt
	}

	loaded, err := d.refresh()
	if err != nil {
		_ = d.Close()
		return nil, err
	}

	d.Config = d.newClientConfig(loaded)

	return d, nil
}

// Close stops watching the credential files. The embedded configuration, and
// clones of it, keep working with the credentials loaded last. Calling Close
// more than once is safe.
func (d *DynamicTLSConfig) Close() error {
	d.closed.Store(true)

	if d.notifier == nil {
		return nil
	}

	return d.notifier.Close()
}

// newClientConfig derives the embedded configuration from the configuration
// commoncfg built for the initial credentials.
func (d *DynamicTLSConfig) newClientConfig(loaded *tls.Config) *tls.Config {
	cfg := loaded.Clone()

	// Read the client certificate from d at handshake time.
	cfg.Certificates = nil
	cfg.GetClientCertificate = d.getClientCertificate

	if !cfg.InsecureSkipVerify && slices.ContainsFunc(caRefs(d.mtls), isFileSource) {
		cfg.RootCAs = nil
		cfg.InsecureSkipVerify = true // the server is verified by VerifyConnection
		cfg.VerifyConnection = d.verifyConnection
	}

	return cfg
}

// refresh loads the credentials and, if that succeeds, makes them the current
// ones. It returns the configuration commoncfg built from them.
func (d *DynamicTLSConfig) refresh() (*tls.Config, error) {
	d.refreshMu.Lock()
	defer d.refreshMu.Unlock()

	loaded, err := commoncfg.LoadMTLSConfig(d.mtls)
	if err != nil {
		return nil, err
	}

	d.loaded.Store(loaded)

	return loaded, nil
}

// eventHandler is called by the notifier after changes in a watched
// directory. It reloads the credentials, keeping the previous ones if that
// fails.
func (d *DynamicTLSConfig) eventHandler(path string, _ []fsnotify.Event) {
	if d.closed.Load() {
		return
	}

	_, err := d.refresh()
	if err != nil {
		slog.Error("reloading TLS credentials failed, keeping the previous ones", "watched-path", path, "error", err)
		return
	}

	slog.Info("reloaded TLS credentials", "watched-path", path)
}

// getClientCertificate implements tls.Config.GetClientCertificate. Like
// crypto/tls does for tls.Config.Certificates, it sends the certificate only
// if the server can accept it, and otherwise sends none.
func (d *DynamicTLSConfig) getClientCertificate(cri *tls.CertificateRequestInfo) (*tls.Certificate, error) {
	cert := &d.loaded.Load().Certificates[0]

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
		Roots:         d.loaded.Load().RootCAs,
		DNSName:       serverName,
		Intermediates: x509.NewCertPool(),
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
// that crypto/tls accepts in FIPS 140-3 mode: RSA of at least 2048 bits, ECDSA
// on P-256, P-384 or P-521, Ed25519 and ML-DSA. It mirrors the unexported
// check of crypto/tls (Go 1.27, native FIPS module) and is only used to log a
// warning.
func isFIPSAllowedChain(chain []*x509.Certificate) bool {
	if len(chain) == 0 {
		return false
	}

	for _, cert := range chain {
		switch key := cert.PublicKey.(type) {
		case *rsa.PublicKey:
			if key.N.BitLen() < 2048 {
				return false
			}
		case *ecdsa.PublicKey:
			if key.Curve != elliptic.P256() && key.Curve != elliptic.P384() && key.Curve != elliptic.P521() {
				return false
			}
		case ed25519.PublicKey, *mldsa.PublicKey:
		default:
			return false
		}
	}

	return true
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

func isFileSource(ref commoncfg.SourceRef) bool {
	return ref.Source == commoncfg.FileSourceValue && strings.TrimSpace(ref.File.Path) != ""
}
