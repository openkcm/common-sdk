// Package commongrpc provides helpers for creating and managing gRPC servers and clients,
// with support for keepalive configuration, OpenTelemetry instrumentation, reflection,
// health checks, connection pooling, and secure or insecure transport credentials (including mTLS).
//
// This package is designed to integrate with the common configuration model (commoncfg),
// connection pooling (grpcpool), and OpenTelemetry stats handlers (otlp). It enables easy setup
// of production-ready gRPC servers and clients with best practices for observability,
// connection management, and security.
//
// # Example usage (Server)
//
//	cfg := &commoncfg.GRPCServer{
//	    // ... set fields ...
//	    Flags: commoncfg.GRPCServerFlags{
//	        Reflection: true,
//	        Health:     true,
//	    },
//	}
//	grpcServer := commongrpc.NewServer(context.Background(), cfg)
//	// Register your services on grpcServer
//	grpcServer.Serve(lis)
//
// # Example usage (Client)
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
//
// # Features
//
//   - Configurable keepalive enforcement and server/client parameters
//   - Maximum receive message size (server)
//   - OpenTelemetry stats handler integration
//   - Optional gRPC reflection service (server)
//   - Optional gRPC health service (server)
//   - Connection pooling for clients
//   - Secure (mTLS) and insecure transport credentials
//
// # Functions
//
//   - NewServer: Creates and configures a new gRPC server instance.
//   - NewClient: Creates a single gRPC client connection.
//   - NewPooledClient: Initializes a pooled gRPC client.
//
// # Function Documentation
//
// NewServer creates and configures a new gRPC server instance.
//
// It applies keepalive enforcement and server parameters, maximum receive message size,
// and OpenTelemetry stats handlers. Additional grpc.ServerOption values can be provided.
//
// If reflection is enabled in the config, the server will register the reflection service.
// If health checks are enabled, the server will register the gRPC health service.
//
// Parameters:
//   - ctx: Context for logging and server setup
//   - cfg: Pointer to GRPCServer configuration
//   - serverOptions: Additional grpc.ServerOption values
//
// Returns:
//   - *grpc.Server: The configured gRPC server instance
//
// NewClient creates a single gRPC client connection without pooling.
// It configures transport credentials, keepalive parameters, telemetry
// stats handlers, and applies any custom dial options.
//
// Returns an error if the configuration is invalid or the connection fails.
//
// NewPooledClient initializes a pooled gRPC client based on the provided
// configuration. It applies transport security, keepalive parameters,
// OpenTelemetry stats handlers, and any custom dial options.
//
// The client must implement the PooledClient interface to accept the
// created pool. The function returns an error if the configuration is invalid
// or if the pool cannot be created.

package commongrpc
