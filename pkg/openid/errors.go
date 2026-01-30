package openid

import (
	"errors"
	"fmt"
)

var (
	ErrCouldNotBuildURL          = errors.New("could not build URL")
	ErrCouldNotCreateHTTPRequest = errors.New("could not create HTTP request")
	ErrCouldNotDoHTTPRequest     = errors.New("could not do HTTP request")
	ErrCouldNotReadResponseBody  = errors.New("could not read response body")
	ErrNoIntrospectionEndpoint   = errors.New("no introspection endpoint in configuration")
)

type ProviderRespondedNon200Error struct {
	Code int
	Body string
}

func (e ProviderRespondedNon200Error) Error() string {
	return fmt.Sprintf("provider responded with non-200 status code: %d", e.Code)
}

type CouldNotDecodeResponseError struct {
	Err  error
	Body string
}

func (e CouldNotDecodeResponseError) Error() string {
	return fmt.Sprintf("could not decode provider response: %v", e.Err)
}
