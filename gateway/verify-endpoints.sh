#!/usr/bin/env bash
# Verifikasi endpoint baru gateway + 3 bug money-path (BUG A/B/C).
#
# Dua akun: SELLER punya toko & produk, ADMIN pemegang panel sekaligus pembeli
# (aturan "penjual tidak bisa membeli produk sendiri").
# Skrip mengubah data dev (saldo, harga, peran) dan MEMULIHKANNYA di akhir.
#
#   bash gateway/verify-endpoints-v2.sh
set -uo pipefail
API=http://localhost:8080
cd "$(dirname "$0")/.."
TMP="$LOCALAPPDATA/Temp"
OUT="$TMP/gw_verify.log"
: > "$OUT"
pass=0; fail=0
chk() { # chk <label> <diharapkan> <didapat>
  if [ "$2" = "$3" ]; then printf 'PASS  %-50s %s\n' "$1" "$3" | tee -a "$OUT"; pass=$((pass+1));
  else printf 'FAIL  %-50s %s (mau %s)\n' "$1" "$3" "$2" | tee -a "$OUT"; fail=$((fail+1)); fi
}
note() { printf '      %-50s %s\n' "$1" "$2" | tee -a "$OUT"; }
dc() { docker compose exec -T db psql -U aiuser -d aisell -Atc "$1" | tr -d '\r'; }
code() { curl -s -o "$TMP/gw_out.json" -w '%{http_code}' "$@"; }
body() { cat "$TMP/gw_out.json"; }
has() { grep -q "$1" "$TMP/gw_out.json" && echo 1 || echo 0; }
jget() { python -c "
import json,os
d=json.load(open(os.environ['LOCALAPPDATA']+r'\Temp\gw_out.json'))
print($1)"; }
tok() { jget "d['token']"; }
totp() { python -c "
import hmac,hashlib,base64,struct,time,sys
key=base64.b32decode(sys.argv[1],casefold=True)
h=hmac.new(key,struct.pack('>Q',int(time.time())//30),hashlib.sha1).digest()
o=h[-1]&15
print('%06d'%((struct.unpack('>I',h[o:o+4])[0]&0x7fffffff)%1000000))" "$1"; }

# ---------------------------------------------------------------- keadaan awal
# Kunci rate-limit ikut dibersihkan: skrip ini menembak login berkali-kali dan
# kalau sisa kuota dari percobaan sebelumnya habis, semuanya jadi 429 dan bukan
# kegagalan aplikasi. Lingkungan dev — bukan data.
docker compose exec -T redis sh -c 'redis-cli -a "$(cat /run/secrets/redis_pass)" EVAL \
  "local n=0 for _,k in ipairs(redis.call('keys','rl:*')) do redis.call('del',k) n=n+1 end return n" 0' >/dev/null 2>&1
dc "DELETE FROM provider WHERE name='verif'; DELETE FROM cart_item;" >/dev/null
# Sisa produk uji dari jalannya skrip sebelumnya dibersihkan supaya PID yang
# dipilih selalu produk baru ini (bukan baris lama dengan judul sama).
# Urutannya penting: market_order menunjuk ke market_product tanpa ON DELETE.
dc "DELETE FROM market_order WHERE product_id IN (SELECT id FROM market_product WHERE title LIKE 'Produk Verifikasi%' OR title LIKE 'Produk Hapus%');
    DELETE FROM market_review WHERE product_id IN (SELECT id FROM market_product WHERE title LIKE 'Produk Verifikasi%' OR title LIKE 'Produk Hapus%');
    DELETE FROM market_product WHERE title LIKE 'Produk Verifikasi%' OR title LIKE 'Produk Hapus%';" >/dev/null
dc "UPDATE \"user\" SET wallet_balance=50000 WHERE email='admin@test.local';" >/dev/null
dc "UPDATE \"user\" SET wallet_balance=10000, role='SELLER' WHERE email='seller@test.local';" >/dev/null
dc "DELETE FROM daily_reward_claim WHERE reward_date='$(date -u +%F)';" >/dev/null
# Paket uji dari run sebelumnya membuat refund/tagihan chat mengalir ke paket,
# bukan saldo — asersi BUG A/B jadi salah. Bersihkan dulu.
dc "DELETE FROM user_token_package up USING \"user\" u WHERE u.id=up.user_id AND u.email IN ('admin@test.local','seller@test.local');" >/dev/null
dc "DELETE FROM article WHERE title IN ('Artikel Verifikasi','Pengumuman Admin');" >/dev/null
# Monitor uji menumpuk tiap run (nama sama, id beda) — heartbeat ikut lewat ON DELETE CASCADE.
dc "DELETE FROM uptime_monitor WHERE name IN ('Gateway Verif','DB');" >/dev/null

login() { code -X POST "$API/api/auth/login" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$1\",\"password\":\"Test1234!\"}"; }
chk "login admin@test.local" 200 "$(login admin@test.local)"; TA=$(tok)
chk "login seller@test.local" 200 "$(login seller@test.local)"; TS=$(tok)
A=(-H "Authorization: Bearer $TA" -H 'Content-Type: application/json')
S=(-H "Authorization: Bearer $TS" -H 'Content-Type: application/json')
ADMINID=$(dc "SELECT id FROM \"user\" WHERE email='admin@test.local';")
SELLERID=$(dc "SELECT id FROM \"user\" WHERE email='seller@test.local';")

# ---------------------------------------------------------------- publik
chk "GET  /api/models"             200 "$(code "$API/api/models")"
chk "harga model dari model_pricing (bukan hardcode)" 1 "$(jget "1 if any(m['pricing'] for m in d['models']) else 0")"
chk "GET  /api/status"             200 "$(code "$API/api/status")"
chk "GET  /api/articles"           200 "$(code "$API/api/articles")"
chk "GET  /api/market/categories"  200 "$(code "$API/api/market/categories")"
chk "GET  /api/market/products"    200 "$(code "$API/api/market/products")"
chk "GET  /api/auth/providers"     200 "$(code "$API/api/auth/providers")"
chk "providers hanya google+telegram" 2 "$(code "$API/api/auth/providers" >/dev/null; grep -o 'google\|telegram' "$TMP/gw_out.json" | sort -u | wc -l | tr -d ' ')"

# ---------------------------------------------------------------- user
for ep in /api/wallet /api/wallet/transactions /api/wallet/withdrawals /api/packages \
          /api/reward/daily /api/referral /api/affiliate/links /api/cart \
          /api/market/orders /api/account/sessions /api/dashboard /api/dashboard/usage; do
  chk "GET  $ep" 200 "$(code "${A[@]}" "$API$ep")"
done

# ---------------------------------------------------------------- toko (seller)
chk "POST /api/store (buka toko)" 200 "$(code -X POST "${S[@]}" -d '{"name":"Toko Verifikasi"}' "$API/api/store")"
SID=$(dc "SELECT id FROM market_store WHERE user_id='$SELLERID';")
# Toko baru nonaktif (default) — checkout menolak tokonya sampai admin
# mengaktifkan setelah review (schema is_active + AdminPatchStore).
chk "PATCH /api/admin/stores/{id} (admin aktifkan)" 200 "$(code -X PATCH "${A[@]}" -d '{"is_active":true}' "$API/api/admin/stores/$SID")"
chk "GET  /api/store"             200 "$(code "${S[@]}" "$API/api/store")"
chk "POST /api/store/products"    200 "$(code -X POST "${S[@]}" -d '{"title":"Produk Verifikasi","description":"Produk uji verifikasi marketplace","price":25000,"stock":5,"allow_affiliate":true,"affiliate_percent":10}' "$API/api/store/products")"
PID=$(dc "SELECT id FROM market_product WHERE title='Produk Verifikasi' ORDER BY created_at DESC LIMIT 1;")
chk "GET  /api/store/products"    200 "$(code "${S[@]}" "$API/api/store/products")"
chk "PATCH /api/store/products/{id}" 200 "$(code -X PATCH "${S[@]}" -d '{"price":30000}' "$API/api/store/products/$PID")"
chk "GET  /api/store/{id} (publik)" 200 "$(code "$API/api/store/$SID")"
SBAL0=$(dc "SELECT wallet_balance FROM \"user\" WHERE id='$SELLERID';")
chk "POST /api/store/products/{id}/boost (1 hari)" 200 "$(code -X POST "${S[@]}" -d '{"days":1}' "$API/api/store/products/$PID/boost")"
SBAL1=$(dc "SELECT wallet_balance FROM \"user\" WHERE id='$SELLERID';")
python -c "
print(('PASS' if float('$SBAL1')<float('$SBAL0') else 'FAIL')+'  boost memotong saldo seller: $SBAL0 -> $SBAL1')" | tee -a "$OUT"
chk "produk jadi boosted" t "$(dc "SELECT is_boosted FROM market_product WHERE id='$PID';")"
chk "POST /api/store/products (harga negatif ditolak)" 400 "$(code -X POST "${S[@]}" -d '{"title":"Harga Aneh","price":-1,"stock":1}' "$API/api/store/products")"
chk "POST /api/store/products (judul kosong ditolak)" 400 "$(code -X POST "${S[@]}" -d '{"title":"","price":1000,"stock":1}' "$API/api/store/products")"
chk "POST /api/store/products (harga 5000)" 200 "$(code -X POST "${S[@]}" -d '{"title":"Produk Hapus","description":"Produk sementara untuk verifikasi hapus","price":5000,"stock":1}' "$API/api/store/products")"
DPID=$(dc "SELECT id FROM market_product WHERE title='Produk Hapus' ORDER BY created_at DESC LIMIT 1;")
chk "DELETE /api/store/products/{id}" 200 "$(code -X DELETE "${S[@]}" "$API/api/store/products/$DPID")"
# Hapus lunak: baris TETAP ada (pesanan/keranjang/link menunjuk produk_id),
# hanya disembunyikan dari katalog — restore mengembalikannya.
chk "DELETE menyembunyikan produk belum terjual" t "$(dc "SELECT (deleted_at IS NOT NULL AND NOT is_active) FROM market_product WHERE id='$DPID';")"
chk "GET /api/market/products/{id} (terhapus) -> 404" 404 "$(code "$API/api/market/products/$DPID")"
chk "POST /api/store/products/{id}/restore" 200 "$(code -X POST "${S[@]}" "$API/api/store/products/$DPID/restore")"
chk "restore memunculkan produk kembali" t "$(dc "SELECT (deleted_at IS NULL AND is_active) FROM market_product WHERE id='$DPID';")"
chk "GET /api/market/products/{id} (setelah restore)" 200 "$(code "$API/api/market/products/$DPID")"

# ---------------------------------------------------------------- keranjang
chk "POST /api/cart"              200 "$(code -X POST "${A[@]}" -d "{\"product_id\":\"$PID\",\"quantity\":2}" "$API/api/cart")"
CID=$(dc "SELECT id FROM cart_item WHERE user_id='$ADMINID' LIMIT 1;")
chk "PATCH /api/cart/{id} {quantity}" 200 "$(code -X PATCH "${A[@]}" -d '{"quantity":1}' "$API/api/cart/$CID")"
chk "kuantitas tersimpan" 1 "$(dc "SELECT quantity FROM cart_item WHERE id='$CID';")"
chk "GET  /api/cart"              200 "$(code "${A[@]}" "$API/api/cart")"
chk "POST /api/cart (stok berlebih ditolak)" 400 "$(code -X POST "${A[@]}" -d "{\"product_id\":\"$PID\",\"quantity\":9999}" "$API/api/cart")"
chk "DELETE /api/cart/{id}"       200 "$(code -X DELETE "${A[@]}" "$API/api/cart/$CID")"
chk "keranjang bersih setelah delete" 0 "$(dc "SELECT COUNT(*) FROM cart_item WHERE id='$CID';")"
chk "POST /api/cart (ulang)"      200 "$(code -X POST "${A[@]}" -d "{\"product_id\":\"$PID\",\"quantity\":1}" "$API/api/cart")"

# ---------------------------------------------------------------- checkout + escrow
BAL0=$(dc "SELECT wallet_balance FROM \"user\" WHERE id='$ADMINID';")
STORE0=$(dc "SELECT wallet_balance FROM market_store WHERE id='$SID';")
chk "POST /api/market/checkout {cart:true}" 200 "$(code -X POST "${A[@]}" -d '{"cart":true}' "$API/api/market/checkout")"
OID=$(dc "SELECT id FROM market_order WHERE buyer_id='$ADMINID' ORDER BY created_at DESC LIMIT 1;")
chk "order langsung PAID" PAID "$(dc "SELECT status FROM market_order WHERE id='$OID';")"
chk "keranjang dikosongkan" 0 "$(dc "SELECT COUNT(*) FROM cart_item WHERE user_id='$ADMINID';")"
chk "stok produk turun 5 -> 4" 4 "$(dc "SELECT stock FROM market_product WHERE id='$PID';")"
BAL1=$(dc "SELECT wallet_balance FROM \"user\" WHERE id='$ADMINID';")
python -c "
print(('PASS' if float('$BAL1')<float('$BAL0') else 'FAIL')+'  saldo pembeli turun: $BAL0 -> $BAL1')" | tee -a "$OUT"
chk "escrow DITAHAN dulu (toko belum dibayar)" "$STORE0" "$(dc "SELECT wallet_balance FROM market_store WHERE id='$SID';")"
chk "POST /api/market/checkout (keranjang kosong)" 400 "$(code -X POST "${A[@]}" -d '{"cart":true}' "$API/api/market/checkout")"
dc "UPDATE \"user\" SET wallet_balance=0 WHERE id='$ADMINID';" >/dev/null
chk "POST /api/market/orders (saldo habis ditolak)" 402 "$(code -X POST "${A[@]}" -d "{\"product_id\":\"$PID\",\"qty\":1}" "$API/api/market/orders")"
dc "UPDATE \"user\" SET wallet_balance=50000 WHERE id='$ADMINID';" >/dev/null
chk "DELETE produk yang sudah dipesan -> 409" 409 "$(code -X DELETE "${S[@]}" "$API/api/store/products/$PID")"
chk "produk tetap ada setelah delete ditolak" 1 "$(dc "SELECT CASE WHEN COUNT(*)>0 THEN 1 ELSE 0 END FROM market_product WHERE id='$PID';")"

# ---------------------------------------------------------------- confirm + escrow cair + ulasan
chk "POST /api/market/orders/{id}/confirm" 200 "$(code -X POST "${A[@]}" "$API/api/market/orders/$OID/confirm")"
chk "order jadi COMPLETED" COMPLETED "$(dc "SELECT status FROM market_order WHERE id='$OID';")"
python -c "
s=float('$(dc "SELECT wallet_balance FROM market_store WHERE id='$SID';")')
print(('PASS' if s>0 else 'FAIL')+'  escrow cair ke toko setelah confirm: %.2f'%s)" | tee -a "$OUT"
chk "GET  /api/market/reviews?store_id=" 200 "$(code "$API/api/market/reviews?store_id=$SID")"
chk "POST /api/market/reviews"    200 "$(code -X POST "${A[@]}" -d "{\"store_id\":\"$SID\",\"product_id\":\"$PID\",\"rating\":5,\"comment\":\"bagus\"}" "$API/api/market/reviews")"
chk "ulasan tersimpan" 1 "$(dc "SELECT CASE WHEN COUNT(*)>0 THEN 1 ELSE 0 END FROM market_review WHERE store_id='$SID';")"
chk "POST /api/market/reviews (rating 9 ditolak)" 400 "$(code -X POST "${A[@]}" -d "{\"store_id\":\"$SID\",\"rating\":9}" "$API/api/market/reviews")"
chk "ulasan tanpa pembelian ditolak" 403 "$(code -X POST -H "Authorization: Bearer $TS" -H 'Content-Type: application/json' -d "{\"store_id\":\"$SID\",\"product_id\":\"$PID\",\"rating\":5}" "$API/api/market/reviews")"

# ---------------------------------------------------------------- cancel: refund + stok
chk "POST /api/market/orders (beli langsung)" 200 "$(code -X POST "${A[@]}" -d "{\"product_id\":\"$PID\",\"qty\":1}" "$API/api/market/orders")"
OID2=$(dc "SELECT id FROM market_order WHERE buyer_id='$ADMINID' AND status='PAID' ORDER BY created_at DESC LIMIT 1;")
STOCK0=$(dc "SELECT stock FROM market_product WHERE id='$PID';")
BALC0=$(dc "SELECT wallet_balance FROM \"user\" WHERE id='$ADMINID';")
chk "POST /api/market/orders/{id}/cancel" 200 "$(code -X POST "${A[@]}" "$API/api/market/orders/$OID2/cancel")"
chk "order jadi CANCELLED" CANCELLED "$(dc "SELECT status FROM market_order WHERE id='$OID2';")"
chk "stok dikembalikan" "$(python -c "print(int('$STOCK0')+1)")" "$(dc "SELECT stock FROM market_product WHERE id='$PID';")"
BALC1=$(dc "SELECT wallet_balance FROM \"user\" WHERE id='$ADMINID';")
python -c "
print(('PASS' if float('$BALC1')>float('$BALC0') else 'FAIL')+'  refund batal masuk: $BALC0 -> $BALC1')" | tee -a "$OUT"
chk "refund tercatat di buku besar" 1 "$(dc "SELECT CASE WHEN COUNT(*)>0 THEN 1 ELSE 0 END FROM credit_transaction WHERE type='REFUND' AND invoice_id='$OID2';")"
chk "POST cancel ulang -> 409" 409 "$(code -X POST "${A[@]}" "$API/api/market/orders/$OID2/cancel")"

# ---------------------------------------------------------------- penarikan
dc "UPDATE \"user\" SET wallet_balance=200000 WHERE id='$ADMINID';" >/dev/null
chk "POST /api/wallet/withdrawals (di bawah minimum)" 400 "$(code -X POST "${A[@]}" -d '{"amount":1000,"payment_type":"BANK"}' "$API/api/wallet/withdrawals")"
chk "POST /api/wallet/withdrawals" 200 "$(code -X POST "${A[@]}" -d '{"amount":60000,"payment_type":"BANK"}' "$API/api/wallet/withdrawals")"
WID=$(dc "SELECT id FROM withdrawal WHERE user_id='$ADMINID' ORDER BY created_at DESC LIMIT 1;")
chk "ajuan PENDING" PENDING "$(dc "SELECT status FROM withdrawal WHERE id='$WID';")"
BALW0=$(dc "SELECT wallet_balance FROM \"user\" WHERE id='$ADMINID';")
chk "POST /api/admin/withdrawals/{id}/reject" 200 "$(code -X POST "${A[@]}" "$API/api/admin/withdrawals/$WID/reject")"
chk "penarikan REJECTED" REJECTED "$(dc "SELECT status FROM withdrawal WHERE id='$WID';")"
BALW1=$(dc "SELECT wallet_balance FROM \"user\" WHERE id='$ADMINID';")
python -c "
print(('PASS' if float('$BALW1')>float('$BALW0') else 'FAIL')+'  saldo kembali setelah reject: $BALW0 -> $BALW1')" | tee -a "$OUT"
chk "POST /api/wallet/withdrawals (ajuan kedua)" 200 "$(code -X POST "${A[@]}" -d '{"amount":60000,"payment_type":"BANK"}' "$API/api/wallet/withdrawals")"
WID2=$(dc "SELECT id FROM withdrawal WHERE user_id='$ADMINID' ORDER BY created_at DESC LIMIT 1;")
chk "POST /api/admin/withdrawals/{id}/approve" 200 "$(code -X POST "${A[@]}" "$API/api/admin/withdrawals/$WID2/approve")"
chk "penarikan APPROVED" APPROVED "$(dc "SELECT status FROM withdrawal WHERE id='$WID2';")"
chk "approve dua kali -> 409" 409 "$(code -X POST "${A[@]}" "$API/api/admin/withdrawals/$WID2/approve")"
chk "penarikan masuk audit_log" 1 "$(dc "SELECT CASE WHEN COUNT(*)>0 THEN 1 ELSE 0 END FROM audit_log WHERE action='WITHDRAWAL_APPROVED';")"

# ---------------------------------------------------------------- topup + paket
chk "POST /api/wallet/topup"      200 "$(code -X POST "${A[@]}" -d '{"amount":25000}' "$API/api/wallet/topup")"
TID=$(dc "SELECT id FROM credit_transaction WHERE type='TOPUP' AND user_id='$ADMINID' ORDER BY created_at DESC LIMIT 1;")
chk "topup belum menambah saldo (PENDING)" PENDING "$(dc "SELECT status FROM credit_transaction WHERE id='$TID';")"
BALT0=$(dc "SELECT wallet_balance FROM \"user\" WHERE id='$ADMINID';")
chk "POST /api/admin/topup/{id}/complete" 200 "$(code -X POST "${A[@]}" "$API/api/admin/topup/$TID/complete")"
BALT1=$(dc "SELECT wallet_balance FROM \"user\" WHERE id='$ADMINID';")
python -c "
print(('PASS' if abs(float('$BALT1')-float('$BALT0')-25000)<0.01 else 'FAIL')+'  topup complete menambah 25000: $BALT0 -> $BALT1')" | tee -a "$OUT"
PKG=$(dc "SELECT id FROM token_package WHERE is_active ORDER BY price LIMIT 1;")
BALP0=$(dc "SELECT wallet_balance FROM \"user\" WHERE id='$ADMINID';")
chk "POST /api/packages/{id}/buy" 200 "$(code -X POST "${A[@]}" "$API/api/packages/$PKG/buy")"
chk "paket tercatat di user_token_package" 1 "$(dc "SELECT CASE WHEN COUNT(*)>0 THEN 1 ELSE 0 END FROM user_token_package WHERE user_id='$ADMINID';")"
python -c "
print(('PASS' if float('$(dc "SELECT wallet_balance FROM \"user\" WHERE id='$ADMINID';")')<float('$BALP0') else 'FAIL')+'  harga paket dipotong')" | tee -a "$OUT"
chk "POST /api/packages/{id}/buy (paket tak ada)" 404 "$(code -X POST "${A[@]}" "$API/api/packages/00000000-0000-0000-0000-000000000000/buy")"

# ---------------------------------------------------------------- reward / referral / afiliasi
chk "GET  /api/reward/daily belum diklaim" False "$(code "${A[@]}" "$API/api/reward/daily" >/dev/null; jget "d['claimed']")"
chk "POST /api/reward/daily (klaim)" 200 "$(code -X POST "${A[@]}" "$API/api/reward/daily")"
chk "POST /api/reward/daily (klaim ulang ditolak)" 400 "$(code -X POST "${A[@]}" "$API/api/reward/daily")"
chk "hadiah masuk buku besar" 1 "$(dc "SELECT CASE WHEN COUNT(*)>0 THEN 1 ELSE 0 END FROM credit_transaction WHERE type='REWARD' AND user_id='$ADMINID';")"
chk "GET  /api/referral"          200 "$(code "${A[@]}" "$API/api/referral")"
chk "referral punya kode"         1 "$(jget "1 if d['code'] else 0")"
RCODE=$(jget "d['code']")

# ---- parity new-api: bonus undangan saat pendaftaran (aff_code)
AFFEMAIL="verif-aff-$RANDOM@test.local"
BAL0=$(dc "SELECT wallet_balance FROM \"user\" WHERE id='$ADMINID';")
RB1=$(dc "SELECT COUNT(*) FROM user_token_package utp JOIN token_package tp ON tp.id=utp.token_package_id WHERE utp.user_id='$ADMINID' AND tp.name='Reward Referral';")
REG=$(code -X POST -H 'Content-Type: application/json' -d "{\"first_name\":\"Verif\",\"last_name\":\"Referral\",\"email\":\"$AFFEMAIL\",\"password\":\"password123\",\"password_confirm\":\"password123\",\"aff_code\":\"$RCODE\"}" "$API/api/auth/register")
chk "aff: register + aff_code" 201 "$REG"
AFFUID=$(dc "SELECT id FROM \"user\" WHERE email='$AFFEMAIL';")
chk "aff: signup tercatat" 1 "$(dc "SELECT COUNT(*) FROM referral_signup WHERE invitee_id='$AFFUID' AND inviter_id='$ADMINID';")"
# Bonus instan saat daftar dihapus (anti-farming akun buangan): lunas saat
# topup PERTAMA invitee, lewat SettleTopup/StripeWebhook. Baseline RB1 = sisa
# paket reward referral lama milik admin, dibandingkan delta. Reward = PAKET
# TOKEN API (bukan wallet) — keputusan: referral token, afiliasi wallet.
chk "aff: bonus TIDAK instan saat signup" 0 "$(dc "SELECT GREATEST(0, COUNT(*) - $RB1) FROM user_token_package utp JOIN token_package tp ON tp.id=utp.token_package_id WHERE utp.user_id='$ADMINID' AND tp.name='Reward Referral';")"
chk "aff: saldo invitee masih 0" 0 "$(dc "SELECT wallet_balance::int FROM \"user\" WHERE id='$AFFUID';")"
# Topup pertama invitee → bonus kedua sisi + komisi inviter. (jget membaca
# respons code() terakhir dari gw_out.json — login dulu, baru ambil token.)
code -X POST -H 'Content-Type: application/json' -d "{\"email\":\"$AFFEMAIL\",\"password\":\"password123\"}" "$API/api/auth/login" >/dev/null
ITOK=$(jget "d['token']")
chk "aff: invitee ajukan topup" 200 "$(code -X POST -H "Authorization: Bearer $ITOK" -H 'Content-Type: application/json' -d '{"amount":10000}' "$API/api/wallet/topup")"
TUPTX=$(dc "SELECT id FROM credit_transaction WHERE user_id='$AFFUID' AND type='TOPUP' AND status='PENDING' ORDER BY created_at DESC LIMIT 1;")
chk "aff: admin selesaikan topup" 200 "$(code -X POST "${A[@]}" "$API/api/admin/topup/$TUPTX/complete")"
# Kedua sisi dapat PAKET TOKEN 'Reward Referral': invitee (akun segar) = 1;
# inviter (admin) = +1 dari baseline. Wallet KEDUA sisi tidak berubah oleh bonus.
chk "aff: bonus kedua sisi lunas pasca-topup" 2 "$(dc "SELECT (SELECT COUNT(*) FROM user_token_package utp JOIN token_package tp ON tp.id=utp.token_package_id WHERE utp.user_id='$AFFUID' AND tp.name='Reward Referral') + (SELECT GREATEST(0, COUNT(*) - $RB1) FROM user_token_package utp JOIN token_package tp ON tp.id=utp.token_package_id WHERE utp.user_id='$ADMINID' AND tp.name='Reward Referral');")"
chk "aff: tidak dibayar dua kali (topup ulang)" 2 "$(code -X POST -H "Authorization: Bearer $ITOK" -H 'Content-Type: application/json' -d '{"amount":10000}' "$API/api/wallet/topup" >/dev/null; code -X POST "${A[@]}" "$API/api/admin/topup/$(dc "SELECT id FROM credit_transaction WHERE user_id='$AFFUID' AND type='TOPUP' AND status='PENDING' ORDER BY created_at DESC LIMIT 1;")/complete" >/dev/null; dc "SELECT (SELECT COUNT(*) FROM user_token_package utp JOIN token_package tp ON tp.id=utp.token_package_id WHERE utp.user_id='$AFFUID' AND tp.name='Reward Referral') + (SELECT GREATEST(0, COUNT(*) - $RB1) FROM user_token_package utp JOIN token_package tp ON tp.id=utp.token_package_id WHERE utp.user_id='$ADMINID' AND tp.name='Reward Referral');")"
chk "aff: kode ngawur tidak gagalkan daftar" 201 "$(code -X POST -H 'Content-Type: application/json' -d "{\"first_name\":\"Verif\",\"last_name\":\"TanpaRef\",\"email\":\"verif-aff2-$RANDOM@test.local\",\"password\":\"password123\",\"password_confirm\":\"password123\",\"aff_code\":\"NGAWUR123\"}" "$API/api/auth/register")"
dc "DELETE FROM \"user\" WHERE email LIKE 'verif-aff%@test.local';" >/dev/null
chk "aff: user uji terhapus ikut jejaknya" 0 "$(dc "SELECT COUNT(*) FROM referral_signup WHERE invitee_id='$AFFUID';")"
chk "POST /api/affiliate/links"   200 "$(code -X POST "${A[@]}" -d "{\"product_id\":\"$PID\"}" "$API/api/affiliate/links")"
ACODE=$(dc "SELECT code FROM affiliate_link WHERE user_id='$ADMINID' ORDER BY created_at DESC LIMIT 1;")
chk "afiliasi produk sendiri ditolak" 400 "$(code -X POST "${S[@]}" -d "{\"product_id\":\"$PID\"}" "$API/api/affiliate/links")"
chk "GET  /api/affiliate/links/{code}/click (publik)" 200 "$(code "$API/api/affiliate/links/$ACODE/click")"
chk "klik afiliasi tercatat" 1 "$(dc "SELECT CASE WHEN COUNT(*)>0 THEN 1 ELSE 0 END FROM affiliate_click WHERE link_id=(SELECT id FROM affiliate_link WHERE code='$ACODE');")"
chk "klik kode ngawur -> 404" 404 "$(code "$API/api/affiliate/links/ZZZZZZZZ/click")"

# ---------------------------------------------------------------- 2FA (TOTP stdlib)
chk "POST /api/account/2fa/setup" 200 "$(code -X POST "${A[@]}" "$API/api/account/2fa/setup")"
SEED=$(jget "d['secret']")
chk "setup mengembalikan URI otpauth" 1 "$(jget "1 if d['uri'].startswith('otpauth://totp/') else 0")"
chk "rahasia 2FA tersimpan terenkripsi (v1.)" "v1." "$(dc "SELECT left(secret,3) FROM user_auth_method WHERE user_id='$ADMINID' AND method='totp';")"
chk "POST /api/account/2fa/enable (kode salah)" 400 "$(code -X POST "${A[@]}" -d '{"code":"000000"}' "$API/api/account/2fa/enable")"
chk "POST /api/account/2fa/enable (TOTP benar)" 200 "$(code -X POST "${A[@]}" -d "{\"code\":\"$(totp "$SEED")\"}" "$API/api/account/2fa/enable")"
chk "2FA aktif" t "$(dc "SELECT confirmed FROM user_auth_method WHERE user_id='$ADMINID' AND method='totp';")"
chk "login 2FA tanpa kode ditolak" 401 "$(code -X POST "$API/api/auth/login" -H 'Content-Type: application/json' -d '{"email":"admin@test.local","password":"Test1234!"}')"
chk "login 2FA dengan kode TOTP" 200 "$(code -X POST "$API/api/auth/login" -H 'Content-Type: application/json' -d "{\"email\":\"admin@test.local\",\"password\":\"Test1234!\",\"totp\":\"$(totp "$SEED")\"}")"
TA=$(tok); A=(-H "Authorization: Bearer $TA" -H 'Content-Type: application/json')
dc "UPDATE user_auth_method SET confirmed=false WHERE user_id='$ADMINID' AND method='totp';" >/dev/null
chk "GET  /api/account/sessions"  200 "$(code "${A[@]}" "$API/api/account/sessions")"
chk "sesi 2FA tadi tercatat" 1 "$(dc "SELECT CASE WHEN COUNT(*)>=2 THEN 1 ELSE 0 END FROM user_session WHERE user_id='$ADMINID';")"
SESSID=$(dc "SELECT id FROM user_session WHERE user_id='$ADMINID' ORDER BY created_at ASC LIMIT 1;")
chk "DELETE /api/account/sessions/{id}" 200 "$(code -X DELETE "${A[@]}" "$API/api/account/sessions/$SESSID")"
chk "sesi terhapus" 0 "$(dc "SELECT COUNT(*) FROM user_session WHERE id='$SESSID';")"
chk "DELETE /api/account/sessions/{id} lagi -> 404" 404 "$(code -X DELETE "${A[@]}" "$API/api/account/sessions/$SESSID")"

# ---------------------------------------------------------------- media (multipart)
python -c "import base64,os;open(os.environ['LOCALAPPDATA']+r'\Temp\gw_up.png','wb').write(base64.b64decode('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFAAH/q842iQAAAABJRU5ErkJggg=='))"
chk "POST /api/media (multipart PNG)" 200 "$(code -X POST -H "Authorization: Bearer $TA" -F "file=@$TMP/gw_up.png;type=image/png" "$API/api/media")"
MID=$(dc "SELECT id FROM media_asset ORDER BY created_at DESC LIMIT 1;")
chk "media tersimpan utuh (70 byte)" 70 "$(dc "SELECT bytes FROM media_asset WHERE id='$MID';")"
chk "GET  /api/media/{id} tanpa auth -> 404" 404 "$(code "$API/api/media/$MID")"
chk "GET  /api/media/{id} pemilik" 200 "$(code "${A[@]}" "$API/api/media/$MID")"
chk "content-type media pemilik = image/png" image/png "$(curl -s "${A[@]}" -o /dev/null -w '%{content_type}' "$API/api/media/$MID")"
chk "byte pertama media pemilik tetap PNG" 89504e47 "$(curl -s "${A[@]}" "$API/api/media/$MID" | head -c 4 | xxd -p)"
printf 'bukan gambar' > "$TMP/gw_bad.txt"
chk "POST /api/media (bukan gambar) -> 415" 415 "$(code -X POST -H "Authorization: Bearer $TA" -F "file=@$TMP/gw_bad.txt" "$API/api/media")"
chk "POST /api/media tanpa auth -> 401" 401 "$(code -X POST -F "file=@$TMP/gw_up.png" "$API/api/media")"

# ---------------------------------------------------------------- artikel
chk "POST /api/articles"          200 "$(code -X POST "${A[@]}" -d '{"title":"Artikel Verifikasi","excerpt":"ringkas","content":"isi artikel verifikasi yang cukup panjang"}' "$API/api/articles")"
ARTID=$(dc "SELECT id FROM article WHERE title='Artikel Verifikasi' ORDER BY created_at DESC LIMIT 1;")
chk "artikel user masuk PENDING" PENDING "$(dc "SELECT status FROM article WHERE id='$ARTID';")"
PSLUG=$(dc "SELECT slug FROM article WHERE id='$ARTID';")
code "$API/api/articles" >/dev/null
chk "artikel PENDING tidak tampil publik" 0 "$(jget "1 if any(a['slug']=='$PSLUG' for a in d['articles']) else 0")"
chk "POST /api/articles (berisi link) ditolak" 400 "$(code -X POST "${A[@]}" -d '{"title":"Artikel Link","content":"lihat http://contoh.com/promo sekarang juga ya"}' "$API/api/articles")"
chk "GET  /api/admin/articles"    200 "$(code "${A[@]}" "$API/api/admin/articles")"
chk "PATCH /api/admin/articles/{id} (publish)" 200 "$(code -X PATCH "${A[@]}" -d '{"published":true}' "$API/api/admin/articles/$ARTID")"
code "$API/api/articles" >/dev/null
chk "artikel PUBLISHED tampil publik" 1 "$(jget "1 if any(a['slug']=='$PSLUG' for a in d['articles']) else 0")"
ASLUG=$(dc "SELECT slug FROM article WHERE id='$ARTID';")
chk "GET  /api/articles/{slug}"   200 "$(code "$API/api/articles/$ASLUG")"
chk "POST /api/admin/articles (publish langsung)" 200 "$(code -X POST "${A[@]}" -d '{"title":"Pengumuman Admin","content":"pengumuman resmi dari admin platform ya","published":true}' "$API/api/admin/articles")"
chk "artikel admin langsung PUBLISHED" PUBLISHED "$(dc "SELECT status FROM article WHERE title='Pengumuman Admin' ORDER BY created_at DESC LIMIT 1;")"

# ---------------------------------------------------------------- admin
chk "POST /api/market/orders (untuk refund admin)" 200 "$(code -X POST "${A[@]}" -d "{\"product_id\":\"$PID\",\"qty\":1}" "$API/api/market/orders")"
OIDREF=$(dc "SELECT id FROM market_order WHERE buyer_id='$ADMINID' AND status='PAID' ORDER BY created_at DESC LIMIT 1;")
chk "GET  /api/admin/summary"     200 "$(code "${A[@]}" "$API/api/admin/summary")"
chk "GET  /api/admin/users"       200 "$(code "${A[@]}" "$API/api/admin/users")"
chk "GET  /api/admin/orders"      200 "$(code "${A[@]}" "$API/api/admin/orders")"
chk "GET  /api/admin/products"    200 "$(code "${A[@]}" "$API/api/admin/products")"
chk "PATCH /api/admin/products (pin_order)" 200 "$(code -X PATCH "${A[@]}" -d "{\"id\":\"$PID\",\"is_active\":true,\"pin_order\":1}" "$API/api/admin/products")"
chk "pin_order tersimpan" 1 "$(dc "SELECT pin_order FROM market_product WHERE id='$PID';")"
chk "moderasi produk masuk audit_log" 1 "$(dc "SELECT CASE WHEN COUNT(*)>0 THEN 1 ELSE 0 END FROM audit_log WHERE action='PRODUCT_MODERATE' AND target='$PID';")"
# Search + kategori publik (fitur marketplace anonim). Di sini produk uji sudah
# ada DAN aktif (pin_order di atas mengaktifkannya) — di blok publik belum ada.
chk "search ?q=verif menemukan produk"   1 "$(code "$API/api/market/products?q=verif" >/dev/null; has title)"
chk "search ?q=ngawur kosong"            0 "$(code "$API/api/market/products?q=ZZZNOPE" >/dev/null; has title)"
chk "filter ?category=slug valid"      200 "$(code "$API/api/market/products?category=data-dataset")"
chk "filter ?category=ngawur kosong"     0 "$(code "$API/api/market/products?category=slugngawur" >/dev/null; has title)"
chk "kategori menyebut slug"             1 "$(code "$API/api/market/categories" >/dev/null; has data-dataset)"
chk "GET  /api/admin/wallet"      200 "$(code "${A[@]}" "$API/api/admin/wallet")"
chk "POST /api/admin/orders/{id}/refund (order PAID)" 200 "$(code -X POST "${A[@]}" "$API/api/admin/orders/$OIDREF/refund")"
chk "refund admin -> CANCELLED" CANCELLED "$(dc "SELECT status FROM market_order WHERE id='$OIDREF';")"
chk "refund admin masuk audit_log" 1 "$(dc "SELECT CASE WHEN COUNT(*)>0 THEN 1 ELSE 0 END FROM audit_log WHERE action='ORDER_REFUND' AND target='$OIDREF';")"
chk "POST refund pada order selesai -> 409" 409 "$(code -X POST "${A[@]}" "$API/api/admin/orders/$OID/refund")"
chk "GET  /api/admin/uptime"      200 "$(code "${A[@]}" "$API/api/admin/uptime")"
chk "POST /api/admin/uptime (add)" 200 "$(code -X POST "${A[@]}" -d '{"action":"add","name":"Gateway Verif","kind":"HTTP","url":"http://gateway:8080/healthz"}' "$API/api/admin/uptime")"
chk "monitor tercatat" 1 "$(dc "SELECT CASE WHEN COUNT(*)>0 THEN 1 ELSE 0 END FROM uptime_monitor WHERE name='Gateway Verif';")"
chk "POST /api/admin/uptime (host internal diizinkan saat dev)" 200 "$(code -X POST "${A[@]}" -d '{"action":"add","name":"DB","kind":"HTTP","url":"http://127.0.0.1:22"}' "$API/api/admin/uptime")"
chk "POST /api/admin/uptime (url non-http ditolak)" 400 "$(code -X POST "${A[@]}" -d '{"action":"add","name":"Aneh","kind":"HTTP","url":"file:///etc/passwd"}' "$API/api/admin/uptime")"
chk "POST /api/admin/uptime (url kosong ditolak)" 400 "$(code -X POST "${A[@]}" -d '{"action":"add","name":"Kosong","kind":"HTTP","url":""}' "$API/api/admin/uptime")"
chk "GET  /api/admin/health"      200 "$(code "${A[@]}" "$API/api/admin/health")"
chk "GET  /api/admin/audit"       200 "$(code "${A[@]}" "$API/api/admin/audit")"
chk "audit_log terisi (>=10 aksi)" 1 "$(dc "SELECT CASE WHEN COUNT(*)>=10 THEN 1 ELSE 0 END FROM audit_log;")"
chk "GET  /api/admin/settings"    200 "$(code "${A[@]}" "$API/api/admin/settings")"
chk "PATCH /api/admin/settings"   200 "$(code -X PATCH "${A[@]}" -d '{"market_fee_percent":"5"}' "$API/api/admin/settings")"
chk "perubahan setting masuk audit_log" 1 "$(dc "SELECT CASE WHEN COUNT(*)>0 THEN 1 ELSE 0 END FROM audit_log WHERE action='SETTINGS_UPDATE';")"
chk "GET  /api/admin/providers"   200 "$(code "${A[@]}" "$API/api/admin/providers")"
chk "non-admin ditolak panel" 403 "$(code "${S[@]}" "$API/api/admin/orders")"
chk "tanpa token ditolak" 401 "$(code "$API/api/admin/orders")"

# ---------------------------------------------------------------- peran + guard admin terakhir
chk "PATCH role sendiri ditolak" 400 "$(code -X PATCH "${A[@]}" -d '{"role":"USER"}' "$API/api/admin/users/$ADMINID/role")"
chk "peran tak dikenal ditolak" 400 "$(code -X PATCH "${A[@]}" -d '{"role":"DEWA"}' "$API/api/admin/users/$SELLERID/role")"
chk "naikkan SELLER -> ADMIN" 200 "$(code -X PATCH "${A[@]}" -d '{"role":"ADMIN"}' "$API/api/admin/users/$SELLERID/role")"
chk "perubahan peran masuk audit_log" 1 "$(dc "SELECT CASE WHEN COUNT(*)>0 THEN 1 ELSE 0 END FROM audit_log WHERE action='USER_ROLE_CHANGE' AND target='$SELLERID';")"
chk "cabut ADMIN saat tinggal 2 -> boleh" 200 "$(code -X PATCH "${A[@]}" -d '{"role":"USER"}' "$API/api/admin/users/$SELLERID/role")"
dc "UPDATE \"user\" SET role='ADMIN' WHERE id='$SELLERID';" >/dev/null
chk "admin selalu tersisa >= 1 setelah semua operasi" 1 "$(dc "SELECT CASE WHEN COUNT(*)>=1 THEN 1 ELSE 0 END FROM \"user\" WHERE role IN ('ADMIN','SUPER_ADMIN');")"
chk "cabut peran sendiri ditolak (tak bisa buat 0 admin)" 400 "$(code -X PATCH "${A[@]}" -d '{"role":"USER"}' "$API/api/admin/users/$ADMINID/role")"
dc "UPDATE \"user\" SET role='SELLER' WHERE id='$SELLERID';" >/dev/null

# ================================================================ BUG A
# Charge menulis usage_log 'OK' + biaya penuh SEBELUM provider dipanggil.
# Kalau provider gagal, baris itu harus jadi ERROR + biaya 0 + saldo kembali.
# (Paket dari uji "topup + paket" dihapus dulu: tanpa itu refund mengalir ke
# paket, bukan saldo — asersi di bawah mengasumsikan jalur saldo.)
dc "DELETE FROM user_token_package up USING \"user\" u WHERE u.id=up.user_id AND u.email='admin@test.local';" >/dev/null
dc "UPDATE \"user\" SET wallet_balance=50000 WHERE id='$ADMINID';" >/dev/null
# Baris OK sisa run sebelumnya akan mencemari jumlah di bawah — bersihkan dulu.
dc "DELETE FROM usage_log WHERE model='gpt-4o-mini';" >/dev/null
BALB=$(dc "SELECT wallet_balance FROM \"user\" WHERE id='$ADMINID';")
REF0=$(dc "SELECT COUNT(*) FROM credit_transaction WHERE type='REFUND' AND user_id='$ADMINID';")
chk "POST /api/chat, tanpa provider -> 502" 502 "$(code -X POST "${A[@]}" -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"halo"}]}' "$API/api/chat")"
BALA=$(dc "SELECT wallet_balance FROM \"user\" WHERE id='$ADMINID';")
chk "BUG A: status usage_log = ERROR" ERROR "$(dc "SELECT status FROM usage_log WHERE model='gpt-4o-mini' ORDER BY created_at DESC LIMIT 1;")"
chk "BUG A: biaya baris gagal = 0" 0.000000 "$(dc "SELECT cost FROM usage_log WHERE model='gpt-4o-mini' ORDER BY created_at DESC LIMIT 1;")"
chk "BUG A: saldo tidak terpotong" "$BALB" "$BALA"
chk "BUG A: baris REFUND bertambah" "$(python -c "print(int('$REF0')+1)")" "$(dc "SELECT COUNT(*) FROM credit_transaction WHERE type='REFUND' AND user_id='$ADMINID';")"
chk "BUG A: laporan usage tidak menghitung panggilan gagal" 0.000000 "$(code "${A[@]}" "$API/api/dashboard/usage" >/dev/null; jget "'%.6f'%sum(u['cost'] for u in d['usage'] if u['status']=='OK' and u['model']=='gpt-4o-mini')")"

# ================================================================ BUG B
chk "BUG B: model di luar katalog -> 400" 400 "$(code -X POST "${A[@]}" -d '{"model":"tidak-ada","messages":[{"role":"user","content":"x"}]}' "$API/api/chat")"
code -X POST "${A[@]}" -d '{"model":"tidak-ada","messages":[{"role":"user","content":"x"}]}' "$API/api/chat" >/dev/null
chk "BUG B: pesannya soal katalog, bukan saldo" 1 "$(has "tidak ada di katalog")"
chk "BUG B: pesan tidak menyebut saldo" 0 "$(has "saldo")"
dc "UPDATE \"user\" SET wallet_balance=0 WHERE id='$ADMINID';" >/dev/null
chk "BUG B: saldo habis -> 402" 402 "$(code -X POST "${A[@]}" -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"x"}]}' "$API/api/chat")"
code -X POST "${A[@]}" -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"x"}]}' "$API/api/chat" >/dev/null
chk "BUG B: pesan = saldo tidak cukup" 1 "$(has "saldo tidak cukup")"
dc "UPDATE \"user\" SET wallet_balance=50000 WHERE id='$ADMINID';" >/dev/null

# ================================================================ BUG C
dc "INSERT INTO provider (name,label,type,base_url,api_key,is_active,priority) VALUES ('verif','Verifikasi','openai','https://api.example.com/v1','x',false,0);" >/dev/null
PROVID=$(dc "SELECT id FROM provider WHERE name='verif';")
chk "PATCH /api/admin/providers (set kunci)" 200 "$(code -X PATCH "${A[@]}" -d "{\"id\":\"$PROVID\",\"api_key\":\"sk-plaintextverifikasi1234\"}" "$API/api/admin/providers")"
chk "BUG C: DB menyimpan ciphertext (v1.)" "v1." "$(dc "SELECT left(api_key,3) FROM provider WHERE id='$PROVID';")"
chk "BUG C: kunci mentah tidak ada di kolom DB" 0 "$(dc "SELECT COUNT(*) FROM provider WHERE api_key LIKE '%plaintextverifikasi%';")"
code "${A[@]}" "$API/api/admin/providers" >/dev/null
chk "BUG C: kunci mentah tidak bocor lewat API" 0 "$(jget "1 if 'plaintextverifikasi' in json.dumps(d) else 0")"
chk "BUG C: API hanya beri masker 4 karakter terakhir" "••••1234" "$(jget "[p['key_mask'] for p in d['providers'] if p['name']=='verif'][0]")"
dc "DELETE FROM provider WHERE name='verif';" >/dev/null

# ================================================================ AUDIT (kontrak + authz + uang)
# Halaman frontend memanggil endpoint ini; sebelum audit sebagian belum ada
# (404) atau mengembalikan nama field yang berbeda.
# Kuota rate-limit user dibersihkan dulu (sama seperti di awal skrip): blok ini
# menambah belasan panggilan dan tanpa itu semuanya jadi 429.
docker compose exec -T redis sh -c 'redis-cli -a "$(cat /run/secrets/redis_pass)" EVAL \
  "local n=0 for _,k in ipairs(redis.call('keys','rl:user:*')) do redis.call('del',k) n=n+1 end return n" 0' >/dev/null 2>&1
chk "GET  /api/account/2fa" 200 "$(code "${A[@]}" "$API/api/account/2fa")"
chk "2FA: field active ada" 1 "$(jget "'active' in d and 1 or 0")"

chk "GET  /api/reward/daily" 200 "$(code "${A[@]}" "$API/api/reward/daily")"
chk "reward: claimed_today + streak + history" 1 \
  "$(jget "1 if ('claimed_today' in d and 'streak' in d and 'history' in d) else 0")"

chk "GET  /api/referral" 200 "$(code "${A[@]}" "$API/api/referral")"
chk "referral: myCode + summary" 1 "$(jget "1 if ('myCode' in d and 'summary' in d) else 0")"

chk "GET  /api/affiliate/links" 200 "$(code "${A[@]}" "$API/api/affiliate/links")"
chk "afiliasi: summary.earnings" 1 "$(jget "1 if 'earnings' in d.get('summary',{}) else 0")"

# Keranjang: checkout tanpa body ({}), bentuk yang dikirim halaman /keranjang.
dc "DELETE FROM cart_item WHERE user_id='$ADMINID';" >/dev/null
dc "INSERT INTO cart_item (user_id, product_id, quantity) VALUES ('$ADMINID','$PID',1);" >/dev/null
chk "POST /api/market/checkout (body {} = keranjang)" 200 \
  "$(code -X POST "${A[@]}" -d '{}' "$API/api/market/checkout")"
chk "checkout keranjang: order terbentuk" 1 \
  "$(dc "SELECT CASE WHEN COUNT(*)>0 THEN 1 ELSE 0 END FROM market_order WHERE buyer_id='$ADMINID' AND status='PAID';")"
chk "checkout keranjang: keranjang dikosongkan" 0 \
  "$(dc "SELECT COUNT(*) FROM cart_item WHERE user_id='$ADMINID';")"

# Ulasan tidak boleh dipakai menyerang toko lain: produk yang dibeli di toko A
# tidak bisa dipakai mengulas toko B.
OTHERSID=$(dc "SELECT id FROM market_store WHERE id <> '$SID' LIMIT 1;")
chk "ulasan: produk bukan milik toko -> 400" 400 \
  "$(code -X POST "${A[@]}" -d "{\"store_id\":\"$OTHERSID\",\"product_id\":\"$PID\",\"rating\":5}" "$API/api/market/reviews")"
chk "ulasan toko tanpa pembelian -> 403" 403 \
  "$(code -X POST -H "Authorization: Bearer $TS" -H 'Content-Type: application/json' \
        -d "{\"store_id\":\"$SID\",\"rating\":5}" "$API/api/market/reviews")"

# Tujuan pencairan harus ikut tersimpan, bukan dibuang.
dc "UPDATE \"user\" SET wallet_balance=100000 WHERE id='$ADMINID';" >/dev/null
chk "POST /api/wallet/withdrawals" 200 \
  "$(code -X POST "${A[@]}" -d '{"amount":50000,"bank_account":"BCA 1234567890"}' "$API/api/wallet/withdrawals")"
chk "penarikan: tujuan tersimpan di DB" "BCA 1234567890" \
  "$(dc "SELECT destination FROM withdrawal WHERE user_id='$ADMINID' ORDER BY created_at DESC LIMIT 1;")"
chk "GET /api/admin/wallet tampilkan tujuan" 1 \
  "$(code "${A[@]}" "$API/api/admin/wallet" >/dev/null; jget "1 if any(w.get('bank_account') for w in d.get('withdrawals',[])) else 0")"
WID=$(dc "SELECT id FROM withdrawal WHERE user_id='$ADMINID' ORDER BY created_at DESC LIMIT 1;")
chk "POST /api/admin/withdrawals/{id}/reject" 200 \
  "$(code -X POST "${A[@]}" "$API/api/admin/withdrawals/$WID/reject")"
chk "reject: saldo penarikan kembali" 100000 \
  "$(dc "SELECT wallet_balance::int FROM \"user\" WHERE id='$ADMINID';")"

# PATCH /api/admin/products/{id} — bentuk yang dipanggil /admin/products.
chk "PATCH /api/admin/products/{id}" 200 \
  "$(code -X PATCH "${A[@]}" -d '{"is_active":true}' "$API/api/admin/products/$PID")"

# Idempotensi uang: klaim hadiah harian dua kali hanya membayar sekali.
dc "DELETE FROM daily_reward_claim WHERE user_id='$ADMINID' AND reward_date='$(date -u +%F)';" >/dev/null
chk "POST /api/reward/daily (klaim ke-1)" 200 "$(code -X POST "${A[@]}" "$API/api/reward/daily")"
chk "POST /api/reward/daily (klaim ke-2 ditolak)" 400 "$(code -X POST "${A[@]}" "$API/api/reward/daily")"
chk "hadiah harian tersimpan satu baris" 1 \
  "$(dc "SELECT COUNT(*) FROM daily_reward_claim WHERE user_id='$ADMINID' AND reward_date='$(date -u +%F)';")"
dc "DELETE FROM daily_reward_claim WHERE user_id='$ADMINID';" >/dev/null

# ---------------------------------------------------------------- bersihkan artikel uji
dc "DELETE FROM article WHERE title IN ('Artikel Verifikasi','Pengumuman Admin');" >/dev/null

# ================================================================ PORT new-api
# Streaming SSE, endpoint kompatibel OpenAI/Anthropic (/v1/*), kode redeem,
# kuota per API key, admin CRUD model + cek koneksi provider.
#
# Mock upstream (gateway/bin/mock-openai.py) dijalankan lokal dan providernya
# didaftarkan lewat jalur admin — jadi registry memuatnya lewat Reload yang
# sama dipakai web admin, bukan pintasan SQL. Semua data uji + provider mock
# dibersihkan di akhir blok supaya blok BUG A pada run berikutnya tetap bersih.
# Model uji = gpt-4o (bukan gpt-4o-mini) supaya baris usage uji tidak ikut
# terhitung oleh pemeriksaan BUG A yang menyaring model gpt-4o-mini.
MOSEL=gpt-4o
python gateway/bin/mock-openai.py 18081 &
MOCKPID=$!
sleep 1
docker compose exec -T redis sh -c 'redis-cli -a "$(cat /run/secrets/redis_pass)" EVAL \
  "local n=0 for _,k in ipairs(redis.call('keys','rl:*')) do redis.call('del',k) n=n+1 end return n" 0' >/dev/null 2>&1
dc "DELETE FROM provider WHERE name='verif-stream';" >/dev/null
dc "INSERT INTO provider (name,label,type,base_url,api_key,is_active,priority,models) VALUES ('verif-stream','Mock SSE','openai','http://host.docker.internal:18081/v1','sk-mock',true,50,'');" >/dev/null
MOCKPROV=$(dc "SELECT id FROM provider WHERE name='verif-stream';")
chk "PATCH /api/admin/providers (aktifkan provider mock)" 200 \
  "$(code -X PATCH "${A[@]}" -d "{\"id\":\"$MOCKPROV\",\"is_active\":true,\"models\":\"\"}" "$API/api/admin/providers")"

# ---- API key + /v1/models
# Sisa run sebelumnya meninggalkan baris api_key non-aktif bernama sama; key
# baru + penyaringan is_active di bawah membuat run berulang tetap bersih.
dc "DELETE FROM api_key WHERE user_id='$ADMINID' AND name='verif-v1';" >/dev/null
chk "POST /api/keys (key uji v1)" 200 "$(code -X POST "${A[@]}" -d '{"name":"verif-v1"}' "$API/api/keys")"
VKEY=$(jget "d['key']")
VH=$(printf 'Authorization: Bearer %s' "$VKEY")
K=(-H "$VH" -H 'Content-Type: application/json')
VKID=$(dc "SELECT id FROM api_key WHERE user_id='$ADMINID' AND name='verif-v1' ORDER BY created_at DESC LIMIT 1;")
chk "GET  /v1/models tanpa key -> 401" 401 "$(code "$API/v1/models")"
chk "GET  /v1/models dengan API key" 200 "$(code "${K[@]}" "$API/v1/models")"
chk "/v1/models: object=list" list "$(jget "d['object']")"
chk "/v1/models: katalog tidak kosong" 1 "$(jget "1 if len(d['data'])>0 else 0")"
chk "/v1/models: format OpenAI (id+object+owned_by)" 1 \
  "$(jget "1 if all(k in d['data'][0] for k in ('id','object','created','owned_by')) else 0")"
chk "GET  /api/keys menampilkan credit_used/credit_limit" 1 \
  "$(code "${A[@]}" "$API/api/keys" >/dev/null; jget "1 if all('credit_used' in k for k in d['keys']) else 0")"

# ---- chat kompatibel OpenAI: non-stream, stream, path lama, playground
chk "POST /v1/chat/completions (non-stream)" 200 \
  "$(code -X POST "${K[@]}" -d "{\"model\":\"$MOSEL\",\"messages\":[{\"role\":\"user\",\"content\":\"halo\"}]}" "$API/v1/chat/completions")"
chk "jawaban provider diteruskan apa adanya" 1 "$(has "Halo dari mock")"
chk "POST /v1/chat/completions stream:true -> SSE" 200 \
  "$(code -N -X POST "${K[@]}" -d "{\"model\":\"$MOSEL\",\"messages\":[{\"role\":\"user\",\"content\":\"halo\"}],\"stream\":true}" "$API/v1/chat/completions")"
chk "SSE berisi chunk 'data: {'" 1 "$(has "data: {")"
chk "SSE diakhiri data: [DONE]" 1 "$(has "\[DONE\]")"
chk "SSE >= 3 chunk delta + pemakaian nyata" 1 \
  "$(python -c "n=sum(1 for l in open(r'$TMP/gw_out.json',encoding='utf-8') if l.startswith('data: {')); print(1 if n>=3 else 0)")"
chk "stream: status usage_log = OK" OK \
  "$(dc "SELECT status FROM usage_log WHERE model='$MOSEL' ORDER BY created_at DESC LIMIT 1;")"
chk "stream: settle menagih biaya nyata (>0)" 1 \
  "$(python -c "import subprocess;v=float(subprocess.check_output(['docker','compose','exec','-T','db','psql','-U','aiuser','-d','aisell','-Atc',\"SELECT cost FROM usage_log WHERE model='$MOSEL' ORDER BY created_at DESC LIMIT 1\"]).decode().strip().replace(chr(13),''));print(1 if v>0 else 0)")"
chk "stream: baris usage_log menyimpan api_key_id" 1 \
  "$(dc "SELECT CASE WHEN api_key_id='$VKID' THEN 1 ELSE 0 END FROM usage_log WHERE model='$MOSEL' ORDER BY created_at DESC LIMIT 1;")"
chk "kuota key: credit_used ikut naik saat charge" 1 \
  "$(dc "SELECT CASE WHEN credit_used > 0 THEN 1 ELSE 0 END FROM api_key WHERE id='$VKID';")"
chk "POST /api/v1/chat (path lama tetap jalan)" 200 \
  "$(code -X POST "${K[@]}" -d "{\"model\":\"$MOSEL\",\"messages\":[{\"role\":\"user\",\"content\":\"halo\"}]}" "$API/api/v1/chat")"
chk "POST /api/playground/execute (alias chat JWT)" 200 \
  "$(code -X POST "${A[@]}" -d "{\"model\":\"$MOSEL\",\"messages\":[{\"role\":\"user\",\"content\":\"halo\"}]}" "$API/api/playground/execute")"

# ---- Anthropic /v1/messages
chk "POST /v1/messages (format Anthropic)" 200 \
  "$(code -X POST "${K[@]}" -d "{\"model\":\"$MOSEL\",\"max_tokens\":64,\"system\":[{\"type\":\"text\",\"text\":\"ringkas\"}],\"messages\":[{\"role\":\"user\",\"content\":[{\"type\":\"text\",\"text\":\"halo\"}]}]}" "$API/v1/messages")"
chk "/v1/messages: type=message" message "$(jget "d['type']")"
chk "/v1/messages: content[0] blok text" text "$(jget "d['content'][0]['type']")"
chk "/v1/messages: teks provider diteruskan" 1 "$(has "Halo dari mock")"
chk "/v1/messages: usage input/output tokens" 1 \
  "$(jget "1 if d['usage']['input_tokens']>0 and d['usage']['output_tokens']>0 else 0")"
chk "/v1/messages: stream:true ditolak 400 (ponytail)" 400 \
  "$(code -X POST "${K[@]}" -d "{\"model\":\"$MOSEL\",\"stream\":true,\"messages\":[{\"role\":\"user\",\"content\":\"x\"}]}" "$API/v1/messages")"

# ---- routing model per provider (kolom provider.models)
chk "PATCH models=claude-3-5-sonnet" 200 \
  "$(code -X PATCH "${A[@]}" -d "{\"id\":\"$MOCKPROV\",\"is_active\":true,\"models\":\"claude-3-5-sonnet\"}" "$API/api/admin/providers")"
chk "model di luar CSV provider -> 502" 502 \
  "$(code -X POST "${K[@]}" -d "{\"model\":\"$MOSEL\",\"messages\":[{\"role\":\"user\",\"content\":\"x\"}]}" "$API/v1/chat/completions")"
chk "model di dalam CSV provider -> 200" 200 \
  "$(code -X POST "${K[@]}" -d '{"model":"claude-3-5-sonnet","messages":[{"role":"user","content":"x"}]}' "$API/v1/chat/completions")"
chk "PATCH models='' (kembali melayani semua)" 200 \
  "$(code -X PATCH "${A[@]}" -d "{\"id\":\"$MOCKPROV\",\"is_active\":true,\"models\":\"\"}" "$API/api/admin/providers")"
chk "semua model terlayani lagi -> 200" 200 \
  "$(code -X POST "${K[@]}" -d "{\"model\":\"$MOSEL\",\"messages\":[{\"role\":\"user\",\"content\":\"x\"}]}" "$API/v1/chat/completions")"

# ---- cek kuota via API key + whitelist model + rate limit per key
chk "GET  /v1/key (kuota via key) -> 200" 200 "$(code "${K[@]}" "$API/v1/key")"
chk "/v1/key: credit_used > 0 (terisi dari trafik)" 1 "$(jget "1 if d['credit_used']>0 else 0")"
chk "GET  /v1/key tanpa key -> 401" 401 "$(code "$API/v1/key")"

# ---- kuota per API key (credit_limit/credit_used)
dc "UPDATE api_key SET credit_used=0, credit_limit=0.000001 WHERE id='$VKID';" >/dev/null
chk "credit_limit terlalu kecil -> 402 (charge)" 402 \
  "$(code -X POST "${K[@]}" -d "{\"model\":\"$MOSEL\",\"messages\":[{\"role\":\"user\",\"content\":\"x\"}]}" "$API/v1/chat/completions")"
dc "UPDATE api_key SET credit_used=10, credit_limit=5 WHERE id='$VKID';" >/dev/null
chk "credit_used >= credit_limit -> 402 (middleware)" 402 "$(code "${K[@]}" "$API/v1/models")"
dc "UPDATE api_key SET credit_used=0, credit_limit=NULL WHERE id='$VKID';" >/dev/null
chk "kuota dilepas -> 200" 200 "$(code "${K[@]}" "$API/v1/models")"

# ---- whitelist model + rate limit per key (blok kuota di atas me-reset K)
dc "DELETE FROM api_key WHERE name='verif-v1-wl';" >/dev/null
chk "POST /api/keys (dengan whitelist model)" 200 "$(code -X POST "${A[@]}" -d '{"name":"verif-v1-wl","models":"claude-3-5-sonnet"}' "$API/api/keys")"
WKEY=$(jget "d['key']")
W=(-H "Authorization: Bearer $WKEY" -H 'Content-Type: application/json')
chk "whitelist: model di luar daftar -> 403" 403 \
  "$(code -X POST "${W[@]}" -d "{\"model\":\"$MOSEL\",\"messages\":[{\"role\":\"user\",\"content\":\"x\"}]}" "$API/v1/chat/completions")"
chk "whitelist: model di dalam daftar -> 200" 200 \
  "$(code -X POST "${W[@]}" -d '{"model":"claude-3-5-sonnet","messages":[{"role":"user","content":"x"}]}' "$API/v1/chat/completions")"
dc "DELETE FROM api_key WHERE name='verif-v1-wl';" >/dev/null

dc "DELETE FROM api_key WHERE name='verif-v1-rl';" >/dev/null
chk "POST /api/keys (rate_limit=2/menit)" 200 "$(code -X POST "${A[@]}" -d '{"name":"verif-v1-rl","rate_limit":2}' "$API/api/keys")"
RKEY=$(jget "d['key']")
R=(-H "Authorization: Bearer $RKEY")
code "${R[@]}" "$API/v1/key" >/dev/null
code "${R[@]}" "$API/v1/key" >/dev/null
chk "rate_limit per key: request ke-3 -> 429" 429 "$(code "${R[@]}" "$API/v1/key")"
chk "rate_limit per key: key lain tetap 200" 200 "$(code "${K[@]}" "$API/v1/key")"
dc "DELETE FROM api_key WHERE name='verif-v1-rl';" >/dev/null

# ---- tambah provider via UI (POST /api/admin/providers, parity new-api add channel)
dc "DELETE FROM provider WHERE name='verif-new';" >/dev/null
chk "POST /api/admin/providers (create)" 200 \
  "$(code -X POST "${A[@]}" -d '{"name":"verif-new","label":"Verif Baru","type":"openai","base_url":"http://host.docker.internal:18081/v1","api_key":"sk-upstream-verif","priority":10,"models":"gpt-4o"}' "$API/api/admin/providers")"
VNPROV=$(dc "SELECT id FROM provider WHERE name='verif-new';")
chk "create: api_key tersimpan sebagai ciphertext" "v1." "$(dc "SELECT left(api_key,3) FROM provider WHERE id='$VNPROV';")"
chk "create: masker 4 terakhir di daftar" "••••" "$(code "${A[@]}" "$API/api/admin/providers" >/dev/null; jget "[p['key_mask'][:4] for p in d['providers'] if p['name']=='verif-new'][0]")"
chk "create: base_url non-http ditolak" 400 \
  "$(code -X POST "${A[@]}" -d '{"name":"verif-bad","base_url":"ftp://x.example","api_key":"k"}' "$API/api/admin/providers")"
chk "create: api_key kosong ditolak" 400 \
  "$(code -X POST "${A[@]}" -d '{"name":"verif-bad2","base_url":"http://x.example/v1"}' "$API/api/admin/providers")"
dc "DELETE FROM provider WHERE name='verif-new' OR name='verif-bad' OR name='verif-bad2';" >/dev/null
# Registry masih memuat verif-new: PATCH apa pun memicu Reload, buang dia dari memori.
code -X PATCH "${A[@]}" -d "{\"id\":\"$MOCKPROV\",\"is_active\":true,\"models\":\"\"}" "$API/api/admin/providers" >/dev/null
chk "create: provider uji terhapus" 0 "$(dc "SELECT COUNT(*) FROM provider WHERE name LIKE 'verif-%' AND name <> 'verif-stream';")"

# ---- kode redeem: tukar kode -> saldo (atomik)
RB0=$(dc "SELECT wallet_balance FROM \"user\" WHERE id='$ADMINID';")
RT0=$(dc "SELECT COUNT(*) FROM credit_transaction WHERE user_id='$ADMINID' AND type='REDEEM';")
chk "POST /api/admin/redeem (buat kode)" 200 "$(code -X POST "${A[@]}" -d '{"amount":5000,"count":1}' "$API/api/admin/redeem")"
RCODE=$(jget "d['codes'][0]")
chk "format kode AISELL-XXXX-XXXX" 1 "$(echo "$RCODE" | grep -qE '^AISELL-[A-Z0-9]{4}-[A-Z0-9]{4}$' && echo 1 || echo 0)"
chk "POST /api/redeem (pemakaian pertama)" 200 \
  "$(code -X POST "${A[@]}" -d "{\"code\":\"$RCODE\"}" "$API/api/redeem")"
RB1=$(dc "SELECT wallet_balance FROM \"user\" WHERE id='$ADMINID';")
chk "saldo naik 5000 setelah redeem" "5000.0" \
  "$(python -c "print(round(float('$RB1')-float('$RB0'),6))")"
chk "credit_transaction type REDEEM bertambah" "$((RT0+1))" \
  "$(dc "SELECT COUNT(*) FROM credit_transaction WHERE user_id='$ADMINID' AND type='REDEEM';")"
chk "kode sekali pakai dipakai dua kali -> 400" 400 \
  "$(code -X POST "${A[@]}" -d "{\"code\":\"$RCODE\"}" "$API/api/redeem")"
chk "pesannya 'kode sudah dipakai'" 1 "$(has "kode sudah dipakai")"
chk "kode tidak dikenal -> 404" 404 \
  "$(code -X POST "${A[@]}" -d '{"code":"AISELL-XXXX-YYYY"}' "$API/api/redeem")"
chk "POST /api/redeem tanpa code -> 400" 400 "$(code -X POST "${A[@]}" -d '{}' "$API/api/redeem")"

# multi-pakai (max_uses=2) + kode kedaluwarsa
chk "POST /api/admin/redeem (max_uses=2)" 200 \
  "$(code -X POST "${A[@]}" -d '{"amount":1000,"count":1,"max_uses":2}' "$API/api/admin/redeem")"
RC2=$(jget "d['codes'][0]")
chk "multi-pakai ke-1 -> 200" 200 "$(code -X POST "${A[@]}" -d "{\"code\":\"$RC2\"}" "$API/api/redeem")"
chk "multi-pakai ke-2 -> 200" 200 "$(code -X POST "${A[@]}" -d "{\"code\":\"$RC2\"}" "$API/api/redeem")"
chk "multi-pakai ke-3 ditolak -> 400" 400 "$(code -X POST "${A[@]}" -d "{\"code\":\"$RC2\"}" "$API/api/redeem")"
chk "used_count tercatat 2" 2 "$(dc "SELECT used_count FROM redeem_code WHERE code='$RC2';")"
dc "INSERT INTO redeem_code (code, amount, max_uses, expires_at) VALUES ('AISELL-EXPI-RED1',1000,NULL,now() - interval '1 day');" >/dev/null
chk "kode kedaluwarsa -> 400" 400 "$(code -X POST "${A[@]}" -d '{"code":"AISELL-EXPI-RED1"}' "$API/api/redeem")"
chk "GET  /api/admin/redeem" 200 "$(code "${A[@]}" "$API/api/admin/redeem")"
chk "daftar redeem memuat kode yang dibuat" 1 \
  "$(jget "1 if any(x['code']=='$RCODE' for x in d['redeems']) else 0")"
RID=$(dc "SELECT id FROM redeem_code WHERE code='$RC2';")
chk "DELETE /api/admin/redeem/{id}" 200 "$(code -X DELETE "${A[@]}" "$API/api/admin/redeem/$RID")"
chk "kode terhapus tidak bisa dipakai" 404 \
  "$(code -X POST "${A[@]}" -d "{\"code\":\"$RC2\"}" "$API/api/redeem")"

# ---- admin CRUD model (+ token_multiplier: harga tidak lagi per 1k)
chk "POST /api/admin/models" 200 \
  "$(code -X POST "${A[@]}" -d '{"name":"model-verif","description":"uji","token_multiplier":1.5,"context_length":8000,"max_output_tokens":2048}' "$API/api/admin/models")"
VMID=$(jget "d['id']")
chk "token_multiplier tersimpan di ai_model" "1.50" \
  "$(dc "SELECT token_multiplier::text FROM ai_model WHERE id='$VMID';")"
chk "POST /api/admin/models (type tidak dikenal) -> 400" 400 \
  "$(code -X POST "${A[@]}" -d '{"name":"model-dewa","type":"DEWA"}' "$API/api/admin/models")"
chk "POST /api/admin/models (nama duplikat) -> 409" 409 \
  "$(code -X POST "${A[@]}" -d '{"name":"model-verif"}' "$API/api/admin/models")"
chk "PATCH /api/admin/models/{id}" 200 \
  "$(code -X PATCH "${A[@]}" -d '{"sort_weight":9,"token_multiplier":2,"max_output_tokens":4096}' "$API/api/admin/models/$VMID")"
chk "perubahan model + multiplier tersimpan" "9/2.00/4096" \
  "$(dc "SELECT m.sort_weight::text||'/'||m.token_multiplier::text||'/'||m.max_output_tokens::text FROM ai_model m WHERE m.id='$VMID';")"
chk "GET  /api/admin/models" 200 "$(code "${A[@]}" "$API/api/admin/models")"
chk "katalog admin memuat model baru" 1 \
  "$(jget "1 if any(m['name']=='model-verif' and float(m['token_multiplier'])==2 for m in d['models']) else 0")"
chk "token_multiplier negatif ditolak -> 400" 400 \
  "$(code -X PATCH "${A[@]}" -d '{"token_multiplier":-1}' "$API/api/admin/models/$VMID")"
chk "DELETE /api/admin/models/{id}" 200 "$(code -X DELETE "${A[@]}" "$API/api/admin/models/$VMID")"
chk "model + harganya ikut terhapus" "0/0" \
  "$(dc "SELECT (SELECT COUNT(*) FROM ai_model WHERE id='$VMID')::text||'/'||(SELECT COUNT(*) FROM model_pricing WHERE model_id='$VMID')::text;")"

# ---- cek koneksi provider
HL0=$(dc "SELECT COUNT(*) FROM model_health_log;")
chk "POST /api/admin/provider/{id}/check" 200 "$(code -X POST "${A[@]}" "$API/api/admin/provider/$MOCKPROV/check")"
chk "check: ok=true + latency_ms" 1 "$(jget "1 if d['ok'] and d['latency_ms']>=0 else 0")"
chk "provider.is_healthy di-set true" t "$(dc "SELECT is_healthy FROM provider WHERE id='$MOCKPROV';")"
chk "provider.last_checked terisi" 1 \
  "$(dc "SELECT CASE WHEN last_checked IS NOT NULL THEN 1 ELSE 0 END FROM provider WHERE id='$MOCKPROV';")"
chk "baris model_health_log bertambah" "$((HL0+1))" "$(dc "SELECT COUNT(*) FROM model_health_log;")"
chk "health_log status=ok" ok "$(dc "SELECT status FROM model_health_log ORDER BY checked_at DESC LIMIT 1;")"

# ---- bersihkan: provider mock, key, kode, baris usage uji
dc "DELETE FROM provider WHERE name='verif-stream';" >/dev/null
dc "DELETE FROM redeem_code WHERE created_by='$ADMINID' AND created_at > now() - interval '10 minutes';" >/dev/null
dc "DELETE FROM redeem_code WHERE code='AISELL-EXPI-RED1';" >/dev/null
dc "UPDATE api_key SET is_active=false, revoked_at=now(), credit_limit=NULL, credit_used=0 WHERE id='$VKID';" >/dev/null
kill "$MOCKPID" 2>/dev/null
# Registry memori masih memuat provider mock yang barisnya baru dihapus. Reload
# lewat jalur admin yang sama: baris dummy non-aktif, PATCH tanpa transisi status
# (kalau baris aktif terakhir dinonaktifkan, guard menolaknya).
dc "INSERT INTO provider (name,label,type,base_url,api_key,is_active,priority) VALUES ('verif-reload','Reload','openai','http://127.0.0.1:1/v1','x',false,0);" >/dev/null
RELOADID=$(dc "SELECT id FROM provider WHERE name='verif-reload';")
code -X PATCH "${A[@]}" -d "{\"id\":\"$RELOADID\",\"is_active\":false}" "$API/api/admin/providers" >/dev/null
dc "DELETE FROM provider WHERE name='verif-reload';" >/dev/null
chk "registry bersih lagi: /api/chat tanpa provider -> 502" 502 \
  "$(code -X POST "${A[@]}" -d "{\"model\":\"$MOSEL\",\"messages\":[{\"role\":\"user\",\"content\":\"x\"}]}" "$API/api/chat")"

# ---------------------------------------------------------------- pulihkan data dev
dc "UPDATE \"user\" SET wallet_balance=50000, role='ADMIN' WHERE id='$ADMINID';" >/dev/null
dc "UPDATE \"user\" SET wallet_balance=10000, role='SELLER' WHERE id='$SELLERID';" >/dev/null
dc "UPDATE user_auth_method SET confirmed=false WHERE method='totp';" >/dev/null

echo
echo "== $pass PASS / $fail FAIL =="
exit $([ "$fail" -eq 0 ] && echo 0 || echo 1)
