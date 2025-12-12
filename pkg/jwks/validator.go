package jwks

import (
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
)

type X5cValidator struct {
	CACertPool *x509.CertPool
	Subject    *Subject
}

type Subject struct {
	CN  string
	Org string
}

var (
	ErrCACertNotLoaded     = errors.New("ca cert is not loaded")
	ErrX5cEmpty            = errors.New("x5c is empty")
	ErrInvalidCertEncoding = errors.New("invalid cert encoding")
	ErrParseCertificate    = errors.New("parse certificate")
	ErrUnknownSubj         = errors.New("unknown subject")
	ErrUnknownSubjCN       = errors.New("subject cn unknown")
	ErrUnknownSubjOrg      = errors.New("subject org unknown")
)

// NewX5cValidator creates a new X5cValidator instance using the provided CA certificate and subject.
// It returns an error if the CA certificate or subject is nil.
// The returned X5cValidator is initialized with a certificate pool containing the CA certificate
// and the specified subject.
//
// Parameters:
//   - ca:   The CA certificate to trust for validation.
//   - subj: The subject information to be used for validation.
//
// Returns:
//   - *X5cValidator: The initialized validator.
//   - error:         An error if initialization fails.
func NewX5cValidator(ca *x509.Certificate, subj *Subject) (*X5cValidator, error) {
	if ca == nil {
		return nil, ErrCACertNotLoaded
	}

	if subj == nil {
		return nil, fmt.Errorf("%w: subj is nil", ErrUnknownSubj)
	}

	result := &X5cValidator{}

	pool := x509.NewCertPool()
	pool.AddCert(ca)
	result.CACertPool = pool

	result.Subject = subj

	return result, nil
}

// Validate checks the provided x5c certificate chain for validity.
//
// The first certificate in the x5c slice is treated as the leaf certificate, and any subsequent
// certificates are treated as intermediates. The function decodes each certificate from base64,
// parses it, and adds intermediates to a certificate pool. The leaf certificate is then verified
// against the CA certificate pool and intermediates. After successful verification, the function
// checks the Common Name (CN) and Organization (Org) fields of the leaf certificate.
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

	err = v.checkCN(leaf)
	if err != nil {
		return err
	}

	return v.checkOrg(leaf)
}

func (v *X5cValidator) checkOrg(leaf *x509.Certificate) error {
	orgs := leaf.Subject.Organization
	if len(orgs) == 0 {
		return ErrUnknownSubjOrg
	}

	if orgs[0] != v.Subject.Org {
		return ErrUnknownSubjOrg
	}

	return nil
}

func (v *X5cValidator) checkCN(cert *x509.Certificate) error {
	dnsNames := cert.DNSNames
	if len(dnsNames) > 0 {
		if slices.Contains(dnsNames, v.Subject.CN) {
			return nil
		}

		return fmt.Errorf("%w: %v", ErrUnknownSubjCN, dnsNames)
	}

	if cert.Subject.CommonName == v.Subject.CN {
		return nil
	}

	return fmt.Errorf("%w: expected CN %s, got %s", ErrUnknownSubjCN, v.Subject.CN, cert.Subject.CommonName)
}
