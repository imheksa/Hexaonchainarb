package application

import "github.com/0xAtelerix/sdk/gosdk/kv"

const (
	// PricesBucket stores latest price per DEX per pair
	// key: "<dex>:<pool_address>" → value: price bytes (big.Float serialized)
	PricesBucket = "prices"

	// OpportunitiesBucket stores detected arbitrage opportunities
	// key: "<timestamp>" → value: JSON-encoded ArbitrageOpportunity
	OpportunitiesBucket = "opportunities"

	// StatsBucket stores running performance stats
	// key: "total_profit", "total_trades", etc.
	StatsBucket = "stats"

	// ConfigBucket stores runtime config (min profit BPS, active pairs, etc.)
	ConfigBucket = "config"
)

func Tables() kv.TableCfg {
	return kv.TableCfg{
		PricesBucket:        {},
		OpportunitiesBucket: {},
		StatsBucket:         {},
		ConfigBucket:        {},
	}
}
