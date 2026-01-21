package jwtsigning

import (
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
)

// Validator validates x5c certificate chains from JWKs against a configured CA pool
// and checks that the leaf certificate subject matches an expected subject string.
type Validator struct {
	// caCertPool is the set of trusted root certificates used to verify chains.
	caCertPool *x509.CertPool
	// subject is the exact subject string expected on the leaf certificate.
	subject string
}

var (
	// ErrCACertNotLoaded is returned when a Validator is constructed without a CA certificate.
	ErrCACertNotLoaded = errors.New("ca cert is not loaded")
	// ErrX5cEmpty is returned when a JWK has no x5c certificates.
	ErrX5cEmpty = errors.New("x5c is empty")
	// ErrInvalidCertEncoding is returned when an x5c entry is not valid base64.
	ErrInvalidCertEncoding = errors.New("invalid cert encoding")
	// ErrParseCertificate is returned when a decoded x5c entry cannot be parsed as a certificate.
	ErrParseCertificate = errors.New("parse certificate")
	// ErrUnknownSubj is returned when a certificate subject does not match the expected subject.
	ErrUnknownSubj = errors.New("unknown subject")
)

// NewValidator returns a Validator initialized with the given CA certificate and subject.
// The CA certificate is added to a new x509.CertPool used for validation.
// Returns an error if the CA certificate is nil or the subject is empty.
func NewValidator(ca *x509.Certificate, subject string) (*Validator, error) {
	if ca == nil {
		return nil, ErrCACertNotLoaded
	}

	if subject == "" {
		return nil, fmt.Errorf("%w: subj is empty", ErrUnknownSubj)
	}

	result := &Validator{}

	pool := x509.NewCertPool()
	pool.AddCert(ca)
	result.caCertPool = pool

	result.subject = subject

	return result, nil
}

// Validate checks the provided x5c certificate chain for validity.
// It expects the provided Key to include an x5c slice where the first element
// is the leaf certificate followed by any intermediates. Each entry must be a
// base64-encoded DER certificate. The chain is verified against the Validator's
// CA pool and the leaf subject must match the Validator's Subject.
// Returns an error if the chain cannot be decoded, parsed, verified or if the
// leaf subject does not match.
func (v *Validator) Validate(key Key) error {
	x5c := key.X5c
	if len(x5c) == 0 {
		return ErrX5cEmpty
	}

	var leaf *x509.Certificate

	intermediates := x509.NewCertPool()

	for index, b64 := range x5c {
		der, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return fmt.Errorf("%w: at certificate index %d: %s", ErrInvalidCertEncoding, index, err.Error())
		}

		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return fmt.Errorf("%w: at certificate index %d: %s", ErrParseCertificate, index, err.Error())
		}

		if index == 0 {
			leaf = cert
			continue
		}

		intermediates.AddCert(cert)
	}

	opts := x509.VerifyOptions{
		Roots:         v.caCertPool,
		Intermediates: intermediates,
	}

	_, err := leaf.Verify(opts)
	if err != nil {
		return err
	}

	return v.checkFullSubject(leaf)
}

// checkFullSubject verifies that the subject of the provided leaf certificate
// matches the expected subject stored in the Validator. It returns an error
// when the subjects do not match.
func (v *Validator) checkFullSubject(leaf *x509.Certificate) error {
	if leaf.Subject.ToRDNSequence().String() != v.subject {
		return fmt.Errorf("%w: leaf subject doesn't match", ErrUnknownSubj)
	}

	return nil
}
