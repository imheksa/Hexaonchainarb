package application

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/0xAtelerix/sdk/gosdk/kv"
)

// PriceData holds the latest known price of a token pair at a specific DEX.
type PriceData struct {
	DEX       string     `json:"dex"`
	Pool      string     `json:"pool"`
	Token0    string     `json:"token0"`
	Token1    string     `json:"token1"`
	// Price = Token1 per Token0 (e.g., USDT per WBNB)
	// Stored as numerator/denominator to avoid float precision loss
	PriceNum  string     `json:"price_num"`
	PriceDen  string     `json:"price_den"`
	BlockNum  uint64     `json:"block_num"`
	Timestamp uint64     `json:"timestamp"`
}

func priceKey(dex, pool string) []byte {
	return []byte(fmt.Sprintf("%s:%s", dex, pool))
}

// StorePrice persists the latest price for a pool to the DB.
func StorePrice(dbTx kv.RwTx, data PriceData) error {
	b, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return dbTx.Put(PricesBucket, priceKey(data.DEX, data.Pool), b)
}

// LoadPrice retrieves the latest price for a pool from the DB.
func LoadPrice(dbTx kv.RoTx, dex, pool string) (*PriceData, error) {
	raw, err := dbTx.GetOne(PricesBucket, priceKey(dex, pool))
	if err != nil || len(raw) == 0 {
		return nil, ErrPriceDataMissing
	}
	var p PriceData
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Price returns the price as a *big.Rat for precise arithmetic.
func (p *PriceData) Price() *big.Rat {
	num := new(big.Int)
	den := new(big.Int)
	num.SetString(p.PriceNum, 10)
	den.SetString(p.PriceDen, 10)
	if den.Sign() == 0 {
		return new(big.Rat)
	}
	return new(big.Rat).SetFrac(num, den)
}

// ─── Uniswap V3 price from sqrtPriceX96 ─────────────────────────────────────

// SqrtPriceX96ToPrice converts a Uniswap V3 sqrtPriceX96 to token1/token0 price.
// price = (sqrtPriceX96 / 2^96)^2
// For WBNB/USDT with both 18 decimals, no decimal adjustment needed.
func SqrtPriceX96ToPrice(sqrtPriceX96 *big.Int) *big.Rat {
	// Q96 = 2^96
	q96 := new(big.Int).Lsh(big.NewInt(1), 96)

	// numerator = sqrtPriceX96^2
	num := new(big.Int).Mul(sqrtPriceX96, sqrtPriceX96)
	// denominator = Q96^2 = 2^192
	den := new(big.Int).Mul(q96, q96)

	return new(big.Rat).SetFrac(num, den)
}

// ─── PancakeSwap V2 price from swap amounts ───────────────────────────────────

// AmountsToPrice calculates effective price from a swap event.
// Returns token1/token0 price (USDT per WBNB if token0=WBNB, token1=USDT).
func AmountsToPrice(amountIn, amountOut *big.Int, token0IsInput bool) *big.Rat {
	if amountIn.Sign() == 0 {
		return new(big.Rat)
	}
	if token0IsInput {
		// amountIn is token0, amountOut is token1 → price = token1/token0
		return new(big.Rat).SetFrac(amountOut, amountIn)
	}
	// amountIn is token1, amountOut is token0 → price = token1/token0 = amountIn/amountOut
	return new(big.Rat).SetFrac(amountIn, amountOut)
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// LoadBothPrices loads prices for both DEXes for the WBNB/USDT pair.
func LoadBothPrices(ctx context.Context, db kv.RoDB) (*PriceData, *PriceData, error) {
	tx, err := db.BeginRo(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	uniPrice, err := LoadPrice(tx, DEXUniswapV3, UniswapV3PoolWBNB_USDT_500)
	if err != nil {
		return nil, nil, fmt.Errorf("uniswap price: %w", err)
	}
	pcakePrice, err := LoadPrice(tx, DEXPancakeV2, PancakeV2PoolWBNB_USDT)
	if err != nil {
		return nil, nil, fmt.Errorf("pancakeswap price: %w", err)
	}
	return uniPrice, pcakePrice, nil
}
