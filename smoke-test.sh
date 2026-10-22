#!/usr/bin/env bash
# Smoke test jalur uang: login -> beli -> saldo & stok turun -> beli berlebih ditolak.
# Pakai: bash smoke-test.sh   (stack docker compose harus jalan)
set -euo pipefail
cd "$(dirname "$0")"
API=${API:-http://localhost:8080}
# python di Windows menulis \r\n; buang CR agar nilai shell bersih.
j() { python -c "import sys,json;d=json.load(sys.stdin);print($1)" | tr -d '\r'; }
py() { python -c "$1" | tr -d '\r'; }
# ponytail: curl MSYS gagal menulis ke /dev/null (exit 23), pakai file temp.
TMPF=$(mktemp)
# Nilai uji harus mulai dari titik yang sama tiap kali.
fresh() {
  command -v docker >/dev/null 2>&1 || { echo "(lewati reset: docker tidak ada)"; return 0; }
  docker compose exec -T db psql -U aiuser -d aisell -q -c \
    "UPDATE \"user\" SET wallet_balance=50000 WHERE email='admin@test.local';
     UPDATE market_product SET stock=99;" >/dev/null
  # ponytail: FLUSHDB di dev saja — membuang counter rate-limit login supaya
  # smoke test bisa jalan berulang. Di server, hapus baris ini.
  docker compose exec -T redis sh -c 'redis-cli -a "$(cat /run/secrets/redis_pass)" FLUSHDB' >/dev/null 2>&1
}

fresh
TOKEN=$(curl -sf -X POST "$API/api/auth/login" -H 'Content-Type: application/json' \
  -d '{"email":"admin@test.local","password":"Test1234!"}' | j "d['token']")

# Pilih produk yang benar-benar dapat dibeli oleh saldo fixture. Urutan listing
# tidak stabil (produk manual yang mahal bisa muncul paling atas).
read -r PID TITLE PRICE STOCK < <(curl -sf "$API/api/market/products" | py "
import sys,json
products=json.load(sys.stdin)['products']
p=next((x for x in products if 0 < float(x['price']) <= 50000 and int(x['stock']) > 0), None)
assert p, 'tidak ada produk aktif dengan harga <= saldo fixture Rp50.000'
print(p['id'],p['title'].replace(' ','_'),p['price'],p['stock'])")
BEFORE=$(curl -sf -H "Authorization: Bearer $TOKEN" "$API/api/dashboard" | j "d['balance']")
echo "produk: $TITLE (harga $PRICE, stok $STOCK) | saldo awal $BEFORE"

stock_of() { curl -sf "$API/api/market/products" | py "
import sys,json
print([p['stock'] for p in json.load(sys.stdin)['products'] if p['id']=='$PID'][0])"; }

curl -sf -X POST -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d "{\"product_id\":\"$PID\",\"qty\":1}" "$API/api/market/orders" >/dev/null

AFTER=$(curl -sf -H "Authorization: Bearer $TOKEN" "$API/api/dashboard" | j "d['balance']")
STOCK_AFTER=$(stock_of)

python - "$BEFORE" "$AFTER" "$PRICE" "$STOCK" "$STOCK_AFTER" <<'PY'
import sys
before, after, price, s0, s1 = map(float, sys.argv[1:6])
assert after == before - price, f"saldo: {before} -> {after}, harus turun {price}"
assert s1 == s0 - 1, f"stok: {s0} -> {s1}, harus turun 1"
print(f"OK saldo {before} -> {after}, stok {s0:.0f} -> {s1:.0f}")
PY

# Beli berlebih harus ditolak (402) dan TIDAK mengubah stok.
QTY=$(py "print(int($AFTER // $PRICE) + 2)")
# ponytail: tulis ke file, bukan /dev/null — curl MSYS gagal write ke /dev/null (exit 23).
CODE=$(curl -s -o "$TMPF" -w '%{http_code}' -X POST -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d "{\"product_id\":\"$PID\",\"qty\":$QTY}" "$API/api/market/orders")
STOCK_LAST=$(stock_of)

python - "$CODE" "$STOCK_AFTER" "$STOCK_LAST" <<'PY'
import sys
code, s1, s2 = sys.argv[1], float(sys.argv[2]), float(sys.argv[3])
assert code == '402', f"beli berlebih harus 402, dapat {code}"
assert s2 == s1, f"stok berubah setelah order gagal: {s1} -> {s2}"
print(f"OK order gagal ({code}) tidak mengubah stok")
PY
