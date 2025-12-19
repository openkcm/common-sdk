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

type Client struct {
	cli *http.Client
	url *url.URL
}

var (
	ErrKeyNotFound     = errors.New("key not found")
	ErrInvalidURL      = errors.New("invalid url")
	ErrHTTPStatusNotOK = errors.New("http status not ok")
)

func NewClient(endpoint string) (*Client, error) {
	u, ok := isValidEndpoint(endpoint)
	if !ok {
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

	return &Client{cli: cli, url: u}, nil
}

func isValidEndpoint(endpoint string) (*url.URL, bool) {
	u, err := url.ParseRequestURI(endpoint)
	if err == nil && u.Scheme != "" && u.Host != "" {
		return u, true
	}

	return nil, false
}

func (c *Client) Get(ctx context.Context) (*JWKS, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url.String(), nil)
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
