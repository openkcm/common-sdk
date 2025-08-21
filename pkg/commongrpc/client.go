package commongrpc

import (
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/grpcpool"
	"github.com/openkcm/common-sdk/pkg/otlp"
)

var (
	ErrEmptyAddress = errors.New("grpc address is empty")
)

type PooledClient interface {
	SetPool(pool *grpcpool.Pool)
}

func NewPooledClient(client PooledClient, cfg *commoncfg.GRPCClient, dialOptions ...grpc.DialOption) error {
	if cfg.Address == "" {
		return ErrEmptyAddress
	}

	opts := make([]grpc.DialOption, 0)
	opts = append(opts,
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    cfg.Attributes.KeepaliveTime,
			Timeout: cfg.Attributes.KeepaliveTimeout,
		}),
		grpc.WithStatsHandler(otlp.NewClientHandler()),
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

func createFactory(address string, dialOptions ...grpc.DialOption) grpcpool.ClientFactory {
	return func() (*grpc.ClientConn, error) {
		return grpc.NewClient(address, dialOptions...)
	}
}

func NewClient(cfg *commoncfg.GRPCClient, dialOptions ...grpc.DialOption) (*grpc.ClientConn, error) {
	if cfg.Address == "" {
		return nil, ErrEmptyAddress
	}

	opts := make([]grpc.DialOption, 0)
	opts = append(opts,
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    cfg.Attributes.KeepaliveTime,
			Timeout: cfg.Attributes.KeepaliveTimeout,
		}),
		grpc.WithStatsHandler(otlp.NewClientHandler()),
	)
	opts = append(opts, dialOptions...)

	return grpc.NewClient(cfg.Address, opts...)
}
