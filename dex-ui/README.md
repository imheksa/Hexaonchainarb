# Orderbox Salvage Console

UI mandiri (satu file HTML) untuk berinteraksi dengan smart contract DEX orderbook
di Ethereum yang frontend/domainnya sudah mati, tetapi contract-nya masih berjalan.

**Jawaban singkat: ya, sangat mungkin.** Frontend sebuah DEX hanyalah "jendela" ke
smart contract. Selama contract-nya masih ada di Ethereum (dan smart contract tidak
bisa dimatikan begitu saja), aset Anda masih tercatat di dalamnya dan bisa diakses
lewat jendela baru — yaitu file ini.

## Cara menjalankan

**Disarankan — lewat server lokal** (MetaMask paling stabil di `http://localhost`):

```bash
cd dex-ui
node server.mjs          # butuh Node.js 18+, tanpa dependensi apa pun
# buka http://localhost:8080
```

Alternatif tanpa Node: `python3 -m http.server 8080` dari folder `dex-ui`,
atau buka `index.html` langsung di browser (klik dua kali) — semua fitur baca
tetap jalan dari `file://`, hanya sebagian wallet yang rewel di mode itu.

Lalu:

1. Panel **1 · Koneksi**: klik **Hubungkan** (RPC publik gratis sudah terisi),
   alamat EtherDelta 2 dan preset ABI sudah default — klik **Muat Contract**.
2. Panel-panel berikutnya muncul otomatis: cek saldo, harga/trade, orderbook
   buy/sell, tarik dana, dan explorer semua fungsi contract.

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

## Melihat ukuran order buy & sell (orderbook)

Panel 4 memindai event `Order` on-chain untuk satu token, lalu menyusun dua sisi:

- **Buy** — maker membayar ETH untuk membeli token (`tokenGet` = token,
  `tokenGive` = ETH). Ukuran = jumlah token yang diminta.
- **Sell** — maker menjual token untuk ETH (`tokenGive` = token,
  `tokenGet` = ETH). Ukuran = jumlah token yang ditawarkan.

Untuk tiap order, sisa volume diverifikasi langsung ke contract lewat
`availableVolume(...)` — order yang sudah terisi penuh, dibatalkan, atau
kedaluwarsa (blok `expires` terlewati) otomatis disaring. Tabel diurutkan
seperti orderbook sungguhan (buy tertinggi dulu, sell terendah dulu), lengkap
dengan total kedalaman per sisi.

Batasan yang perlu diketahui: orderbook EtherDelta dulunya **mayoritas
off-chain** — order ditandatangani dan disimpan di server etherdelta.com yang
kini mati. Yang bisa direkonstruksi dari blockchain hanyalah order yang
dipasang on-chain lewat fungsi `order()`. Pada DEX yang sudah lama mati,
wajar bila kedua sisi kosong.

## Menarik dana

1. Cek saldo di panel 2 — salin angka "mentah" (satuan wei).
2. Hubungkan wallet di panel 4 (hanya lewat MetaMask/Rabby — halaman ini tidak
   pernah meminta seed phrase/private key).
3. Masukkan alamat token (kosongkan untuk ETH) dan jumlah mentah, klik **Withdraw**.
4. Uji dengan jumlah kecil dulu sebelum menarik semuanya.

Jika nama fungsi withdraw contract Anda berbeda dari pola klasik, gunakan panel 5
(explorer) — semua fungsi dari ABI bisa dipanggil dari sana.

## Hosting di GitHub Pages

Workflow `.github/workflows/deploy-pages.yml` men-deploy folder ini ke GitHub
Pages (`https://imheksa.github.io/Hexaonchainarb/`) setiap ada push yang
menyentuh `dex-ui/`.

**Dua syarat dari GitHub (dua-duanya wajib):**

1. **Repo harus public** (atau private dengan paket GitHub Pro/Team). Jadikan
   public lewat *Settings → General → Danger Zone → Change visibility*. Ingat:
   seluruh isi repo (termasuk kode bot arbitrase) ikut menjadi publik —
   pastikan tidak ada rahasia yang ter-commit.

2. **Aktifkan Pages dengan sumber "GitHub Actions" satu kali.** Buka
   *Settings → Pages → Build and deployment → Source* lalu pilih
   **GitHub Actions**. Langkah ini membuat situs Pages-nya; tanpa ini workflow
   gagal dengan "Create Pages site failed — Resource not accessible by
   integration" karena token workflow tidak berwenang membuat situs dari nol.

Setelah kedua langkah itu, jalankan ulang workflow (tab *Actions* → run yang
gagal → *Re-run jobs*, atau cukup push apa pun yang menyentuh `dex-ui/`). Situs
akan live di `https://imheksa.github.io/Hexaonchainarb/`.

Alternatif yang sepenuhnya privat, tanpa mengubah apa pun di GitHub: jalankan
`node server.mjs` di komputer sendiri.

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
