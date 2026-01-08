package jwtsigning_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/openkcm/common-sdk/pkg/jwtsigning"
)

func TestNew(t *testing.T) {
	_, cert1 := generateKeysAndCert(t)
	_, cert2 := generateKeysAndCert(t)
	// given
	tts := []struct {
		name     string
		keyInput []jwtsigning.Input
		expErr   error
	}{
		{
			name: "should fail if X509 certificate is nil",
			keyInput: []jwtsigning.Input{
				{
					Kty:       jwtsigning.KeyTypeRSA,
					Kid:       "kid1",
					X509Certs: nil,
				},
			},
			expErr: jwtsigning.ErrCertificateNotFound,
		},
		{
			name: "should fail if x509 certificate is empty",
			keyInput: []jwtsigning.Input{
				{
					Kty:       jwtsigning.KeyTypeRSA,
					Kid:       "kid1",
					X509Certs: []x509.Certificate{},
				},
			},
			expErr: jwtsigning.ErrCertificateNotFound,
		},
		{
			name: "should fail if kid is duplicate",
			keyInput: []jwtsigning.Input{
				{
					Kty: jwtsigning.KeyTypeRSA,
					Kid: "kid1",
					X509Certs: []x509.Certificate{
						*cert1,
					},
				},
				{
					Kty: jwtsigning.KeyTypeRSA,

					Kid: "kid1",
					X509Certs: []x509.Certificate{
						*cert2,
					},
				},
			},
			expErr: jwtsigning.ErrDuplicateKID,
		},
		{
			name: "should fail if key type is not RSA",
			keyInput: []jwtsigning.Input{
				{
					Kty: "UNKNOWN",
					Kid: "kid1",
					X509Certs: []x509.Certificate{
						*cert1,
					},
				},
			},
			expErr: jwtsigning.ErrKeyTypeUnsupported,
		},
	}
	for _, tt := range tts {
		t.Run(tt.name, func(t *testing.T) {
			// when
			result, err := jwtsigning.NewJWKS(tt.keyInput...)

			// then
			assert.Nil(t, result)
			assert.Error(t, err)
			assert.ErrorIs(t, err, tt.expErr)
		})
	}

	t.Run("should be successful", func(t *testing.T) {
		// given
		prvKey1, cert1 := generateKeysAndCert(t)
		prvKey2, cert2 := generateKeysAndCert(t)

		tts := []struct {
			name            string
			privateKeyOrder []*rsa.PrivateKey
			keyInputs       []jwtsigning.Input
		}{
			{
				name:            "with one keyInput",
				privateKeyOrder: []*rsa.PrivateKey{prvKey1},
				keyInputs: []jwtsigning.Input{
					{
						Kty:    jwtsigning.KeyTypeRSA,
						Alg:    "PS256",
						Use:    "sig",
						KeyOps: []string{"verify"},
						Kid:    "kid1",
						X509Certs: []x509.Certificate{
							*cert1,
						},
					},
				},
			},
			{
				name:            "with multiple keyInput",
				privateKeyOrder: []*rsa.PrivateKey{prvKey1, prvKey2},
				keyInputs: []jwtsigning.Input{
					{
						Kty:    jwtsigning.KeyTypeRSA,
						Alg:    "PS256",
						Use:    "sig",
						KeyOps: []string{"verify"},
						Kid:    "kid1",
						X509Certs: []x509.Certificate{
							*cert1,
						},
					},
					{
						Kty:    jwtsigning.KeyTypeRSA,
						Alg:    "PS256",
						Use:    "sig",
						KeyOps: []string{"verify"},
						Kid:    "kid2",
						X509Certs: []x509.Certificate{
							*cert2,
						},
					},
				},
			},
		}

		asserter := func(t *testing.T, privKey *rsa.PrivateKey, keyInput jwtsigning.Input, key jwtsigning.Key) {
			t.Helper()

			cert := keyInput.X509Certs[0]
			pbKey := privKey.PublicKey
			expN := pbKey.N.String()
			expE := strconv.Itoa(pbKey.E)
			// der to base64 encoded
			// for this test we are using the cert itself
			expX5c := base64.StdEncoding.EncodeToString(cert.Raw)

			assert.Equal(t, keyInput.Kty, key.Kty)
			assert.Equal(t, keyInput.Alg, key.Alg)
			assert.Equal(t, keyInput.Use, key.Use)
			assert.Equal(t, keyInput.KeyOps, key.KeyOps)
			assert.Equal(t, keyInput.Kid, key.Kid)
			assert.Len(t, key.X5c, 1)
			assert.Equal(t, expX5c, key.X5c[0])
			assert.Equal(t, expN, key.N)
			assert.Equal(t, expE, key.E)
		}

		for _, tt := range tts {
			t.Run(tt.name, func(t *testing.T) {
				// when
				result, err := jwtsigning.NewJWKS(tt.keyInputs...)

				// then
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Len(t, result.Keys, len(tt.keyInputs))

				for i, key := range result.Keys {
					asserter(t, tt.privateKeyOrder[i], tt.keyInputs[i], key)
				}
			})
		}
	})
}

func TestEncodeAndDecode(t *testing.T) {
	subj := &jwtsigning.JWKS{
		Keys: []jwtsigning.Key{
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

	t.Run("should create a jwks.json file and read", func(t *testing.T) {
		// given
		file := filepath.Join(t.TempDir(), uuid.NewString()+".json")
		fw, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE, 0o600)
		assert.NoError(t, err)

		defer fw.Close()

		fr, err := os.Open(file)
		assert.NoError(t, err)

		defer fr.Close()

		// when
		err = subj.Encode(fw)
		assert.NoError(t, err)

		// then
		dSubj, err := jwtsigning.NewJWKS()
		assert.NoError(t, err)

		err = dSubj.Decode(fr)
		assert.NoError(t, err)
		assert.Equal(t, subj, dSubj)
	})

	t.Run("should overwrite the existing  jwks.json file", func(t *testing.T) {
		// given
		file := filepath.Join(t.TempDir(), uuid.NewString()+".json")
		fw, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE, 0o600)
		assert.NoError(t, err)

		defer fw.Close()

		fr, err := os.Open(file)
		assert.NoError(t, err)

		defer fr.Close()

		// when
		err = subj.Encode(fw)
		assert.NoError(t, err)

		// then
		dSubj, err := jwtsigning.NewJWKS()
		assert.NoError(t, err)

		err = dSubj.Decode(fr)
		assert.NoError(t, err)
		assert.Equal(t, subj, dSubj)

		// given
		newJWKS := &jwtsigning.JWKS{
			Keys: []jwtsigning.Key{
				{
					Kty:    jwtsigning.KeyTypeRSA,
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
		err = newJWKS.Encode(fw)
		assert.NoError(t, err)

		// then
		err = dSubj.Decode(fr)
		assert.NoError(t, err)
		assert.Equal(t, newJWKS, dSubj)
	})
}

func TestDecode(t *testing.T) {
	t.Run("should return error if", func(t *testing.T) {
		t.Run("file is empty", func(t *testing.T) {
			// given
			file := filepath.Join(t.TempDir(), uuid.NewString()+".json")
			fw, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE, 0o600)
			assert.NoError(t, err)

			defer fw.Close()

			fr, err := os.Open(file)
			assert.NoError(t, err)

			defer fr.Close()

			subj, err := jwtsigning.NewJWKS()
			assert.NoError(t, err)

			// when
			err = subj.Decode(fr)

			// then
			assert.Error(t, err)
			assert.NotNil(t, subj)
		})

		t.Run("file is empty json", func(t *testing.T) {
			// given
			file := filepath.Join(t.TempDir(), uuid.NewString()+".json")
			fw, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE, 0o600)
			assert.NoError(t, err)

			defer fw.Close()

			fr, err := os.Open(file)
			assert.NoError(t, err)

			defer fr.Close()

			subj, err := jwtsigning.NewJWKS()
			assert.NoError(t, err)

			// write a empty json
			err = subj.Encode(fw)
			assert.NoError(t, err)

			// when
			err = subj.Decode(fr)

			// then
			assert.Error(t, err)
			assert.ErrorIs(t, err, jwtsigning.ErrCertificateNotFound)
			assert.NotNil(t, subj)
		})

		t.Run("file has an invalid key", func(t *testing.T) {
			// given
			file := filepath.Join(t.TempDir(), uuid.NewString()+".json")
			fw, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE, 0o600)
			assert.NoError(t, err)

			defer fw.Close()

			fr, err := os.Open(file)
			assert.NoError(t, err)

			defer fr.Close()

			missingAlgKey := jwtsigning.JWKS{
				Keys: []jwtsigning.Key{
					{
						Kty:    jwtsigning.KeyTypeRSA,
						Alg:    "alg",
						Use:    "use",
						KeyOps: []string{"use"},
						Kid:    "kid",
						X5c:    []string{"cert1"},
						N:      "n",
						E:      "e",
					},
					{
						Kty:    jwtsigning.KeyTypeRSA,
						Alg:    "",
						Use:    "use",
						KeyOps: []string{"use"},
						Kid:    "kid",
						X5c:    []string{"cert1"},
						N:      "n",
						E:      "e",
					},
				},
			}

			// write a empty json
			err = missingAlgKey.Encode(fw)
			assert.NoError(t, err)

			// when
			err = missingAlgKey.Decode(fr)

			// then
			assert.Error(t, err)
			assert.ErrorIs(t, err, jwtsigning.ErrInvalidKey)
			assert.NotNil(t, missingAlgKey)
		})
	})
}

func TestKeyValidate(t *testing.T) {
	t.Run("should return error if", func(t *testing.T) {
		// given
		tts := []struct {
			name string
			subj jwtsigning.Key
		}{
			{
				name: "key type is empty",
				subj: jwtsigning.Key{
					Kty: "",
				},
			},
			{
				name: "alg is empty",
				subj: jwtsigning.Key{
					Kty: jwtsigning.KeyTypeRSA,
					Alg: "",
				},
			},
			{
				name: "use is empty",
				subj: jwtsigning.Key{
					Kty: jwtsigning.KeyTypeRSA,
					Alg: "PS256",
					Use: "",
				},
			},
			{
				name: "keyops is nil",
				subj: jwtsigning.Key{
					Kty:    jwtsigning.KeyTypeRSA,
					Alg:    "PS256",
					Use:    "sec",
					KeyOps: nil,
				},
			},

			{
				name: "keyops is empty",
				subj: jwtsigning.Key{
					Kty:    jwtsigning.KeyTypeRSA,
					Alg:    "PS256",
					Use:    "sec",
					KeyOps: []string{},
				},
			},
			{
				name: "keyops is having empty values",
				subj: jwtsigning.Key{
					Kty:    jwtsigning.KeyTypeRSA,
					Alg:    "PS256",
					Use:    "sec",
					KeyOps: []string{"rec", " ", "sig"},
				},
			},
			{
				name: "kid is empty",
				subj: jwtsigning.Key{
					Kty:    jwtsigning.KeyTypeRSA,
					Alg:    "PS256",
					Use:    "sec",
					KeyOps: []string{"rec", "sig"},
					Kid:    "",
				},
			},
			{
				name: "x5c is nil",
				subj: jwtsigning.Key{
					Kty:    jwtsigning.KeyTypeRSA,
					Alg:    "PS256",
					Use:    "sec",
					KeyOps: []string{"rec", "sig"},
					Kid:    "kid",
					X5c:    nil,
				},
			},
			{
				name: "x5c is empty",
				subj: jwtsigning.Key{
					Kty:    jwtsigning.KeyTypeRSA,
					Alg:    "PS256",
					Use:    "sec",
					KeyOps: []string{"rec", "sig"},
					Kid:    "kid",
					X5c:    []string{},
				},
			},
			{
				name: "x5c is having empty values",
				subj: jwtsigning.Key{
					Kty:    jwtsigning.KeyTypeRSA,
					Alg:    "PS256",
					Use:    "sec",
					KeyOps: []string{"rec", "sig"},
					Kid:    "kid",
					X5c:    []string{"cert1", " ", "cert2"},
				},
			},
			{
				name: "n is empty ",
				subj: jwtsigning.Key{
					Kty:    jwtsigning.KeyTypeRSA,
					Alg:    "PS256",
					Use:    "sec",
					KeyOps: []string{"rec", "sig"},
					Kid:    "kid",
					X5c:    []string{"cert1", "cert2"},
					N:      "",
				},
			},
			{
				name: "e is empty ",
				subj: jwtsigning.Key{
					Kty:    jwtsigning.KeyTypeRSA,
					Alg:    "PS256",
					Use:    "sec",
					KeyOps: []string{"rec", "sig"},
					Kid:    "kid",
					X5c:    []string{"cert1", "cert2"},
					N:      "n",
					E:      "",
				},
			},
		}

		for _, tt := range tts {
			t.Run(tt.name, func(t *testing.T) {
				// when
				err := tt.subj.Validate()

				// then
				assert.Error(t, err)
				assert.ErrorIs(t, err, jwtsigning.ErrInvalidKey)
			})
		}
	})
	t.Run("should not return error if key is valid", func(t *testing.T) {
		// given
		subj := jwtsigning.Key{
			Kty:    jwtsigning.KeyTypeRSA,
			Alg:    "PS256",
			Use:    "use",
			KeyOps: []string{"rec", "sig"},
			Kid:    "kid",
			X5c:    []string{"cert1", "cert2"},
			N:      "n",
			E:      "e",
		}

		// when
		err := subj.Validate()

		// then
		assert.NoError(t, err)
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
