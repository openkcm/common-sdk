package commongrpc

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/commontls"
)

// DynamicClientConn is a gRPC client connection whose mTLS credentials follow
// certificate rotation on disk.
//
// It embeds the *grpc.ClientConn to use, which also makes it a
// grpc.ClientConnInterface, so it can be passed directly to generated client
// constructors:
//
//	conn, err := commongrpc.NewDynamicClientConn(cfg, 2*time.Second)
//	if err != nil {
//	    return err
//	}
//	defer conn.Close()
//
//	client := pb.NewMyServiceClient(conn)
//
// The connection is created once and never replaced, so clients built from
// it, or from conn.ClientConn, stay valid for its whole life, and a rotation
// never interrupts RPCs or streams in flight.
//
// When cfg.SecretRef is of type commoncfg.MTLSSecretType, the transport
// credentials are backed by a
// [github.com/openkcm/common-sdk/pkg/tls.DynamicTLSConfig]: the client
// certificate (with its intermediates) and the certificate authorities that
// verify the server are reloaded when their files change, and every new TLS
// handshake uses the reloaded ones. See that type for which sources are
// watched, how the server is verified and what happens when a reload fails.
//
// A rotation does not tear down connections that are already established:
// they keep the certificate they were opened with until gRPC opens new ones.
// gRPC does so when a connection breaks or when the server closes it, which a
// server does regularly when it sets a maximum connection age
// (keepalive.ServerParameters.MaxConnectionAge, or MaxConnectionAge in
// commoncfg.GRPCServer attributes). Set one on the server so that clients move
// to their rotated certificates within that age.
//
// Without mTLS (no SecretRef, or commoncfg.InsecureSecretType) it is a plain
// client connection and nothing is watched.
//
// A DynamicClientConn is safe for concurrent use. Call Close when it is no
// longer needed.
type DynamicClientConn struct {
	*grpc.ClientConn

	tlsConfig *commontls.DynamicTLSConfig // nil without mTLS

	closeOnce sync.Once
	closeErr  error
}

// NewDynamicClientConn creates the client connection described by cfg, with
// the same keepalive parameters, telemetry stats handler and dialOptions as
// NewClient.
//
// If cfg.SecretRef is of type commoncfg.MTLSSecretType, it loads the mTLS
// credentials and starts watching the directories of their file sources; see
// DynamicClientConn. throttleInterval groups file changes: a reload runs once
// no further change has been seen for throttleInterval, so writing a
// certificate and its key in quick succession causes a single reload.
// cfg.SecretRef.MTLS is read again on every reload and must not be modified
// afterwards.
//
// The server certificate is verified against the authority gRPC dials, as
// with NewClient: the value of grpc.WithAuthority, otherwise the server name
// in the mTLS attributes, otherwise the host of the target, including when
// that is an IP address. gRPC rejects a grpc.WithAuthority that differs from
// the server name in the attributes.
//
// It returns an error if cfg.Address is empty, if the secret type is not
// supported, if the credentials cannot be loaded, if a directory to watch
// does not exist, or if the connection cannot be created. Nothing is left
// running when it returns an error.
func NewDynamicClientConn(cfg *commoncfg.GRPCClient, throttleInterval time.Duration, dialOptions ...grpc.DialOption) (*DynamicClientConn, error) {
	if cfg.SecretRef == nil || cfg.SecretRef.Type != commoncfg.MTLSSecretType {
		conn, err := NewClient(cfg, dialOptions...)
		if err != nil {
			return nil, err
		}

		return &DynamicClientConn{ClientConn: conn}, nil
	}

	if cfg.Address == "" {
		return nil, ErrEmptyAddress
	}

	tlsConfig, err := commontls.NewDynamicTLSConfig(throttleInterval, &cfg.SecretRef.MTLS)
	if err != nil {
		return nil, err
	}

	creds := &authorityTLSCredentials{
		TransportCredentials: credentials.NewTLS(tlsConfig.Config),
		config:               tlsConfig.Config,
	}

	conn, err := newClient(cfg, creds, dialOptions...)
	if err != nil {
		_ = tlsConfig.Close()
		return nil, err
	}

	return &DynamicClientConn{ClientConn: conn, tlsConfig: tlsConfig}, nil
}

// Close stops watching the credential files, if any, and closes the
// underlying client connection, which ends all RPCs and streams on it. Calling
// Close more than once is safe; later calls return the result of the first.
// The DynamicClientConn must not be used after Close.
func (dcc *DynamicClientConn) Close() error {
	dcc.closeOnce.Do(func() {
		var errs []error

		if dcc.tlsConfig != nil {
			errs = append(errs, dcc.tlsConfig.Close())
		}

		if dcc.ClientConn != nil {
			errs = append(errs, dcc.ClientConn.Close())
		}

		dcc.closeErr = errors.Join(errs...)
	})

	return dcc.closeErr
}

// IsClientConnNil reports whether the underlying ClientConn is nil. It is
// never nil for a DynamicClientConn returned by NewDynamicClientConn.
func (dcc *DynamicClientConn) IsClientConnNil() bool {
	return dcc.ClientConn == nil
}

// HasClientConn reports whether the underlying ClientConn is set. It always is
// for a DynamicClientConn returned by NewDynamicClientConn.
func (dcc *DynamicClientConn) HasClientConn() bool {
	return dcc.ClientConn != nil
}

// authorityTLSCredentials are the transport credentials of a
// DynamicClientConn. When its certificate authorities come from files, the
// server is verified by tls.Config.VerifyConnection against the server name in
// SNI, and SNI never carries an IP address. ClientHandshake receives the
// authority gRPC dials, so these credentials pass its host to
// VerifyConnection when SNI is empty, and a server dialled by IP address is
// verified against that address, as with static credentials.
type authorityTLSCredentials struct {
	credentials.TransportCredentials

	config *tls.Config
}

func (c *authorityTLSCredentials) ClientHandshake(ctx context.Context, authority string, rawConn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	verify := c.config.VerifyConnection
	if verify == nil {
		return c.TransportCredentials.ClientHandshake(ctx, authority, rawConn)
	}

	// The same server name gRPC derives from the authority for the handshake.
	serverName, _, err := net.SplitHostPort(authority)
	if err != nil {
		serverName = authority
	}

	cfg := c.config.Clone()
	cfg.VerifyConnection = func(cs tls.ConnectionState) error {
		if cs.ServerName == "" {
			cs.ServerName = serverName
		}

		return verify(cs)
	}

	return credentials.NewTLS(cfg).ClientHandshake(ctx, authority, rawConn)
}

func (c *authorityTLSCredentials) Clone() credentials.TransportCredentials {
	return &authorityTLSCredentials{TransportCredentials: c.TransportCredentials.Clone(), config: c.config}
}
