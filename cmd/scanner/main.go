package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"hexaonchainarb/scanner"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	cfgPath := flag.String("config", "scanner.json", "path to scanner config (JSON)")
	debug := flag.Bool("debug", false, "enable debug log level")
	flag.Parse()

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: "15:04:05"})
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	cfg, err := scanner.LoadConfig(*cfgPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load config")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := scanner.New(cfg).Run(ctx); err != nil {
		log.Fatal().Err(err).Msg("Scanner error")
	}
}
