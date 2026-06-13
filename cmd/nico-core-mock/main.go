package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/jumpojoy/nico-core-mock/internal/config"
	"github.com/jumpojoy/nico-core-mock/internal/libvirt"
	"github.com/jumpojoy/nico-core-mock/internal/server"
)

func main() {
	configPath := flag.String("config", "/config/machines.yaml", "path to machines YAML file")
	listenAddr := flag.String("listen", ":11079", "gRPC listen address")
	libvirtEndpoint := flag.String("libvirt-endpoint", "", "libvirt URI (e.g. qemu+tcp://host:16509/system); when set, only powered-on domains matching machine id are exposed")
	libvirtRefreshInterval := flag.Duration("libvirt-refresh-interval", 30*time.Second, "how often to refresh libvirt domain power state")
	flag.Parse()

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	inventory, err := config.Load(*configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load machines config")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	powerChecker := libvirt.PowerChecker(libvirt.NoopChecker{})
	if *libvirtEndpoint != "" {
		filter, err := libvirt.NewPowerFilter(ctx, *libvirtEndpoint, *libvirtRefreshInterval)
		if err != nil {
			log.Fatal().Err(err).Str("endpoint", *libvirtEndpoint).Msg("failed to initialize libvirt power filter")
		}
		powerChecker = filter
		log.Info().
			Str("endpoint", *libvirtEndpoint).
			Dur("refresh_interval", *libvirtRefreshInterval).
			Msg("libvirt power filtering enabled")
	}

	if err := server.Run(ctx, *listenAddr, inventory, powerChecker); err != nil {
		log.Fatal().Err(err).Msg("gRPC server stopped with error")
	}
}
