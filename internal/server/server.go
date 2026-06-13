package server

import (
	"context"
	"net"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/jumpojoy/nico-core-mock/internal/config"
	libvirtfilter "github.com/jumpojoy/nico-core-mock/internal/libvirt"
	forgev1 "github.com/NVIDIA/infra-controller/rest-api/workflow-schema/schema/site-agent/workflows/v1"
)

var runLogger = log.With().Str("component", "nico-core-mock").Logger()

// Run starts the gRPC server on listenAddr until ctx is cancelled.
func Run(ctx context.Context, listenAddr string, inventory *config.Inventory, powerChecker libvirtfilter.PowerChecker) error {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}

	nicoServer := NewFromInventory(inventory, powerChecker)
	srv := grpc.NewServer(grpc.UnaryInterceptor(func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		nicoServer.mu.Lock()
		defer nicoServer.mu.Unlock()
		runLogger.Debug().Str("method", info.FullMethod).Msg("gRPC request")
		resp, err := handler(ctx, req)
		if err != nil {
			runLogger.Debug().Str("method", info.FullMethod).Err(err).Msg("gRPC request failed")
		}
		return resp, err
	}))
	reflection.Register(srv)
	forgev1.RegisterForgeServer(srv, nicoServer)

	go func() {
		<-ctx.Done()
		runLogger.Info().Msg("shutting down gRPC server")
		srv.GracefulStop()
	}()

	runLogger.Info().
		Str("addr", listenAddr).
		Int("machines", len(inventory.Machines)).
		Bool("libvirt_filter", powerChecker.Enabled()).
		Msg("started gRPC server")

	if powerChecker.Enabled() {
		visible := 0
		for id := range inventory.Machines {
			if powerChecker.IsPoweredOn(id) {
				visible++
			}
		}
		runLogger.Debug().
			Int("inventory_machines", len(inventory.Machines)).
			Int("visible_machines", visible).
			Msg("libvirt machine visibility")
	}

	err = srv.Serve(listener)
	if err != nil && ctx.Err() == nil {
		return err
	}
	return nil
}
