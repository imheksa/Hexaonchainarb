package scanner

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Scanner orchestrates all chain monitors, the detector, the API, and Telegram.
type Scanner struct {
	cfg      *Config
	store    *PriceStore
	detector *Detector
	api      *API
	telegram *TelegramNotifier
}

func New(cfg *Config) *Scanner {
	store := NewPriceStore(cfg.MaxPriceAgeSec)

	var tg *TelegramNotifier
	if cfg.Telegram != nil && cfg.Telegram.BotToken != "" && cfg.Telegram.ChatID != "" {
		tg = NewTelegramNotifier(*cfg.Telegram)
		log.Info().Str("chat_id", cfg.Telegram.ChatID).Msg("Telegram notifications enabled")
	}

	detector := NewDetector(store, cfg.MinProfitBPS, tg)
	api := NewAPI(cfg.APIPort, store)
	return &Scanner{cfg: cfg, store: store, detector: detector, api: api, telegram: tg}
}

// Run starts all goroutines and blocks until ctx is cancelled.
func (s *Scanner) Run(ctx context.Context) error {
	log.Info().
		Int("chains", len(s.cfg.Chains)).
		Int64("min_profit_bps", s.cfg.MinProfitBPS).
		Msg("Scanner starting")

	var wg sync.WaitGroup

	// onUpdate is called each time any price is updated; triggers an immediate scan.
	onUpdate := func() { s.detector.Scan() }

	// One chain monitor goroutine per configured chain.
	for _, chain := range s.cfg.Chains {
		chain := chain
		wg.Add(1)
		go func() {
			defer wg.Done()
			NewChainMonitor(chain, s.store, onUpdate).Run(ctx)
		}()
	}

	// Periodic scan catches cross-chain windows where two chains update independently.
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.detector.Scan()
			}
		}
	}()

	// Telegram notifier.
	if s.telegram != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.telegram.Run(ctx)
		}()
	}

	// HTTP API.
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.api.Start(ctx)
	}()

	wg.Wait()
	log.Info().Msg("Scanner stopped")
	return nil
}
