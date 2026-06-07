package scanner

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Scanner orchestrates all chain monitors, the detector, and the API.
type Scanner struct {
	cfg      *Config
	store    *PriceStore
	detector *Detector
	api      *API
}

func New(cfg *Config) *Scanner {
	store := NewPriceStore(cfg.MaxPriceAgeSec)
	detector := NewDetector(store, cfg.MinProfitBPS)
	api := NewAPI(cfg.APIPort, store)
	return &Scanner{cfg: cfg, store: store, detector: detector, api: api}
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
