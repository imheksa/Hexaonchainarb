package main

import (
	"context"
	"flag"
	"os/signal"
	"syscall"

	gosdk "github.com/0xAtelerix/sdk/gosdk"
	"github.com/0xAtelerix/sdk/gosdk/rpc"
	"github.com/rs/zerolog/log"

	"hexaonchainarb/application"
	"hexaonchainarb/application/api"
)

func main() {
	configPath := flag.String("config", "", "path to config YAML file")
	flag.Parse()

	var cfg *gosdk.InitConfig
	var err error
	if *configPath != "" {
		cfg, err = gosdk.LoadConfig(*configPath)
		if err != nil {
			log.Fatal().Err(err).Str("path", *configPath).Msg("Failed to load config")
		}
	} else {
		cfg = &gosdk.InitConfig{}
	}

	ctx := gosdk.SetupLogger(context.Background(), cfg.LogLevel)
	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, cfg); err != nil {
		log.Fatal().Err(err).Msg("Arbitrage bot exited with error")
	}
}

func run(ctx context.Context, cfg *gosdk.InitConfig) error {
	// ── 1. Initialize Pelagos Appchain storage ───────────────────────────────
	appInit, err := gosdk.InitApp[application.Transaction, application.Receipt](ctx, *cfg,
		gosdk.WithTables(application.Tables()),
	)
	if err != nil {
		return err
	}
	defer appInit.Close()

	// ── 2. Genesis: set default config on first boot ─────────────────────────
	if err := application.InitializeGenesis(ctx, appInit.Storage.AppchainDB()); err != nil {
		return err
	}

	// ── 3. External block processor (price monitor + arb detector) ───────────
	extProcessor := application.NewExtBlockProcessor(appInit.Storage.Multichain())

	// ── 4. Create Appchain runtime ───────────────────────────────────────────
	app := gosdk.NewAppchain(
		appInit.Storage,
		appInit.Config,
		gosdk.NewDefaultBatchProcessor[application.Transaction, application.Receipt](
			extProcessor,
			appInit.Storage.Multichain(),
			appInit.Storage.Subscriber(),
		),
		application.BlockConstructor,
	)

	// ── 5. Set up JSON-RPC server ─────────────────────────────────────────────
	rpcServer := rpc.NewStandardRPCServer(nil)
	rpcServer.AddMiddleware(api.NewLoggingMiddleware(log.Logger))
	rpc.AddStandardMethods[application.Transaction, application.Receipt](
		rpcServer,
		appInit.Storage.AppchainDB(),
		appInit.Storage.TxPool(),
		appInit.Config.ChainID,
	)
	// Register arbitrage-specific RPC methods
	api.New(rpcServer, appInit.Storage.AppchainDB()).AddRPCMethods()

	log.Info().
		Uint64("chain_id", *appInit.Config.ChainID).
		Str("rpc_port", appInit.Config.RPCPort).
		Msg("Hexa Arb Bots starting")

	// ── 6. Run Appchain + RPC concurrently ───────────────────────────────────
	errCh := make(chan error, 2)

	go func() { errCh <- app.Run(ctx) }()
	go func() { errCh <- rpcServer.StartHTTPServer(ctx, appInit.Config.RPCPort) }()

	select {
	case <-ctx.Done():
		log.Info().Msg("Shutting down gracefully")
		return nil
	case err := <-errCh:
		return err
	}
}
