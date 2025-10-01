// Package commongrpc provides helpers for creating and managing gRPC clients
// with support for connection pooling, telemetry instrumentation, keepalive
// configuration, and secure or insecure transport credentials (including mTLS).
//
// It is designed to integrate with the common configuration model (commoncfg),
// connection pooling (grpcpool), and OpenTelemetry stats handlers (otlp).
//
// Typical usage:
//
//	cfg := &commoncfg.GRPCClient{
//	    Address: "localhost:50051",
//	    Pool: commoncfg.GRPCPool{
//	        InitialCapacity: 2,
//	        MaxCapacity:     10,
//	    },
//	    Attributes: commoncfg.GRPCKeepalive{
//	        KeepaliveTime:    10 * time.Second,
//	        KeepaliveTimeout: 5 * time.Second,
//	    },
//	    SecretRef: &commoncfg.SecretRef{
//	        Type: commoncfg.MTLSSecretType,
//	        MTLS: commoncfg.MTLSSecret{ /* paths to certs */ },
//	    },
//	}
//
//	// Using a pooled client
//	myClient := NewMyServiceClient()
//	if err := commongrpc.NewPooledClient(myClient, cfg); err != nil {
//	    log.Fatalf("failed to init client: %v", err)
//	}
//
//	// Or using a single client connection
//	conn, err := commongrpc.NewClient(cfg)
//	if err != nil {
//	    log.Fatalf("failed to dial: %v", err)
//	}
//	defer conn.Close()
package commongrpc

import (
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/grpcpool"
	"github.com/openkcm/common-sdk/pkg/otlp"
)

var (
	// ErrEmptyAddress is returned when the gRPC client configuration
	// does not specify a target address.
	ErrEmptyAddress = errors.New("grpc address is empty")

	// ErrUnsupportedSecretType is returned when a provided SecretRef
	// type is not supported for creating transport credentials.
	ErrUnsupportedSecretType = errors.New("unsupported secret type")
)

// PooledClient is an interface that clients must implement to support
// connection pooling. The SetPool method is called by NewPooledClient
// with a configured grpcpool.Pool.
type PooledClient interface {
	SetPool(pool *grpcpool.Pool)
}

// NewPooledClient initializes a pooled gRPC client based on the provided
// configuration. It applies transport security, keepalive parameters,
// OpenTelemetry stats handlers, and any custom dial options.
//
// The client must implement the PooledClient interface to accept the
// created pool. The function returns an error if the configuration is invalid
// or if the pool cannot be created.
//
// Example:
//
//	err := commongrpc.NewPooledClient(myClient, cfg,
//	    grpc.WithBlock(),
//	)
func NewPooledClient(client PooledClient, cfg *commoncfg.GRPCClient, dialOptions ...grpc.DialOption) error {
	if cfg.Address == "" {
		return ErrEmptyAddress
	}

	creds, err := computeTransportCredentials(cfg)
	if err != nil {
		return err
	}

	opts := make([]grpc.DialOption, 0)
	opts = append(opts,
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    cfg.Attributes.KeepaliveTime,
			Timeout: cfg.Attributes.KeepaliveTimeout,
		}),
		grpc.WithStatsHandler(otlp.NewClientHandler()),
		grpc.WithTransportCredentials(creds),
	)
	opts = append(opts, dialOptions...)

	clientPool, err := grpcpool.New(
		createFactory(cfg.Address, opts...),
		grpcpool.WithInitialCapacity(cfg.Pool.InitialCapacity),
		grpcpool.WithMaxCapacity(cfg.Pool.MaxCapacity),
		grpcpool.WithIdleTimeout(cfg.Pool.IdleTimeout),
		grpcpool.WithMaxLifeDuration(cfg.Pool.MaxLifeDuration),
	)
	if err != nil {
		return err
	}

	client.SetPool(clientPool)

	return nil
}

// createFactory returns a grpcpool.ClientFactory function that dials
// a new gRPC ClientConn using the given address and dial options.
//
// Parameters:
//   - address: the gRPC server address to connect to
//   - dialOptions: additional grpc.DialOption values to use when dialing
//
// Returns:
//   - grpcpool.ClientFactory: a function that creates new gRPC ClientConn instances
func createFactory(address string, dialOptions ...grpc.DialOption) grpcpool.ClientFactory {
	return func() (*grpc.ClientConn, error) {
		return grpc.NewClient(address, dialOptions...)
	}
}

// NewClient creates a single gRPC client connection without pooling.
// It configures transport credentials, keepalive parameters, telemetry
// stats handlers, and applies any custom dial options.
//
// Returns an error if the configuration is invalid or the connection fails.
//
// Example:
//
//	conn, err := commongrpc.NewClient(cfg, grpc.WithBlock())
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer conn.Close()
func NewClient(cfg *commoncfg.GRPCClient, dialOptions ...grpc.DialOption) (*grpc.ClientConn, error) {
	if cfg.Address == "" {
		return nil, ErrEmptyAddress
	}

	creds, err := computeTransportCredentials(cfg)
	if err != nil {
		return nil, err
	}

	opts := make([]grpc.DialOption, 0)
	opts = append(opts,
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    cfg.Attributes.KeepaliveTime,
			Timeout: cfg.Attributes.KeepaliveTimeout,
		}),
		grpc.WithStatsHandler(otlp.NewClientHandler()),
		grpc.WithTransportCredentials(creds),
	)
	opts = append(opts, dialOptions...)

	return grpc.NewClient(cfg.Address, opts...)
}

// computeTransportCredentials determines the appropriate gRPC
// transport credentials based on the provided client configuration.
//
// Supported secret types are:
//   - InsecureSecretType: uses insecure transport credentials.
//   - MTLSSecretType: loads mutual TLS configuration from secret refs.
//
// Returns ErrUnsupportedSecretType if the type is not recognized.
func computeTransportCredentials(cfg *commoncfg.GRPCClient) (credentials.TransportCredentials, error) {
	creds := insecure.NewCredentials()

	if cfg.SecretRef == nil {
		return creds, nil
	}

	switch cfg.SecretRef.Type {
	case commoncfg.InsecureSecretType:
		creds = insecure.NewCredentials()
	case commoncfg.MTLSSecretType:
		tlsConfig, err := commoncfg.LoadMTLSConfig(&cfg.SecretRef.MTLS)
		if err != nil {
			return nil, err
		}
		creds = credentials.NewTLS(tlsConfig)
	default:
		return nil, ErrUnsupportedSecretType
	}

	return creds, nil
}
