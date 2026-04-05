package api

import (
	"context"
	"encoding/binary"
	"encoding/json"

	"github.com/0xAtelerix/sdk/gosdk/kv"
	"github.com/0xAtelerix/sdk/gosdk/rpc"
	app "hexaonchainarb/application"
)

// CustomRPC registers arbitrage-specific JSON-RPC methods.
type CustomRPC struct {
	rpcServer *rpc.StandardRPCServer
	db        kv.RoDB
}

func New(rpcServer *rpc.StandardRPCServer, db kv.RoDB) *CustomRPC {
	return &CustomRPC{rpcServer: rpcServer, db: db}
}

func (c *CustomRPC) AddRPCMethods() {
	c.rpcServer.AddMethod("arb_getPrices", c.GetPrices)
	c.rpcServer.AddMethod("arb_getOpportunities", c.GetOpportunities)
	c.rpcServer.AddMethod("arb_getStats", c.GetStats)
	c.rpcServer.AddMethod("arb_getConfig", c.GetConfig)
}

// ─── arb_getPrices ────────────────────────────────────────────────────────────

type PricesResponse struct {
	UniswapV3  *app.PriceData `json:"uniswap_v3"`
	PancakeV2  *app.PriceData `json:"pancake_v2"`
	SpreadBPS  float64        `json:"spread_bps"`
}

func (c *CustomRPC) GetPrices(ctx context.Context, _ []any) (any, error) {
	tx, err := c.db.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	uniPrice, err := app.LoadPrice(tx, app.DEXUniswapV3, app.UniswapV3PoolWBNB_USDT_500)
	if err != nil {
		return nil, err
	}
	pcakePrice, err := app.LoadPrice(tx, app.DEXPancakeV2, app.PancakeV2PoolWBNB_USDT)
	if err != nil {
		return nil, err
	}

	uniRat := uniPrice.Price()
	pcakeRat := pcakePrice.Price()

	var spreadBPS float64
	if pcakeRat.Sign() > 0 {
		// spread = |uniPrice - pcakePrice| / min(uniPrice, pcakePrice) * 10000
		diff := new(big.Rat).Sub(uniRat, pcakeRat)
		if diff.Sign() < 0 {
			diff.Neg(diff)
		}
		minPrice := pcakeRat
		if uniRat.Cmp(pcakeRat) < 0 {
			minPrice = uniRat
		}
		spreadRat := new(big.Rat).Quo(diff, minPrice)
		spreadRat.Mul(spreadRat, new(big.Rat).SetInt64(10000))
		f, _ := spreadRat.Float64()
		spreadBPS = f
	}

	return PricesResponse{
		UniswapV3: uniPrice,
		PancakeV2: pcakePrice,
		SpreadBPS: spreadBPS,
	}, nil
}

// ─── arb_getOpportunities ─────────────────────────────────────────────────────

type OpportunityRecord struct {
	BlockNum uint64 `json:"block_num"`
	Data     any    `json:"data"`
}

func (c *CustomRPC) GetOpportunities(ctx context.Context, params []any) (any, error) {
	limit := 20
	if len(params) > 0 {
		if l, ok := params[0].(float64); ok && l > 0 {
			limit = int(l)
		}
	}

	tx, err := c.db.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	cursor, err := tx.Cursor(app.OpportunitiesBucket)
	if err != nil {
		return nil, err
	}
	defer cursor.Close()

	var results []OpportunityRecord
	// Iterate in reverse (latest first)
	for k, v, err := cursor.Last(); err == nil && k != nil && len(results) < limit; k, v, err = cursor.Prev() {
		blockNum := binary.BigEndian.Uint64(k)
		var data any
		json.Unmarshal(v, &data) //nolint:errcheck
		results = append(results, OpportunityRecord{BlockNum: blockNum, Data: data})
	}

	return results, nil
}

// ─── arb_getStats ─────────────────────────────────────────────────────────────

type StatsResponse struct {
	TotalTrades    int64  `json:"total_trades"`
	TotalProfitWei string `json:"total_profit_wei"`
	FailedTrades   int64  `json:"failed_trades"`
}

func (c *CustomRPC) GetStats(ctx context.Context, _ []any) (any, error) {
	tx, err := c.db.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	readInt64 := func(key string) int64 {
		raw, _ := tx.GetOne(app.StatsBucket, []byte(key))
		if len(raw) == 8 {
			return int64(binary.BigEndian.Uint64(raw))
		}
		return 0
	}

	profitRaw, _ := tx.GetOne(app.StatsBucket, []byte("total_profit_wei"))

	return StatsResponse{
		TotalTrades:    readInt64("total_trades"),
		TotalProfitWei: string(profitRaw),
		FailedTrades:   readInt64("failed_trades"),
	}, nil
}

// ─── arb_getConfig ────────────────────────────────────────────────────────────

type ConfigResponse struct {
	Enabled      string `json:"enabled"`
	MinProfitBPS int64  `json:"min_profit_bps"`
	MaxBorrowWei string `json:"max_borrow_wei"`
}

func (c *CustomRPC) GetConfig(ctx context.Context, _ []any) (any, error) {
	tx, err := c.db.BeginRo(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	enabled, _ := tx.GetOne(app.ConfigBucket, []byte("enabled"))
	maxBorrow, _ := tx.GetOne(app.ConfigBucket, []byte("max_borrow_wei"))

	var minProfit int64
	if raw, _ := tx.GetOne(app.ConfigBucket, []byte("min_profit_bps")); len(raw) > 0 {
		json.Unmarshal(raw, &minProfit) //nolint:errcheck
	}

	return ConfigResponse{
		Enabled:      string(enabled),
		MinProfitBPS: minProfit,
		MaxBorrowWei: string(maxBorrow),
	}, nil
}
