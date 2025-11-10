package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"reflect"
	"testing"

	"github.com/openkcm/common-sdk/v2/pkg/auth"
)

func TestEndToEnd(t *testing.T) {
	// Arrange
	rsaPrivateKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		t.Fatalf("could not generate RSA key: %s", err)
	}

	rsaPublicKey := &rsaPrivateKey.PublicKey

	defClientData := &auth.ClientData{
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

	// create the test cases
	tests := []struct {
		name       string
		clientData *auth.ClientData
		privateKey any
		publicKey  any
		wantError  bool
		wantError2 bool
		wantError3 bool
	}{
		{
			name:       "invalid signature algorithm",
			clientData: &auth.ClientData{},
			wantError:  true,
		}, {
			name:       "invalid private key",
			clientData: defClientData,
			privateKey: "not a private key",
			wantError:  true,
		}, {
			name:       "invalid public key",
			clientData: defClientData,
			privateKey: rsaPrivateKey,
			publicKey:  "not a public key",
			wantError3: true,
		}, {
			name:       "ok",
			clientData: defClientData,
			privateKey: rsaPrivateKey,
			publicKey:  rsaPublicKey,
		},
	}

	// run the tests
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Act Encode
			b64data, b64sig, err := tc.clientData.Encode(tc.privateKey)

			// Assert Encode
			if tc.wantError {
				if err == nil {
					t.Error("expected error, but got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %s", err)
				} else {
					// Act Decode
					clientData, err2 := auth.DecodeFrom(b64data)

					// Assert Decode
					if tc.wantError2 {
						if err2 == nil {
							t.Error("expected error, but got nil")
						}
					} else {
						if err2 != nil {
							t.Errorf("unexpected error: %s", err2)
						} else {
							if reflect.DeepEqual(clientData, tc.clientData) {
								t.Error("client data does not match")
							}

							// Act Verify
							err3 := clientData.Verify(tc.publicKey, b64sig)

							// Assert Verify
							if tc.wantError3 {
								if err3 == nil {
									t.Error("expected error, but got nil")
								}
							} else {
								if err3 != nil {
									t.Errorf("unexpected error: %s", err3)
								}
							}
						}
					}
				}
			}
		})
	}
}
