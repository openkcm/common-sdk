package commontls

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/mldsa"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"slices"
	"time"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
)

// clockSkew is how far in the future a NotBefore may lie for a certificate to
// still count as valid, to tolerate clocks that are slightly behind the one
// of the issuer.
const clockSkew = 5 * time.Minute

// problem is a defect of loaded credentials. Credentials with a problem may
// still work, for example a chain with an extra certificate, so problems do
// not prevent a DynamicTLSConfig from being created; they decide whether a
// reload may replace the active credentials (see accept).
type problem string

// Problems of a client identity.
const (
	problemLeafExpired      problem = "the client certificate has expired"
	problemLeafNotYetValid  problem = "the client certificate is not valid yet"
	problemNotForClientAuth problem = "the client certificate is not valid for client authentication"
	problemIntermediateTime problem = "an intermediate certificate has expired or is not valid yet"
	problemMalformedChain   problem = "the certificate file holds a certificate that cannot be parsed"
	problemUnrelatedCert    problem = "the certificate file holds certificates outside the chain of the client certificate"
)

// Problems of certificate authorities.
const (
	problemNoAuthority      problem = "no certificate authority could be read"
	problemNoValidAuthority problem = "no certificate authority is currently valid"
	problemMalformedPEM     problem = "the certificate authorities hold a malformed or incomplete PEM block, or one that is not a certificate"
	problemTextOutsidePEM   problem = "the certificate authorities hold text outside PEM blocks"
)

// problemWeakKey is a problem of both.
const problemWeakKey problem = "a key is weaker than RSA 2048 bits or ECDSA P-256"

const (
	pemBeginMarker     = "-----BEGIN "
	pemEndMarker       = "-----END "
	pemCertificateType = "CERTIFICATE"
)

// identity is a client certificate chain with its private key.
type identity struct {
	certificate *tls.Certificate
	// chain holds the parsed certificates of the chain, the client
	// certificate first; certificates that cannot be parsed are left out.
	chain     []*x509.Certificate
	malformed bool
	digest    [sha256.Size]byte
}

// loadIdentity reads and parses the client certificate and its key. It
// returns an error, like commoncfg does, if they cannot be read or parsed or
// do not match; other defects are reported by problems.
func loadIdentity(mtls *commoncfg.MTLS) (*identity, error) {
	certPEM, err := commoncfg.ExtractValueFromSourceRef(&mtls.Cert)
	if err != nil {
		return nil, err
	}

	keyPEM, err := commoncfg.ExtractValueFromSourceRef(&mtls.CertKey)
	if err != nil {
		return nil, err
	}

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	id := &identity{certificate: &cert, digest: digest(certPEM, keyPEM)}

	for i, der := range cert.Certificate {
		if i == 0 && cert.Leaf != nil {
			id.chain = append(id.chain, cert.Leaf)
			continue
		}

		parsed, err := x509.ParseCertificate(der)
		if err != nil {
			if i == 0 {
				return nil, err
			}

			id.malformed = true

			continue
		}

		id.chain = append(id.chain, parsed)
	}

	return id, nil
}

// sum returns the hash of the files the identity was read from.
func (id *identity) sum() [sha256.Size]byte {
	return id.digest
}

// leaf returns the client certificate.
func (id *identity) leaf() *x509.Certificate {
	return id.chain[0]
}

// problems returns the defects of the identity at time now.
func (id *identity) problems(now time.Time) []problem {
	var problems []problem

	leaf := id.leaf()

	switch {
	case now.After(leaf.NotAfter):
		problems = append(problems, problemLeafExpired)
	case now.Add(clockSkew).Before(leaf.NotBefore):
		problems = append(problems, problemLeafNotYetValid)
	}

	if len(leaf.ExtKeyUsage) > 0 &&
		!slices.Contains(leaf.ExtKeyUsage, x509.ExtKeyUsageClientAuth) &&
		!slices.Contains(leaf.ExtKeyUsage, x509.ExtKeyUsageAny) {
		problems = append(problems, problemNotForClientAuth)
	}

	if id.malformed {
		problems = append(problems, problemMalformedChain)
	}

	path, unrelated := chainPath(id.chain)
	if unrelated {
		problems = append(problems, problemUnrelatedCert)
	}

	for _, cert := range path {
		if isWeakKey(cert.PublicKey) {
			problems = append(problems, problemWeakKey)
		}

		if cert != leaf && !isValidAt(cert, now) {
			problems = append(problems, problemIntermediateTime)
		}
	}

	return normalize(problems)
}

// usable reports whether servers can accept the identity at all.
func (id *identity) usable(now time.Time) bool {
	problems := id.problems(now)

	return !slices.Contains(problems, problemLeafExpired) && !slices.Contains(problems, problemLeafNotYetValid)
}

// chainPath follows the chain from the client certificate through the other
// certificates of chain, in whatever order they are, each one signed by the
// next. It returns that path and whether some certificates are not on it.
func chainPath(chain []*x509.Certificate) ([]*x509.Certificate, bool) {
	path := []*x509.Certificate{chain[0]}
	used := make([]bool, len(chain))
	used[0] = true

	for current := chain[0]; ; {
		next := -1

		for i, candidate := range chain {
			if !used[i] && current.CheckSignatureFrom(candidate) == nil {
				next = i
				break
			}
		}

		if next < 0 {
			break
		}

		used[next] = true
		current = chain[next]
		path = append(path, current)
	}

	return path, slices.Contains(used, false)
}

// authorities is a set of certificate authorities that verify the server.
type authorities struct {
	// pool is never nil: an empty pool trusts no server, while a nil pool
	// would make crypto/x509 fall back to the system authorities.
	pool          *x509.CertPool
	certs         []*x509.Certificate
	parseProblems []problem
	digest        [sha256.Size]byte
}

// loadAuthorities reads and parses the certificate authorities of refs. It
// returns an error, like commoncfg does, if a source cannot be read; parsing
// defects are reported by problems.
func loadAuthorities(refs []commoncfg.SourceRef) (*authorities, error) {
	a := &authorities{pool: x509.NewCertPool()}
	sources := make([][]byte, 0, len(refs))

	for _, ref := range refs {
		data, err := commoncfg.ExtractValueFromSourceRef(&ref)
		if err != nil {
			return nil, err
		}

		sources = append(sources, data)

		certs, problems := parsePEMCertificates(data)
		a.parseProblems = append(a.parseProblems, problems...)

		for _, cert := range certs {
			a.pool.AddCert(cert)
			a.certs = append(a.certs, cert)
		}
	}

	a.parseProblems = normalize(a.parseProblems)
	a.digest = digest(sources...)

	return a, nil
}

// sum returns the hash of the sources the authorities were read from.
func (a *authorities) sum() [sha256.Size]byte {
	return a.digest
}

// problems returns the defects of the authorities at time now.
func (a *authorities) problems(now time.Time) []problem {
	problems := slices.Clone(a.parseProblems)

	switch {
	case len(a.certs) == 0:
		problems = append(problems, problemNoAuthority)
	case !slices.ContainsFunc(a.certs, func(c *x509.Certificate) bool { return isValidAt(c, now) }):
		problems = append(problems, problemNoValidAuthority)
	}

	if slices.ContainsFunc(a.certs, func(c *x509.Certificate) bool { return isWeakKey(c.PublicKey) }) {
		problems = append(problems, problemWeakKey)
	}

	return normalize(problems)
}

// usable reports whether the authorities can verify any server at all.
func (a *authorities) usable(now time.Time) bool {
	problems := a.problems(now)

	return !slices.Contains(problems, problemNoAuthority) && !slices.Contains(problems, problemNoValidAuthority)
}

// parsePEMCertificates parses the certificates of a PEM bundle strictly. It
// returns the certificates it could parse, and as problems a block that is
// incomplete, malformed, has headers or is not a certificate (the same blocks
// x509.CertPool.AppendCertsFromPEM skips), and text outside blocks other than
// "#" comment lines. A file cut short while it is written ends in an
// incomplete block.
func parsePEMCertificates(data []byte) ([]*x509.Certificate, []problem) {
	var (
		certs    []*x509.Certificate
		problems []problem
	)

	for rest := data; ; {
		begin := bytes.Index(rest, []byte(pemBeginMarker))
		if begin < 0 {
			if hasText(rest) {
				problems = append(problems, problemTextOutsidePEM)
			}

			return certs, problems
		}

		if hasText(rest[:begin]) {
			problems = append(problems, problemTextOutsidePEM)
		}

		rest = rest[begin:]

		// Decode exactly this block: pem.Decode would otherwise skip a
		// malformed block and decode the next one.
		end := bytes.Index(rest, []byte(pemEndMarker))
		if end < 0 {
			return certs, append(problems, problemMalformedPEM)
		}

		blockEnd := len(rest)
		if newline := bytes.IndexByte(rest[end:], '\n'); newline >= 0 {
			blockEnd = end + newline + 1
		}

		segment := rest[:blockEnd]
		rest = rest[blockEnd:]

		block, remainder := pem.Decode(segment)
		if block == nil || len(bytes.TrimSpace(remainder)) > 0 || bytes.Count(segment, []byte(pemBeginMarker)) != 1 ||
			block.Type != pemCertificateType || len(block.Headers) > 0 {
			problems = append(problems, problemMalformedPEM)
			continue
		}

		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			problems = append(problems, problemMalformedPEM)
			continue
		}

		certs = append(certs, cert)
	}
}

// hasText reports whether data holds anything but blank lines and "#"
// comment lines.
func hasText(data []byte) bool {
	for line := range bytes.Lines(data) {
		line = bytes.TrimSpace(line)
		if len(line) > 0 && line[0] != '#' {
			return true
		}
	}

	return false
}

// isValidAt reports whether cert is within its validity period at time now,
// allowing for clockSkew on its start.
func isValidAt(cert *x509.Certificate, now time.Time) bool {
	return !now.After(cert.NotAfter) && !now.Add(clockSkew).Before(cert.NotBefore)
}

// isWeakKey reports whether pub is weaker than the keys crypto/tls accepts in
// FIPS 140-3 mode: RSA of at least 2048 bits, ECDSA on P-256, P-384 or P-521,
// Ed25519 and ML-DSA. Other key types count as weak.
func isWeakKey(pub any) bool {
	switch key := pub.(type) {
	case *rsa.PublicKey:
		return key.N.BitLen() < 2048
	case *ecdsa.PublicKey:
		return key.Curve != elliptic.P256() && key.Curve != elliptic.P384() && key.Curve != elliptic.P521()
	case ed25519.PublicKey, *mldsa.PublicKey:
		return false
	default:
		return true
	}
}

// accept reports whether reloaded credentials with the problems newProblems
// may replace the active ones, which have activeProblems: when they bring no
// problem the active ones do not already have, so that a reload can never
// make things worse, or when they are usable while the active ones are not,
// for example because the active certificate has expired.
func accept(newProblems, activeProblems []problem, newUsable, activeUsable bool) bool {
	for _, p := range newProblems {
		if !slices.Contains(activeProblems, p) {
			return newUsable && !activeUsable
		}
	}

	return true
}

// normalize sorts problems and removes duplicates.
func normalize(problems []problem) []problem {
	slices.Sort(problems)
	return slices.Compact(problems)
}

// digest hashes parts, each prefixed with its length so that different
// splits of the same bytes do not collide.
func digest(parts ...[]byte) [sha256.Size]byte {
	h := sha256.New()

	for _, part := range parts {
		_ = binary.Write(h, binary.BigEndian, uint64(len(part)))
		_, _ = h.Write(part)
	}

	var sum [sha256.Size]byte

	copy(sum[:], h.Sum(nil))

	return sum
}
