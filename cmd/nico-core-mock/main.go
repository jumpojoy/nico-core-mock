package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/jumpojoy/nico-core-mock/internal/config"
	"github.com/jumpojoy/nico-core-mock/internal/server"
)

func main() {
	configPath := flag.String("config", "/config/machines.yaml", "path to machines YAML file")
	listenAddr := flag.String("listen", ":11079", "gRPC listen address")
	flag.Parse()

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	inventory, err := config.Load(*configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load machines config")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := server.Run(ctx, *listenAddr, inventory); err != nil {
		log.Fatal().Err(err).Msg("gRPC server stopped with error")
	}
}
