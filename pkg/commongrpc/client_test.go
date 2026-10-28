package commongrpc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commongrpc"
	"github.com/openkcm/common-sdk/pkg/grpcpool"
)

// pooledClient is the kind of client NewPooledClient fills: it keeps the
// pool it is given.
type pooledClient struct {
	pool *grpcpool.Pool
}

func (c *pooledClient) SetPool(pool *grpcpool.Pool) {
	c.pool = pool
}

func TestNewPooledClient(t *testing.T) {
	t.Parallel()

	f := newFixture(t)
	srv := f.startServer(t, serverOptions{})

	cfg := f.clientConfig(srv.addr, nil)
	cfg.Pool = commoncfg.GRPCPool{InitialCapacity: 1, MaxCapacity: 2}

	client := &pooledClient{}
	require.NoError(t, commongrpc.NewPooledClient(client, cfg, fastReconnect()))
	require.NotNil(t, client.pool)
	t.Cleanup(func() { _ = client.pool.Close() })

	// Two connections at once, one created up front and one on demand.
	for range 2 {
		conn, err := client.pool.Get(t.Context())
		require.NoError(t, err)

		t.Cleanup(func() { _ = conn.Close() })

		cn, err := clientCN(t.Context(), healthpb.NewHealthClient(conn.ClientConn))
		require.NoError(t, err)
		assert.Equal(t, "client-v1", cn)
	}
}

func TestNewPooledClientErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cfg     func(f *fixture) *commoncfg.GRPCClient
		wantErr error
	}{
		{
			name: "empty address",
			cfg: func(*fixture) *commoncfg.GRPCClient {
				return &commoncfg.GRPCClient{}
			},
			wantErr: commongrpc.ErrEmptyAddress,
		},
		{
			name: "unsupported secret type",
			cfg: func(*fixture) *commoncfg.GRPCClient {
				return &commoncfg.GRPCClient{
					Address:   "localhost:12345",
					SecretRef: &commoncfg.SecretRef{Type: "unknown"},
				}
			},
			wantErr: commongrpc.ErrUnsupportedSecretType,
		},
		{
			name: "credentials that cannot be loaded",
			cfg: func(f *fixture) *commoncfg.GRPCClient {
				cfg := f.clientConfig("localhost:12345", nil)
				cfg.SecretRef.MTLS.CertKey = fileRef(f.path("missing.key"))

				return cfg
			},
		},
		{
			name: "initial capacity above the maximum",
			cfg: func(*fixture) *commoncfg.GRPCClient {
				return &commoncfg.GRPCClient{
					Address: "localhost:12345",
					Pool:    commoncfg.GRPCPool{InitialCapacity: 3, MaxCapacity: 1},
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := &pooledClient{}

			err := commongrpc.NewPooledClient(client, tt.cfg(newFixture(t)))
			require.Error(t, err)
			assert.Nil(t, client.pool, "no pool may be set on failure")

			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			}
		})
	}
}

func TestNewClientErrors(t *testing.T) {
	t.Parallel()

	f := newFixture(t)

	cfg := f.clientConfig("localhost:12345", nil)
	cfg.SecretRef.MTLS.CertKey = fileRef(f.path("missing.key"))

	conn, err := commongrpc.NewClient(cfg)
	require.Error(t, err)
	assert.Nil(t, conn)

	conn, err = commongrpc.NewClient(&commoncfg.GRPCClient{})
	require.ErrorIs(t, err, commongrpc.ErrEmptyAddress)
	assert.Nil(t, conn)
}

func TestNewServerRegistersOptionalServices(t *testing.T) {
	t.Parallel()

	const (
		reflectionService = "grpc.reflection.v1.ServerReflection"
		healthService     = "grpc.health.v1.Health"
	)

	tests := []struct {
		name           string
		flags          commoncfg.Flags
		wantReflection bool
		wantHealth     bool
	}{
		{name: "no optional service"},
		{name: "reflection", flags: commoncfg.Flags{Reflection: true}, wantReflection: true},
		{name: "health", flags: commoncfg.Flags{Health: true}, wantHealth: true},
		{name: "both", flags: commoncfg.Flags{Reflection: true, Health: true}, wantReflection: true, wantHealth: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			srv := commongrpc.NewServer(t.Context(), &commoncfg.GRPCServer{Flags: tt.flags, MaxRecvMsgSize: 4 << 20})
			t.Cleanup(srv.Stop)

			services := srv.GetServiceInfo()

			_, hasReflection := services[reflectionService]
			_, hasHealth := services[healthService]

			assert.Equal(t, tt.wantReflection, hasReflection, "reflection service registered")
			assert.Equal(t, tt.wantHealth, hasHealth, "health service registered")
		})
	}
}
