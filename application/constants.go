package application

// ─── Chain IDs ───────────────────────────────────────────────────────────────

const (
	BNBChainID uint64 = 56
)

// ─── Token Addresses (BNB Chain) ─────────────────────────────────────────────

const (
	WBNB = "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c"
	USDT = "0x55d398326f99059fF775485246999027B3197955" // 18 decimals on BNB
	USDC = "0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d" // 18 decimals on BNB
	BUSD = "0xe9e7CEA3DedcA5984780Bafc599bD69ADd087D56"
	CAKE = "0x0E09FaBB73Bd3Ade0a17ECC321fD13a19e81cE82"
)

// ─── DEX Contracts (BNB Chain) ────────────────────────────────────────────────

const (
	// Uniswap V3 on BNB Chain
	UniswapV3RouterBNB  = "0xB971eF87ede563556b2ED4b1C0b0019111Dd85d2"
	UniswapV3FactoryBNB = "0xdB1d10011AD0Ff90774D0C6Bb92e5C5c8b4461F"

	// PancakeSwap V2 on BNB Chain
	PancakeV2RouterBNB  = "0x10ED43C718714eb63d5aA57B78B54704E256024E"
	PancakeV2FactoryBNB = "0xcA143Ce32Fe78f1f7019d7d551a6402fC5350c73"

	// Aave V3 Flash Loan Provider on BNB Chain
	AaveV3PoolBNB = "0x6807dc923806fE8Fd134338EABCA509979a7e07"

	// Our deployed ArbitrageExecutor contract (fill after deploy)
	ArbitrageExecutorBNB = ""
)

// ─── Monitored Pools ─────────────────────────────────────────────────────────
// Pool addresses for specific pairs. Set after querying factory.getPool / factory.getPair

const (
	// Uniswap V3: WBNB/USDT 0.05% fee tier
	UniswapV3PoolWBNB_USDT_500 = "0x36696169C63e42cd08ce11f5deeBbCeBae652050"

	// PancakeSwap V2: WBNB/USDT
	PancakeV2PoolWBNB_USDT = "0x16b9a82891338f9bA80E2D6970FddA79D1eb0daE"
)

// ─── DEX Event Signatures (topic0) ───────────────────────────────────────────

const (
	// Uniswap V3: Swap(address,address,int256,int256,uint160,uint128,int24)
	UniswapV3SwapTopic = "0xc42079f94a6350d7e6235f29174924f928cc2ac818eb64fed8004e115fbcca67"

	// PancakeSwap V2: Swap(address,uint256,uint256,uint256,uint256,address)
	PancakeV2SwapTopic = "0xd78ad95fa46c994b6551d0da85fc275fe613ce37657fb8d5e3d130840159d822"
)

// ─── Fee Tiers (in basis points) ─────────────────────────────────────────────

const (
	UniswapV3Fee500BPS = 5   // 0.05%
	PancakeV2FeeBPS    = 25  // 0.25%
	AaveFlashLoanBPS   = 5   // 0.05%
	MinProfitBPS       = 10  // 0.10% minimum net profit to trigger arbitrage
)

// ─── DEX identifiers ─────────────────────────────────────────────────────────

const (
	DEXUniswapV3   = "uniswap_v3"
	DEXPancakeV2   = "pancake_v2"
)

// ─── Arbitrage direction ──────────────────────────────────────────────────────

const (
	DirUniswapToPancake uint8 = 0 // buy on Uniswap, sell on PancakeSwap
	DirPancakeToUniswap uint8 = 1 // buy on PancakeSwap, sell on Uniswap
)
