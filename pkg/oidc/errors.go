package oidc

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidURI                 = errors.New("invalid URI")
	ErrInvalidURLScheme           = errors.New("invalid URL scheme")
	ErrCouldNotGetWellKnownConfig = errors.New("could not get well known OpenID configuration")
	ErrCouldNotBuildURL           = errors.New("could not build URL")
	ErrCouldNotCreateHTTPRequest  = errors.New("could not create HTTP request")
	ErrCouldNotDoHTTPRequest      = errors.New("could not do HTTP request")
	ErrCouldNotReadResponseBody   = errors.New("could not read response body")
	ErrNoIntrospectionEndpoint    = errors.New("no introspection endpoint in configuration")
	ErrTokenIntrospectionDisabled = errors.New("token introspection is disabled")
)

type ProviderRespondedNon200Error struct {
	Code int
	Body string
}

func (e ProviderRespondedNon200Error) Error() string {
	return fmt.Sprintf("provider responded with non-200 status code: %d", e.Code)
}

type CouldNotUnmarshallResponseError struct {
	Err  error
	Body string
}

func (e CouldNotUnmarshallResponseError) Error() string {
	return fmt.Sprintf("could not decode provider response: %v", e.Err)
}

type CouldNotFindKeyForKeyIDError struct {
	KeyID string
}

func (e CouldNotFindKeyForKeyIDError) Error() string {
	return "could not find key for key ID: " + e.KeyID
}
