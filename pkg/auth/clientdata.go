// Package auth provides a way to encode, decode, and verify client data using signatures.
// The data and the signature is base64 URL encoded and passed as HTTP headers.
// This comprises information like:
// - the client subject (e.g. from a JWT token or an x509 client certificate)
// - the client type (e.g. user or technical user)
// - the client email
// - the client region (e.g. x509 client certificates representing a remote service)
// - the client groups (e.g. user groups or service groups)
// At the gateway, the client data is encoded and signed using a private key.
// Consuming services can decode the client data and verify the signature using a public key.
package auth

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
)

const (
	HeaderClientData          = "x-client-data"
	HeaderClientDataSignature = "x-client-data-signature"
)

var (
	ErrInvalidClientDataSignatureAlgorithm = errors.New("invalid client data signature algorithm")
	ErrInvalidClientDataSignature          = errors.New("invalid client data signature")
	ErrInvalidClientData                   = errors.New("invalid client data")
	ErrInvalidPrivateKey                   = errors.New("invalid private key")
	ErrInvalidPublicKey                    = errors.New("invalid public key")
)

type SignatureAlgorithm string

const (
	SignatureAlgorithmRS256 SignatureAlgorithm = "RS256"
)

type ClientData struct {
	Subject string   `json:"sub"`
	Type    string   `json:"type"`
	Email   string   `json:"mail"`
	Region  string   `json:"reg"`
	Groups  []string `json:"groups,omitempty"`

	// KeyID is a unique identifier for the key used to sign the client data.
	// This way the consumer can determine which key to use to verify the signature
	// and when to fetch a new public key.
	KeyID string `json:"kid"`
	// SignatureAlgorithm is the algorithm used to sign the client data.
	SignatureAlgorithm SignatureAlgorithm `json:"alg"`

	b64data string
}

// DecodeFrom decodes the base64 URL encoded client data and unmarshals it into a ClientData struct.
func DecodeFrom(b64data string) (*ClientData, error) {
	jsonString, err := base64.RawURLEncoding.DecodeString(b64data)
	if err != nil {
		return nil, errors.Join(ErrInvalidClientData, err)
	}

	clientData := &ClientData{
		b64data: b64data,
	}
	if err := json.Unmarshal(jsonString, clientData); err != nil {
		return nil, errors.Join(ErrInvalidClientData, err)
	}

	return clientData, nil
}

// Verify verifies the signature of the client data using the provided public key.
func (c *ClientData) Verify(publicKey any, b64sig string) error {
	switch c.SignatureAlgorithm {
	case SignatureAlgorithmRS256:
		signature, err := base64.RawURLEncoding.DecodeString(b64sig)
		if err != nil {
			return errors.Join(ErrInvalidClientDataSignature, err)
		}
		hashedData := sha256.Sum256([]byte(c.b64data))
		rsaPublicKey, ok := publicKey.(*rsa.PublicKey)
		if !ok {
			return ErrInvalidPublicKey
		}
		return rsa.VerifyPKCS1v15(rsaPublicKey, crypto.SHA256, hashedData[:], signature)
	}
	return ErrInvalidClientDataSignatureAlgorithm
}

// Encode encodes the client data and signs it using the provided private key.
// Both values are returned as base64 URL encoded strings.
func (c *ClientData) Encode(privateKey any) (string, string, error) {
	jsonString, err := json.Marshal(c)
	if err != nil {
		return "", "", err
	}
	b64data := base64.RawURLEncoding.EncodeToString(jsonString)
	switch c.SignatureAlgorithm {
	case SignatureAlgorithmRS256:
		hashedData := sha256.Sum256([]byte(b64data))
		rsaPrivateKey, ok := privateKey.(*rsa.PrivateKey)
		if !ok {
			return "", "", ErrInvalidPrivateKey
		}
		signature, err := rsa.SignPKCS1v15(nil, rsaPrivateKey, crypto.SHA256, hashedData[:])
		if err != nil {
			return "", "", err
		}
		b64sig := base64.RawURLEncoding.EncodeToString(signature)
		return b64data, b64sig, nil
	}
	return "", "", ErrInvalidClientDataSignatureAlgorithm
}
