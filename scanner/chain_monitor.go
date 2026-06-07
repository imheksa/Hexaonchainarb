package scanner

import (
	"context"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/rs/zerolog/log"
)

const (
	topicUniV3Swap = "0xc42079f94a6350d7e6235f29174924f928cc2ac818eb64fed8004e115fbcca67"
	topicV2Swap    = "0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822"

	reconnectDelay = 5 * time.Second
)

// poolMeta holds pre-computed metadata for a monitored pool.
type poolMeta struct {
	pool      PoolConfig
	pair      PairConfig
	chainID   uint64
	chainName string
}

// ChainMonitor subscribes to swap logs on a single EVM chain via WebSocket
// and writes updated prices to the shared PriceStore.
type ChainMonitor struct {
	cfg      ChainConfig
	store    *PriceStore
	onUpdate func()
	pools    map[string]*poolMeta // lowercase address → meta
}

func NewChainMonitor(cfg ChainConfig, store *PriceStore, onUpdate func()) *ChainMonitor {
	pools := make(map[string]*poolMeta, 16)
	for _, pair := range cfg.Pairs {
		for _, pool := range pair.Pools {
			addr := strings.ToLower(pool.Address)
			pools[addr] = &poolMeta{
				pool:      pool,
				pair:      pair,
				chainID:   cfg.ChainID,
				chainName: cfg.Name,
			}
		}
	}
	return &ChainMonitor{cfg: cfg, store: store, onUpdate: onUpdate, pools: pools}
}

// Run keeps the subscription alive, reconnecting on any error.
func (m *ChainMonitor) Run(ctx context.Context) {
	for {
		if err := m.subscribe(ctx); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Warn().
				Err(err).
				Str("chain", m.cfg.Name).
				Dur("retry_in", reconnectDelay).
				Msg("Chain monitor disconnected, reconnecting")
			select {
			case <-ctx.Done():
				return
			case <-time.After(reconnectDelay):
			}
		}
	}
}

func (m *ChainMonitor) subscribe(ctx context.Context) error {
	client, err := ethclient.DialContext(ctx, m.cfg.WSS)
	if err != nil {
		return err
	}
	defer client.Close()

	log.Info().
		Str("chain", m.cfg.Name).
		Uint64("chain_id", m.cfg.ChainID).
		Int("pools", len(m.pools)).
		Msg("Chain monitor connected")

	// Build address list for FilterQuery
	addrs := make([]common.Address, 0, len(m.pools))
	for addr := range m.pools {
		addrs = append(addrs, common.HexToAddress(addr))
	}

	query := ethereum.FilterQuery{
		Addresses: addrs,
		Topics: [][]common.Hash{{
			common.HexToHash(topicUniV3Swap),
			common.HexToHash(topicV2Swap),
		}},
	}

	logCh := make(chan ethtypes.Log, 128)
	sub, err := client.SubscribeFilterLogs(ctx, query, logCh)
	if err != nil {
		return err
	}
	defer sub.Unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-sub.Err():
			return err
		case vlog := <-logCh:
			m.processLog(vlog)
		}
	}
}

func (m *ChainMonitor) processLog(vlog ethtypes.Log) {
	if len(vlog.Topics) == 0 {
		return
	}
	addr := strings.ToLower(vlog.Address.Hex())
	meta, ok := m.pools[addr]
	if !ok {
		return
	}

	topic0 := vlog.Topics[0].Hex()
	var price *big.Rat

	switch topic0 {
	case topicUniV3Swap:
		price = m.parseV3Swap(vlog, meta)
	case topicV2Swap:
		price = m.parseV2Swap(vlog, meta)
	}

	if price == nil || price.Sign() == 0 {
		return
	}

	key := PriceKey{
		ChainID: meta.chainID,
		DEX:     meta.pool.DEX,
		Pool:    addr,
	}

	m.store.SetPrice(key, &PriceData{
		Key:       key,
		Symbol:    meta.pair.Symbol,
		ChainName: meta.chainName,
		Token0:    meta.pair.Token0,
		Token1:    meta.pair.Token1,
		PriceNum:  new(big.Int).Set(price.Num()),
		PriceDen:  new(big.Int).Set(price.Denom()),
		FeeBPS:    meta.pool.FeeBPS,
		BlockNum:  vlog.BlockNumber,
		UpdatedAt: time.Now(),
	})

	pf, _ := price.Float64()
	log.Debug().
		Str("chain", meta.chainName).
		Str("dex", meta.pool.DEX).
		Str("symbol", meta.pair.Symbol).
		Float64("price", pf).
		Uint64("block", vlog.BlockNumber).
		Msg("Price updated")

	if m.onUpdate != nil {
		m.onUpdate()
	}
}

// parseV3Swap decodes a Uniswap V3 Swap event and returns token1/token0 price.
//
//	Swap(address indexed sender, address indexed recipient,
//	     int256 amount0, int256 amount1, uint160 sqrtPriceX96, uint128 liquidity, int24 tick)
//	non-indexed data layout: [amount0(32), amount1(32), sqrtPriceX96(32), liquidity(32), tick(32)]
func (m *ChainMonitor) parseV3Swap(vlog ethtypes.Log, meta *poolMeta) *big.Rat {
	if len(vlog.Data) < 5*32 {
		return nil
	}
	sqrtPriceX96 := new(big.Int).SetBytes(vlog.Data[64:96])
	return SqrtPriceX96ToPrice(sqrtPriceX96, meta.pair.Token0Decimals, meta.pair.Token1Decimals)
}

// parseV2Swap decodes a Uniswap V2/PancakeSwap V2 Swap event and returns token1/token0 price.
//
//	Swap(address indexed sender,
//	     uint256 amount0In, uint256 amount1In, uint256 amount0Out, uint256 amount1Out,
//	     address indexed to)
func (m *ChainMonitor) parseV2Swap(vlog ethtypes.Log, meta *poolMeta) *big.Rat {
	if len(vlog.Data) < 4*32 {
		return nil
	}
	amount0In := new(big.Int).SetBytes(vlog.Data[0:32])
	amount1In := new(big.Int).SetBytes(vlog.Data[32:64])
	amount0Out := new(big.Int).SetBytes(vlog.Data[64:96])
	amount1Out := new(big.Int).SetBytes(vlog.Data[96:128])

	switch {
	case amount0In.Sign() > 0 && amount1Out.Sign() > 0:
		// token0 in, token1 out
		return AmountsToPrice(amount0In, amount1Out, true,
			meta.pair.Token0Decimals, meta.pair.Token1Decimals)
	case amount1In.Sign() > 0 && amount0Out.Sign() > 0:
		// token1 in, token0 out
		return AmountsToPrice(amount1In, amount0Out, false,
			meta.pair.Token0Decimals, meta.pair.Token1Decimals)
	}
	return nil
}
