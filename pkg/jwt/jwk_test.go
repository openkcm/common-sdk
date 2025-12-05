package jwt_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/pkg/jwt"
)

func TestNewJWKS(t *testing.T) {
	// given
	tts := []struct {
		name     string
		keyInput []jwt.KeyInput
		expErr   error
	}{
		{
			name: "should fail if KeyAndCerts is nil",
			keyInput: []jwt.KeyInput{
				{
					Kty:         jwt.KeyTypeRSA,
					KeyAndCerts: nil,
				},
			},
			expErr: jwt.ErrCertificateNotFound,
		},
		{
			name: "should fail if KeyAndCerts is empty",
			keyInput: []jwt.KeyInput{
				{
					Kty:         jwt.KeyTypeRSA,
					KeyAndCerts: []jwt.Cert{},
				},
			},
			expErr: jwt.ErrCertificateNotFound,
		},
		{
			name: "should fail if X5c certificate is nil",
			keyInput: []jwt.KeyInput{
				{
					Kty: jwt.KeyTypeRSA,
					KeyAndCerts: []jwt.Cert{
						{
							Kid:     "kid1",
							X5Certs: nil,
						},
					},
				},
			},
			expErr: jwt.ErrRSAPublicKeyNotFound,
		},
		{
			name: "should fail x5c certificate is empty",
			keyInput: []jwt.KeyInput{
				{
					Kty: jwt.KeyTypeRSA,
					KeyAndCerts: []jwt.Cert{
						{
							Kid:     "kid1",
							X5Certs: []x509.Certificate{},
						},
					},
				},
			},
			expErr: jwt.ErrRSAPublicKeyNotFound,
		},
		{
			name: "should fail if kid is duplicate",
			keyInput: []jwt.KeyInput{
				{
					Kty: jwt.KeyTypeRSA,
					KeyAndCerts: []jwt.Cert{
						{
							Kid: "kid1",
							X5Certs: []x509.Certificate{
								{},
							},
						},
						{
							Kid: "kid1",
							X5Certs: []x509.Certificate{
								{},
							},
						},
					},
				},
			},
			expErr: jwt.ErrDuplicateKID,
		},
		{
			name: "should fail if key type is not RSA",
			keyInput: []jwt.KeyInput{
				{
					Kty: "UNKNOWN",
					KeyAndCerts: []jwt.Cert{
						{
							Kid: "kid1",
							X5Certs: []x509.Certificate{
								{},
							},
						},
					},
				},
			},
			expErr: jwt.ErrKeyTypeUnsupported,
		},
	}
	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			// when
			result, err := jwt.NewJWKS(tt.keyInput...)

			// then
			assert.Nil(t, result)
			assert.Error(t, err)
			assert.ErrorIs(t, err, tt.expErr)
		})
	}

	t.Run("should be successful", func(t *testing.T) {
		// given
		prvKey, cert := generateKeysAndCert(t)
		pbKey := prvKey.PublicKey
		expN := pbKey.N.String()
		expE := strconv.Itoa(pbKey.E)
		// der to base64 encoded
		// for this test we are using the cert itself
		expX5c := base64.StdEncoding.EncodeToString(cert.Raw)

		keyInput := jwt.KeyInput{
			Kty:    jwt.KeyTypeRSA,
			Alg:    "PS256",
			Use:    "sig",
			KeyOps: []string{"verify"},
			KeyAndCerts: []jwt.Cert{
				{
					Kid: "kid1",
					X5Certs: []x509.Certificate{
						*cert,
					},
				},
			},
		}

		// when
		result, err := jwt.NewJWKS(keyInput)

		// then
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Keys, 1)
		assert.Equal(t, keyInput.Kty, result.Keys[0].Kty)
		assert.Equal(t, keyInput.Alg, result.Keys[0].Alg)
		assert.Equal(t, keyInput.Use, result.Keys[0].Use)
		assert.Equal(t, keyInput.KeyOps, result.Keys[0].KeyOps)
		assert.Equal(t, keyInput.KeyAndCerts[0].Kid, result.Keys[0].Kid)
		assert.Len(t, result.Keys[0].X5c, 1)
		assert.Equal(t, expX5c, result.Keys[0].X5c[0])
		assert.Equal(t, expN, result.Keys[0].N)
		assert.Equal(t, expE, result.Keys[0].E)
	})
}

func TestHandlerFunc(t *testing.T) {
	// given
	_, cert := generateKeysAndCert(t)

	keyInput := jwt.KeyInput{
		Kty:    jwt.KeyTypeRSA,
		Alg:    "PS256",
		Use:    "sig",
		KeyOps: []string{"verify"},
		KeyAndCerts: []jwt.Cert{
			{
				Kid: "kid1",
				X5Certs: []x509.Certificate{
					*cert,
				},
			},
		},
	}

	result, err := jwt.NewJWKS(keyInput)
	assert.NoError(t, err)

	// when
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	// when
	result.HandlerFunc()(rec, req)

	// then
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Equal(t, http.StatusOK, rec.Result().StatusCode)
	b, err := io.ReadAll(rec.Body)
	assert.NoError(t, err)

	actJWK := jwt.JWKS{}
	err = json.Unmarshal(b, &actJWK)
	assert.NoError(t, err)

	assert.Equal(t, *result, actJWK)
}

func TestWriteAndLoad(t *testing.T) {
	subj := &jwt.JWKS{
		Keys: []jwt.Key{
			{
				Kty:    "RSA",
				Alg:    "PS256",
				Use:    "sig",
				KeyOps: []string{"verify"},
				Kid:    "kid",
				X5c:    []string{"x5c"},
				N:      "RSA N",
				E:      "RSA E",
			},
		},
	}

	t.Run("should create a jwks.json file", func(t *testing.T) {
		// given
		file := filepath.Join(t.TempDir(), uuid.NewString()+".json")

		// when
		err := subj.Write(file)
		assert.NoError(t, err)

		// then
		jwks, err := jwt.NewJWKS()
		assert.NoError(t, err)
		act, err := jwks.Load(file)
		assert.NoError(t, err)
		assert.Equal(t, subj, act)
	})

	t.Run("should overwrite the existing  jwks.json file", func(t *testing.T) {
		// given
		file := filepath.Join(t.TempDir(), uuid.NewString()+".json")

		// when
		err := subj.Write(file)
		assert.NoError(t, err)

		// then
		jwks, err := jwt.NewJWKS()
		assert.NoError(t, err)

		act, err := jwks.Load(file)
		assert.NoError(t, err)
		assert.Equal(t, subj, act)

		// given
		subj2 := &jwt.JWKS{
			Keys: []jwt.Key{
				{
					Kty:    "RSA1",
					Alg:    "PS2561",
					Use:    "sig1",
					KeyOps: []string{"verify1"},
					Kid:    "kid1",
					X5c:    []string{"x5c1"},
					N:      "RSA N1",
					E:      "RSA E2",
				},
			},
		}

		// when
		err = subj2.Write(file)
		assert.NoError(t, err)

		// then
		act, err = jwks.Load(file)
		assert.NoError(t, err)
		assert.Equal(t, subj2, act)
	})
}

func generateKeysAndCert(t *testing.T) (*rsa.PrivateKey, *x509.Certificate) {
	t.Helper()

	prvKey, err := rsa.GenerateKey(rand.Reader, 3072)
	require.NoError(t, err)

	ml := x509.Certificate{
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(5, 0, 0),
		SerialNumber: big.NewInt(123123),
		Subject: pkix.Name{
			CommonName:   "CommonName",
			Organization: []string{"Organization"},
		},
		BasicConstraintsValid: true,
	}

	certByte, err := x509.CreateCertificate(rand.Reader, &ml, &ml, &prvKey.PublicKey, prvKey)
	require.NoError(t, err)

	cert, err := x509.ParseCertificate(certByte)
	require.NoError(t, err)

	return prvKey, cert
}
