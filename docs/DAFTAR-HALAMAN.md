# DAFTAR HALAMAN / TAB — ai-sell-modern

Status: **2026-09-21 (final batch)**. Sumber fitur = `../server` (Fastify+Prisma, 85 endpoint / 41 model) + `../app` (Flutter, **sudah dibuang**).
Aturan: **fitur dipindah, data TIDAK dijembatani.** Postgres `aisell` di-seed ulang dari nol; `_legacy/prisma/prod2.db` tetap read-only sebagai referensi. Dua backend boleh hidup bersamaan tanpa bridge.

## 1. Tab / page yang sudah ada & jalan

31 route web, semuanya HTTP 200 di `next dev` (port 3000). API di kolom kanan = gateway Go (port 8080).

### Publik (tanpa login)
| Tab / Page | Route web | API |
|---|---|---|
| Landing | `/` | — |
| Masuk / Daftar | `/masuk` | `POST /api/auth/login`, `/api/auth/register` |
| Lupa password | `/lupa-password` | `POST /api/auth/forgot` |
| Reset password | `/reset-password` | `POST /api/auth/reset` |
| Blog / artikel | `/artikel`, `/artikel/[slug]` | `GET /api/articles`, `/api/articles/{slug}` |
| Status layanan | `/status` | `GET /api/status`, `GET /api/models` |
| Marketplace (katalog) | `/market` | `GET /api/market/products` |
| Docs developer (publik) | `/docs` | `GET /api/models` |

### User (login)
| Tab / Page | Route web | API |
|---|---|---|
| Dashboard | `/dashboard` | `GET /api/dashboard` |
| Chat / Playground | `/chat` | `GET /api/models`, `POST /api/chat` |
| API Keys | `/api-keys` | `GET/POST /api/keys`, `DELETE /api/keys/{id}` |
| Wallet & topup & tarik saldo | `/wallet` | `GET /api/wallet`, `/wallet/transactions`, `POST /wallet/topup`, `POST /wallet/withdrawals` |
| Paket token | `/pricing` | `GET /api/packages`, `POST /api/packages/{id}/buy` |
| Tukar kode voucher | `/redeem` | `POST /api/redeem` |
| Keranjang + checkout | `/keranjang` | `GET/POST/PATCH/DELETE /api/cart`, `POST /api/market/checkout` |
| Pesanan saya | `/pesanan` | `GET /api/market/orders`, `POST …/{id}/confirm`, `…/{id}/cancel` |
| Toko seller (produk, boost) | `/toko` | `GET/POST /api/store`, `GET/POST/PATCH/DELETE /api/store/products[/{id}]`, `POST …/boost` |
| Reward harian | `/reward` | `GET/POST /api/reward/daily` |
| Referral | `/referral` | `GET /api/referral`, `POST /api/referral/claim` |
| Afiliasi produk | `/afiliasi` | `GET/POST /api/affiliate/links` |
| Tulis artikel | `/tulis-artikel` | `POST /api/articles` |
| Akun & keamanan (2FA TOTP, sesi) | `/pengaturan` | `GET/POST /api/account/2fa*`, `GET/DELETE /api/account/sessions[/{id}]` |
| Chat via API key (programmatic) | — | `POST /api/v1/chat` |

### Admin (role ADMIN / SUPER_ADMIN)
| Tab / Page | Route web | API |
|---|---|---|
| Users + ubah peran | `/admin/users` | `GET /api/admin/users`, `PATCH /api/admin/users/{id}/role` |
| Provider (kunci terenkripsi, toggle) | `/admin/providers` | `GET /api/admin/providers`, `PATCH …/providers`, `POST …/provider/toggle` |
| Models + pricing (CRUD, check provider) | `/admin/models` | `GET/POST /api/admin/models`, `PATCH/DELETE /api/admin/models/{id}`, `POST /api/admin/provider/{id}/check` |
| Summary | `/dashboard` (ringkas) | `GET /api/admin/summary` |
| Orders + refund | `/admin/orders` | `GET /api/admin/orders`, `POST …/orders/{id}/refund` |
| Produk (approve / boost / pin) | `/admin/products` | `GET /api/admin/products`, `PATCH /api/admin/products/{id}` |
| Wallet & penarikan | `/admin/wallet` | `GET /api/admin/wallet`, `POST …/withdrawals/{id}/approve\|reject`, `POST …/topup/{id}/complete` |
| Uptime monitor | `/admin/uptime` | `GET/POST /api/admin/uptime` |
| Setting platform / SEO | `/admin/pengaturan` | `GET/PATCH /api/admin/settings` |
| Artikel (moderasi) | `/admin/artikel` | `GET /api/admin/articles`, `PATCH /api/admin/articles/{id}` |
| Health model | `/admin/health` | `GET /api/admin/health` |
| Audit log | `/admin/audit` | `GET /api/admin/audit` |

## 2. Skema DB — LIVE 41 tabel

`gateway/sql/schema.sql`, **idempoten** (tabel dasar `IF NOT EXISTS`, kolom baru `ALTER … ADD COLUMN IF NOT EXISTS`) → bisa di-apply ulang ke DB yang sudah jalan tanpa reset volume.

| Kelompok | Tabel |
|---|---|
| Inti | `user`, `api_key`, `credit_transaction`, `usage_log` (partisi bulanan), `provider` |
| Marketplace | `market_product`, `market_store`, `market_category`, `cart_item`, `market_order` (escrow), `market_review`, `market_boost`, `media_asset` |
| Dompet | `token_package`, `user_token_package`, `subscription`, `free_tier_quota`, `user_topup_accumulation`, `daily_reward_claim`, `withdrawal` |
| Referral/afiliasi | `referral_code`, `referral_signup`, `referral_commission`, `affiliate_link`, `affiliate_click`, `affiliate_conversion` |
| Model & harga | `ai_model`, `model_badge`, `model_pricing`, `model_health_log` |
| Akun & keamanan | `user_session`, `oauth_account`, `verification_token`, `user_auth_method` (2FA), `audit_log` |
| Konten & ops | `article`, `article_link_violation`, `platform_setting`, `uptime_monitor`, `uptime_heartbeat` |

Seed katalog dasar (kategori, 4 model + harga, 3 paket token, 7 platform_setting) ditulis ulang di dalam file yang sama — **tidak ada satu baris pun dari `prod2.db`**.

## 3. Fitur lama yang sudah punya padanan

Marketplace (toko/kategori/keranjang/ulasan/boost/media/escrow), keuangan (topup, penarikan, paket token, langganan, free tier, reward harian, topup akumulasi), referral (kode, undangan, komisi 30%), afiliasi produk (link, klik, konversi), keamanan akun (2FA TOTP stdlib, sesi, lupa/reset password), audit log, artikel + deteksi link, SEO/platform setting, uptime monitor + heartbeat, health model, katalog model & harga dari DB (tidak lagi hardcode).

Data lama TIDAK dipindah: `prod2.db` (5 user, 10 produk, 8 order, 60 usage log, 282 health log) tetap sebagai referensi read-only.

## 4. Bug yang dulu terbuka — semua TERTUTUP

| # | Bug | Perbaikan | Bukti |
|---|---|---|---|
| 1 | Kunci provider plaintext (`registry.go:37`, komentar bohong "sudah dienkripsi") | `internal/secret/secret.go`: AES-256-GCM, kunci dari env `SECRET_KEY`, ciphertext `v1.`+base64; API hanya balas masker 4 karakter | DB `LIKE 'v1.%'` = 1, plaintext di DB = 0, bocor lewat API = 0, masker `••••1234` |
| 2 | Order menggantung PENDING tanpa escrow/refund, `qty > stock` cek-lalu-tulis | `market.go`: checkout escrow (dana ditahan → cair saat confirm), cancel = refund + restore stok, `qty` divalidasi di dalam query | verifikasi escrow/cair/cancel-refund lulus |
| 3 | Jejak billing bohong: provider gagal → `usage_log` tetap `OK` + biaya penuh; semua error dilaporkan `402 saldo tidak cukup` | `billing.SettleUsage()` memperbaiki di satu tempat (status→ERROR, cost→0, refund dalam satu transaksi); `errors.Is(ErrInsufficientCredit)` → 402, sisanya 500 | usage_log status = `ERROR`, cost `0.000000`, saldo tetap 50000, laporan usage = 0; model di luar katalog → 400, saldo habis → 402 |
| 4 | Manajemen peran hilang (tidak ada ganti role / audit / guard admin terakhir) | `PATCH /api/admin/users/{id}/role` + `audit_log`, menolak mencabut admin terakhir | audit_log terisi, sisa admin ≥ 1 setelah semua operasi |

Temuan audit lanjutan yang juga sudah diperbaiki: withdrawal tidak menyimpan tujuan transfer (`bank_account` dibuang) dan topup tidak menyimpan metode; ulasan bisa menyerang toko lain (produk tidak divalidasi milik toko + ulasan tanpa syarat beli); produk `is_active=false` bocor ke katalog publik; route publik tanpa rate limit (`AffiliateClick` menulis DB per klik); `SECRET_KEY` kosong tidak fail-fast; 8+ mismatch nama field antara web dan gateway.

## 5. Verifikasi (dijalankan ulang di tangan saya, bukan klaim agen)

```bash
bash gateway/verify-endpoints.sh   # == 194 PASS / 0 FAIL ==  (exit 0)
cd gateway && gofmt -l . && go vet ./... && go build ./... && go test ./...
# gofmt kosong, vet/build exit 0, test: ok internal/auth, ok internal/secret
cd web && npx tsc --noEmit         # exit 0
curl tiap route web                # 31/31 → HTTP 200, 0 marker error di HTML
```
Gateway yang jalan = image terbaru (dibangun 07:28:26Z, source terakhir 07:27:55Z) — tidak ada source yang tertinggal di luar container.

## 6. Gelombang 2 — port fitur new-api + referensi gateway (2026-09-21 malam)

| Fitur | Endpoint | Status |
|---|---|---|
| Streaming SSE chat | `POST /api/chat` + `/api/playground/execute` dengan `stream:true`; UI `/chat` render bertahap + tombol Stop | ✅ bukti stream nyata (chunk `data:` + `[DONE]`) |
| Endpoint OpenAI-compatible | `GET /v1/models`, `POST /v1/chat/completions` (alias lama `/api/v1/chat` tetap) | ✅ live, 4 model katalog |
| Endpoint Anthropic | `POST /v1/messages` (non-stream) | ✅ live |
| Kode redeem | `POST /api/redeem` + admin `GET/POST/DELETE /api/admin/redeem` | ✅ end-to-end (buat → tukar → saldo naik → pakai ulang ditolak) |
| Kuota per API key | `api_key.credit_limit/credit_used`, tolak 402 saat lewat | ✅ |
| CRUD model katalog | `GET/POST/PATCH/DELETE /api/admin/models` + harga | ✅ |
| CHECK koneksi provider | `POST /api/admin/provider/{id}/check` → `is_healthy` + `model_health_log` | ✅ (mock server, latency tercatat) |
| Routing model per provider | `provider.models` CSV (kosong = semua) | ✅ |
| `/pricing` ala zemith | toggle IDR/USD, kartu paket, tabel model per family, FAQ | ✅ 200, render live |
| `/docs` developer | contoh cURL/Python/JS, base_url, daftar model | ✅ 200 |
| `/admin/models` | CRUD model + CHECK provider | ✅ 200 |
| Verify script | 265 PASS / 0 FAIL (2 run berturut-turut bersih) | ✅ |

## 7. Uptime Kuma (health/uptime)

- Container `louislam/uptime-kuma:1` di `docker-compose.yml`, UI di **http://localhost:3001** (volume `kuma`).
- Gateway = **push monitor**: `StartKumaPusher` menembak `/healthz` sendiri tiap menit lalu kirim status/latensi ke Kuma. Aktif kalau `UPTIME_KUMA_TOKEN` diisi (token dari monitor tipe *Push* di UI Kuma); kalau kosong fitur push mati, Kuma tetap jalan sebagai dashboard mandiri.
- Kuma **tidak** menyentuh DB gateway. Halaman `/status` dan panel `/admin/uptime` tetap dari tabel `uptime_monitor`/`uptime_heartbeat`; monitor itu dikelola `/admin/uptime`, dan heartbeat sekarang diisi otomatis tiap menit oleh `StartUptimeScheduler` (sebelumnya hanya saat admin menekan tombol run → `/status` selalu "unknown").
- Cek: `GET /api/status` → `{"monitors":[{"name":"Gateway API","status":"up","latency_ms":0,...}],"overall":"OPERATIONAL"}`.
- Cek unit: `go test ./internal/handler/ -run Probe` (3 tes: 200 → up, 500 → down + pesan kode, bentuk query push).

## 8. Perbaikan akses landing (2026-09-21)

- `/` bisa dibuka siapa pun, **termasuk yang sudah login**. Sebelumnya `app/page.tsx` memaksa `router.replace('/dashboard')` begitu `user` ada — pengguna login yang mau lihat etalase tidak punya jalan masuk sama sekali.
- Header landing menyesuaikan: pengunjung → "Masuk", pengguna login → "Dashboard"; CTA hero jadi "Buka dashboard". Link `/masuk` hilang saat sudah login (tidak ada lagi tautan yang salah arah).
- Bukti (Chrome headless + CDP, token di `localStorage`): `pathname` tetap `/`, teks header `Dashboard`, CTA `Buka dashboard`, kartu etalase tampil, tanpa redirect.
- Plafon rate limit sesi dinaikkan 60 → **600 request/jam per user** (`apiLimit`): dashboard memuat belasan endpoint per halaman, dan 60 terlalu cepat habis sampai halaman tampak rusak (429).

## 9. Status per MODEL (board ala AI-gateway) + Uptime Kuma

Kuma memantau **endpoint HTTP** (`/healthz`) — dia tidak tahu apa itu model, TTFT, atau
latensi provider. Board "Model Status" di screenshot referensi karena itu **bukan** Kuma,
dan sekarang kita punya padanannya di `/status`:

| Bagian | Sumber | Catatan |
|---|---|---|
| Banner + "Semua sistem normal" | `GET /api/status` → `overall` (monitor **dan** model) | `OPERATIONAL` / `DEGRADED` |
| Ketersediaan rata-rata 24 jam · Model dipantau | agregat `model_health_log` | tampil `-` bila belum ada probe |
| Baris per model: titik status, badge, Ketersediaan, P90, rata-rata | `models[]` di `/api/status` | P90 = `percentile_disc(0.9)` |
| Spark bar 24 jam (hijau/kuning/oranye/merah per jam) | status terburuk per jam dalam 24 jam | jam tanpa probe = kosong, bukan "mati" |
| Monitor Layanan | tabel `uptime_monitor`/`uptime_heartbeat`, diisi `StartUptimeScheduler` (1 menit) | ini bagian yang sejalan dengan Kuma |

- Pengisi data: `StartModelHealthProbe` (5 menit) — ping `/chat/completions` `max_tokens=1`
  ke tiap provider aktif × model yang dilayaninya (`provider.models` CSV; kosong = katalog
  kecuali EMBEDDING/RERANK). Tombol CHECK di `/admin/models` tetap ada untuk uji manual.
- Trafik nyata user **tidak** ikut dihitung di availability: `usage_log` tidak menyimpan
  latensi. Availability = murni hasil probe (sama seperti "sampel" di Kuma).
- Cek unit: `go test ./internal/handler/ -run 'Spark|Probe'` (pemetaan 24 ember + probe up/down).
- Uptime Kuma tetap berdiri sendiri: container `uptime-kuma` di `:3001`, gateway push
  `/healthz` tiap menit bila `UPTIME_KUMA_TOKEN` diisi.

## 10. Referral (parity new-api)

Alur mengikuti new-api (`aff_code` di pendaftaran, bukan klaim manual belakangan):

| Kegiatan | new-api | Kita |
|---|---|---|
| Daftar lewat link undangan | `?aff=` → bonus `QuotaForInvitee`/`QuotaForInviter` saat register | `POST /api/auth/register` terima `aff_code` → `affiliateSignup` bayar bonus kedua sisi + catat `referral_signup` (unik per invitee) |
| Kegagalan kode | pendaftaran tetap jalan | sama — kode ngawur tidak menggagalkan akun |
| Komisi topup | ada | sudah ada dari dulu (`referral_commission`, PAID atomik saat topup disetujui) |
| Halaman undangan | link invite | `/referral` menampilkan kode + link `/masuk?aff=KODE` (tombol salin) |
| Klaim manual | tidak ada | tetap dipertahankan sebagai jalur cadangan; dedupe `unique(invitee_id)` bikin bonus tidak bisa digandakan antara jalur register vs klaim |

- Jalur uang bonus ada di SATU fungsi `referralBonus` (commerce.go) — klaim manual dan
  register memakai fungsi yang sama; ubah sekali, dua jalur ikut.
- `verify-endpoints.sh`: +7 cek aff (register+kode, signup tercatat, bonus kedua sisi,
  buku besar, kode ngawur, pembersihan) — total **272 PASS / 0 FAIL**.
- Risiko jujur (sama dengan new-api): pendaftaran tidak butuh verifikasi email, jadi
  bonus per-akun bisa di-farm dengan akun palsu. Batasi dengan `max_uses` di kode
  atau aktifkan verifikasi email kalau nanti jadi masalah.

## 12. Antrian parity new-api #1–#4 (selesai, terverifikasi)

Empat gap vs OmniRoute/9router ditutup (urutan dari user: #1 → #4 → #2 → #3):

1. **#1 Tambah provider dari UI** — `POST /api/admin/providers` (name+api_key wajib, base_url harus http/https, kunci di-`secret.Encrypt` sebelum sentuh DB); halaman `/admin/providers` kini punya form tambah + masker `••••xxxx`.
2. **#4 Kuota via API key** — `GET /v1/key` dengan Bearer key itu sendiri: balikin sisa kuota (`credit_limit`, `credit_used`, aktif, rate limit, whitelist). Parity new-api key check.
3. **#2 Whitelist model per key** — kolom `api_key.models` (CSV, `''`=semua); `CreateKey`/`ListKeys` ikut; chat OpenAI **dan** Anthropic ditolak `403` bila model di luar whitelist (enforcer di `runChat`, satu tempat untuk dua protokol).
4. **#3 Rate limit per key** — `api_key.rate_limit` (request/menit, 0=default 60/menit) ditegakkan Redis; **bucket per KEY bukan per user** (`rate.APIKeyRateKey(keyID)`) — dua key satu user punya jatah sendiri; lewat jatah → `429`.

Bukti verify (tamabhan 14 cek): `POST /api/admin/providers` 200 + ciphertext `v1.` di DB + masker `••••`; base_url `ftp://` & api_key kosong → 400; `GET /v1/key` 200/`credit_used` terisi/tanpa key 401; whitelist di luar 403, di dalam 200; key `rate_limit=2` → request ke-3 `429` sementara key lain tetap 200. `verify-endpoints.sh` **286 PASS / 0 FAIL**; gofmt/vet/`go test` (4 paket) dan `tsc --noEmit` bersih; gateway rebuilt.

Web: `/admin/providers` +form tambah; `/api-keys` +field whitelist model (kosong = semua model) dan info model di daftar key.

## 11. Sisa yang belum dikerjakan (jujur)

- **Idempotency key checkout** — submit `items[]` dua kali berturut-turut bisa jadi 2 order. Butuh header idempotency key di gateway + web. Sengaja tidak dibuat.
- **Verifikasi visual 375px** dilakukan lewat headless Chrome + CDP (`--headless=new --remote-debugging-port=9222` + WebSocket Node, tanpa puppeteer): `documentElement.scrollWidth` = 375 di `/pricing`, `/docs`, `/chat`, `/redeem`, `/admin/models` (tanpa overflow horizontal).
- **Lupa password belum mengirim email** (`ponytail` — sambungkan SMTP saat production). Metode masuk aktif: manual, Google, dan Telegram. GitHub/Discord sengaja dihapus.
- `/admin/users` & `/admin/providers` masih pakai `api.get` + state manual (bukan `useFetch`); sudah punya loading/empty/error.
- **Ulasan toko-level** (tanpa memilih produk) belum ada tombolnya di UI meski `POST` tanpa `product_id` sah.

## 7. Cara pakai ulang

```bash
export PATH="$HOME/AppData/Local/Programs/DockerDesktop/resources/bin:$PATH"
# DB/Redis/Gateway
docker compose up -d --build
# apply ulang skema ke DB yang sudah jalan (idempoten)
docker compose exec -T db psql -U aiuser -d aisell -f /docker-entrypoint-initdb.d/00-schema.sql
# Frontend (dev, hot reload)
cd web && npm run dev
```

## 13. Audit paritas web lama → web baru (selesai 2026-09-22)
Route-diff skrip `server/src/routes` (Fastify+prod2.db) vs gateway menemukan 4 gap; semua gateway-level dibuat, diuji 286/0, tsc 0:
- **Pagination** 5 endpoint (produk, artikel, admin users, transaksi wallet, pesanan pembeli) — `pageOf` (per+1, tanpa COUNT), balasan +`page`+`has_more`; UI `Pagination.tsx`.
- **Produk by id** `GET /api/market/products/{id}` (404 bila kosong/nonaktif) + halaman detail `/market/[id]`.
- **Pesanan masuk toko** `GET /api/store/orders` (+`seller_payout`) — section baru di `/toko`.
- **2FA disable** `POST /api/account/2fa/disable` (kode TOTP wajib) — form di `/pengaturan`.
- **Analytics** `GET /api/analytics?range=24h|7d|30d&scope=user|seller|platform` + halaman `/analytics` (bar chart CSS murni, funnel platform, model teratas).
- **Stripe top-up** `POST /api/wallet/stripe/checkout` + webhook `POST /api/webhooks/stripe` (HMAC stdlib, idempoten; kosong env = 503 jelas; test `TestStripeSignature`).
- **Homepage ala vyceai**: nav, marketplace+gambar, model cards (badge konteks/tools/provider, harga per 1M, salin ID, status+latensi), pricing 3 tier, blog, docs, FAQ (6 tanya dari l10n.dart), footer 4 kolom.
- **/wallet satu tab**: Tukar Kode+Hadiah+Referral jadi tab (`?tab=`), `/redeem` `/reward` `/referral` redirect.
- **Media**: upload gambar produk & artikel via `/api/media` (`ImageField`), tampil di market/artikel/homepage; PATCH produk kini ikut menyimpan `image_url`.
- Badge paritas yang dipasang: chip Tim/Komunitas (is_admin_post), stok, Terbaru/Habis. Chip UNGGULAN/TERJUAL/DILIHAT/AFILIASI TERBUKA di-skip — API list tidak mengirim datanya (lihat catatan agen).

## 14. Redesign visual 3 babak (Fanny/GROK-V.1 ultra, 2026-09-22)
- Babak 1: fondasi — `web/DESIGN-SYSTEM.md` (referensi: skill ui-ux-pro-max, 21st.dev, riset web 2026), glassmorphism gelap #0b0f17 + cyan #06b6d4, Plus Jakarta Sans, page transition `.page-in` (fade+translateY 320ms) via `<main key={pathname}>` di AppShell, `prefers-reduced-motion` dihormati, tanpa framer-motion.
- Babak 2: halaman publik ke standar sama (pricing, docs, market, artikel, status, masuk, landing sections), badge tier FREE/PAID + token multiplier di kartu & tabel model.
- Babak 3: halaman auth-gated (wallet, api-keys, pesanan, toko, pengaturan, chat, dashboard, analytics, 4 admin) — verifikasi visual via CDP + inject token localStorage; tanpa overflow di 375/1440 di semua rute.
- Perbaikan ikutan: padding bawah nav sidebar (item terakhir tak lagi menempel footer), affordance fade tepi kanan tab-pills mobile.
- Gate: `tsc --noEmit` 0 error, 31 rute web 200 OK, screenshot di `%LOCALAPPDATA%/Temp/aisell-shots/`.

## §15. Audit menyeluruh ARGUS + bug uang yang tertangkap (2026-09-22)

ARGUS (GROK-V.1, read-only) menyapu 6 area: stack, kebersihan kode, regresi
endpoint, tagihan token, web, kebocoran data publik.

**Hasil:** healthz/redis/PONG/44 tabel/semua kolom baru PASS · gofmt+vet+build
bersih · go test 4 paket ok (TestStripeSignature PASS) · verify 286/0 · tsc 0 ·
26 rute 200 · tak ada kredensial/JWT/email bocor di HTML publik (satu-satunya
hit `nama@email.com` = placeholder form) · `/api/admin/models` tanpa token → 401.

**Satu-satunya blocker:** tagihan token tidak bisa dibuktikan live karena tabel
`provider` kosong (9router belum disambung) → chat 502 `no active provider`.

### Bug uang nyata yang ditemukan saat menutup blocker itu
`SettleUsage` hanya **mengembalikan kelebihan** estimasi, tidak pernah
**menagih kekurangan**. Terbukti live: total token 1.500 (1000+500, cache 200
tercatat) tapi saldo cuma terpotong **102** — sisanya gratis. Setiap panggilan
dengan prompt lebih panjang dari `max_tokens` = kebocoran uang.

Fix: cabang dua arah di `charge.go` (kelebihan → refund, kekurangan → tagih +
catat `credit_transaction`), dan `api_key.credit_used` kini disesuaikan untuk
selisih positif maupun negatif.

### Bukti setelah fix (upstream mock, usage tetap 1000+500)
| Uji | Hasil |
|---|---|
| PAID ×1 | saldo −1500 |
| PAID ×3 | saldo −4500 |
| FREE kuota tersedia | saldo −0, `free_quota.used` = 1500 |
| FREE kuota habis (19999/20000) | saldo −1500 |

Checks permanen: `gateway/smoke-token-billing.sh` (5 PASS/0 FAIL, self-cleaning)
+ `gateway/testdata/mock-usage.py`.

### Perbaikan lain
- **Gateway healthcheck** di `docker-compose.yml` → container kini `(healthy)`, bukan cuma `Up`.
- Catatan operasional: registry provider gateway hidup **di memori** (load saat
  start) — menonaktifkan provider lewat DB saja tidak mengubah apa yang
  dilayani; harus lewat API admin atau restart.

## 16. ARGUS bisnis/user-kejam (deleg_238f8d16) + fix (2026-09-22)
8 temuan → 8 fix di gateway, verify 290/0, smoke SEMUA PASS, go test 4 paket, tsc 0:
1. HIGH kuota FREE parsial+race → consumeFreeQuota tx FOR UPDATE, ambil min(sisa,tagihan); sisanya ke saldo.
2. HIGH qty order → qty<1 ditolak 400 (dulu diam-diam jadi 1); total=price*qty sudah benar (audit salah field "quantity").
3. HIGH paket token tak pernah terpakai → ChargeTx pakai paket aktif dulu (FIFO expires_at, FOR UPDATE), sisanya saldo; refund kembali ke paket; ledger USAGE hanya bagian dompet.
4. MED self-referral farming → bonus instan dihapus; lunas di topup PERTAMA invitee (payDeferredReferralBonus di SettleTopup + StripeWebhook).
5. MED spam klik afiliasi → dedup (link,ip_hash) 24 jam; akar: ipOf() memotong port (dulu hash beda tiap request).
6. MED self-buy/toko nonaktif → checkoutOrder: owner==buyer 400; toko nonaktif 400; +PATCH /api/admin/stores/{id} (aktivasi admin yang belum pernah ada).
7. LOW max_tokens<1 → 400 sebelum pra-bayar.
8. LOW kontrak error → "bad json: <err Go>" jadi "bad json" (CreateKey, chatProxy).
Script: verify +smoke penuh (paket, kuota parsial, dedup klik, aktivasi toko, aff deferral). Model delegasi Fanny = cx/gpt-5.6-terra medium.
