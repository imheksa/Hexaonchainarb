package application

import (
	"context"
	"encoding/json"

	"github.com/0xAtelerix/sdk/gosdk/kv"
	"github.com/rs/zerolog/log"
)

// InitializeGenesis sets up default runtime config on first boot.
func InitializeGenesis(ctx context.Context, db kv.RwDB) error {
	tx, err := db.BeginRw(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Skip if already initialized
	existing, _ := tx.GetOne(ConfigBucket, []byte("genesis_initialized"))
	if len(existing) > 0 {
		return tx.Commit()
	}

	// Default: bot enabled
	if err := tx.Put(ConfigBucket, []byte("enabled"), []byte("true")); err != nil {
		return err
	}

	// Default: minimum profit = 10 BPS (0.10%)
	minProfit, _ := json.Marshal(int64(MinProfitBPS))
	if err := tx.Put(ConfigBucket, []byte("min_profit_bps"), minProfit); err != nil {
		return err
	}

	// Default: max borrow = 10,000 USDT (in wei, 18 decimals)
	// 10_000 * 10^18 — stored as string to avoid overflow
	if err := tx.Put(ConfigBucket, []byte("max_borrow_wei"), []byte("10000000000000000000000")); err != nil {
		return err
	}

	// Initialize stats
	zero, _ := json.Marshal(uint64(0))
	for _, key := range []string{"total_trades", "total_profit_wei", "failed_trades"} {
		if err := tx.Put(StatsBucket, []byte(key), zero); err != nil {
			return err
		}
	}

	if err := tx.Put(ConfigBucket, []byte("genesis_initialized"), []byte("true")); err != nil {
		return err
	}

	log.Info().Msg("Genesis initialized: arbitrage bot config set to defaults")
	return tx.Commit()
}
