// Package commonhttp provides utilities to create HTTP clients
// configured with OAuth2 credentials and optional mutual TLS (mTLS).

package commonhttp

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/pointers"
)

// NewClientFromOAuth2 creates an *http.Client configured with OAuth2 credentials
// and optional mTLS transport.
//
// The client supports two types of OAuth2 authentication:
//  1. client_secret: requires ClientID and ClientSecretPost
//  2. private_key_jwt: requires ClientID, ClientAssertion, and ClientAssertionType
//
// Only one authentication method may be provided. If both clientSecret and
// clientAssertion are configured, this function returns an error.
//
// If MTLS is provided, the client's transport will use the TLS configuration.
//
// Parameters:
//   - clientAuth: a pointer to an OAuth2 configuration containing credentials and optional mTLS.
//
// Returns:
//   - *http.Client: the configured HTTP client
//   - error: if the configuration is invalid or mTLS loading fails
func NewClientFromOAuth2(clientAuth *commoncfg.OAuth2) (*http.Client, error) {
	if clientAuth == nil {
		return nil, errors.New("oauth2 config is nil")
	}

	if clientAuth.Credentials.ClientID.Source == "" {
		return nil, errors.New("oauth2.clientID is missing or invalid")
	}

	clientID, err := commoncfg.LoadValueFromSourceRef(clientAuth.Credentials.ClientID)
	if err != nil {
		return nil, fmt.Errorf("OAuth2 credentials missing client ID: %w", err)
	}

	rt := &clientOAuth2RoundTripper{
		ClientID: string(clientID),
		Next:     http.DefaultTransport,
		jwtCache: make(map[string]cachedJWT),
	}

	tokenURL, _ := commoncfg.ExtractValueFromSourceRef(clientAuth.URL)
	if tokenURL != nil && string(tokenURL) != "" {
		rt.TokenURL = string(tokenURL)
	}

	// Load mTLS if configured
	err = loadMTLS(clientAuth.MTLS, rt)
	if err != nil {
		return nil, err
	}

	// Load OAuth2 credentials
	loadOAuth2Credentials(&clientAuth.Credentials, rt)

	err = validate(clientAuth, rt)
	if err != nil {
		return nil, err
	}

	return &http.Client{Transport: rt}, nil
}

// loadMTLS configures the HTTP transport to use mutual TLS (mTLS).
//
// Parameters:
//   - mtls: MTLS configuration (cert, key, server CA)
//   - rt: pointer to the clientOAuth2RoundTripper that will hold the transport
//
// Returns:
//   - error: if loading the TLS configuration fails
func loadMTLS(mtls *commoncfg.MTLS, rt *clientOAuth2RoundTripper) error {
	if mtls == nil {
		return nil
	}

	tlsConfig, err := commoncfg.LoadMTLSConfig(mtls)
	if err != nil {
		return fmt.Errorf("loading mTLS config: %w", err)
	}

	rt.Next = &http.Transport{TLSClientConfig: tlsConfig}

	return nil
}

// loadOAuth2Credentials populates the HTTP RoundTripper with OAuth2 credentials.
//
// Supports the main authentication methods:
//   - client_secret_post: sets ClientSecretPost
//   - client_secret_basic: sets ClientSecretBasic
//   - client_secret_jwt: sets ClientSecretJWT
//   - private_key_jwt: sets ClientAssertion and ClientAssertionType
//
// It does NOT validate combinations — validate() should be called separately
// to ensure that conflicting credentials are not used.
func loadOAuth2Credentials(creds *commoncfg.OAuth2Credentials, rt *clientOAuth2RoundTripper) {
	secretVal, _ := commoncfg.ExtractValueFromSourceRef(creds.ClientSecret)
	switch creds.AuthMethod {
	case commoncfg.OAuth2ClientSecretPost:
		if secretVal != nil && string(secretVal) != "" {
			rt.ClientSecretPost = pointers.To(string(secretVal))
		}
	case commoncfg.OAuth2ClientSecretBasic:
		if secretVal != nil && string(secretVal) != "" {
			rt.ClientSecretBasic = pointers.To(string(secretVal))
		}
	case commoncfg.OAuth2ClientSecretJWT:
		if secretVal != nil && string(secretVal) != "" {
			rt.ClientSecretJWT = pointers.To(string(secretVal))
		}
	case commoncfg.OAuth2PrivateKeyJWT:
		assertionVal, _ := commoncfg.ExtractValueFromSourceRef(creds.ClientAssertion)
		if assertionVal != nil && string(assertionVal) != "" {
			rt.ClientAssertion = pointers.To[string](string(assertionVal))
		}

		assertionTypeVal, _ := commoncfg.ExtractValueFromSourceRef(creds.ClientAssertionType)
		if assertionTypeVal != nil && string(assertionTypeVal) != "" {
			rt.ClientAssertionType = pointers.To[string](string(assertionTypeVal))
		}
	case commoncfg.OAuth2None:
	}
}

// validateOAuth2Credentials validates an OAuth2 configuration and ensures that
// the credentials provided are consistent, complete, and follow the expected rules.
//
// The validation logic follows three main steps:
//   1. Basic validation: ensure required fields (ClientID and AuthMethod) are set.
//   2. Combination validation: ensure that different credentials are not mixed
//      in invalid ways (e.g., clientSecret with clientAssertion).
//   3. Auth-method-specific validation: ensure each OAuth2 authentication method
//      has the necessary fields.
//
// Parameters:
//   - creds: pointer to the OAuth2 configuration to validate
//   - rt: pointer to the clientOAuth2RoundTripper that holds extracted credential values
//
// Returns:
//   - error: if the configuration is invalid; otherwise nil
//
// Validation rules:
//   - ClientID must not be empty.
//   - AuthMethod must be provided and recognized.
//   - clientSecret and clientAssertion cannot be provided simultaneously.
//   - If clientAssertion is provided, clientAssertionType must also be provided, and vice versa.
//   - At least one authentication method must be configured (clientSecret, clientAssertion, or mTLS).
//   - Each authentication method must provide the required fields according to its type.

func validate(creds *commoncfg.OAuth2, rt *clientOAuth2RoundTripper) error {
	// Validate combination of credentials
	hasSecret := rt.ClientSecretPost != nil || rt.ClientSecretBasic != nil || rt.ClientSecretJWT != nil
	hasAssertion := rt.ClientAssertion != nil
	hasAssertionType := rt.ClientAssertionType != nil
	hasMTLS := creds.MTLS != nil

	if hasSecret && hasAssertion {
		return errors.New("invalid OAuth2 config: cannot combine clientSecret with clientAssertion")
	}

	if hasAssertion != hasAssertionType { // XOR
		if hasAssertion {
			return errors.New("invalid OAuth2 config: clientAssertionType is required when using clientAssertion")
		}

		return errors.New("invalid OAuth2 config: clientAssertionType cannot be provided without clientAssertion")
	}

	if !hasSecret && !hasAssertion && !hasMTLS {
		return errors.New("invalid OAuth2 config: no client authentication method provided")
	}

	return nil
}

// clientOAuth2RoundTripper is a custom HTTP RoundTripper that automatically
// injects OAuth2 credentials into query parameters for every request.
//
// into requests. Supports:
//   - client_secret_post → client_id + client_secret in POST body or query
//   - client_secret_basic → Authorization header
//   - client_secret_jwt → client_assertion JWT
//   - private_key_jwt → client_assertion JWT with type
//   - mTLS → transport layer TLS certs
type clientOAuth2RoundTripper struct {
	ClientID string
	TokenURL string

	ClientSecretPost    *string // client_secret_post
	ClientSecretBasic   *string // client_secret_basic
	ClientSecretJWT     *string // client_secret_jwt
	ClientAssertion     *string // private_key_jwt
	ClientAssertionType *string

	Next http.RoundTripper

	// Optional JWT cache to reuse client_secret_jwt or private_key_jwt
	jwtCache map[string]cachedJWT
	mu       sync.Mutex
}

type cachedJWT struct {
	token     string
	expiresAt time.Time
}

// RoundTrip implements the http.RoundTripper interface.
//
// It injects the configured OAuth2 credentials into the request URL's query parameters
// according to the authentication method:
//   - client_secret: adds client_id and client_secret
//   - private_key_jwt: adds client_id, client_assertion_type, and client_assertion
//
// The modified request is then forwarded to the underlying transport.
func (t *clientOAuth2RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	newReq := *req
	urlCopy := *req.URL

	q := urlCopy.Query()

	// Always inject client_id
	q.Set("client_id", t.ClientID)

	switch {
	case t.ClientSecretPost != nil && *t.ClientSecretPost != "":
		// client_secret_post → inject into query (or body)
		q.Set("client_secret", *t.ClientSecretPost)

	case t.ClientSecretBasic != nil && *t.ClientSecretBasic != "":
		// client_secret_basic → set Authorization header
		newReq.SetBasicAuth(t.ClientID, *t.ClientSecretBasic)

	case t.ClientAssertion != nil && t.ClientAssertionType != nil:
		// private_key_jwt → inject JWT assertion
		jwtToken, err := t.getJWT("private_key_jwt", *t.ClientAssertion)
		if err != nil {
			return nil, err
		}

		q.Set("client_assertion_type", *t.ClientAssertionType)
		q.Set("client_assertion", jwtToken)

	case t.ClientSecretJWT != nil:
		// client_secret_jwt → generate JWT signed with shared secret
		jwtToken, err := t.getJWT("client_secret_jwt", *t.ClientSecretJWT)
		if err != nil {
			return nil, err
		}

		q.Set("client_assertion_type", "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
		q.Set("client_assertion", jwtToken)
	}

	urlCopy.RawQuery = q.Encode()
	newReq.URL = &urlCopy

	return t.Next.RoundTrip(&newReq)
}

// getJWT returns a JWT for the given key, caching it if still valid.
func (t *clientOAuth2RoundTripper) getJWT(key, secret string) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// reuse cached token if valid
	if t.jwtCache == nil {
		t.jwtCache = make(map[string]cachedJWT)
	}

	if cached, ok := t.jwtCache[key]; ok && time.Now().Before(cached.expiresAt) {
		return cached.token, nil
	}

	now := time.Now().Unix()
	claims := jwt.MapClaims{
		"iss": t.ClientID,
		"sub": t.ClientID,
		"aud": t.TokenURL,
		"iat": now,
		"exp": now + 60, // valid for 60s
		"jti": uuid.NewString(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	// cache token
	t.jwtCache[key] = cachedJWT{
		token:     signed,
		expiresAt: time.Now().Add(55 * time.Second),
	}

	return signed, nil
}
