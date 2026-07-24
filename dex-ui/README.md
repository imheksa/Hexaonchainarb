# Orderbox Salvage Console

UI mandiri (satu file HTML) untuk berinteraksi dengan smart contract DEX orderbook
di Ethereum yang frontend/domainnya sudah mati, tetapi contract-nya masih berjalan.

**Jawaban singkat: ya, sangat mungkin.** Frontend sebuah DEX hanyalah "jendela" ke
smart contract. Selama contract-nya masih ada di Ethereum (dan smart contract tidak
bisa dimatikan begitu saja), aset Anda masih tercatat di dalamnya dan bisa diakses
lewat jendela baru — yaitu file ini.

## Cara menjalankan

Tidak perlu server, tidak perlu install apa pun:

1. Buka `dex-ui/index.html` langsung di browser (klik dua kali, atau `File → Open`).
   Gunakan browser yang sudah terpasang MetaMask/Rabby jika ingin menarik dana.
2. Panel **1 · Koneksi**: klik **Hubungkan** (RPC publik gratis sudah terisi),
   lalu masukkan alamat smart contract DEX dan klik **Muat Contract**.
3. Panel-panel berikutnya akan muncul otomatis: cek saldo, pindai harga/trade,
   tarik dana, dan explorer semua fungsi contract.

> Indikator hijau berdenyut di kanan atas menampilkan nomor blok Ethereum terbaru —
> bukti live bahwa jaringan (dan contract Anda) masih berjalan.

## Contract yang sudah dikonfigurasi: EtherDelta 2

Alamat `0x8d12A197cB00D4747a1fe03395095ce2A5CC6819` sudah terisi sebagai default.
Ini adalah contract **EtherDelta 2** (label resmi di Etherscan), DEX orderbook
klasik yang di-deploy 9 Februari 2017. Frontend etherdelta.com sudah lama mati,
tetapi contract-nya tetap berjalan dan masih menyimpan dana deposit jutaan dolar
milik para penggunanya. ABI preset di UI ini adalah ABI EtherDelta, jadi pilihan
**Preset EtherDelta** bekerja langsung tanpa API key.

Cara menarik dana dari EtherDelta:

- Saldo: `balanceOf(token, wallet_anda)` — panel 2 (untuk ETH, token = `0x0`)
- Tarik ETH: `withdraw(jumlah_wei)` — panel 4 dengan kolom token kosong
- Tarik token: `withdrawToken(alamat_token, jumlah)` — panel 4 dengan alamat token

## Lupa alamat contract-nya?

1. Buka [etherscan.io](https://etherscan.io) dan cari alamat wallet Anda.
2. Telusuri riwayat transaksi lama saat Anda deposit/trading di DEX tersebut.
3. Alamat tujuan (`To`) pada transaksi deposit itu adalah alamat contract DEX-nya.
4. Klik alamat itu di Etherscan — biasanya ada label nama DEX-nya.

## Tiga cara memuat ABI

ABI adalah "daftar menu" fungsi contract. Pilih salah satu di panel 1:

| Sumber | Kapan dipakai |
|---|---|
| **Etherscan otomatis** | Contract terverifikasi di Etherscan (kebanyakan DEX lama terverifikasi). Buat API key gratis di [etherscan.io/apis](https://etherscan.io/apis). |
| **Preset EtherDelta-style** | Contract tidak terverifikasi, tapi DEX-nya bergaya orderbook klasik (`deposit`/`withdraw`/`balanceOf(token,user)` — pola EtherDelta yang ditiru ratusan DEX era 2017–2019). |
| **Tempel manual** | Anda punya JSON ABI dari sumber lain (repo GitHub DEX-nya, arsip, dsb). |

## Memeriksa harga token

DEX orderbook tidak menyimpan "harga" sebagai satu angka — harga terbentuk dari
order dan trade. Panel 3 memindai event `Trade` pada rentang blok yang Anda tentukan
dan menghitung harga token/ETH dari tiap trade yang terjadi.

Catatan penting: jika DEX-nya sudah lama mati, kemungkinan besar **tidak ada trade
baru** — artinya tidak ada harga pasar yang hidup di contract itu. Nilai token Anda
lebih relevan dilihat di pasar lain (CoinGecko, DEX lain seperti Uniswap). Yang
terpenting dari contract lama ini biasanya bukan harganya, melainkan **saldo Anda
yang masih tersimpan** (panel 2) dan **cara menariknya** (panel 4).

## Menarik dana

1. Cek saldo di panel 2 — salin angka "mentah" (satuan wei).
2. Hubungkan wallet di panel 4 (hanya lewat MetaMask/Rabby — halaman ini tidak
   pernah meminta seed phrase/private key).
3. Masukkan alamat token (kosongkan untuk ETH) dan jumlah mentah, klik **Withdraw**.
4. Uji dengan jumlah kecil dulu sebelum menarik semuanya.

Jika nama fungsi withdraw contract Anda berbeda dari pola klasik, gunakan panel 5
(explorer) — semua fungsi dari ABI bisa dipanggil dari sana.

## Alternatif tanpa UI ini

Untuk contract terverifikasi, Etherscan sendiri menyediakan tab
**Contract → Read Contract / Write Contract** yang bisa dipakai untuk hal yang sama.
UI ini lebih nyaman karena memformat saldo per desimal token, menghitung harga dari
event Trade, dan menyatukan semuanya dalam satu layar — tetapi Etherscan adalah
pembanding yang baik untuk memverifikasi hasil.

## Keamanan

- File ini statis, berjalan lokal, tanpa server dan tanpa tracking. Library
  ethers.js sudah dibundel di folder ini (`ethers.umd.min.js`), bukan dari CDN.
- Koneksi keluar hanya ke: RPC publik yang Anda isi, API Etherscan (untuk ABI),
  dan Google Fonts (opsional — tanpa internet font akan jatuh ke font sistem).
- Transaksi tulis selalu ditandatangani di wallet Anda; periksa detailnya di
  popup wallet sebelum menyetujui.
- Jangan pernah memasukkan seed phrase atau private key di mana pun.
