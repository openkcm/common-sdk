package jwt

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

// JWKS represents a JSON Web Key Set, containing multiple JWK keys.
type JWKS struct {
	Keys []Key `json:"keys"`
}

// KeyType specifies the type of cryptographic key (e.g., "RSA").
type KeyType string

// Key defines the structure of a single JSON Web Key.
type Key struct {
	Kty    KeyType  `json:"kty"`     // Key type (e.g., "RSA")
	Alg    string   `json:"alg"`     // Algorithm intended for use with the key
	Use    string   `json:"use"`     // Intended use of the public key
	KeyOps []string `json:"key_ops"` // Permitted operations for the key
	Kid    string   `json:"kid"`     // Key ID
	X5c    []string `json:"x5c"`     // X.509 certificate chain
	N      string   `json:"n"`       // RSA modulus
	E      string   `json:"e"`       // RSA public exponent
}

// KeyInput is used to build JWKS from a set of keys and certificates.
type KeyInput struct {
	Kty         KeyType
	Alg         string
	Use         string
	KeyOps      []string
	KeyAndCerts []Cert
}

// Cert contains a key ID, associated X.509 certificates, and the private key.
type Cert struct {
	Kid     string
	X5Certs []x509.Certificate
}

// KeyTypeRSA is the constant for RSA key type.
var KeyTypeRSA KeyType = "RSA"

var (
	// ErrRSAPublicKeyNotFound is returned when a certificate does not contain an RSA public key.
	ErrRSAPublicKeyNotFound = errors.New("not a RSA public key")
	// ErrCertificateNotFound is returned when no certificate is provided.
	ErrCertificateNotFound = errors.New("certificate not found")
	// ErrDuplicateKID is returned when duplicate key IDs are detected.
	ErrDuplicateKID = errors.New("duplicate kid")
	// ErrKeyTypeUnsupported is returned when an unsupported key type is encountered.
	ErrKeyTypeUnsupported = errors.New("key type unsupported")
)

// NewJWKS constructs a JWKS from one or more KeyInput values.
// It ensures each key has a unique KID and at least one certificate.
func NewJWKS(keyInputs ...KeyInput) (*JWKS, error) {
	keysLength := 0

	kids := map[string]struct{}{}

	for _, keyInput := range keyInputs {
		keyLen := len(keyInput.KeyAndCerts)
		if keyLen == 0 {
			return nil, ErrCertificateNotFound
		}

		for _, certs := range keyInput.KeyAndCerts {
			_, ok := kids[certs.Kid]
			if ok {
				return nil, fmt.Errorf("%s %w", certs.Kid, ErrDuplicateKID)
			}

			kids[certs.Kid] = struct{}{}
		}

		keysLength += keyLen
	}

	result := &JWKS{
		Keys: make([]Key, 0, keysLength),
	}

	for _, keyInput := range keyInputs {
		keys, err := keyInput.build()
		if err != nil {
			return nil, err
		}

		result.Keys = append(result.Keys, keys...)
	}

	return result, nil
}

// Write serializes the JWKS object to JSON and atomically writes it to the specified file.
// The operation is POSIX-safe and prevents partial writes or file corruption.
func (j *JWKS) Write(fileName string) error {
	b, err := json.Marshal(j)
	if err != nil {
		return err
	}

	dir := filepath.Dir(fileName)

	tmpFile, err := os.CreateTemp(dir, "jwks-*")
	if err != nil {
		return err
	}

	_, err = tmpFile.Write(b)
	if err != nil {
		tmpFile.Close()
		os.Remove(tmpFile.Name())

		return err
	}

	defer os.Remove(tmpFile.Name())

	err = tmpFile.Sync()
	if err != nil {
		tmpFile.Close()

		return err
	}

	err = tmpFile.Close()
	if err != nil {
		return err
	}

	return os.Rename(tmpFile.Name(), fileName)
}

// Load reads a JWKS from the specified file and unmarshals it from JSON.
func (j *JWKS) Load(name string) (*JWKS, error) {
	b, err := os.ReadFile(name)
	if err != nil {
		return nil, err
	}

	result := JWKS{}

	err = json.Unmarshal(b, &result)

	return &result, err
}

// HandlerFunc returns an http.HandlerFunc that serves the JWKS (JSON Web Key Set)
// as a JSON response.
func (j *JWKS) HandlerFunc() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		b, err := json.Marshal(j)
		if err != nil {
			http.Error(w, "jwks error", http.StatusInternalServerError)
			return
		}

		_, err = w.Write(b)
		if err != nil {
			http.Error(w, "failed to write data", http.StatusInternalServerError)
			return
		}
	}
}

// build creates a slice of Key objects from the KeyInput.
// It encodes certificates and extracts RSA public key parameters.
func (i KeyInput) build() ([]Key, error) {
	result := make([]Key, 0, len(i.KeyAndCerts))
	for _, keyAndCert := range i.KeyAndCerts {
		x5cs := make([]string, 0, len(keyAndCert.X5Certs))

		var firstCert x509.Certificate

		for i, cert := range keyAndCert.X5Certs {
			if i == 0 {
				firstCert = cert
			}

			bCert := base64.StdEncoding.EncodeToString(cert.Raw)
			x5cs = append(x5cs, bCert)
		}

		key := Key{
			Kty:    i.Kty,
			Alg:    i.Alg,
			Use:    i.Use,
			KeyOps: i.KeyOps,
			Kid:    keyAndCert.Kid,
			X5c:    x5cs,
		}
		switch i.Kty {
		case KeyTypeRSA:
			publicKey, ok := firstCert.PublicKey.(*rsa.PublicKey)
			if !ok {
				return nil, fmt.Errorf("kid %s %w", keyAndCert.Kid, ErrRSAPublicKeyNotFound)
			}

			key.N = publicKey.N.String()
			key.E = strconv.Itoa(publicKey.E)
		default:
			return nil, fmt.Errorf("[%s] %w", i.Kty, ErrKeyTypeUnsupported)
		}

		result = append(result, key)
	}

	return result, nil
}
