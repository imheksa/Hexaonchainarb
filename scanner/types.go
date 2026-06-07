package scanner

import (
	"fmt"
	"math/big"
	"time"
)

// PriceKey uniquely identifies a price feed: chain + DEX + pool.
type PriceKey struct {
	ChainID uint64
	DEX     string
	Pool    string // lowercase address
}

func (k PriceKey) String() string {
	short := k.Pool
	if len(short) > 10 {
		short = short[:10] + "…"
	}
	return fmt.Sprintf("chain:%d/%s/%s", k.ChainID, k.DEX, short)
}

// PriceData is the latest observed price for a pool.
type PriceData struct {
	Key       PriceKey
	Symbol    string
	ChainName string
	Token0    string
	Token1    string
	PriceNum  *big.Int
	PriceDen  *big.Int
	FeeBPS    int
	BlockNum  uint64
	UpdatedAt time.Time
}

func (p *PriceData) Price() *big.Rat {
	if p.PriceDen == nil || p.PriceDen.Sign() == 0 {
		return new(big.Rat)
	}
	return new(big.Rat).SetFrac(p.PriceNum, p.PriceDen)
}

func (p *PriceData) PriceFloat() float64 {
	f, _ := p.Price().Float64()
	return f
}

// Opportunity is a detected arbitrage: buy on one DEX/chain, sell on another.
type Opportunity struct {
	ID          string    `json:"id"`
	Symbol      string    `json:"symbol"`
	CrossChain  bool      `json:"cross_chain"`

	BuyChainID  uint64    `json:"buy_chain_id"`
	BuyChain    string    `json:"buy_chain"`
	BuyDEX      string    `json:"buy_dex"`
	BuyPool     string    `json:"buy_pool"`
	BuyPrice    float64   `json:"buy_price"`
	BuyFeeBPS   int       `json:"buy_fee_bps"`

	SellChainID uint64    `json:"sell_chain_id"`
	SellChain   string    `json:"sell_chain"`
	SellDEX     string    `json:"sell_dex"`
	SellPool    string    `json:"sell_pool"`
	SellPrice   float64   `json:"sell_price"`
	SellFeeBPS  int       `json:"sell_fee_bps"`

	SpreadBPS   int64     `json:"spread_bps"`
	// ProfitBPS after DEX fees. Cross-chain: does NOT include bridge fees.
	ProfitBPS   int64     `json:"profit_bps"`
	Note        string    `json:"note,omitempty"`

	DetectedAt  time.Time `json:"detected_at"`
}
