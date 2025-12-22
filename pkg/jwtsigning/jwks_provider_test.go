package jwtsigning_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/pkg/jwks"
	"github.com/openkcm/common-sdk/pkg/jwtsigning"
)

var (
	validSubjString = "CN=CA,OU=Clients,O=SE,L=Canary,C=US"
	validSubject    = pkix.Name{
		CommonName: "CA", Organization: []string{"SE"}, Country: []string{"US"}, Locality: []string{"Canary"},
		OrganizationalUnit: []string{"Clients"},
	}
)

func TestJWKSProvider(t *testing.T) {
	t.Run("init", func(t *testing.T) {
		t.Run("successful", func(t *testing.T) {
			// given

			// when
			result := jwtsigning.NewJWKSProvider()

			// then
			assert.NotNil(t, result)
		})
	})

	t.Run("AddCli", func(t *testing.T) {
		t.Run("should be successful", func(t *testing.T) {
			// given
			result := jwtsigning.NewJWKSProvider()
			assert.NotNil(t, result)

			// when
			err := result.AddCli("issuer", jwtsigning.ClientCache{Client: &jwks.Client{}, Validator: &jwks.Validator{}})

			// then
			assert.NoError(t, err)
		})
		t.Run("should return error if", func(t *testing.T) {
			tts := []struct {
				name   string
				input  jwtsigning.ClientCache
				expErr error
			}{
				{
					name: "client is nil",
					input: jwtsigning.ClientCache{
						Validator: &jwks.Validator{},
					},
					expErr: jwtsigning.ErrNoClientFound,
				},
				{
					name: "validator is nil",
					input: jwtsigning.ClientCache{
						Client: &jwks.Client{},
					},
					expErr: jwtsigning.ErrNoValidatorFound,
				},
			}

			for _, tt := range tts {
				t.Run(tt.name, func(t *testing.T) {
					// given
					result := jwtsigning.NewJWKSProvider()
					assert.NotNil(t, result)

					// when
					err := result.AddCli("issuer", tt.input)

					// then
					assert.Error(t, err)
					assert.ErrorIs(t, err, tt.expErr)
				})
			}
		})
	})

	t.Run("VerificationKey", func(t *testing.T) {
		t.Run("successful", func(t *testing.T) {
			t.Run("if kid is valid", func(t *testing.T) {
				// given
				jwk, rootCa, expPubKeys := generateJWKS(t)
				assert.Len(t, expPubKeys, 2)

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					b, err := json.Marshal(jwk)
					assert.NoError(t, err)

					_, err = w.Write(b)
					assert.NoError(t, err)
				}))

				defer srv.Close()

				cli, err := jwks.NewClient(srv.URL)
				assert.NoError(t, err)

				validator, err := jwks.NewValidator(rootCa, validSubjString)
				assert.NoError(t, err)

				subj := jwtsigning.NewJWKSProvider()
				err = subj.AddCli("issuer-1", jwtsigning.ClientCache{Client: cli, Validator: validator})
				assert.NoError(t, err)

				for kid, key := range expPubKeys {
					// when
					result, err := subj.VerificationKey(t.Context(), "issuer-1", kid)

					// then
					assert.NoError(t, err)
					assert.Equal(t, key, result)
				}
			})
		})

		t.Run("cache", func(t *testing.T) {
			t.Run("should not call client if the kid is already cached", func(t *testing.T) {
				// given
				expCalls := 0
				jwk, rootCa, expPubKeys := generateJWKS(t)
				assert.Len(t, expPubKeys, 2)

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					expCalls++

					b, err := json.Marshal(jwk)
					assert.NoError(t, err)

					_, err = w.Write(b)
					assert.NoError(t, err)
				}))

				defer srv.Close()

				cli, err := jwks.NewClient(srv.URL)
				assert.NoError(t, err)

				validator, err := jwks.NewValidator(rootCa, validSubjString)
				assert.NoError(t, err)

				subj := jwtsigning.NewJWKSProvider()
				err = subj.AddCli("issuer-1", jwtsigning.ClientCache{Client: cli, Validator: validator})
				assert.NoError(t, err)

				for kid, expKey := range expPubKeys {
					// when
					result, err := subj.VerificationKey(t.Context(), "issuer-1", kid)

					// then
					assert.NoError(t, err)
					assert.Equal(t, expKey, result)
					assert.Equal(t, 1, expCalls)
				}
			})

			t.Run("should call client if kid is not there in cache", func(t *testing.T) {
				// given
				expCalls := 0
				jwk, rootCa, expPubKeys := generateJWKS(t)
				assert.Len(t, expPubKeys, 2)

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					expCalls++

					b, err := json.Marshal(jwk)
					assert.NoError(t, err)

					_, err = w.Write(b)
					assert.NoError(t, err)
				}))

				defer srv.Close()

				cli, err := jwks.NewClient(srv.URL)
				assert.NoError(t, err)

				validator, err := jwks.NewValidator(rootCa, validSubjString)
				assert.NoError(t, err)

				subj := jwtsigning.NewJWKSProvider()
				err = subj.AddCli("issuer-1", jwtsigning.ClientCache{Client: cli, Validator: validator})
				assert.NoError(t, err)

				for kid, expKey := range expPubKeys {
					result, err := subj.VerificationKey(t.Context(), "issuer-1", kid)
					assert.NoError(t, err)
					assert.Equal(t, expKey, result)
					assert.Equal(t, 1, expCalls)
				}

				// when
				result, err := subj.VerificationKey(t.Context(), "issuer-1", "new-kid")

				// then
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Equal(t, 2, expCalls)
			})

			t.Run("should retain the old cache if the client returns an empty keys for a new kid", func(t *testing.T) {
				// given
				expCalls := 0
				jwk, rootCa, expPubKeys := generateJWKS(t)
				assert.Len(t, expPubKeys, 2)

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					expCalls++
					if expCalls >= 2 {
						b, err := json.Marshal(jwks.JWKS{Keys: []jwks.Key{}})
						assert.NoError(t, err)

						_, err = w.Write(b)
						assert.NoError(t, err)

						return
					}

					b, err := json.Marshal(jwk)
					assert.NoError(t, err)

					_, err = w.Write(b)
					assert.NoError(t, err)
				}))

				defer srv.Close()

				cli, err := jwks.NewClient(srv.URL)
				assert.NoError(t, err)

				validator, err := jwks.NewValidator(rootCa, validSubjString)
				assert.NoError(t, err)

				subj := jwtsigning.NewJWKSProvider()
				err = subj.AddCli("issuer-1", jwtsigning.ClientCache{Client: cli, Validator: validator})
				assert.NoError(t, err)

				result, err := subj.VerificationKey(t.Context(), "issuer-1", "kid-1")
				assert.NoError(t, err)
				assert.NotNil(t, expPubKeys, result)
				assert.Equal(t, 1, expCalls)

				for kid, expKey := range expPubKeys {
					result, err = subj.VerificationKey(t.Context(), "issuer-1", kid)
					assert.NoError(t, err)
					assert.Equal(t, expKey, result)
					assert.Equal(t, 1, expCalls)
				}

				// calling with new kid where we get a empty keys response
				result, err = subj.VerificationKey(t.Context(), "issuer-1", "new-kid")
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Equal(t, 2, expCalls)

				// checking if old kid is still retained
				for kid, key := range expPubKeys {
					// when
					result, err = subj.VerificationKey(t.Context(), "issuer-1", kid)

					// then
					assert.NoError(t, err)
					assert.Equal(t, key, result)
					assert.Equal(t, 2, expCalls)
				}
			})
		})

		t.Run("should return error ", func(t *testing.T) {
			t.Run("if issuer client is not found", func(t *testing.T) {
				// given
				subj := jwtsigning.NewJWKSProvider()

				// when
				result, err := subj.VerificationKey(t.Context(), "unknown-issuer", "kid")

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, err, jwtsigning.ErrNoClientFound)
				assert.Nil(t, result)
			})

			t.Run("if issuer client returns an error", func(t *testing.T) {
				// given
				_, rootCa, _ := generateJWKS(t)
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))

				defer srv.Close()

				cli, err := jwks.NewClient(srv.URL)
				assert.NoError(t, err)

				validator, err := jwks.NewValidator(rootCa, "invalid subject rDNS")
				assert.NoError(t, err)

				subj := jwtsigning.NewJWKSProvider()
				err = subj.AddCli("issuer-1", jwtsigning.ClientCache{Client: cli, Validator: validator})
				assert.NoError(t, err)

				// when
				result, err := subj.VerificationKey(t.Context(), "issuer-1", "kid-1")

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, err, jwks.ErrHTTPStatusNotOK)
				assert.Nil(t, result)
			})

			t.Run("if validator returns an error", func(t *testing.T) {
				// given
				jwk, rootCa, _ := generateJWKS(t)
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					b, err := json.Marshal(jwk)
					assert.NoError(t, err)

					_, err = w.Write(b)
					assert.NoError(t, err)
				}))

				defer srv.Close()

				cli, err := jwks.NewClient(srv.URL)
				assert.NoError(t, err)

				validator, err := jwks.NewValidator(rootCa, "invalid subject rDNS")
				assert.NoError(t, err)

				subj := jwtsigning.NewJWKSProvider()
				err = subj.AddCli("issuer-1", jwtsigning.ClientCache{Client: cli, Validator: validator})
				assert.NoError(t, err)

				// when
				result, err := subj.VerificationKey(t.Context(), "issuer-1", "kid-1")

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, err, jwtsigning.ErrKidNoPublicKeyFound)
				assert.Nil(t, result)
			})
		})
	})
}

func rootCertificate(t *testing.T, notBefore time.Time, notAfter time.Time) (*rsa.PrivateKey, []byte) {
	t.Helper()

	rootKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Root CA", Organization: []string{"Root Organization"}},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}

	rootDer, err := x509.CreateCertificate(
		rand.Reader,
		rootTmpl,
		rootTmpl,
		rootKey.Public(),
		rootKey)
	require.NoError(t, err)

	return rootKey, rootDer
}

func intermediateCertificate(t *testing.T, rootDer []byte, notBefore time.Time, notAfter time.Time, rootKey *rsa.PrivateKey) (*rsa.PrivateKey, []byte) {
	t.Helper()

	rootCert, err := x509.ParseCertificate(rootDer)
	require.NoError(t, err)

	intKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	intTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(2),
		Subject:               pkix.Name{CommonName: "Intermediate CA", Organization: []string{"Intermediate Organization"}},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	intDer, err := x509.CreateCertificate(rand.Reader,
		intTmpl,
		rootCert,
		intKey.Public(),
		rootKey)
	require.NoError(t, err)

	return intKey, intDer
}

func leafCertWithParams(t *testing.T, intDer []byte, notBefore time.Time, notAfter time.Time, intKey *rsa.PrivateKey, subject pkix.Name) []byte {
	t.Helper()

	intCert, err := x509.ParseCertificate(intDer)
	require.NoError(t, err)

	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      subject,
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
	}

	leafDer, err := x509.CreateCertificate(rand.Reader,
		leafTmpl,
		intCert,
		leafKey.Public(),
		intKey)
	require.NoError(t, err)

	return leafDer
}

func generateJWKS(t *testing.T) (jwks.JWKS, *x509.Certificate, map[string]*rsa.PublicKey) {
	t.Helper()

	validNotBefore := time.Now()
	validNotAfter := validNotBefore.Add(24 * 7 * time.Hour)

	rootKey, rootDer := rootCertificate(t, validNotBefore, validNotAfter)
	rootCa, err := x509.ParseCertificate(rootDer)
	assert.NoError(t, err)

	intKey1, intDer1 := intermediateCertificate(t, rootDer, validNotBefore, validNotAfter, rootKey)
	leafDer1 := leafCertWithParams(t, intDer1, validNotBefore, validNotAfter, intKey1, validSubject)

	leafCert1, err := x509.ParseCertificate(leafDer1)
	assert.NoError(t, err)

	pubKey1, ok := leafCert1.PublicKey.(*rsa.PublicKey)
	assert.True(t, ok)

	intKey2, intDer2 := intermediateCertificate(t, rootDer, validNotBefore, validNotAfter, rootKey)
	leafDer2 := leafCertWithParams(t, intDer2, validNotBefore, validNotAfter, intKey2, validSubject)

	leafCert2, err := x509.ParseCertificate(leafDer2)
	assert.NoError(t, err)

	pubKey2, ok := leafCert2.PublicKey.(*rsa.PublicKey)
	assert.True(t, ok)

	x5c1 := []string{
		base64.StdEncoding.EncodeToString(leafDer1),
		base64.StdEncoding.EncodeToString(intDer1),
	}
	x5c2 := []string{
		base64.StdEncoding.EncodeToString(leafDer2),
		base64.StdEncoding.EncodeToString(intDer2),
	}

	result := map[string]*rsa.PublicKey{
		"kid-1": pubKey1,
		"kid-2": pubKey2,
	}

	j := jwks.JWKS{
		Keys: []jwks.Key{
			{
				Kty:    "sign",
				Alg:    "alg",
				Use:    "use",
				KeyOps: []string{"encryption"},
				Kid:    "kid-1",
				X5c:    x5c1,
				N:      pubKey1.N.String(),
				E:      strconv.Itoa(pubKey1.E),
			},
			{
				Kty:    "sign",
				Alg:    "alg",
				Use:    "use",
				KeyOps: []string{"encryption"},
				Kid:    "kid-2",
				X5c:    x5c2,
				N:      pubKey2.N.String(),
				E:      strconv.Itoa(pubKey2.E),
			},
		},
	}

	return j, rootCa, result
}
