package jwks

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
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

// Input is used to build JWKS from a set of keys and certificates.
type Input struct {
	Kty       KeyType
	Alg       string
	Use       string
	KeyOps    []string
	Kid       string
	X509Certs []x509.Certificate
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
	// ErrInvalidKey is returned when key validation fails.
	ErrInvalidKey = errors.New("invalid key")
)

// New constructs a JWKS from one or more KeyInput values.
// It ensures each key has a unique KID and at least one certificate.
func New(inputs ...Input) (*JWKS, error) {
	processedKids := make(map[string]struct{}, len(inputs))

	result := &JWKS{
		Keys: make([]Key, 0, len(inputs)),
	}

	for _, input := range inputs {
		if len(input.X509Certs) == 0 {
			return nil, ErrCertificateNotFound
		}

		kid := input.Kid

		_, ok := processedKids[kid]
		if ok {
			return nil, fmt.Errorf("%w %s", ErrDuplicateKID, kid)
		}

		processedKids[kid] = struct{}{}

		keys, err := input.build()
		if err != nil {
			return nil, err
		}

		result.Keys = append(result.Keys, keys)
	}

	return result, nil
}

// Encode writes the JWKS (JSON Web Key Set) to the provided io.Writer in JSON format.
// Returns an error if encoding fails.
func (j *JWKS) Encode(w io.Writer) error {
	return json.NewEncoder(w).Encode(j)
}

// Decode reads JSON data from the provided io.Reader and populates the JWKS struct.
// It returns an error if decoding fails or if no keys are found in the JWKS.
// If the JWKS contains no keys, ErrCertificateNotFound is returned.
func (j *JWKS) Decode(r io.Reader) error {
	// this will make sure the old data is in not replaced incase of an error
	tJWKS := JWKS{}

	err := json.NewDecoder(r).Decode(&tJWKS)
	if err != nil {
		return err
	}

	if len(tJWKS.Keys) == 0 {
		return ErrCertificateNotFound
	}

	for _, key := range tJWKS.Keys {
		err := key.Validate()
		if err != nil {
			return err
		}
	}

	j.Keys = tJWKS.Keys

	return nil
}

func (k Key) Validate() error {
	if k.Kty != KeyTypeRSA {
		return fmt.Errorf("%w %s", ErrInvalidKey, "type is invalid")
	}

	if isEmpty(k.Alg) {
		return fmt.Errorf("%w %s", ErrInvalidKey, "alg is invalid")
	}

	if isEmpty(k.Use) {
		return fmt.Errorf("%w %s", ErrInvalidKey, "use is invalid")
	}

	if hasEmptyValues(k.KeyOps) {
		return fmt.Errorf("%w %s", ErrInvalidKey, "keyops is empty")
	}

	if isEmpty(k.Kid) {
		return fmt.Errorf("%w %s", ErrInvalidKey, "kid is invalid")
	}

	if hasEmptyValues(k.X5c) {
		return fmt.Errorf("%w %s", ErrInvalidKey, "x5c is empty")
	}

	if isEmpty(k.N) {
		return fmt.Errorf("%w %s", ErrInvalidKey, "n is invalid")
	}

	if isEmpty(k.E) {
		return fmt.Errorf("%w %s", ErrInvalidKey, "e is invalid")
	}

	return nil
}

// build constructs a Key from the KeyInput.
// It encodes the provided X.509 certificates to base64 and sets them in the Key's X5c field.
// For RSA keys, it extracts the modulus (N) and exponent (E) from the first certificate's public key.
// Returns an error if the key type is unsupported or if the RSA public key cannot be found.
func (i Input) build() (Key, error) {
	x5cs := make([]string, 0, len(i.X509Certs))

	var firstCert x509.Certificate

	for i, cert := range i.X509Certs {
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
		Kid:    i.Kid,
		X5c:    x5cs,
	}

	switch i.Kty {
	case KeyTypeRSA:
		publicKey, ok := firstCert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return Key{}, fmt.Errorf("%w kid %s", ErrRSAPublicKeyNotFound, i.Kid)
		}

		key.N = publicKey.N.String()
		key.E = strconv.Itoa(publicKey.E)
	default:
		return Key{}, fmt.Errorf("%w [%s]", ErrKeyTypeUnsupported, i.Kty)
	}

	return key, nil
}

// hasEmptyValues checks if the provided slice of strings contains any empty values.
func hasEmptyValues(values []string) bool {
	if len(values) == 0 {
		return true
	}

	return slices.ContainsFunc(values, isEmpty)
}

// isEmpty returns true if the given string is empty or contains only whitespace.
func isEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}
