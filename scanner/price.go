package scanner

import "math/big"

// SqrtPriceX96ToPrice converts a Uniswap V3 sqrtPriceX96 value to a
// token1/token0 price as a *big.Rat, adjusted for token decimal differences.
func SqrtPriceX96ToPrice(sqrtPriceX96 *big.Int, token0Decimals, token1Decimals int) *big.Rat {
	q96 := new(big.Int).Lsh(big.NewInt(1), 96)
	num := new(big.Int).Mul(sqrtPriceX96, sqrtPriceX96) // sqrtP^2
	den := new(big.Int).Mul(q96, q96)                    // 2^192

	price := new(big.Rat).SetFrac(num, den)
	adjustDecimals(price, token0Decimals, token1Decimals)
	return price
}

// AmountsToPrice calculates token1/token0 price from a V2 swap event amounts,
// adjusted for token decimal differences.
func AmountsToPrice(amountIn, amountOut *big.Int, token0IsInput bool, token0Decimals, token1Decimals int) *big.Rat {
	if amountIn.Sign() == 0 || amountOut.Sign() == 0 {
		return new(big.Rat)
	}
	var price *big.Rat
	if token0IsInput {
		// token0 in → token1 out: price = token1/token0
		price = new(big.Rat).SetFrac(amountOut, amountIn)
	} else {
		// token1 in → token0 out: price = token1/token0 = amountIn/amountOut
		price = new(big.Rat).SetFrac(amountIn, amountOut)
	}
	adjustDecimals(price, token0Decimals, token1Decimals)
	return price
}

// adjustDecimals multiplies price by 10^(token0Dec - token1Dec) so the result
// is always expressed in "human" units (e.g. USDT per BNB).
func adjustDecimals(price *big.Rat, token0Dec, token1Dec int) {
	diff := token0Dec - token1Dec
	if diff == 0 {
		return
	}
	exp := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(abs(diff))), nil)
	adj := new(big.Rat).SetInt(exp)
	if diff > 0 {
		price.Mul(price, adj)
	} else {
		price.Quo(price, adj)
	}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
