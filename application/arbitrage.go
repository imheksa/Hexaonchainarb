package application

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/big"

	"github.com/0xAtelerix/sdk/gosdk/apptypes"
	"github.com/0xAtelerix/sdk/gosdk/external"
	"github.com/0xAtelerix/sdk/gosdk/kv"
	"github.com/0xAtelerix/sdk/gosdk/library"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/rs/zerolog/log"
)

// ArbitrageOpportunity describes a profitable swap route.
type ArbitrageOpportunity struct {
	BuyDEX        string
	SellDEX       string
	BuyPool       string
	SellPool      string
	Token0        string // WBNB
	Token1        string // USDT (borrow token)
	PriceBuy      *big.Rat // token1/token0 on the buy DEX (cheaper)
	PriceSell     *big.Rat // token1/token0 on the sell DEX (more expensive)
	Direction     uint8    // DirUniswapToPancake or DirPancakeToUniswap
	BorrowAmtWei  *big.Int // how much token1 (USDT) to flash-borrow
	ProfitBPS     int64    // expected profit in basis points
}

// ─── Detection ───────────────────────────────────────────────────────────────

// DetectOpportunity checks two price feeds and returns an opportunity if profitable.
// uniPrice and pcakePrice are token1/token0 (USDT per WBNB).
func DetectOpportunity(uniPrice, pcakePrice *PriceData, minProfitBPS int64, maxBorrowWei *big.Int) *ArbitrageOpportunity {
	uniRat := uniPrice.Price()
	pcakeRat := pcakePrice.Price()

	if uniRat.Sign() == 0 || pcakeRat.Sign() == 0 {
		return nil
	}

	// Try direction: buy on PancakeSwap (cheaper), sell on Uniswap (higher price)
	// profitBPS when WBNB price is higher on Uniswap:
	// net = sellPrice*(1-uniFee) - buyPrice*(1+pcakeFee) - flashLoanFee
	// expressed as BPS relative to buyPrice
	if opp := calcOpportunity(
		pcakePrice, uniPrice,
		DirPancakeToUniswap,
		PancakeV2FeeBPS, UniswapV3Fee500BPS,
		minProfitBPS, maxBorrowWei,
	); opp != nil {
		return opp
	}

	// Try direction: buy on Uniswap (cheaper), sell on PancakeSwap (higher price)
	if opp := calcOpportunity(
		uniPrice, pcakePrice,
		DirUniswapToPancake,
		UniswapV3Fee500BPS, PancakeV2FeeBPS,
		minProfitBPS, maxBorrowWei,
	); opp != nil {
		return opp
	}

	return nil
}

// calcOpportunity returns an opportunity if buying on buyDEX and selling on sellDEX is profitable.
// buyFeeBPS and sellFeeBPS are the DEX fees in basis points.
func calcOpportunity(buy, sell *PriceData, direction uint8, buyFeeBPS, sellFeeBPS int, minProfitBPS int64, maxBorrowWei *big.Int) *ArbitrageOpportunity {
	buyPrice := buy.Price()  // USDT per WBNB on buy DEX
	sellPrice := sell.Price() // USDT per WBNB on sell DEX

	// We want: sellPrice * (1 - sellFee) > buyPrice * (1 + flashLoanFee)
	// i.e., sellPrice/buyPrice > (1 + flashLoanFee) / (1 - sellFee)
	// For buy: we're paying buyPrice * (1 + buyFee) per WBNB (slippage approximated as fee cost)
	// For sell: we receive sellPrice * (1 - sellFee) per WBNB

	// profit per WBNB (in USDT units) =
	//   sellPrice*(10000-sellFeeBPS)/10000 - buyPrice*(10000+buyFeeBPS)/10000 - flashLoanFee*borrow/10000
	// We simplify: treat borrow as buyPrice amount, then:
	//   effectiveSell = sellPrice * (1 - sellFee/10000)
	//   effectiveBuy  = buyPrice  * (1 + buyFee/10000)
	//   flashFee      = AaveFlashLoanBPS / 10000
	// profitBPS = (effectiveSell/effectiveBuy - 1 - flashFee) * 10000

	bps := int64(10000)
	sellMul := new(big.Rat).SetFrac(big.NewInt(bps-int64(sellFeeBPS)), big.NewInt(bps))
	buyMul := new(big.Rat).SetFrac(big.NewInt(bps+int64(buyFeeBPS)), big.NewInt(bps))
	flashMul := new(big.Rat).SetFrac(big.NewInt(int64(AaveFlashLoanBPS)), big.NewInt(bps))

	effectiveSell := new(big.Rat).Mul(sellPrice, sellMul)
	effectiveBuy := new(big.Rat).Mul(buyPrice, buyMul)

	if effectiveBuy.Sign() == 0 {
		return nil
	}

	// ratio = effectiveSell / effectiveBuy
	ratio := new(big.Rat).Quo(effectiveSell, effectiveBuy)

	// profitRat = ratio - 1 - flashFee
	one := new(big.Rat).SetInt64(1)
	profitRat := new(big.Rat).Sub(ratio, one)
	profitRat.Sub(profitRat, flashMul)

	// profitBPS = profitRat * 10000
	profitBPSRat := new(big.Rat).Mul(profitRat, new(big.Rat).SetInt64(bps))
	profitBPSInt, _ := profitBPSRat.Float64()
	profitBPS := int64(profitBPSInt)

	if profitBPS < minProfitBPS {
		return nil
	}

	// Borrow amount: use maxBorrowWei (or a portion based on pool depth in future)
	borrowAmt := new(big.Int).Set(maxBorrowWei)

	log.Info().
		Str("buy_dex", buy.DEX).
		Str("sell_dex", sell.DEX).
		Int64("profit_bps", profitBPS).
		Str("borrow_amt", borrowAmt.String()).
		Msg("Arbitrage opportunity detected")

	return &ArbitrageOpportunity{
		BuyDEX:       buy.DEX,
		SellDEX:      sell.DEX,
		BuyPool:      buy.Pool,
		SellPool:     sell.Pool,
		Token0:       buy.Token0,
		Token1:       buy.Token1,
		PriceBuy:     buyPrice,
		PriceSell:    sellPrice,
		Direction:    direction,
		BorrowAmtWei: borrowAmt,
		ProfitBPS:    profitBPS,
	}
}

// ─── Execution via Pelagos ExternalTransaction ───────────────────────────────

// BuildExternalTx encodes the call to ArbitrageExecutor.executeArbitrage()
// and wraps it in a Pelagos ExternalTransaction for TSS signing.
func BuildExternalTx(opp *ArbitrageOpportunity) (apptypes.ExternalTransaction, error) {
	if ArbitrageExecutorBNB == "" {
		return apptypes.ExternalTransaction{}, ErrContractNotDeployed
	}

	payload, err := encodeExecuteArbitrage(
		common.HexToAddress(opp.Token1),    // tokenBorrow (USDT)
		opp.BorrowAmtWei,                   // amount
		common.HexToAddress(opp.Token0),    // tokenTarget (WBNB)
		uint32(UniswapV3Fee500BPS*10),      // uniswapFee (500 = 0.05% in Uniswap notation)
		opp.Direction,                      // direction
	)
	if err != nil {
		return apptypes.ExternalTransaction{}, fmt.Errorf("%w: %v", ErrABIEncoding, err)
	}

	return external.NewExTxBuilder(payload, library.ChainID(BNBChainID)).Build()
}

// encodeExecuteArbitrage manually ABI-encodes the call:
//   executeArbitrage(address tokenBorrow, uint256 amount, address tokenTarget, uint24 uniswapFee, uint8 direction)
func encodeExecuteArbitrage(tokenBorrow common.Address, amount *big.Int, tokenTarget common.Address, uniswapFee uint32, direction uint8) ([]byte, error) {
	// Method selector: keccak256("executeArbitrage(address,uint256,address,uint24,uint8)")[:4]
	sig := crypto.Keccak256([]byte("executeArbitrage(address,uint256,address,uint24,uint8)"))
	selector := sig[:4]

	// ABI encoding: each param padded to 32 bytes
	encoded := make([]byte, 0, 4+5*32)
	encoded = append(encoded, selector...)

	// address tokenBorrow (12 zero bytes + 20 address bytes)
	encoded = append(encoded, padLeft(tokenBorrow.Bytes(), 32)...)

	// uint256 amount (32 bytes, big-endian)
	amtBytes := amount.Bytes()
	encoded = append(encoded, padLeft(amtBytes, 32)...)

	// address tokenTarget
	encoded = append(encoded, padLeft(tokenTarget.Bytes(), 32)...)

	// uint24 uniswapFee
	feeBuf := make([]byte, 32)
	binary.BigEndian.PutUint32(feeBuf[28:], uniswapFee)
	encoded = append(encoded, feeBuf...)

	// uint8 direction
	dirBuf := make([]byte, 32)
	dirBuf[31] = direction
	encoded = append(encoded, dirBuf...)

	return encoded, nil
}

func padLeft(b []byte, size int) []byte {
	padded := make([]byte, size)
	copy(padded[size-len(b):], b)
	return padded
}

// ─── Stats persistence ────────────────────────────────────────────────────────

// RecordOpportunity stores the detected opportunity in DB for audit/API access.
func RecordOpportunity(dbTx kv.RwTx, blockNum uint64, opp *ArbitrageOpportunity) error {
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, blockNum)

	val := fmt.Sprintf(
		`{"buy":"%s","sell":"%s","profit_bps":%d,"borrow":"%s"}`,
		opp.BuyDEX, opp.SellDEX, opp.ProfitBPS, opp.BorrowAmtWei.String(),
	)
	return dbTx.Put(OpportunitiesBucket, key, []byte(val))
}

// IncrementStat adds delta to a numeric stat stored in StatsBucket.
func IncrementStat(ctx context.Context, db kv.RwDB, key string, delta int64) error {
	tx, err := db.BeginRw(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	raw, _ := tx.GetOne(StatsBucket, []byte(key))
	var current int64
	if len(raw) == 8 {
		current = int64(binary.BigEndian.Uint64(raw))
	}
	current += delta

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, uint64(current))
	if err := tx.Put(StatsBucket, []byte(key), buf); err != nil {
		return err
	}
	return tx.Commit()
}
