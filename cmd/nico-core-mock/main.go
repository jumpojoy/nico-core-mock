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
	"golang.org/x/term"

	"github.com/jumpojoy/nico-core-mock/internal/config"
	"github.com/jumpojoy/nico-core-mock/internal/libvirt"
	"github.com/jumpojoy/nico-core-mock/internal/server"
)

func main() {
	configPath := flag.String("config", "/config/machines.yaml", "path to machines YAML file")
	listenAddr := flag.String("listen", ":11079", "gRPC listen address")
	logLevel := flag.String("log-level", "debug", "log level: trace, debug, info, warn, error")
	libvirtEndpoint := flag.String("libvirt-endpoint", "", "libvirt URI (e.g. qemu+tcp://host:16509/system); when set, only powered-on domains matching machine id are exposed")
	libvirtRefreshInterval := flag.Duration("libvirt-refresh-interval", 30*time.Second, "how often to refresh libvirt domain power state")
	flag.Parse()

	initLogging(resolveLogLevel(*logLevel))

	inventory, err := config.Load(*configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load machines config")
	}

	log.Debug().
		Str("config", *configPath).
		Int("machines", len(inventory.Machines)).
		Int("expected_machines", len(inventory.ExpectedMachines)).
		Msg("loaded machines config")

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

func resolveLogLevel(flagLevel string) string {
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		return v
	}
	return flagLevel
}

func initLogging(level string) {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix

	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		log.Fatal().Str("level", level).Msg("invalid log level")
	}
	zerolog.SetGlobalLevel(lvl)

	if term.IsTerminal(int(os.Stderr.Fd())) {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	} else {
		log.Logger = zerolog.New(os.Stderr).With().Timestamp().Logger()
	}

	log.Info().Str("level", lvl.String()).Msg("logging configured")
}
