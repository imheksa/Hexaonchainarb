package scanner

import (
	"encoding/json"
	"fmt"
	"os"
)

type PoolType string

const (
	PoolTypeV2 PoolType = "v2"
	PoolTypeV3 PoolType = "v3"
)

// PoolConfig describes a single liquidity pool on a DEX.
type PoolConfig struct {
	DEX     string   `json:"dex"`
	Address string   `json:"address"`
	FeeBPS  int      `json:"fee_bps"`
	Type    PoolType `json:"type"` // "v2" or "v3"
}

// PairConfig describes a token pair with one or more pools across DEXes.
type PairConfig struct {
	Symbol         string       `json:"symbol"`
	Token0         string       `json:"token0"`
	Token1         string       `json:"token1"`
	Token0Decimals int          `json:"token0_decimals"`
	Token1Decimals int          `json:"token1_decimals"`
	Pools          []PoolConfig `json:"pools"`
}

// ChainConfig describes one EVM chain to monitor.
type ChainConfig struct {
	ChainID uint64       `json:"chain_id"`
	Name    string       `json:"name"`
	WSS     string       `json:"wss"`  // WebSocket RPC endpoint
	Pairs   []PairConfig `json:"pairs"`
}

// Config is the top-level scanner configuration.
type Config struct {
	Chains         []ChainConfig   `json:"chains"`
	MinProfitBPS   int64           `json:"min_profit_bps"`
	APIPort        string          `json:"api_port"`
	MaxPriceAgeSec int             `json:"max_price_age_sec"`
	Telegram       *TelegramConfig `json:"telegram,omitempty"`
}

func LoadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open config %s: %w", path, err)
	}
	defer f.Close()

	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}

	// defaults
	if cfg.MinProfitBPS == 0 {
		cfg.MinProfitBPS = 10
	}
	if cfg.APIPort == "" {
		cfg.APIPort = ":8090"
	}
	if cfg.MaxPriceAgeSec == 0 {
		cfg.MaxPriceAgeSec = 30
	}

	if len(cfg.Chains) == 0 {
		return nil, fmt.Errorf("no chains configured")
	}
	for _, ch := range cfg.Chains {
		if ch.WSS == "" {
			return nil, fmt.Errorf("chain %s: missing wss endpoint", ch.Name)
		}
	}
	return &cfg, nil
}
