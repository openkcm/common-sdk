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

// NewClientFromOAuth2 creates a new HTTP client configured with OAuth2 credentials
// and optional mutual TLS (mTLS) transport.
//
// This function prepares an *http.Client that automatically injects OAuth2 credentials
// into outgoing requests using a custom RoundTripper. The client can use multiple
// OAuth2 authentication methods and optionally mTLS.
//
// Supported authentication methods:
//   - post (client_secret_post): injects "client_id" and "client_secret" into the
//     request query parameters (or POST body, depending on usage).
//   - basic (client_secret_basic): sets the HTTP Basic Authorization header with
//     clientID and clientSecret.
//   - jwt (client_secret_jwt): generates a JWT signed with a shared secret, injected
//     as "client_assertion" with type "urn:ietf:params:oauth:client-assertion-type:jwt-bearer".
//   - private (private_key_jwt): uses a JWT assertion provided in ClientAssertion along
//     with ClientAssertionType, injected as query parameters.
//   - none: PKCE flow (no client_secret required)
//
// Only one authentication method may be configured at a time. If multiple conflicting
// credentials are provided, this function returns an error.
//
// If mTLS configuration is provided, the client's transport will use the specified
// TLS certificates for client authentication.
//
// Parameters:
//   - clientAuth: pointer to an OAuth2 configuration containing credentials, optional mTLS,
//     and the authentication method to use.
//
// Returns:
//   - *http.Client: an HTTP client that automatically applies the specified OAuth2 credentials
//     and mTLS configuration to requests.
//   - error: if the configuration is invalid, required fields are missing, or mTLS loading fails.
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

	// error has been ignored intentionally as value might be not present
	if clientAuth.URL != nil {
		tokenURL, err := commoncfg.ExtractValueFromSourceRef(clientAuth.URL)
		if err != nil {
			return nil, fmt.Errorf("OAuth2 credentials missing URL: %w", err)
		}
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

// loadMTLS configures the HTTP transport to use mutual TLS (mTLS) for a given
// clientOAuth2RoundTripper.
//
// This function reads the certificate, key, and optional CA configuration from
// the provided MTLS configuration and sets up a tls.Config for the RoundTripper's
// underlying transport.
//
// Parameters:
//   - mtls: pointer to an MTLS configuration containing paths or sources for
//     client certificate, client key, and server CA. If nil, no mTLS
//     configuration is applied.
//   - rt: pointer to a clientOAuth2RoundTripper that will have its transport
//     wrapped with the TLS configuration.
//
// Returns:
//   - error: if loading the certificates or creating the TLS configuration fails.
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

// loadOAuth2Credentials populates the clientOAuth2RoundTripper with OAuth2 credentials.
//
// This function extracts values from the provided OAuth2 configuration and
// sets the corresponding fields in the round-tripper for use in HTTP requests.
//
// Supported authentication methods:
//   - post: sets ClientSecretPost for client_secret_post authentication.
//   - basic: sets ClientSecretBasic for client_secret_basic authentication.
//   - jwt: sets ClientSecretJWT for client_secret_jwt authentication.
//   - private: sets ClientAssertion and ClientAssertionType for private_key_jwt authentication.
//   - none: no credentials are set (PKCE or unauthenticated flow).
//
// Notes:
//   - This function does NOT perform validation of combinations; call validate()
//     separately to ensure the configuration is consistent.
//   - Extracts the actual values from SourceRef fields, such as environment variables,
//     files, or plain values, using commoncfg.ExtractValueFromSourceRef.
//
// Parameters:
//   - creds: pointer to the OAuth2Credentials containing the source references
//     and authentication method type.
//   - rt: pointer to the clientOAuth2RoundTripper to populate with extracted credential values.
func loadOAuth2Credentials(creds *commoncfg.OAuth2Credentials, rt *clientOAuth2RoundTripper) {
	secretVal, _ := commoncfg.ExtractValueFromSourceRef(creds.ClientSecret)
	if secretVal != nil && string(secretVal) != "" {
		switch creds.AuthMethod {
		case commoncfg.OAuth2ClientSecretPost:
			rt.ClientSecretPost = pointers.To(string(secretVal))
		case commoncfg.OAuth2ClientSecretBasic:
			rt.ClientSecretBasic = pointers.To(string(secretVal))
		case commoncfg.OAuth2ClientSecretJWT:
			rt.ClientSecretJWT = pointers.To(string(secretVal))
		}
	}
	switch creds.AuthMethod {
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

// validate checks the consistency and completeness of an OAuth2 configuration.
//
// It ensures that the provided credentials follow expected rules and do not conflict.
//
// The validation performs three main types of checks:
//  1. Combination validation: ensures that different credential types are not mixed
//     in invalid ways (e.g., clientSecret combined with clientAssertion).
//  2. Field presence validation: ensures that required fields are present when a
//     particular authentication method is used.
//  3. Authentication method availability: ensures that at least one valid
//     authentication method (clientSecret, clientAssertion, or mTLS) is configured.
//
// Parameters:
//   - creds: pointer to the OAuth2 configuration to validate.
//   - rt: pointer to the clientOAuth2RoundTripper holding extracted credential values.
//
// Returns:
//   - error: descriptive error if the configuration is invalid; otherwise nil.
//
// Validation rules:
//   - clientSecret (post, basic, or jwt) and clientAssertion cannot be used together.
//   - If clientAssertion is provided, clientAssertionType must also be provided, and vice versa.
//   - At least one authentication method must be configured: clientSecret, clientAssertion, or mTLS.
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
// injects OAuth2 credentials into HTTP requests.
//
// It supports multiple authentication methods:
//   - post → client_id + client_secret in POST body or query parameters
//   - basic → Authorization header
//   - jwt → client_assertion JWT signed with HMAC(secret)
//   - private → client_assertion JWT signed with private key
//   - mTLS → transport layer TLS certificates
type clientOAuth2RoundTripper struct {
	// ClientID is the OAuth2 client ID used in all authentication methods.
	ClientID string

	// TokenURL is the token endpoint URL used as the "aud" claim when generating JWTs.
	TokenURL string

	// ClientSecretPost is used for client_secret_post authentication.
	// If set, client_id + client_secret are sent in the POST body or query parameters.
	ClientSecretPost *string

	// ClientSecretBasic is used for client_secret_basic authentication.
	// If set, client_id + client_secret are sent in the HTTP Basic Auth header.
	ClientSecretBasic *string

	// ClientSecretJWT is used for client_secret_jwt authentication.
	// If set, a JWT is signed with this secret and sent as client_assertion.
	ClientSecretJWT *string

	// ClientAssertion is used for private_key_jwt authentication.
	// Contains the JWT assertion string signed with a private key.
	ClientAssertion *string

	// ClientAssertionType is the type of JWT assertion (e.g., "urn:ietf:params:oauth:client-assertion-type:jwt-bearer")
	// Required if ClientAssertion is set.
	ClientAssertionType *string

	// Next is the underlying HTTP RoundTripper to which the requests are forwarded after injecting credentials.
	Next http.RoundTripper

	// jwtCache is an optional in-memory cache of JWTs to avoid regenerating tokens for repeated requests.
	jwtCache map[string]cachedJWT

	// mu protects access to the jwtCache map in concurrent requests.
	mu sync.Mutex
}

type cachedJWT struct {
	token     string
	expiresAt time.Time
}

// RoundTrip implements the http.RoundTripper interface for clientOAuth2RoundTripper.
//
// It automatically injects OAuth2 credentials into outgoing HTTP requests according
// to the configured authentication method, then forwards the request to the underlying transport.
//
// Supported authentication methods:
//   - post (client_secret_post): injects "client_id" and "client_secret" into the
//     request query parameters (or POST body, depending on usage).
//   - basic (client_secret_basic): sets the HTTP Basic Authorization header with
//     clientID and clientSecret.
//   - jwt (client_secret_jwt): generates a JWT signed with a shared secret, injected
//     as "client_assertion" with type "urn:ietf:params:oauth:client-assertion-type:jwt-bearer".
//   - private (private_key_jwt): uses a JWT assertion provided in ClientAssertion along
//     with ClientAssertionType, injected as query parameters.
//   - mTLS: handled separately via TLS transport configuration.
//
// Behavior:
//   - Always injects the "client_id" parameter.
//   - Chooses the authentication method based on which credentials are set.
//   - If multiple methods are set incorrectly, behavior is undefined (validation
//     should catch conflicts before usage).
//   - For JWT methods, caches tokens for reuse until expiration.
//
// Parameters:
//   - req: the outgoing HTTP request.
//
// Returns:
//   - *http.Response: the response from the underlying transport.
//   - error: if credential injection fails or JWT generation fails.
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

// getJWT generates or retrieves a cached JWT for the specified key and secret.
//
// This method is used internally by clientOAuth2RoundTripper to provide
// JWT-based authentication for both `client_secret_jwt` and `private_key_jwt`
// OAuth2 flows.
//
// Behavior:
//   - If a valid cached JWT exists for the given key, it is returned directly.
//   - Otherwise, a new JWT is generated, signed with the provided secret, and cached.
//   - The JWT contains standard claims:
//   - "iss" (issuer): the client ID
//   - "sub" (subject): the client ID
//   - "aud" (audience): the TokenURL
//   - "iat" (issued at): current Unix timestamp
//   - "exp" (expiration): current Unix timestamp + 60 seconds
//   - "jti" (JWT ID): a random UUID
//
// Parameters:
//   - key: a unique identifier for the JWT type (e.g., "client_secret_jwt" or "private_key_jwt")
//   - secret: the shared secret or private key used to sign the JWT
//
// Returns:
//   - string: the signed JWT
//   - error: if signing the JWT fails
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
