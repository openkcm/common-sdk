// Package commongrpc provides helpers for creating and managing gRPC clients with
// support for connection pooling, telemetry, and secure mTLS credential management.
//
// One of the core features of this package is the ability to automatically refresh
// gRPC client connections when underlying TLS certificates or keys are updated on disk.
// This enables long-lived clients to handle certificate rotation without downtime.
package commongrpc

import (
	"log/slog"
	"path/filepath"
	"strings"
	"sync/atomic"

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
// certificate path. When changes are detected, the client connection is torn
// down and recreated with the latest credentials.
type DynamicClientConn struct {
	clientConn  atomic.Pointer[grpc.ClientConn]
	cfg         *commoncfg.GRPCClient
	dialOptions []grpc.DialOption

	notifier *notifier.Notifier
}

// NewDynamicClientConn creates a new DynamicClientConn for the given gRPC client
// configuration. It establishes a file watcher for mTLS certificate changes
// if the provided configuration includes a SecretRef with a certificate path.
//
// The returned DynamicClientConn will not establish a connection until Start()
// is called.
//
// Example:
//
//	cfg := &commoncfg.GRPCClient{
//	    Address: "my-grpc-server:443",
//	    SecretRef: &commoncfg.SecretRef{
//	        Type: commoncfg.MTLSSecretType,
//	        MTLS: commoncfg.MTLSSecret{
//	            Cert: commoncfg.FileRef{Path: "/etc/certs/tls.crt"},
//	            Key:  commoncfg.FileRef{Path: "/etc/certs/tls.key"},
//	            CA:   commoncfg.FileRef{Path: "/etc/certs/ca.crt"},
//	        },
//	    },
//	}
//
//	client, _ := commongrpc.NewDynamicClientConn(cfg)
//	_ = client.Start()
//	conn := client.Get()
func NewDynamicClientConn(cfg *commoncfg.GRPCClient, dialOptions ...grpc.DialOption) (*DynamicClientConn, error) {
	rc := &DynamicClientConn{
		clientConn:  atomic.Pointer[grpc.ClientConn]{},
		cfg:         cfg,
		dialOptions: dialOptions,
	}

	if cfg.SecretRef != nil {
		var paths []string

		certPath := strings.TrimSpace(cfg.SecretRef.MTLS.Cert.File.Path)
		if certPath != "" {
			paths = append(paths, filepath.Dir(certPath))
		}

		keyPath := strings.TrimSpace(cfg.SecretRef.MTLS.CertKey.File.Path)
		if keyPath != "" {
			paths = append(paths, filepath.Dir(keyPath))
		}

		caPath := strings.TrimSpace(cfg.SecretRef.MTLS.ServerCA.File.Path)
		if caPath != "" {
			paths = append(paths, filepath.Dir(caPath))
		}

		nt, err := notifier.Create(
			notifier.OnPaths(paths...),
			notifier.WithEventHandler(rc.eventHandler),
			notifier.WithBurstNumber(0),
		)
		if err != nil {
			return nil, err
		}

		err = nt.Start()
		if err != nil {
			return nil, err
		}

		rc.notifier = nt
	}

	rc.refreshGRPCClientConn()

	return rc, nil
}

// Close stops the file watcher (if active) and releases the underlying
// gRPC client connection. After calling Close, the DynamicClientConn
// should not be reused.
func (dcc *DynamicClientConn) Close() error {
	if dcc.notifier == nil {
		return nil
	}

	defer func() {
		dcc.notifier = nil
	}()

	_ = dcc.notifier.Close()

	conn := dcc.clientConn.Swap(nil)
	if conn != nil {
		return conn.Close()
	}

	return nil
}

// ClientConn returns the current gRPC client connection. If Start() has not
// been called yet, or if connection creation failed, this may return nil.
func (dcc *DynamicClientConn) ClientConn() *grpc.ClientConn {
	return dcc.clientConn.Load()
}

// eventHandler is invoked by the underlying file watcher when the
// certificate file changes. It triggers a refresh of the client connection.
func (dcc *DynamicClientConn) eventHandler(_ string, _ []fsnotify.Event) {
	dcc.refreshGRPCClientConn()
}

// refreshGRPCClientConnection tears down the existing gRPC connection
// and replaces it with a newly established one based on the latest
// configuration and credentials. If connection creation fails, the
// error is logged and the previous connection remains in use.
func (dcc *DynamicClientConn) refreshGRPCClientConn() {
	newClient, err := NewClient(dcc.cfg, dcc.dialOptions...)
	if err != nil {
		slog.Error("creation of grpc client failed", "error", err)
		return
	}

	oldClient := dcc.clientConn.Swap(newClient)
	if oldClient != nil {
		_ = oldClient.Close()
	}
}
