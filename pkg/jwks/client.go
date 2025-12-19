package jwks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client represents a JWKS (JSON Web Key Set) client that fetches keys from a remote endpoint.
// It uses an HTTP client to perform requests to the specified endpoint.
type Client struct {
	cli      *http.Client // HTTP client used for making requests
	endpoint string       // URL of the JWKS endpoint
}

var (
	ErrKeyNotFound     = errors.New("key not found")
	ErrInvalidURL      = errors.New("invalid url")
	ErrHTTPStatusNotOK = errors.New("http status not ok")
)

// NewClient creates and returns a new Client for the given JWKS endpoint.
// It validates the endpoint URL and configures an HTTP client with connection pooling and timeouts.
// Returns an error if the endpoint is invalid.
func NewClient(endpoint string) (*Client, error) {
	u, err := parseURL(endpoint)
	if err != nil {
		return nil, ErrInvalidURL
	}

	cli := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        20,
			MaxConnsPerHost:     20,
			MaxIdleConnsPerHost: 20,
			IdleConnTimeout:     30 * time.Second,
		},
		Timeout: 10 * time.Second,
	}

	return &Client{cli: cli, endpoint: u.String()}, nil
}

// Get retrieves the JSON Web Key Set (JWKS) from the configured endpoint.
// It sends an HTTP GET request using the provided context, checks for a successful response,
// reads and unmarshals the response body into a JWKS struct, and returns it.
// Returns an error if the request fails, the status is not OK, or the response cannot be parsed.
func (c *Client) Get(ctx context.Context) (*JWKS, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint, nil)
	if err != nil {
		return nil, err
	}

	res, err := c.cli.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w status is %d", ErrHTTPStatusNotOK, res.StatusCode)
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	jwks := JWKS{}

	err = json.Unmarshal(b, &jwks)
	if err != nil {
		return nil, err
	}

	return &jwks, nil
}

func parseURL(endpoint string) (*url.URL, error) {
	u, err := url.ParseRequestURI(endpoint)
	if err == nil && u.Scheme != "" && u.Host != "" {
		return u, nil
	}

	return nil, ErrInvalidURL
}
