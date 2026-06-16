package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/openkcm/common-sdk/pkg/auth"
)

func TestEndToEnd(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 4096)
	assert.NoError(t, err)

	clientData := &auth.ClientData{
		// Mandatory user attributes
		Identifier: "test-subject",
		Email:      "test-email",
		GivenName:  "test-given-name",
		FamilyName: "test-family-name",
		Groups:     []string{"group1", "group2"},

		// Optional user attributes
		Type:   "test-type",
		Region: "test-region",

		// Authentication context
		AuthContext: map[string]string{
			"issuer":    "test-issuer",
			"client_id": "test-client-id",
		},

		SignatureAlgorithm: auth.SignatureAlgorithmRS256,
	}

	ErrDecode := errors.New("error decoding")
	ErrEncode := errors.New("error encoding")
	ErrVerify := errors.New("error verifying")

	// create the test cases
	tests := []struct {
		name              string
		clientData        *auth.ClientData
		privateKey        any
		publicKey         rsa.PublicKey
		err               error
		postDecodeNowFunc func() time.Time
		ttl               time.Duration
	}{
		{
			name:              "invalid signature algorithm",
			clientData:        &auth.ClientData{},
			err:               ErrEncode,
			postDecodeNowFunc: time.Now,
		}, {
			name:              "invalid private key",
			clientData:        clientData,
			privateKey:        "not a private key",
			err:               ErrEncode,
			postDecodeNowFunc: time.Now,
		}, {
			name:              "expired client data using default ttl (1min)",
			clientData:        clientData,
			privateKey:        key,
			publicKey:         key.PublicKey,
			err:               ErrVerify,
			postDecodeNowFunc: func() time.Time { return time.Now().Add(time.Minute * 2) },
		}, {
			name:              "not fail if expoire is less than custom ttl (5min)",
			clientData:        clientData,
			privateKey:        key,
			publicKey:         key.PublicKey,
			postDecodeNowFunc: func() time.Time { return time.Now().Add(time.Minute * 2) },
			ttl:               5 * time.Minute,
		}, {
			name:              "ok",
			clientData:        clientData,
			privateKey:        key,
			publicKey:         key.PublicKey,
			postDecodeNowFunc: time.Now,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b64data, b64sig, err := tc.clientData.Encode(tc.privateKey, auth.WithTTL(tc.ttl))

			if errors.Is(tc.err, ErrEncode) {
				assert.Error(t, err)
				return
			} else {
				assert.NoError(t, err)
			}

			clientData, err := auth.DecodeFrom(b64data)

			if errors.Is(tc.err, ErrDecode) {
				assert.Error(t, err)
				return
			} else {
				assert.NoError(t, err)
			}

			auth.SetNowFunc(tc.postDecodeNowFunc)

			err = clientData.Verify(tc.publicKey, b64sig)

			if errors.Is(tc.err, ErrVerify) {
				assert.Error(t, err)
				return
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
