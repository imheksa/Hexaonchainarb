package scanner

import (
	"fmt"
	"math/big"
	"time"

	"github.com/rs/zerolog/log"
)

// Detector scans all fresh prices in the store and emits opportunities.
type Detector struct {
	store        *PriceStore
	minProfitBPS int64
}

func NewDetector(store *PriceStore, minProfitBPS int64) *Detector {
	return &Detector{store: store, minProfitBPS: minProfitBPS}
}

// Scan checks every pair of prices for the same symbol and records any
// profitable arbitrage opportunities.
func (d *Detector) Scan() {
	symbols := d.store.UniqueSymbols()
	for _, sym := range symbols {
		prices := d.store.PricesBySymbol(sym)
		if len(prices) < 2 {
			continue
		}
		for i := 0; i < len(prices); i++ {
			for j := i + 1; j < len(prices); j++ {
				// Try both directions
				if opp := d.calcOpportunity(prices[i], prices[j]); opp != nil {
					d.store.AddOpportunity(opp)
					d.log(opp)
				}
				if opp := d.calcOpportunity(prices[j], prices[i]); opp != nil {
					d.store.AddOpportunity(opp)
					d.log(opp)
				}
			}
		}
	}
}

// calcOpportunity returns an opportunity if buying on `buy` and selling on `sell` is
// profitable after DEX fees. Cross-chain opportunities are flagged but do NOT
// deduct bridge fees — those depend on the bridge used.
func (d *Detector) calcOpportunity(buy, sell *PriceData) *Opportunity {
	bps := int64(10000)
	buyPrice := buy.Price()
	sellPrice := sell.Price()

	if buyPrice.Sign() == 0 || sellPrice.Sign() == 0 {
		return nil
	}

	// effectiveBuy  = buyPrice  × (1 + buyFee/10000)   — you pay more when buying
	// effectiveSell = sellPrice × (1 - sellFee/10000)  — you receive less when selling
	buyMul := ratFrac(bps+int64(buy.FeeBPS), bps)
	sellMul := ratFrac(bps-int64(sell.FeeBPS), bps)

	effectiveBuy := new(big.Rat).Mul(buyPrice, buyMul)
	effectiveSell := new(big.Rat).Mul(sellPrice, sellMul)

	if effectiveBuy.Sign() == 0 {
		return nil
	}

	// profitBPS = (effectiveSell/effectiveBuy − 1) × 10000
	ratio := new(big.Rat).Quo(effectiveSell, effectiveBuy)
	profit := new(big.Rat).Sub(ratio, new(big.Rat).SetInt64(1))
	profitBPSRat := new(big.Rat).Mul(profit, new(big.Rat).SetInt64(bps))
	profitBPS, _ := profitBPSRat.Float64()

	if int64(profitBPS) < d.minProfitBPS {
		return nil
	}

	buyPriceF, _ := buyPrice.Float64()
	sellPriceF, _ := sellPrice.Float64()
	spreadBPS := int64((sellPriceF/buyPriceF - 1) * float64(bps))

	crossChain := buy.Key.ChainID != sell.Key.ChainID
	note := ""
	if crossChain {
		note = "cross-chain: bridge fee not included in profit_bps"
	}

	return &Opportunity{
		ID:          fmt.Sprintf("%d", time.Now().UnixNano()),
		Symbol:      buy.Symbol,
		CrossChain:  crossChain,
		BuyChainID:  buy.Key.ChainID,
		BuyChain:    buy.ChainName,
		BuyDEX:      buy.Key.DEX,
		BuyPool:     buy.Key.Pool,
		BuyPrice:    buyPriceF,
		BuyFeeBPS:   buy.FeeBPS,
		SellChainID: sell.Key.ChainID,
		SellChain:   sell.ChainName,
		SellDEX:     sell.Key.DEX,
		SellPool:    sell.Key.Pool,
		SellPrice:   sellPriceF,
		SellFeeBPS:  sell.FeeBPS,
		SpreadBPS:   spreadBPS,
		ProfitBPS:   int64(profitBPS),
		Note:        note,
		DetectedAt:  time.Now(),
	}
}

func (d *Detector) log(opp *Opportunity) {
	kind := "intra-chain"
	if opp.CrossChain {
		kind = "cross-chain"
	}
	log.Info().
		Str("type", kind).
		Str("symbol", opp.Symbol).
		Str("buy", fmt.Sprintf("%s @ %s", opp.BuyDEX, opp.BuyChain)).
		Str("sell", fmt.Sprintf("%s @ %s", opp.SellDEX, opp.SellChain)).
		Float64("buy_price", opp.BuyPrice).
		Float64("sell_price", opp.SellPrice).
		Int64("spread_bps", opp.SpreadBPS).
		Int64("profit_bps", opp.ProfitBPS).
		Msg("Opportunity found")
}

func ratFrac(num, den int64) *big.Rat {
	return new(big.Rat).SetFrac(big.NewInt(num), big.NewInt(den))
}
