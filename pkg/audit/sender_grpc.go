package audit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/plog/plogotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/encoding/gzip"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/utils"
)

const (
	// defaultRequestTimeout is the default timeout for a single gRPC request.
	defaultRequestTimeout = 30 * time.Second

	// otlpLogsService is the gRPC service name for OTLP logs.
	otlpLogsService = "opentelemetry.proto.collector.logs.v1.LogsService"
)

// ConnectionState represents the current state of the gRPC connection.
type ConnectionState int

const (
	// ConnectionStateIdle indicates the connection is idle.
	ConnectionStateIdle ConnectionState = iota
	// ConnectionStateConnecting indicates the connection is being established.
	ConnectionStateConnecting
	// ConnectionStateReady indicates the connection is ready for use.
	ConnectionStateReady
	// ConnectionStateTransientFailure indicates the connection has experienced a transient failure.
	ConnectionStateTransientFailure
	// ConnectionStateShutdown indicates the connection is shutdown.
	ConnectionStateShutdown
)

// String returns the string representation of the connection state.
func (s ConnectionState) String() string {
	switch s {
	case ConnectionStateIdle:
		return "IDLE"
	case ConnectionStateConnecting:
		return "CONNECTING"
	case ConnectionStateReady:
		return "READY"
	case ConnectionStateTransientFailure:
		return "TRANSIENT_FAILURE"
	case ConnectionStateShutdown:
		return "SHUTDOWN"
	default:
		return "UNKNOWN"
	}
}

// ConnectionStateCallback is called when the gRPC connection state changes.
type ConnectionStateCallback func(state ConnectionState)

// grpcSenderConfig holds configuration for the gRPC sender.
type grpcSenderConfig struct {
	requestTimeout time.Duration
	compression    bool
	onStateChange  ConnectionStateCallback
}

// grpcSender implements Sender using gRPC transport.
// Unlike the HTTP sender, this sender does NOT do internal batching.
// Batching is handled at the Auditor level for gRPC to properly integrate
// with retry semantics.
type grpcSender struct {
	conn           *grpc.ClientConn
	client         plogotlp.GRPCClient
	metadata       metadata.MD
	requestTimeout time.Duration

	// Connection monitoring
	onStateChange ConnectionStateCallback
	stopMonitor   context.CancelFunc
}

// GRPCSenderOption configures a grpcSender.
type GRPCSenderOption func(*grpcSenderConfig)

// WithRequestTimeout sets the timeout for individual gRPC requests.
// Values <= 0 are ignored. Defaults to 30s.
func WithRequestTimeout(d time.Duration) GRPCSenderOption {
	return func(c *grpcSenderConfig) {
		if d > 0 {
			c.requestTimeout = d
		}
	}
}

// WithCompression enables gzip compression for gRPC requests.
func WithCompression(enabled bool) GRPCSenderOption {
	return func(c *grpcSenderConfig) {
		c.compression = enabled
	}
}

// WithConnectionStateCallback sets a callback to be invoked when connection state changes.
func WithConnectionStateCallback(cb ConnectionStateCallback) GRPCSenderOption {
	return func(c *grpcSenderConfig) {
		c.onStateChange = cb
	}
}

// newGRPCSender creates a new gRPC-based sender.
func newGRPCSender(cfg *commoncfg.Audit, opts ...GRPCSenderOption) (*grpcSender, error) {
	if cfg.GRPCClient == nil {
		return nil, errors.New("audit: grpcClient config is required for gRPC protocol")
	}

	// Apply default config
	senderCfg := &grpcSenderConfig{
		requestTimeout: defaultRequestTimeout,
		compression:    true, // Enable compression by default
	}

	// Apply options
	for _, opt := range opts {
		opt(senderCfg)
	}

	dialOpts, md, err := buildGRPCDialOptions(cfg, senderCfg.compression)
	if err != nil {
		return nil, fmt.Errorf("audit: failed to build gRPC dial options: %w", err)
	}

	conn, err := grpc.NewClient(cfg.Endpoint, dialOpts...)
	if err != nil {
		return nil, fmt.Errorf("audit: failed to create gRPC client: %w", err)
	}

	s := &grpcSender{
		conn:           conn,
		client:         plogotlp.NewGRPCClient(conn),
		metadata:       md,
		requestTimeout: senderCfg.requestTimeout,
		onStateChange:  senderCfg.onStateChange,
	}

	// Start connection state monitoring if callback is provided
	if senderCfg.onStateChange != nil {
		ctx, cancel := context.WithCancel(context.Background())
		s.stopMonitor = cancel

		go s.monitorConnectionState(ctx)
	}

	return s, nil
}

// buildGRPCDialOptions constructs gRPC dial options from config.
// Returns dial options and optional metadata for auth headers.
func buildGRPCDialOptions(cfg *commoncfg.Audit, compression bool) ([]grpc.DialOption, metadata.MD, error) {
	grpcCfg := cfg.GRPCClient
	opts := make([]grpc.DialOption, 0, 6)

	var md metadata.MD

	// Configure keepalive
	opts = append(opts, grpc.WithKeepaliveParams(keepalive.ClientParameters{
		Time:    grpcCfg.Attributes.KeepaliveTime,
		Timeout: grpcCfg.Attributes.KeepaliveTimeout,
	}))

	// Configure compression
	if compression {
		opts = append(opts, grpc.WithDefaultCallOptions(grpc.UseCompressor(gzip.Name)))
	}

	// Configure retry policy via service config
	serviceConfig := buildServiceConfig(grpcCfg.Attributes.Retry)
	opts = append(opts, grpc.WithDefaultServiceConfig(serviceConfig))

	// Configure credentials/security
	if grpcCfg.SecretRef != nil {
		switch grpcCfg.SecretRef.Type {
		case commoncfg.MTLSSecretType:
			tlsConfig, err := commoncfg.LoadMTLSConfig(&grpcCfg.SecretRef.MTLS)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to load mTLS config: %w", err)
			}

			opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig)))

		case commoncfg.BasicSecretType:
			username, err := commoncfg.ExtractValueFromSourceRef(&grpcCfg.SecretRef.Basic.Username)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to extract basic auth username: %w", err)
			}

			password, err := commoncfg.ExtractValueFromSourceRef(&grpcCfg.SecretRef.Basic.Password)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to extract basic auth password: %w", err)
			}

			md = metadata.Pairs("Authorization", "Basic "+utils.BasicAuth(string(username), string(password)))

			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

		case commoncfg.InsecureSecretType:
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

		default:
			return nil, nil, fmt.Errorf("unsupported secret type for gRPC: %s", grpcCfg.SecretRef.Type)
		}
	} else {
		// Default to insecure if no secretRef is provided
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	return opts, md, nil
}

// buildServiceConfig creates a gRPC service config JSON with retry policy.
func buildServiceConfig(retry commoncfg.GRPCRetry) string {
	// Apply defaults for zero values
	maxAttempts := retry.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}

	initialBackoff := retry.InitialBackoff
	if initialBackoff <= 0 {
		initialBackoff = 100 * time.Millisecond
	}

	maxBackoff := retry.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = 1 * time.Second
	}

	backoffMultiplier := retry.BackoffMultiplier
	if backoffMultiplier < 1 {
		backoffMultiplier = 2
	}

	// Build retryable status codes array
	statusCodes := retry.RetryableStatusCodes
	if len(statusCodes) == 0 {
		statusCodes = []string{"UNAVAILABLE", "RESOURCE_EXHAUSTED"}
	}

	config := map[string]any{
		"methodConfig": []map[string]any{
			{
				"name": []map[string]string{
					{"service": otlpLogsService},
				},
				"retryPolicy": map[string]any{
					"maxAttempts":          maxAttempts,
					"initialBackoff":       fmt.Sprintf("%.3fs", initialBackoff.Seconds()),
					"maxBackoff":           fmt.Sprintf("%.3fs", maxBackoff.Seconds()),
					"backoffMultiplier":    float64(backoffMultiplier),
					"retryableStatusCodes": statusCodes,
				},
			},
		},
	}

	data, err := json.Marshal(config)
	if err != nil {
		// This should never happen with the fixed structure above,
		// but return empty config to satisfy the linter
		return "{}"
	}

	return string(data)
}

// Send transmits logs via gRPC using the OTLP logs service.
func (s *grpcSender) Send(ctx context.Context, logs plog.Logs) error {
	// Ensure we have a timeout
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(ctx, s.requestTimeout)
		defer cancel()
	}

	// Add metadata to context if present (for auth headers)
	if s.metadata != nil {
		ctx = metadata.NewOutgoingContext(ctx, s.metadata)
	}

	// Create export request from plog.Logs
	req := plogotlp.NewExportRequestFromLogs(logs)

	// Send the request
	_, err := s.client.Export(ctx, req)
	if err != nil {
		return s.classifyError(err)
	}

	return nil
}

// classifyError maps gRPC status codes to permanent vs transient errors.
func (s *grpcSender) classifyError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		// Not a gRPC status error, treat as transient
		return fmt.Errorf("audit: gRPC request failed: %w", err)
	}

	// Classify based on gRPC status code
	switch st.Code() {
	case codes.InvalidArgument,
		codes.NotFound,
		codes.AlreadyExists,
		codes.PermissionDenied,
		codes.Unauthenticated,
		codes.FailedPrecondition,
		codes.Unimplemented:
		// These are permanent errors - do not retry
		return fmt.Errorf("%w: gRPC %s: %s", ErrPermanent, st.Code(), st.Message())

	case codes.Unavailable,
		codes.DeadlineExceeded,
		codes.ResourceExhausted,
		codes.Aborted,
		codes.Internal,
		codes.Unknown:
		// These are transient errors - will retry
		return fmt.Errorf("audit: transient gRPC error %s: %s", st.Code(), st.Message())

	case codes.Canceled:
		// Context canceled, likely shutdown - treat as transient but won't retry anyway
		return fmt.Errorf("audit: gRPC request canceled: %w", err)

	default:
		// Unknown status code, treat as transient
		return fmt.Errorf("audit: gRPC error %s: %s", st.Code(), st.Message())
	}
}

// monitorConnectionState monitors the gRPC connection state and invokes the callback on changes.
func (s *grpcSender) monitorConnectionState(ctx context.Context) {
	lastState := s.conn.GetState()
	s.onStateChange(toConnectionState(lastState))

	for {
		// Wait for state change or context cancellation
		if !s.conn.WaitForStateChange(ctx, lastState) {
			// Context was cancelled
			return
		}

		newState := s.conn.GetState()
		if newState != lastState {
			lastState = newState
			s.onStateChange(toConnectionState(newState))
		}
	}
}

// toConnectionState converts gRPC connectivity.State to ConnectionState.
func toConnectionState(s connectivity.State) ConnectionState {
	switch s {
	case connectivity.Idle:
		return ConnectionStateIdle
	case connectivity.Connecting:
		return ConnectionStateConnecting
	case connectivity.Ready:
		return ConnectionStateReady
	case connectivity.TransientFailure:
		return ConnectionStateTransientFailure
	case connectivity.Shutdown:
		return ConnectionStateShutdown
	default:
		return ConnectionStateIdle
	}
}

// ConnectionState returns the current gRPC connection state.
func (s *grpcSender) ConnectionState() ConnectionState {
	return toConnectionState(s.conn.GetState())
}

// Close closes the underlying gRPC connection.
func (s *grpcSender) Close() error {
	// Stop connection monitoring
	if s.stopMonitor != nil {
		s.stopMonitor()
	}

	if s.conn != nil {
		return s.conn.Close()
	}

	return nil
}
