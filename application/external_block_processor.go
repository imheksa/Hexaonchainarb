package application

import (
	"context"
	"encoding/json"
	"math/big"
	"strings"

	"github.com/0xAtelerix/sdk/gosdk"
	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/kv"
	evmtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/rs/zerolog/log"
)

// ExtBlockProcessor is the core of the arbitrage bot.
// It is called by the Pelagos runtime for every external BNB Chain block.
// It parses DEX swap events, updates prices, detects arbitrage, and emits ExternalTransactions.
type ExtBlockProcessor struct {
	msa gosdk.MultichainStateAccessor
}

func NewExtBlockProcessor(msa gosdk.MultichainStateAccessor) *ExtBlockProcessor {
	return &ExtBlockProcessor{msa: msa}
}

// ProcessBlock is called by the Pelagos SDK for each external block in a batch.
func (p *ExtBlockProcessor) ProcessBlock(b apptypes.ExternalBlock, dbTx kv.RwTx) ([]apptypes.ExternalTransaction, error) {
	if b.ChainID != BNBChainID {
		return nil, nil
	}
	return p.processBNBBlock(b, dbTx)
}

func (p *ExtBlockProcessor) processBNBBlock(b apptypes.ExternalBlock, dbTx kv.RwTx) ([]apptypes.ExternalTransaction, error) {
	receipts, err := p.msa.EVMReceipts(context.Background(), b)
	if err != nil {
		return nil, err
	}

	var priceUpdated bool

	for _, receipt := range receipts {
		for _, vlog := range receipt.Logs {
			if len(vlog.Topics) == 0 {
				continue
			}
			topic0 := vlog.Topics[0].Hex()
			addr := strings.ToLower(vlog.Address.Hex())

			switch {
			case topic0 == UniswapV3SwapTopic && strings.EqualFold(addr, UniswapV3PoolWBNB_USDT_500):
				if err := p.processUniswapV3Swap(dbTx, vlog, b.BlockNumber); err != nil {
					log.Warn().Err(err).Msg("Failed to process Uniswap V3 swap")
				} else {
					priceUpdated = true
				}

			case topic0 == PancakeV2SwapTopic && strings.EqualFold(addr, PancakeV2PoolWBNB_USDT):
				if err := p.processPancakeV2Swap(dbTx, vlog, b.BlockNumber); err != nil {
					log.Warn().Err(err).Msg("Failed to process PancakeSwap V2 swap")
				} else {
					priceUpdated = true
				}
			}
		}
	}

	if !priceUpdated {
		return nil, nil
	}

	// Check for arbitrage opportunity after price update
	return p.checkAndTriggerArbitrage(dbTx, b.BlockNumber)
}

// ─── Uniswap V3 Swap Event ────────────────────────────────────────────────────
//
// event Swap(
//   address indexed sender,
//   address indexed recipient,
//   int256  amount0,       ← non-indexed data[0]
//   int256  amount1,       ← non-indexed data[1]
//   uint160 sqrtPriceX96, ← non-indexed data[2]
//   uint128 liquidity,     ← non-indexed data[3]
//   int24   tick           ← non-indexed data[4]
// )

func (p *ExtBlockProcessor) processUniswapV3Swap(dbTx kv.RwTx, vlog evmtypes.Log, blockNum uint64) error {
	if len(vlog.Data) < 5*32 {
		return ErrMissingParameters
	}

	// sqrtPriceX96 is at offset 64 (bytes 64..96)
	sqrtPriceX96 := new(big.Int).SetBytes(vlog.Data[64:96])
	price := SqrtPriceX96ToPrice(sqrtPriceX96)

	num := price.Num()
	den := price.Denom()

	log.Debug().
		Str("dex", DEXUniswapV3).
		Str("sqrt_price", sqrtPriceX96.String()).
		Str("price_num", num.String()).
		Str("price_den", den.String()).
		Uint64("block", blockNum).
		Msg("Uniswap V3 price updated")

	return StorePrice(dbTx, PriceData{
		DEX:      DEXUniswapV3,
		Pool:     UniswapV3PoolWBNB_USDT_500,
		Token0:   WBNB,
		Token1:   USDT,
		PriceNum: num.String(),
		PriceDen: den.String(),
		BlockNum: blockNum,
	})
}

// ─── PancakeSwap V2 Swap Event ────────────────────────────────────────────────
//
// event Swap(
//   address indexed sender,
//   uint256 amount0In,  ← data[0]
//   uint256 amount1In,  ← data[1]
//   uint256 amount0Out, ← data[2]
//   uint256 amount1Out, ← data[3]
//   address indexed to
// )
// token0 = WBNB, token1 = USDT

func (p *ExtBlockProcessor) processPancakeV2Swap(dbTx kv.RwTx, vlog evmtypes.Log, blockNum uint64) error {
	if len(vlog.Data) < 4*32 {
		return ErrMissingParameters
	}

	amount0In := new(big.Int).SetBytes(vlog.Data[0:32])
	amount1In := new(big.Int).SetBytes(vlog.Data[32:64])
	amount0Out := new(big.Int).SetBytes(vlog.Data[64:96])
	amount1Out := new(big.Int).SetBytes(vlog.Data[96:128])

	// Determine direction and calculate price
	var price *big.Rat
	if amount0In.Sign() > 0 && amount1Out.Sign() > 0 {
		// WBNB in, USDT out → price = USDT/WBNB = amount1Out/amount0In
		price = AmountsToPrice(amount0In, amount1Out, true)
	} else if amount1In.Sign() > 0 && amount0Out.Sign() > 0 {
		// USDT in, WBNB out → price = USDT/WBNB = amount1In/amount0Out
		price = AmountsToPrice(amount1In, amount0Out, false)
	} else {
		return nil // unknown direction
	}

	num := price.Num()
	den := price.Denom()

	log.Debug().
		Str("dex", DEXPancakeV2).
		Str("price_num", num.String()).
		Str("price_den", den.String()).
		Uint64("block", blockNum).
		Msg("PancakeSwap V2 price updated")

	return StorePrice(dbTx, PriceData{
		DEX:      DEXPancakeV2,
		Pool:     PancakeV2PoolWBNB_USDT,
		Token0:   WBNB,
		Token1:   USDT,
		PriceNum: num.String(),
		PriceDen: den.String(),
		BlockNum: blockNum,
	})
}

// ─── Arbitrage Detection & Trigger ───────────────────────────────────────────

func (p *ExtBlockProcessor) checkAndTriggerArbitrage(dbTx kv.RwTx, blockNum uint64) ([]apptypes.ExternalTransaction, error) {
	// Check if bot is enabled
	enabledRaw, _ := dbTx.GetOne(ConfigBucket, []byte("enabled"))
	if string(enabledRaw) != "true" {
		return nil, nil
	}

	// Load runtime config
	minProfitBPS := int64(MinProfitBPS)
	if raw, _ := dbTx.GetOne(ConfigBucket, []byte("min_profit_bps")); len(raw) > 0 {
		json.Unmarshal(raw, &minProfitBPS) //nolint:errcheck
	}

	maxBorrowWei := new(big.Int)
	maxBorrowWei.SetString("10000000000000000000000", 10) // default 10,000 USDT
	if raw, _ := dbTx.GetOne(ConfigBucket, []byte("max_borrow_wei")); len(raw) > 0 {
		maxBorrowWei.SetString(string(raw), 10) //nolint:errcheck
	}

	// Load latest prices
	uniPrice, err := LoadPrice(dbTx, DEXUniswapV3, UniswapV3PoolWBNB_USDT_500)
	if err != nil {
		return nil, nil // no price data yet, wait
	}
	pcakePrice, err := LoadPrice(dbTx, DEXPancakeV2, PancakeV2PoolWBNB_USDT)
	if err != nil {
		return nil, nil
	}

	// Detect opportunity
	opp := DetectOpportunity(uniPrice, pcakePrice, minProfitBPS, maxBorrowWei)
	if opp == nil {
		return nil, nil
	}

	// Record in DB
	if err := RecordOpportunity(dbTx, blockNum, opp); err != nil {
		log.Warn().Err(err).Msg("Failed to record opportunity")
	}

	// Build ExternalTransaction → Pelagos will TSS-sign and submit to BNB Chain
	extTx, err := BuildExternalTx(opp)
	if err != nil {
		log.Error().Err(err).Msg("Failed to build external transaction")
		return nil, nil
	}

	log.Info().
		Str("buy_dex", opp.BuyDEX).
		Str("sell_dex", opp.SellDEX).
		Int64("profit_bps", opp.ProfitBPS).
		Uint64("block", blockNum).
		Msg("Arbitrage ExternalTransaction emitted")

	return []apptypes.ExternalTransaction{extTx}, nil
}
