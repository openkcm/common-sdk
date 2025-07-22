package commongrpc

import (
	"context"

	slogctx "github.com/veqryn/slog-context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"

	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"github.com/openkcm/common-sdk/pkg/commoncfg"
	"github.com/openkcm/common-sdk/pkg/health"
	"github.com/openkcm/common-sdk/pkg/otlp"
)

// NewServer create the grpc server
func NewServer(ctx context.Context, cfg *commoncfg.GRPCServer, serverOptions ...grpc.ServerOption) *grpc.Server {
	opts := make([]grpc.ServerOption, 0)

	opts = append(opts,
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             cfg.EfPolMinTime,             // If a client pings more than once every 15 sec, terminate the connection
			PermitWithoutStream: cfg.EfPolPermitWithoutStream, // Allow pings even when there are no active streams
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     cfg.Attributes.MaxConnectionIdle,
			MaxConnectionAge:      cfg.Attributes.MaxConnectionAge,
			MaxConnectionAgeGrace: cfg.Attributes.MaxConnectionAgeGrace,
			Time:                  cfg.Attributes.Time,
			Timeout:               cfg.Attributes.Timeout,
		}),
		grpc.MaxRecvMsgSize(cfg.MaxRecvMsgSize),
		grpc.StatsHandler(otlp.NewServerHandler()),
	)

	opts = append(opts, serverOptions...)

	grpcServer := grpc.NewServer(opts...)

	if cfg.Flags.Reflection {
		reflection.Register(grpcServer)
		slogctx.Info(ctx, "grpc server reflection enabled")
	}

	if cfg.Flags.Health {
		healthpb.RegisterHealthServer(grpcServer, &health.GRPCServer{})
		slogctx.Info(ctx, "grpc server health enabled")
	}

	return grpcServer
}
