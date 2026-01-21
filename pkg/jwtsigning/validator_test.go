package jwtsigning_test

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/common-sdk/pkg/jwtsigning"
)

type generateCertsFunc func() (rootCertDer []byte, intCertDer []byte, leafCertDer []byte)

func TestValidator(t *testing.T) {
	validSubjString := "CN=CA,OU=Clients,O=SE,L=Canary,C=US"
	validSubject := pkix.Name{
		CommonName: "CA", Organization: []string{"SE"}, Country: []string{"US"}, Locality: []string{"Canary"},
		OrganizationalUnit: []string{"Clients"},
	}

	validNotBefore := time.Now()
	invalidNotBefore := time.Now().Add(1 * time.Hour)
	validNotAfter := validNotBefore.Add(24 * 7 * time.Hour)
	invalidNotAfter := time.Now().Add(-1 * time.Hour)

	t.Run("init", func(t *testing.T) {
		// given
		tts := []struct {
			name   string
			ca     *x509.Certificate
			subj   string
			expErr error
		}{
			{
				name:   "should return error if ca is nil",
				ca:     nil,
				subj:   validSubjString,
				expErr: jwtsigning.ErrCACertNotLoaded,
			},
			{
				name:   "should return error if subj is empty",
				ca:     &x509.Certificate{},
				subj:   "",
				expErr: jwtsigning.ErrUnknownSubj,
			},
			{
				name:   "should not return error if subj and ca is not nil",
				ca:     &x509.Certificate{},
				subj:   validSubjString,
				expErr: nil,
			},
		}

		for _, tt := range tts {
			t.Run(tt.name, func(t *testing.T) {
				// when
				result, err := jwtsigning.NewValidator(tt.ca, tt.subj)

				// then
				assert.ErrorIs(t, err, tt.expErr)

				if tt.expErr != nil {
					assert.Nil(t, result)
					return
				}

				assert.NotNil(t, result)
			})
		}
	})

	t.Run("should not return error if", func(t *testing.T) {
		t.Run("if x5c is valid with intermediate certificate", func(t *testing.T) {
			// given
			rootKey, rootDer := rootCertificate(t, validNotBefore, validNotAfter)
			intKey, intDer := intermediateCertificate(t, rootDer, validNotBefore, validNotAfter, rootKey)
			leafDer := leafCertWithParams(t, intDer, validNotBefore, validNotAfter, intKey, validSubject)

			rootCa, actErr := x509.ParseCertificate(rootDer)
			assert.NoError(t, actErr)

			subj, err := jwtsigning.NewValidator(rootCa, validSubjString)
			assert.NoError(t, err)

			x5c := []string{
				base64.StdEncoding.EncodeToString(leafDer),
				base64.StdEncoding.EncodeToString(intDer),
			}

			// when
			actErr = subj.Validate(jwtsigning.Key{X5c: x5c})

			// then
			assert.NoError(t, actErr)
		})

		t.Run("x5c contains only leaf which is signed by root", func(t *testing.T) {
			// given
			rootKey, rootDer := rootCertificate(t, validNotBefore, validNotAfter)
			leafDer := leafCertWithParams(t, rootDer, validNotBefore, validNotAfter, rootKey, validSubject)

			rootCa, actErr := x509.ParseCertificate(rootDer)
			assert.NoError(t, actErr)

			subj, err := jwtsigning.NewValidator(rootCa, validSubjString)
			assert.NoError(t, err)

			x5c := []string{
				base64.StdEncoding.EncodeToString(leafDer),
			}

			// when
			actErr = subj.Validate(jwtsigning.Key{X5c: x5c})

			// then
			assert.NoError(t, actErr)
		})
	})

	t.Run("should return error if", func(t *testing.T) {
		t.Run("x5c", func(t *testing.T) {
			tts := []struct {
				name   string
				x5c    []string
				expErr error
			}{
				{
					name:   "is nil",
					x5c:    nil,
					expErr: jwtsigning.ErrX5cEmpty,
				},
				{
					name:   "is empty",
					x5c:    []string{},
					expErr: jwtsigning.ErrX5cEmpty,
				},
				{
					name:   "has a non base64 character",
					x5c:    []string{":"},
					expErr: jwtsigning.ErrInvalidCertEncoding,
				},
				{
					name:   "has a invalid certificate",
					x5c:    []string{base64.StdEncoding.EncodeToString([]byte("invalid certificate"))},
					expErr: jwtsigning.ErrParseCertificate,
				},
			}

			// given
			subj, err := jwtsigning.NewValidator(&x509.Certificate{}, validSubjString)
			assert.NoError(t, err)

			for _, tt := range tts {
				t.Run(tt.name, func(t *testing.T) {
					// when
					err = subj.Validate(jwtsigning.Key{X5c: tt.x5c})

					// then
					assert.Error(t, err)
					assert.ErrorIs(t, err, tt.expErr)
				})
			}
		})
	})

	t.Run("certificate validation should return error if", func(t *testing.T) {
		tts := []struct {
			name          string
			generateCerts generateCertsFunc
			expErrMsg     string
			expErr        error
		}{
			{
				name: "root Certificate has future notBefore",
				generateCerts: func() ([]byte, []byte, []byte) {
					rootKey, rootDer := rootCertificate(t, invalidNotBefore, validNotAfter)
					intKey, intDer := intermediateCertificate(t, rootDer, validNotBefore, validNotAfter, rootKey)
					leafDer := leafCertWithParams(t, intDer, validNotBefore, validNotAfter, intKey, validSubject)

					return rootDer, intDer, leafDer
				},
				expErrMsg: x509.CertificateInvalidError{Reason: x509.Expired}.Error(),
			},
			{
				name: "root Certificate has expired notAfter",
				generateCerts: func() ([]byte, []byte, []byte) {
					rootKey, rootDer := rootCertificate(t, validNotBefore, invalidNotAfter)
					intKey, intDer := intermediateCertificate(t, rootDer, validNotBefore, validNotAfter, rootKey)
					leafDer := leafCertWithParams(t, intDer, validNotBefore, validNotAfter, intKey, validSubject)

					return rootDer, intDer, leafDer
				},
				expErrMsg: x509.CertificateInvalidError{Reason: x509.Expired}.Error(),
			},
			{
				name: "intermediate Certificate has future notBefore",
				generateCerts: func() ([]byte, []byte, []byte) {
					rootKey, rootDer := rootCertificate(t, validNotBefore, validNotAfter)
					intKey, intDer := intermediateCertificate(t, rootDer, invalidNotBefore, validNotAfter, rootKey)
					leafDer := leafCertWithParams(t, intDer, validNotBefore, validNotAfter, intKey, validSubject)

					return rootDer, intDer, leafDer
				},
				expErrMsg: x509.CertificateInvalidError{Reason: x509.Expired}.Error(),
			},
			{
				name: "intermediate Certificate has expired notAfter",
				generateCerts: func() ([]byte, []byte, []byte) {
					rootKey, rootDer := rootCertificate(t, validNotBefore, validNotAfter)
					intKey, intDer := intermediateCertificate(t, rootDer, validNotBefore, invalidNotAfter, rootKey)
					leafDer := leafCertWithParams(t, intDer, validNotBefore, validNotAfter, intKey, validSubject)

					return rootDer, intDer, leafDer
				},
				expErrMsg: x509.CertificateInvalidError{Reason: x509.Expired}.Error(),
			},
			{
				name: "leaf Certificate has future notBefore",
				generateCerts: func() ([]byte, []byte, []byte) {
					rootKey, rootDer := rootCertificate(t, validNotBefore, validNotAfter)
					intKey, intDer := intermediateCertificate(t, rootDer, validNotBefore, validNotAfter, rootKey)
					leafDer := leafCertWithParams(t, intDer, invalidNotBefore, validNotAfter, intKey, validSubject)

					return rootDer, intDer, leafDer
				},
				expErrMsg: x509.CertificateInvalidError{Reason: x509.Expired}.Error(),
			},
			{
				name: "leaf Certificate has expired notAfter",
				generateCerts: func() ([]byte, []byte, []byte) {
					rootKey, rootDer := rootCertificate(t, validNotBefore, validNotAfter)
					intKey, intDer := intermediateCertificate(t, rootDer, validNotBefore, validNotAfter, rootKey)
					leafDer := leafCertWithParams(t, intDer, validNotBefore, invalidNotAfter, intKey, validSubject)

					return rootDer, intDer, leafDer
				},
				expErrMsg: x509.CertificateInvalidError{Reason: x509.Expired}.Error(),
			},
			{
				name: "root CA is different",
				generateCerts: func() ([]byte, []byte, []byte) {
					_, invalidRootDer := rootCertificate(t, validNotBefore, validNotAfter)
					rootKey, rootDer := rootCertificate(t, validNotBefore, validNotAfter)

					intKey, intDer := intermediateCertificate(t, rootDer, validNotBefore, validNotAfter, rootKey)
					leafDer := leafCertWithParams(t, intDer, validNotBefore, validNotAfter, intKey, validSubject)

					return invalidRootDer, intDer, leafDer
				},
				expErrMsg: x509.UnknownAuthorityError{}.Error(),
			},
			{
				name: "intermediate certificate is different",
				generateCerts: func() ([]byte, []byte, []byte) {
					rootKey, rootDer := rootCertificate(t, validNotBefore, validNotAfter)

					_, invalidIntDer := intermediateCertificate(t, rootDer, validNotBefore, validNotAfter, rootKey)
					intKey, intDer := intermediateCertificate(t, rootDer, validNotBefore, validNotAfter, rootKey)

					leafDer := leafCertWithParams(t, intDer, validNotBefore, validNotAfter, intKey, validSubject)

					return rootDer, invalidIntDer, leafDer
				},
				expErrMsg: x509.UnknownAuthorityError{}.Error(),
			},
			{
				name: "leaf certificate is signed by another intermediate different",
				generateCerts: func() ([]byte, []byte, []byte) {
					rootKey, rootDer := rootCertificate(t, validNotBefore, validNotAfter)

					_, intDer := intermediateCertificate(t, rootDer, validNotBefore, validNotAfter, rootKey)

					invalidIntKey, invalidIntDer := intermediateCertificate(t, rootDer, validNotBefore, validNotAfter, rootKey)
					invalidLeafDer := leafCertWithParams(t, invalidIntDer, validNotBefore, validNotAfter, invalidIntKey, validSubject)

					return rootDer, intDer, invalidLeafDer
				},
				expErrMsg: x509.UnknownAuthorityError{}.Error(),
			},
			{
				name: "leaf certificate has a different subject",
				generateCerts: func() ([]byte, []byte, []byte) {
					rootKey, rootDer := rootCertificate(t, validNotBefore, validNotAfter)

					intKey, intDer := intermediateCertificate(t, rootDer, validNotBefore, validNotAfter, rootKey)

					invalidSubject := pkix.Name{CommonName: "Invalid"}
					leafDer := leafCertWithParams(t, intDer, validNotBefore, validNotAfter, intKey, invalidSubject)

					return rootDer, intDer, leafDer
				},
				expErrMsg: jwtsigning.ErrUnknownSubj.Error(),
			},
		}

		for _, tt := range tts {
			t.Run(tt.name, func(t *testing.T) {
				// given
				rootDer, intDer, leafDer := tt.generateCerts()

				rootCa, actErr := x509.ParseCertificate(rootDer)
				assert.NoError(t, actErr)

				subj, err := jwtsigning.NewValidator(rootCa, validSubjString)
				assert.NoError(t, err)

				x5c := []string{
					base64.StdEncoding.EncodeToString(leafDer),
					base64.StdEncoding.EncodeToString(intDer),
				}

				// when
				actErr = subj.Validate(jwtsigning.Key{X5c: x5c})

				// then
				assert.Error(t, actErr)

				if tt.expErrMsg != "" {
					assert.ErrorContains(t, actErr, tt.expErrMsg)
				}

				if tt.expErr != nil {
					assert.ErrorIs(t, actErr, tt.expErr)
				}
			})
		}
	})

	t.Run("should return error if x5c is not having the intermediate certificate used to create leaf certificate", func(t *testing.T) {
		// given
		rootKey, rootDer := rootCertificate(t, validNotBefore, validNotAfter)
		intKey, intDer := intermediateCertificate(t, rootDer, validNotBefore, validNotAfter, rootKey)
		leafDer := leafCertWithParams(t, intDer, validNotBefore, validNotAfter, intKey, validSubject)

		rootCa, actErr := x509.ParseCertificate(rootDer)
		assert.NoError(t, actErr)

		subj, err := jwtsigning.NewValidator(rootCa, validSubjString)
		assert.NoError(t, err)

		// no intermediate certificate
		x5c := []string{
			base64.StdEncoding.EncodeToString(leafDer),
		}

		// when
		actErr = subj.Validate(jwtsigning.Key{X5c: x5c})

		// then
		assert.Error(t, actErr)
		assert.ErrorContains(t, actErr, x509.UnknownAuthorityError{}.Error())
	})
}
