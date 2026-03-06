package oidc

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProviderRespondedNon200Error(t *testing.T) {
	err := ProviderRespondedNon200Error{
		Code: 500,
		Body: "internal server error",
	}
	assert.Equal(t, "provider responded with non-200 status code: 500", err.Error())
}

func TestCouldNotUnmarshallResponseError(t *testing.T) {
	originalErr := errors.New("json parse error")
	err := CouldNotUnmarshallResponseError{
		Err:  originalErr,
		Body: "invalid body",
	}
	assert.Equal(t, "could not decode provider response: json parse error", err.Error())
}

func TestCouldNotFindKeyForKeyIDError(t *testing.T) {
	err := CouldNotFindKeyForKeyIDError{
		KeyID: "test-key-id",
	}
	assert.Equal(t, "could not find key for key ID: test-key-id", err.Error())
}
