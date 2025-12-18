package jwks

import (
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
)

type X5cValidator struct {
	CACertPool *x509.CertPool
	Subject    string
}

var (
	ErrCACertNotLoaded     = errors.New("ca cert is not loaded")
	ErrX5cEmpty            = errors.New("x5c is empty")
	ErrInvalidCertEncoding = errors.New("invalid cert encoding")
	ErrParseCertificate    = errors.New("parse certificate")
	ErrUnknownSubj         = errors.New("unknown subject")
)

// NewX5cValidator creates a new X5cValidator instance using the provided CA certificate and subject.
// It returns an error if the CA certificate or subject is nil.
// The returned X5cValidator is initialized with a certificate pool containing the CA certificate
// and the specified subject.
//
// Parameters:
//   - ca:   The CA certificate to trust for validation.
//   - subject: The subject information to be used for validation.
//
// Returns:
//   - *X5cValidator: The initialized validator.
//   - error:         An error if initialization fails.
func NewX5cValidator(ca *x509.Certificate, subject string) (*X5cValidator, error) {
	if ca == nil {
		return nil, ErrCACertNotLoaded
	}

	if subject == "" {
		return nil, fmt.Errorf("%w: subj is empty", ErrUnknownSubj)
	}

	result := &X5cValidator{}

	pool := x509.NewCertPool()
	pool.AddCert(ca)
	result.CACertPool = pool

	result.Subject = subject

	return result, nil
}

// Validate checks the provided x5c certificate chain for validity.
//
// Validate checks the provided x5c certificate chain for validity.
// It expects x5c to be a slice of base64-encoded DER certificates, with the leaf certificate first.
// The function decodes and parses each certificate, adds intermediates to a pool,
// and verifies the chain against the CA pool configured in the X5cValidator.
// It returns an error if the chain is invalid, any certificate cannot be decoded or parsed,
// or if the leaf certificate's subject does not match the expected subject.
//
// Parameters:
//   - x5c: A slice of base64-encoded DER certificates, where the first is the leaf.
//
// Returns:
//   - error: An error if validation fails, or nil if the certificate chain is valid.
func (v *X5cValidator) Validate(x5c []string) error {
	if len(x5c) == 0 {
		return ErrX5cEmpty
	}

	var leaf *x509.Certificate

	intermediates := x509.NewCertPool()

	for index, b64 := range x5c {
		der, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrInvalidCertEncoding, err.Error())
		}

		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return fmt.Errorf("%w: %s", ErrParseCertificate, err.Error())
		}

		if index == 0 {
			leaf = cert
			continue
		}

		intermediates.AddCert(cert)
	}

	opts := x509.VerifyOptions{
		Roots:         v.CACertPool,
		Intermediates: intermediates,
	}

	_, err := leaf.Verify(opts)
	if err != nil {
		return err
	}

	return v.checkFullSubject(leaf)
}

// checkFullSubject verifies that the subject of the provided leaf certificate
// matches the expected subject stored in the X5cValidator.
// Returns an error if the subjects do not match.
func (v *X5cValidator) checkFullSubject(leaf *x509.Certificate) error {
	if leaf.Subject.ToRDNSequence().String() != v.Subject {
		return fmt.Errorf("%w %s", ErrUnknownSubj, "leaf subject dont match")
	}

	return nil
}
