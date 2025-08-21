package health

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	slogctx "github.com/veqryn/slog-context"
	healthgrpc "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/grpcpool"
)

var (
	ErrIsNotGrpcPool = errors.New("result is not of type grpc Pool")
)

type GRPCHealthClientService interface {
	healthgrpc.HealthClient
}

var globalGRPCClientPool = sync.Map{}

// CheckGRPCServerHealth does call the grpc server health check API
func CheckGRPCServerHealth(ctx context.Context, grpcCfg *commoncfg.GRPCClient) error {
	if grpcCfg == nil {
		return nil
	}

	client, err := NewGRPCHealthClient(grpcCfg)
	if err != nil {
		return err
	}

	grpcServerHealthResp, err := client.Check(ctx, &healthgrpc.HealthCheckRequest{Service: "self"})
	if err != nil {
		return err
	}

	if grpcServerHealthResp.GetStatus() != healthgrpc.HealthCheckResponse_SERVING {
		return fmt.Errorf("GRPC Server serving on address %s is not in servig state", grpcCfg.Address)
	}

	return nil
}

// NewGRPCHealthClient create a grpc client connect to health check server
func NewGRPCHealthClient(grpcClientCfg *commoncfg.GRPCClient, dialOptions ...grpc.DialOption) (GRPCHealthClientService, error) {
	dialOptions = append(dialOptions,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    grpcClientCfg.Attributes.KeepaliveTime,
			Timeout: grpcClientCfg.Attributes.KeepaliveTimeout,
		}),
	)

	client := &grpcClient{
		serverAddr: grpcClientCfg.Address,
	}

	var err error

	clientPool, ok := globalGRPCClientPool.Load(grpcClientCfg.Address)
	if !ok {
		fac := createGRPCFactory(client, dialOptions...)

		clientPool, err = grpcpool.New(fac,
			grpcpool.WithInitialCapacity(grpcClientCfg.Pool.InitialCapacity),
			grpcpool.WithMaxCapacity(grpcClientCfg.Pool.MaxCapacity),
			grpcpool.WithIdleTimeout(grpcClientCfg.Pool.IdleTimeout),
			grpcpool.WithMaxLifeDuration(grpcClientCfg.Pool.MaxLifeDuration),
		)
		if err != nil {
			return nil, err
		}

		globalGRPCClientPool.Store(grpcClientCfg.Address, clientPool)
	}

	client.clientPool, ok = clientPool.(*grpcpool.Pool)
	if !ok {
		return nil, ErrIsNotGrpcPool
	}

	return client, nil
}

type grpcClient struct {
	serverAddr string

	clientPool *grpcpool.Pool
}

func createGRPCFactory(client *grpcClient, dialOptions ...grpc.DialOption) grpcpool.ClientFactory {
	return func() (*grpc.ClientConn, error) {
		conn, err := grpc.NewClient(client.serverAddr, dialOptions...)
		if err != nil {
			return nil, err
		}

		return conn, err
	}
}

func (client *grpcClient) Check(ctx context.Context, req *healthgrpc.HealthCheckRequest, opts ...grpc.CallOption) (*healthgrpc.HealthCheckResponse, error) {
	conn, err := client.clientPool.Get(ctx)
	if err != nil {
		return nil, err
	}
	defer func(conn *grpcpool.PooledClientConn) {
		err := conn.Close()
		if err != nil {
			slogctx.Error(ctx, "Failed to close client connection", "error", err)
		}
	}(conn)

	healthClient := healthgrpc.NewHealthClient(conn.ClientConn)

	resp, err := healthClient.Check(ctx, req)
	if err != nil {
		conn.MarkUnhealthy()
		return nil, err
	}

	return resp, nil
}

func (client *grpcClient) List(ctx context.Context, req *healthgrpc.HealthListRequest, opts ...grpc.CallOption) (*healthgrpc.HealthListResponse, error) {
	conn, err := client.clientPool.Get(ctx)
	if err != nil {
		return nil, err
	}
	defer func(conn *grpcpool.PooledClientConn) {
		err := conn.Close()
		if err != nil {
			slogctx.Error(ctx, "Failed to close client connection", "error", err)
		}
	}(conn)

	healthClient := healthgrpc.NewHealthClient(conn.ClientConn)

	resp, err := healthClient.List(ctx, req)
	if err != nil {
		conn.MarkUnhealthy()
		return nil, err
	}

	return resp, nil
}

func (client *grpcClient) Watch(ctx context.Context, req *healthgrpc.HealthCheckRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[healthgrpc.HealthCheckResponse], error) {
	conn, err := client.clientPool.Get(ctx)
	if err != nil {
		return nil, err
	}
	defer func(conn *grpcpool.PooledClientConn) {
		err := conn.Close()
		if err != nil {
			slogctx.Error(ctx, "Failed to close client connection", "error", err)
		}
	}(conn)

	healthClient := healthgrpc.NewHealthClient(conn.ClientConn)

	resp, err := healthClient.Watch(ctx, req, opts...)
	if err != nil {
		conn.MarkUnhealthy()
		return nil, err
	}

	return resp, nil
}
