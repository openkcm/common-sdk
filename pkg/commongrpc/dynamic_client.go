// Package commongrpc provides helpers for creating and managing gRPC clients with
// support for connection pooling, telemetry, and secure mTLS credential management.
//
// One of the core features of this package is the ability to automatically refresh
// gRPC client connections when underlying TLS certificates or keys are updated on disk.
// This enables long-lived clients to handle certificate rotation without downtime.
//
// Typical usage:
//
//	cfg := &commoncfg.GRPCClient{
//	    Address: "my-grpc-server:443",
//	    SecretRef: &commoncfg.SecretRef{
//	        .....
//	    },
//	}
//
//	client, err := commongrpc.NewDynamicClientConn(cfg, 2*time.Second)
//	if err != nil {
//	    log.Fatalf("failed to create grpc client: %v", err)
//	}
//	defer client.Close()
//
//	grpcConn := client.ClientConn
//	myService := pb.NewMyServiceClient(grpcConn)
package commongrpc

import (
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"google.golang.org/grpc"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commonfs/notifier"
)

// DynamicClientConn represents a gRPC client connection that automatically
// refreshes itself when TLS certificate or key files change on disk.
//
// This is particularly useful for setups where mTLS secrets are rotated
// frequently (e.g., by a secret manager or service mesh) and the client must
// always maintain a valid secure connection.
//
// DynamicClientConn uses fsnotify to watch for file changes in the configured
// certificate/key/CA paths. When changes are detected, the client connection is
// torn down and recreated with the latest credentials.
type DynamicClientConn struct {
	*grpc.ClientConn

	mu          sync.RWMutex
	cfg         *commoncfg.GRPCClient
	dialOptions []grpc.DialOption

	notifier *notifier.Notifier
}

// NewDynamicClientConn creates a new DynamicClientConn for the given gRPC client
// configuration. If a SecretRef is configured, the client will automatically
// watch the certificate, key, and CA file paths for changes. When a change is
// detected, the gRPC connection is refreshed in place with the latest credentials.
//
// The throttleInterval parameter defines how frequently refresh events can be
// triggered. For example, a throttle interval of 2 seconds ensures that multiple
// rapid file system events (e.g., a cert + key update) result in only one
// reconnection attempt.
//
// If no SecretRef is provided, the client behaves like a normal static gRPC
// client connection.
//
// Example:
//
//	client, err := commongrpc.NewDynamicClientConn(cfg, 2*time.Second)
//	if err != nil {
//	    return err
//	}
//	defer client.Close()
func NewDynamicClientConn(cfg *commoncfg.GRPCClient, throttleInterval time.Duration, dialOptions ...grpc.DialOption) (*DynamicClientConn, error) {
	rc := &DynamicClientConn{
		mu:          sync.RWMutex{},
		cfg:         cfg,
		dialOptions: dialOptions,
	}

	if cfg.SecretRef != nil && cfg.SecretRef.Type == commoncfg.MTLSSecretType {
		err := rc.initAndStartNotifierOnMTLSCredentials(throttleInterval, &cfg.SecretRef.MTLS)
		if err != nil {
			return nil, err
		}
	}

	err := rc.refreshGRPCClientConn()
	if err != nil {
		if rc.notifier != nil {
			_ = rc.notifier.Close()
		}

		return nil, err
	}

	return rc, nil
}

// initAndStartNotifierOnMTLSCredentials initializes and starts a file system notifier
// for the mTLS certificate, key, and optionally the CA files. The notifier triggers
// events whenever these files change, allowing dynamic refresh of the gRPC client credentials.
//
// Parameters:
//   - throttleInterval: the minimum duration between consecutive notifier callback invocations.
//   - mtls: the MTLS configuration containing paths to the client certificate, key, and server CA.
//
// Returns:
//   - error: non-nil if the notifier could not be created or started.
//
// Behavior:
//  1. Collects the directories containing the client certificate, key, and server CA (if provided).
//     Only the directories are monitored, not the individual files directly.
//  2. Creates a new notifier instance with:
//     - monitored paths as collected above
//     - an event handler that triggers DynamicClientConn refresh
//     - the specified throttle interval
//     - zero burst number (only rate limiting based on throttle interval)
//  3. Starts the notifier and stores it in the DynamicClientConn instance.
//  4. Returns any errors encountered during creation or startup of the notifier.
func (dcc *DynamicClientConn) initAndStartNotifierOnMTLSCredentials(throttleInterval time.Duration, mtls *commoncfg.MTLS) error {
	var paths []string

	certPath := strings.TrimSpace(mtls.Cert.File.Path)
	if certPath != "" {
		paths = append(paths, filepath.Dir(certPath))
	}

	keyPath := strings.TrimSpace(mtls.CertKey.File.Path)
	if keyPath != "" {
		paths = append(paths, filepath.Dir(keyPath))
	}

	caPath := strings.TrimSpace(mtls.ServerCA.File.Path)
	if caPath != "" {
		paths = append(paths, filepath.Dir(caPath))
	}

	nt, err := notifier.Create(
		notifier.OnPaths(paths...),
		notifier.WithEventHandler(dcc.eventHandler),
		notifier.WithThrottleInterval(throttleInterval),
		notifier.WithBurstNumber(0),
	)
	if err != nil {
		return err
	}

	err = nt.Start()
	if err != nil {
		return err
	}

	dcc.notifier = nt

	return nil
}

// Close stops the file watcher (if active) and closes the underlying
// gRPC client connection. After calling Close, the DynamicClientConn
// must not be reused.
func (dcc *DynamicClientConn) Close() error {
	if dcc.notifier == nil {
		return nil
	}

	defer func() {
		dcc.notifier = nil
	}()

	_ = dcc.notifier.Close()

	dcc.mu.Lock()
	defer dcc.mu.Unlock()

	return dcc.ClientConn.Close()
}

// IsClientConnNil returns true if the underlying ClientConn is nil.
// It acquires a read (shared) lock so that concurrent readers can check safely.
func (dcc *DynamicClientConn) IsClientConnNil() bool {
	dcc.mu.RLock()
	defer dcc.mu.RUnlock()

	return dcc.ClientConn == nil
}

// HasClientConn returns true if the underlying ClientConn is non-nil.
// It acquires a read (shared) lock so that concurrent readers can check safely.
func (dcc *DynamicClientConn) HasClientConn() bool {
	dcc.mu.RLock()
	defer dcc.mu.RUnlock()

	return dcc.ClientConn != nil // <— Note: fixed bug: originally compared “== nil”
}

// eventHandler is invoked by the file watcher whenever the certificate,
// key, or CA file changes. It triggers a refresh of the gRPC client
// connection with updated credentials.
func (dcc *DynamicClientConn) eventHandler(path string, _ []fsnotify.Event) {
	err := dcc.refreshGRPCClientConn()
	if err != nil {
		slog.Error("refreshing of dynamic grpc client failed", "watched-path", path, "error", err)
	} else {
		slog.Info("refreshing ok for dynamic grpc client", "watched-path", path)
	}
}

// refreshGRPCClientConn closes the existing gRPC connection (if present)
// and creates a new one using the latest configuration and credentials.
// If connection creation fails, the existing connection remains unchanged.
func (dcc *DynamicClientConn) refreshGRPCClientConn() error {
	newClient, err := NewClient(dcc.cfg, dcc.dialOptions...)
	if err != nil {
		return err
	}

	dcc.mu.Lock()
	defer dcc.mu.Unlock()

	if dcc.ClientConn != nil {
		_ = dcc.ClientConn.Close()
		dcc.ClientConn = nil
	}

	dcc.ClientConn = newClient

	return nil
}
