package scanner

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/rs/zerolog/log"
)

// API serves a simple JSON HTTP interface for monitoring the scanner.
type API struct {
	port  string
	store *PriceStore
	start time.Time
}

func NewAPI(port string, store *PriceStore) *API {
	return &API{port: port, store: store, start: time.Now()}
}

func (a *API) Start(ctx context.Context) {
	mux := http.NewServeMux()
	mux.HandleFunc("/prices", a.handlePrices)
	mux.HandleFunc("/opportunities", a.handleOpportunities)
	mux.HandleFunc("/status", a.handleStatus)

	srv := &http.Server{Addr: a.port, Handler: mux}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		srv.Shutdown(shutCtx) //nolint:errcheck
	}()

	log.Info().Str("addr", a.port).Msg("API listening")
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error().Err(err).Msg("API server error")
	}
}

// GET /prices
// Returns all fresh price feeds grouped by symbol.
func (a *API) handlePrices(w http.ResponseWriter, _ *http.Request) {
	type priceRow struct {
		Chain   string  `json:"chain"`
		ChainID uint64  `json:"chain_id"`
		DEX     string  `json:"dex"`
		Pool    string  `json:"pool"`
		Symbol  string  `json:"symbol"`
		Price   float64 `json:"price"`
		FeeBPS  int     `json:"fee_bps"`
		Block   uint64  `json:"block"`
		AgeMs   int64   `json:"age_ms"`
	}

	prices := a.store.AllPrices()
	rows := make([]priceRow, 0, len(prices))
	for _, p := range prices {
		rows = append(rows, priceRow{
			Chain:   p.ChainName,
			ChainID: p.Key.ChainID,
			DEX:     p.Key.DEX,
			Pool:    p.Key.Pool,
			Symbol:  p.Symbol,
			Price:   p.PriceFloat(),
			FeeBPS:  p.FeeBPS,
			Block:   p.BlockNum,
			AgeMs:   time.Since(p.UpdatedAt).Milliseconds(),
		})
	}
	writeJSON(w, rows)
}

// GET /opportunities?limit=50
// Returns recent detected arbitrage opportunities.
func (a *API) handleOpportunities(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	opps := a.store.RecentOpportunities(limit)
	writeJSON(w, opps)
}

// GET /status
func (a *API) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"uptime_sec":         int(time.Since(a.start).Seconds()),
		"prices_tracked":     len(a.store.AllPrices()),
		"total_opportunities": a.store.TotalOpportunities(),
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Warn().Err(err).Msg("Failed to write JSON response")
	}
}
