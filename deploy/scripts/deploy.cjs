const { ethers } = require("hardhat");
require("dotenv").config();

// ─── BNB Chain Testnet Addresses ─────────────────────────────────────────────
// Catatan: Aave V3 tidak tersedia di BSC Testnet (hanya mainnet).
// Untuk testnet kita deploy dengan mock/placeholder TSS address.
// Saat siap mainnet, ganti ke address mainnet di bawah.

const ADDRESSES = {
  testnet: {
    // Aave V3 tidak ada di BSC testnet — pakai address dummy untuk compile/deploy
    // Gunakan PancakeSwap flash loan sebagai alternatif di testnet
    aavePool:       "0x0000000000000000000000000000000000000001", // placeholder
    uniswapRouter:  "0xD99D1c33F9fC3444f8101754aBC46c52416550D1", // PancakeSwap testnet router (sebagai uni substitute)
    pancakeRouter:  "0xD99D1c33F9fC3444f8101754aBC46c52416550D1", // PancakeSwap V2 testnet router
  },
  mainnet: {
    aavePool:       "0x6807dc923806fE8Fd134338EABCA509979a7e07",
    uniswapRouter:  "0xB971eF87ede563556b2ED4b1C0b0019111Dd85d2",
    pancakeRouter:  "0x10ED43C718714eb63d5aA57B78B54704E256024E",
  },
};

async function main() {
  const [deployer] = await ethers.getSigners();
  const network = await ethers.provider.getNetwork();
  const isTestnet = network.chainId === 97;

  console.log("=== Hexa Arb Bots — Contract Deployment ===");
  console.log(`Network  : ${isTestnet ? "BNB Testnet (97)" : "BNB Mainnet (56)"}`);
  console.log(`Deployer : ${deployer.address}`);

  const balance = await ethers.provider.getBalance(deployer.address);
  console.log(`Balance  : ${ethers.utils.formatEther(balance)} BNB`);

  if (balance.eq(0)) {
    console.error("\n❌ Balance 0! Dapatkan testnet BNB di: https://testnet.bnbchain.org/faucet-smart");
    process.exit(1);
  }

  const addr = isTestnet ? ADDRESSES.testnet : ADDRESSES.mainnet;

  // TSS address: sementara pakai deployer address untuk testing
  // Ganti dengan Pelagos TSS aggregate key saat production
  const pelagosTSSAddress = process.env.PELAGOS_TSS_ADDRESS || deployer.address;
  console.log(`TSS Addr : ${pelagosTSSAddress}`);

  console.log("\n📦 Deploying ArbitrageExecutor...");

  const ArbitrageExecutor = await ethers.getContractFactory("ArbitrageExecutor");
  const contract = await ArbitrageExecutor.deploy(
    addr.aavePool,
    addr.uniswapRouter,
    addr.pancakeRouter,
    pelagosTSSAddress,
  );

  await contract.deployed();
  const contractAddress = contract.address;

  console.log("\n✅ ArbitrageExecutor deployed!");
  console.log(`📍 Address : ${contractAddress}`);
  console.log(`🔗 Explorer: ${isTestnet
    ? `https://testnet.bscscan.com/address/${contractAddress}`
    : `https://bscscan.com/address/${contractAddress}`
  }`);

  console.log("\n⚠️  Langkah selanjutnya:");
  console.log(`   Update constants.go:`);
  console.log(`   ArbitrageExecutorBNB = "${contractAddress}"`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
