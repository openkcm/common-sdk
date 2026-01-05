package jwtsigning_test

import (
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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
			err := result.AddCli("issuer", jwtsigning.JWKSClientStore{Client: &jwtsigning.Client{}, Validator: &jwtsigning.Validator{}})

			// then
			assert.NoError(t, err)
		})
		t.Run("should return error if", func(t *testing.T) {
			tts := []struct {
				name   string
				input  jwtsigning.JWKSClientStore
				expErr error
			}{
				{
					name: "client is nil",
					input: jwtsigning.JWKSClientStore{
						Validator: &jwtsigning.Validator{},
					},
					expErr: jwtsigning.ErrNoClientFound,
				},
				{
					name: "validator is nil",
					input: jwtsigning.JWKSClientStore{
						Client: &jwtsigning.Client{},
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
				jwk, rootCa, expPubKeys := generateJWKSResources(t)

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					b, err := json.Marshal(jwk)
					assert.NoError(t, err)

					_, err = w.Write(b)
					assert.NoError(t, err)
				}))

				defer srv.Close()

				cli, err := jwtsigning.NewClient(srv.URL)
				assert.NoError(t, err)

				validator, err := jwtsigning.NewValidator(rootCa, validSubjString)
				assert.NoError(t, err)

				subj := jwtsigning.NewJWKSProvider()
				err = subj.AddCli("issuer-1", jwtsigning.JWKSClientStore{Client: cli, Validator: validator})
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
				expClientCalls := 0
				jwk, rootCa, expPubKeys := generateJWKSResources(t)

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					expClientCalls++

					b, err := json.Marshal(jwk)
					assert.NoError(t, err)

					_, err = w.Write(b)
					assert.NoError(t, err)
				}))

				defer srv.Close()

				cli, err := jwtsigning.NewClient(srv.URL)
				assert.NoError(t, err)

				validator, err := jwtsigning.NewValidator(rootCa, validSubjString)
				assert.NoError(t, err)

				subj := jwtsigning.NewJWKSProvider()
				err = subj.AddCli("issuer-1", jwtsigning.JWKSClientStore{Client: cli, Validator: validator})
				assert.NoError(t, err)

				// when
				// calling kid twice
				for range 2 {
					for kid, expKey := range expPubKeys {
						result, err := subj.VerificationKey(t.Context(), "issuer-1", kid)

						// then
						assert.NoError(t, err)
						assert.Equal(t, expKey, result)
					}
				}

				assert.Equal(t, 1, expClientCalls)
			})

			t.Run("should call client if kid is not there in cache", func(t *testing.T) {
				// given
				expClientCalls := 0
				jwk, rootCa, expPubKeys := generateJWKSResources(t)

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					expClientCalls++

					b, err := json.Marshal(jwk)
					assert.NoError(t, err)

					_, err = w.Write(b)
					assert.NoError(t, err)
				}))

				defer srv.Close()

				cli, err := jwtsigning.NewClient(srv.URL)
				assert.NoError(t, err)

				validator, err := jwtsigning.NewValidator(rootCa, validSubjString)
				assert.NoError(t, err)

				subj := jwtsigning.NewJWKSProvider()
				err = subj.AddCli("issuer-1", jwtsigning.JWKSClientStore{Client: cli, Validator: validator})
				assert.NoError(t, err)

				for kid, expKey := range expPubKeys {
					result, err := subj.VerificationKey(t.Context(), "issuer-1", kid)
					assert.NoError(t, err)
					assert.Equal(t, expKey, result)
				}

				assert.Equal(t, 1, expClientCalls)

				// when
				result, err := subj.VerificationKey(t.Context(), "issuer-1", "new-kid")

				// then
				assert.Error(t, err)
				assert.Nil(t, result)
				assert.Equal(t, 2, expClientCalls)
			})

			t.Run("should retain the old cache if the client returns an empty keys for a new kid", func(t *testing.T) {
				// given
				expClientCalls := 0
				jwk, rootCa, expPubKeys := generateJWKSResources(t)

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					expClientCalls++
					if expClientCalls >= 2 {
						b, err := json.Marshal(jwtsigning.JWKS{Keys: []jwtsigning.Key{}})
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

				cli, err := jwtsigning.NewClient(srv.URL)
				assert.NoError(t, err)

				validator, err := jwtsigning.NewValidator(rootCa, validSubjString)
				assert.NoError(t, err)

				subj := jwtsigning.NewJWKSProvider()
				err = subj.AddCli("issuer-1", jwtsigning.JWKSClientStore{Client: cli, Validator: validator})
				assert.NoError(t, err)

				result, err := subj.VerificationKey(t.Context(), "issuer-1", "kid-1")
				assert.NoError(t, err)
				assert.NotNil(t, expPubKeys, result)
				assert.Equal(t, 1, expClientCalls)

				for kid, expKey := range expPubKeys {
					result, err = subj.VerificationKey(t.Context(), "issuer-1", kid)
					assert.NoError(t, err)
					assert.Equal(t, expKey, result)
				}

				assert.Equal(t, 1, expClientCalls)

				// calling with new kid where we get a empty keys response and error
				result, err = subj.VerificationKey(t.Context(), "issuer-1", "new-kid")
				assert.Error(t, err)
				assert.ErrorIs(t, err, jwtsigning.ErrKidNoPublicKeyFound)
				assert.Nil(t, result)
				assert.Equal(t, 2, expClientCalls)

				// checking if old kid is still retained
				for kid, key := range expPubKeys {
					// when
					result, err = subj.VerificationKey(t.Context(), "issuer-1", kid)

					// then
					assert.NoError(t, err)
					assert.Equal(t, key, result)
				}

				assert.Equal(t, 2, expClientCalls)
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

			t.Run("if issuer client returns an http 500 error", func(t *testing.T) {
				// given
				_, rootCa, _ := generateJWKSResources(t)
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))

				defer srv.Close()

				cli, err := jwtsigning.NewClient(srv.URL)
				assert.NoError(t, err)

				validator, err := jwtsigning.NewValidator(rootCa, validSubjString)
				assert.NoError(t, err)

				subj := jwtsigning.NewJWKSProvider()
				err = subj.AddCli("issuer-1", jwtsigning.JWKSClientStore{Client: cli, Validator: validator})
				assert.NoError(t, err)

				// when
				result, err := subj.VerificationKey(t.Context(), "issuer-1", "kid-1")

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, err, jwtsigning.ErrHTTPStatusNotOK)
				assert.Nil(t, result)
			})

			t.Run("if validator returns an error", func(t *testing.T) {
				// given
				jwk, rootCa, _ := generateJWKSResources(t)
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					b, err := json.Marshal(jwk)
					assert.NoError(t, err)

					_, err = w.Write(b)
					assert.NoError(t, err)
				}))

				defer srv.Close()

				cli, err := jwtsigning.NewClient(srv.URL)
				assert.NoError(t, err)

				validator, err := jwtsigning.NewValidator(rootCa, "invalid subject rDNS")
				assert.NoError(t, err)

				subj := jwtsigning.NewJWKSProvider()
				err = subj.AddCli("issuer-1", jwtsigning.JWKSClientStore{Client: cli, Validator: validator})
				assert.NoError(t, err)

				// when
				result, err := subj.VerificationKey(t.Context(), "issuer-1", "kid-1")

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, err, jwtsigning.ErrKidNoPublicKeyFound)
				assert.Nil(t, result)
			})

			t.Run("if key type is not RSA", func(t *testing.T) {
				// given
				jwk, rootCa, _ := generateJWKSResources(t)

				// changing key type to unknown
				for i, key := range jwk.Keys {
					key.Kty = "unknown"
					jwk.Keys[i] = key
				}

				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					b, err := json.Marshal(jwk)
					assert.NoError(t, err)

					_, err = w.Write(b)
					assert.NoError(t, err)
				}))

				defer srv.Close()

				cli, err := jwtsigning.NewClient(srv.URL)
				assert.NoError(t, err)

				validator, err := jwtsigning.NewValidator(rootCa, validSubjString)
				assert.NoError(t, err)

				subj := jwtsigning.NewJWKSProvider()
				err = subj.AddCli("issuer-1", jwtsigning.JWKSClientStore{Client: cli, Validator: validator})
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

func generateJWKSResources(t *testing.T) (jwtsigning.JWKS, *x509.Certificate, map[string]*rsa.PublicKey) {
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

	j := jwtsigning.JWKS{
		Keys: []jwtsigning.Key{
			{
				Kty:    jwtsigning.KeyTypeRSA,
				Alg:    "alg",
				Use:    "use",
				KeyOps: []string{"encryption"},
				Kid:    "kid-1",
				X5c:    x5c1,
				N:      pubKey1.N.String(),
				E:      strconv.Itoa(pubKey1.E),
			},
			{
				Kty:    jwtsigning.KeyTypeRSA,
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
