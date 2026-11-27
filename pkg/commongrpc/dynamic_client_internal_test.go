package commongrpc

import (
	"crypto/tls"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/credentials"
)

// TestAuthorityTLSCredentialsClone checks that a clone, which gRPC may make of
// transport credentials, keeps the same TLS configuration, and with it the
// reloaded credentials and verification.
func TestAuthorityTLSCredentialsClone(t *testing.T) {
	t.Parallel()

	cfg := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: "localhost"}
	creds := &authorityTLSCredentials{TransportCredentials: credentials.NewTLS(cfg), config: cfg}

	clone, ok := creds.Clone().(*authorityTLSCredentials)
	require.True(t, ok, "the clone must be authorityTLSCredentials")

	assert.NotSame(t, creds, clone)
	assert.Same(t, cfg, clone.config)
	assert.Equal(t, creds.Info(), clone.Info())
}
