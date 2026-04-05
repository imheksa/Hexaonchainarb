// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

// ─── Interfaces ───────────────────────────────────────────────────────────────

interface IERC20 {
    function approve(address spender, uint256 amount) external returns (bool);
    function transfer(address to, uint256 amount) external returns (bool);
    function balanceOf(address account) external view returns (uint256);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
}

// Aave V3 Flash Loan (Simple — borrows one asset, single callback)
interface IPool {
    function flashLoanSimple(
        address receiverAddress,
        address asset,
        uint256 amount,
        bytes calldata params,
        uint16 referralCode
    ) external;
}

interface IFlashLoanSimpleReceiver {
    function executeOperation(
        address asset,
        uint256 amount,
        uint256 premium,
        address initiator,
        bytes calldata params
    ) external returns (bool);
}

// Uniswap V3 Router (exactInputSingle)
interface IUniswapV3Router {
    struct ExactInputSingleParams {
        address tokenIn;
        address tokenOut;
        uint24  fee;
        address recipient;
        uint256 amountIn;
        uint256 amountOutMinimum;
        uint160 sqrtPriceLimitX96;
    }
    function exactInputSingle(ExactInputSingleParams calldata params)
        external returns (uint256 amountOut);
}

// PancakeSwap V2 Router
interface IPancakeV2Router {
    function swapExactTokensForTokens(
        uint256 amountIn,
        uint256 amountOutMin,
        address[] calldata path,
        address to,
        uint256 deadline
    ) external returns (uint256[] memory amounts);

    function getAmountsOut(uint256 amountIn, address[] calldata path)
        external view returns (uint256[] memory amounts);
}

// ─── ArbitrageExecutor ───────────────────────────────────────────────────────

/**
 * @title  ArbitrageExecutor
 * @notice Executes flash-loan-powered arbitrage between Uniswap V3 and PancakeSwap V2 on BNB Chain.
 *         Called by the Pelagos Appchain via TSS (Threshold Signature Scheme).
 *
 * Flow:
 *   1. Pelagos Appchain detects price discrepancy (off-chain, via Reactive contracts)
 *   2. Appchain emits ExternalTransaction → Pelagos validators TSS-sign it
 *   3. TSS aggregate key calls executeArbitrage() on this contract
 *   4. Contract requests flash loan from Aave V3
 *   5. In executeOperation() callback:
 *      a. Buy TOKEN on cheaper DEX
 *      b. Sell TOKEN on more expensive DEX
 *      c. Repay flash loan + fee
 *   6. Profit stays in contract → owner withdraws via withdrawProfit()
 */
contract ArbitrageExecutor is IFlashLoanSimpleReceiver {

    // ── State ─────────────────────────────────────────────────────────────────

    address public owner;
    address public pelagosTSSAddress; // Pelagos TSS aggregate key — only caller allowed

    IPool             public aavePool;
    IUniswapV3Router  public uniswapRouter;
    IPancakeV2Router  public pancakeRouter;

    // ── Events ────────────────────────────────────────────────────────────────

    event ArbitrageExecuted(
        address indexed tokenBorrow,
        uint256 amountBorrowed,
        address indexed tokenTarget,
        uint8   direction,
        uint256 profit
    );

    event ProfitWithdrawn(address indexed token, uint256 amount);

    // ── Constructor ───────────────────────────────────────────────────────────

    constructor(
        address _aavePool,          // 0x6807dc923806fE8Fd134338EABCA509979a7e07
        address _uniswapRouter,     // 0xB971eF87ede563556b2ED4b1C0b0019111Dd85d2
        address _pancakeRouter,     // 0x10ED43C718714eb63d5aA57B78B54704E256024E
        address _pelagosTSSAddress  // TSS aggregate key from Pelagos validator set
    ) {
        owner              = msg.sender;
        pelagosTSSAddress  = _pelagosTSSAddress;
        aavePool           = IPool(_aavePool);
        uniswapRouter      = IUniswapV3Router(_uniswapRouter);
        pancakeRouter      = IPancakeV2Router(_pancakeRouter);
    }

    // ── Modifiers ─────────────────────────────────────────────────────────────

    modifier onlyOwner() {
        require(msg.sender == owner, "ArbitrageExecutor: not owner");
        _;
    }

    modifier onlyPelagos() {
        require(
            msg.sender == pelagosTSSAddress || msg.sender == owner,
            "ArbitrageExecutor: caller not authorized (not Pelagos TSS or owner)"
        );
        _;
    }

    // ── Entry Point (called by Pelagos TSS) ──────────────────────────────────

    /**
     * @notice Initiates a flash-loan arbitrage trade.
     * @param tokenBorrow  Token to borrow from Aave (e.g. USDT)
     * @param amount       Amount to borrow (in tokenBorrow's decimals)
     * @param tokenTarget  Token to buy/sell (e.g. WBNB)
     * @param uniswapFee   Uniswap V3 pool fee tier (e.g. 500 for 0.05%)
     * @param direction    0 = buy on Uniswap V3, sell on PancakeSwap V2
     *                     1 = buy on PancakeSwap V2, sell on Uniswap V3
     */
    function executeArbitrage(
        address tokenBorrow,
        uint256 amount,
        address tokenTarget,
        uint24  uniswapFee,
        uint8   direction
    ) external onlyPelagos {
        require(amount > 0, "ArbitrageExecutor: amount must be > 0");

        bytes memory params = abi.encode(tokenTarget, uniswapFee, direction);

        // Request flash loan → Aave calls executeOperation() in the same tx
        aavePool.flashLoanSimple(
            address(this),
            tokenBorrow,
            amount,
            params,
            0 // referral code
        );
    }

    // ── Aave Flash Loan Callback ──────────────────────────────────────────────

    /**
     * @notice Called by Aave V3 pool during the flash loan.
     *         All arbitrage logic executes here atomically.
     */
    function executeOperation(
        address asset,        // tokenBorrow
        uint256 amount,       // borrowed amount
        uint256 premium,      // Aave fee (0.05%)
        address initiator,
        bytes calldata params
    ) external override returns (bool) {
        require(msg.sender == address(aavePool),   "ArbitrageExecutor: caller must be Aave pool");
        require(initiator  == address(this),        "ArbitrageExecutor: initiator must be this contract");

        (address tokenTarget, uint24 uniswapFee, uint8 direction) =
            abi.decode(params, (address, uint24, uint8));

        uint256 amountOwed = amount + premium;
        uint256 balanceBefore = IERC20(asset).balanceOf(address(this));

        if (direction == 0) {
            // ── Direction 0: Buy on Uniswap V3, Sell on PancakeSwap V2 ──────
            _buyOnUniswap(asset, tokenTarget, amount, uniswapFee);
            uint256 received = IERC20(tokenTarget).balanceOf(address(this));
            _sellOnPancake(tokenTarget, asset, received, amountOwed);
        } else {
            // ── Direction 1: Buy on PancakeSwap V2, Sell on Uniswap V3 ──────
            _buyOnPancake(asset, tokenTarget, amount, amountOwed);
            uint256 received = IERC20(tokenTarget).balanceOf(address(this));
            _sellOnUniswap(tokenTarget, asset, received, amountOwed, uniswapFee);
        }

        // Repay Aave flash loan
        IERC20(asset).approve(address(aavePool), amountOwed);

        uint256 balanceAfter = IERC20(asset).balanceOf(address(this));
        uint256 profit = 0;
        if (balanceAfter > balanceBefore) {
            profit = balanceAfter - balanceBefore;
        }

        emit ArbitrageExecuted(asset, amount, tokenTarget, direction, profit);
        return true;
    }

    // ── Internal Swap Helpers ─────────────────────────────────────────────────

    function _buyOnUniswap(
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint24  fee
    ) internal returns (uint256 amountOut) {
        IERC20(tokenIn).approve(address(uniswapRouter), amountIn);
        amountOut = uniswapRouter.exactInputSingle(
            IUniswapV3Router.ExactInputSingleParams({
                tokenIn:           tokenIn,
                tokenOut:          tokenOut,
                fee:               fee,
                recipient:         address(this),
                amountIn:          amountIn,
                amountOutMinimum:  0, // Pelagos Appchain already validated profitability
                sqrtPriceLimitX96: 0
            })
        );
    }

    function _sellOnPancake(
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 amountOutMin
    ) internal {
        IERC20(tokenIn).approve(address(pancakeRouter), amountIn);
        address[] memory path = new address[](2);
        path[0] = tokenIn;
        path[1] = tokenOut;
        pancakeRouter.swapExactTokensForTokens(
            amountIn,
            amountOutMin,
            path,
            address(this),
            block.timestamp
        );
    }

    function _buyOnPancake(
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 amountOutMin // we need at least enough to repay flash loan
    ) internal {
        IERC20(tokenIn).approve(address(pancakeRouter), amountIn);
        address[] memory path = new address[](2);
        path[0] = tokenIn;
        path[1] = tokenOut;
        pancakeRouter.swapExactTokensForTokens(
            amountIn,
            0,
            path,
            address(this),
            block.timestamp
        );
    }

    function _sellOnUniswap(
        address tokenIn,
        address tokenOut,
        uint256 amountIn,
        uint256 amountOutMin,
        uint24  fee
    ) internal {
        IERC20(tokenIn).approve(address(uniswapRouter), amountIn);
        uniswapRouter.exactInputSingle(
            IUniswapV3Router.ExactInputSingleParams({
                tokenIn:           tokenIn,
                tokenOut:          tokenOut,
                fee:               fee,
                recipient:         address(this),
                amountIn:          amountIn,
                amountOutMinimum:  amountOutMin,
                sqrtPriceLimitX96: 0
            })
        );
    }

    // ── Admin Functions ───────────────────────────────────────────────────────

    /// @notice Withdraw accumulated profit to owner wallet.
    function withdrawProfit(address token) external onlyOwner {
        uint256 balance = IERC20(token).balanceOf(address(this));
        require(balance > 0, "ArbitrageExecutor: no balance to withdraw");
        IERC20(token).transfer(owner, balance);
        emit ProfitWithdrawn(token, balance);
    }

    /// @notice Update the Pelagos TSS address if the validator set rotates.
    function updateTSSAddress(address newTSS) external onlyOwner {
        require(newTSS != address(0), "ArbitrageExecutor: zero address");
        pelagosTSSAddress = newTSS;
    }

    /// @notice Transfer contract ownership.
    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "ArbitrageExecutor: zero address");
        owner = newOwner;
    }

    /// @notice Simulate profitability before committing (view function for off-chain checks).
    function estimateProfit(
        address tokenBorrow,
        uint256 amount,
        address tokenTarget,
        uint8   direction
    ) external view returns (uint256 estimatedProfitBPS) {
        address[] memory pathBuy  = new address[](2);
        address[] memory pathSell = new address[](2);

        if (direction == 0) {
            // Uniswap → Pancake: estimate via Pancake getAmountsOut (approximation)
            pathBuy[0]  = tokenBorrow;
            pathBuy[1]  = tokenTarget;
            pathSell[0] = tokenTarget;
            pathSell[1] = tokenBorrow;
        } else {
            pathBuy[0]  = tokenBorrow;
            pathBuy[1]  = tokenTarget;
            pathSell[0] = tokenTarget;
            pathSell[1] = tokenBorrow;
        }

        uint256[] memory buyAmounts  = pancakeRouter.getAmountsOut(amount, pathBuy);
        uint256[] memory sellAmounts = pancakeRouter.getAmountsOut(buyAmounts[1], pathSell);

        uint256 flashFee = (amount * 5) / 10000; // 0.05% Aave fee
        uint256 amountOwed = amount + flashFee;

        if (sellAmounts[1] > amountOwed) {
            uint256 profit = sellAmounts[1] - amountOwed;
            estimatedProfitBPS = (profit * 10000) / amount;
        }
    }

    receive() external payable {}
}
