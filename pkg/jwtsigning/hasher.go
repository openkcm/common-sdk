package jwtsigning

import (
	"crypto/sha256"
	"encoding/base64"
)

const hashAlgorithm = "SHA256"

type SHA256Hasher struct{}

func (s *SHA256Hasher) HashMessage(body []byte) string {
	sum := sha256.Sum256(body)

	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func (s *SHA256Hasher) ToString() string {
	return hashAlgorithm
}
