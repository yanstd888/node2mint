#!/usr/bin/env bash
# Regression check tagihan TOKEN: potongan = (input + cache-sudah-termasuk + output) x token_multiplier,
# tier FREE ambil kuota gratis harian dulu, habis -> saldo.
#
# Pakai upstream mock lokal (usage tetap 1000+500) jadi tidak butuh provider 9router asli.
# Re-runnable: mengembalikan tier/multiplier model + saldo user ke nilai semula.
#
# Jalankan dari root proyek: bash gateway/smoke-token-billing.sh
set -u
export PATH="$HOME/AppData/Local/Programs/DockerDesktop/resources/bin:$PATH"
export PATH="/c/Users/YM/go/bin:$PATH"

API=${API:-http://localhost:8080}
MOCK_PORT=${MOCK_PORT:-18099}
MODEL=${MODEL:-gpt-4o-mini}
SELLER=${SELLER:-seller@test.local}
PASS=${PASS:-Test1234!}
MOCK_LOG="$LOCALAPPDATA/Temp/mock-usage.log"

dc() { docker compose exec -T db psql -U aiuser -d aisell -tAc "$1"; }
login() {
  curl -s -X POST "$API/api/auth/login" -H 'Content-Type: application/json' \
    -d "{\"email\":\"$1\",\"password\":\"$PASS\"}" |
    python -c "import json,sys;print(json.load(sys.stdin)['token'])" | tr -d '\r'
}
chat() { # token -> http_code
  curl -s -X POST "$API/api/chat" -H "Authorization: Bearer $1" -H 'Content-Type: application/json' \
    -d "{\"model\":\"$MODEL\",\"messages\":[{\"role\":\"user\",\"content\":\"halo\"}],\"max_tokens\":100}" -o /dev/null -w '%{http_code}'
}
saldo() { dc "SELECT wallet_balance FROM \"user\" WHERE id='$SELLERID';" | tr -d '\r'; }

FAIL=0
cek() { # label nilai harapan
  if [ "$2" = "$3" ]; then echo "PASS  $1 = $2"; else echo "FAIL  $1 = $2 (mau $3)"; FAIL=$((FAIL+1)); fi
}

TOK=$(login "$SELLER")
ADM=$(login admin@test.local)
SELLERID=$(dc "SELECT id FROM \"user\" WHERE email='$SELLER';" | tr -d '\r')
MID=$(dc "SELECT id FROM ai_model WHERE name='$MODEL';" | tr -d '\r')
ORIG_TIER=$(dc "SELECT tier FROM ai_model WHERE id='$MID';" | tr -d '\r')
ORIG_MULT=$(dc "SELECT token_multiplier FROM ai_model WHERE id='$MID';" | tr -d '\r')
ORIG_BAL=$(saldo)

# Upstream mock: usage tetap prompt=1000 completion=500 cached=200 -> tagihan 1500 x multiplier.
python "$(dirname "$0")/testdata/mock-usage.py" "$MOCK_PORT" "$MOCK_LOG" >/dev/null 2>&1 &
MOCK_PID=$!

# Bersih-bersih: gateway menyimpan registry provider DI MEMORI (load saat start),
# jadi hapus baris DB saja tidak cukup — gateway tetap melayani chat 200 dan
# verify-endpoints.sh (yang mengharap 502 tanpa provider) jadi FAIL. Karena itu
# restart gateway + matikan mock + pulihkan saldo/model/kuota.
bersih() {
  kill "$MOCK_PID" 2>/dev/null
  MOCKWIN=$(netstat -ano | awk '/LISTENING/ && $2 ~ ":'"$MOCK_PORT"'$" {print $5}' | head -1)
  [ -n "$MOCKWIN" ] && taskkill /PID "$MOCKWIN" /F >/dev/null 2>&1
  dc "UPDATE ai_model SET tier='$ORIG_TIER', token_multiplier=$ORIG_MULT WHERE id='$MID';" >/dev/null
  dc "UPDATE \"user\" SET wallet_balance=$ORIG_BAL WHERE id='$SELLERID';" >/dev/null
  dc "DELETE FROM free_quota WHERE user_id='$SELLERID';" >/dev/null
  dc "DELETE FROM provider WHERE name='mock-usage';" >/dev/null
  docker compose restart gateway >/dev/null 2>&1
}
trap bersih EXIT
sleep 1

# Provider mock hanya didaftarkan kalau belum ada.
if [ "$(dc "SELECT count(*) FROM provider WHERE name='mock-usage';" | tr -d '\r')" = "0" ]; then
  curl -s -X POST "$API/api/admin/providers" -H "Authorization: Bearer $ADM" -H 'Content-Type: application/json' \
    -d "{\"name\":\"mock-usage\",\"label\":\"Mock Usage\",\"base_url\":\"http://host.docker.internal:$MOCK_PORT/v1\",\"api_key\":\"sk-mock-usage\",\"model\":\"$MODEL\"}" >/dev/null
fi

echo "== model=$MODEL seller=$SELLER (usage mock: 1000 input + 500 output) =="

dc "UPDATE ai_model SET tier='PAID', token_multiplier=1 WHERE id='$MID';" >/dev/null
B=$(saldo); C=$(chat "$TOK")
cek "PAID x1 http" "$C" "200"
cek "PAID x1 potong" "$(python -c "print(int(float('$B')-float('$(saldo)')))")" "1500"

dc "UPDATE ai_model SET token_multiplier=3 WHERE id='$MID';" >/dev/null
B=$(saldo); chat "$TOK" >/dev/null
cek "PAID x3 potong" "$(python -c "print(int(float('$B')-float('$(saldo)')))")" "4500"

dc "UPDATE ai_model SET tier='FREE', token_multiplier=1 WHERE id='$MID'; DELETE FROM free_quota WHERE user_id='$SELLERID';" >/dev/null
B=$(saldo); chat "$TOK" >/dev/null
cek "FREE saldo tetap" "$(python -c "print(int(float('$B')-float('$(saldo)')))")" "0"
cek "FREE kuota terpakai" "$(dc "SELECT used FROM free_quota WHERE user_id='$SELLERID';" | tr -d '\r')" "1500"

dc "UPDATE free_quota SET used=19500 WHERE user_id='$SELLERID';" >/dev/null
B=$(saldo); chat "$TOK" >/dev/null
# Kuota sisa 500 menutup SEBAGIAN tagihan 1500; sisanya 1000 dari saldo.
cek "FREE kuota parsial: sisa terpakai" "$(dc "SELECT used FROM free_quota WHERE user_id='$SELLERID';" | tr -d '\r')" "20000"
cek "FREE kuota parsial: sisanya dari saldo" "$(python -c "print(int(float('$B')-float('$(saldo)')))")" "1000"

# --- paket token: dipakai SEBELUM saldo; refund kembali ke paket.
dc "UPDATE ai_model SET tier='PAID', token_multiplier=1 WHERE id='$MID';
    DELETE FROM free_quota WHERE user_id='$SELLERID';
    INSERT INTO token_package (name, token_count, price, is_active) VALUES ('PkgUji',100000,10000,true)
    ON CONFLICT DO NOTHING;" >/dev/null
PKGID=$(dc "SELECT id FROM token_package WHERE name='PkgUji' LIMIT 1;")
dc "INSERT INTO user_token_package (user_id, token_package_id, expires_at, remaining)
    VALUES ('$SELLERID','$PKGID', now()+interval '7 days', 2000);" >/dev/null
UPKID=$(dc "SELECT id FROM user_token_package WHERE user_id='$SELLERID' ORDER BY purchased_at DESC LIMIT 1;")
B=$(saldo); chat "$TOK" >/dev/null
cek "paket 2000 menutup tagihan 1500 (saldo tetap)" "$(python -c "print(int(float('$B')-float('$(saldo)')))")" "0"
cek "paket remaining 2000->500" "$(dc "SELECT remaining FROM user_token_package WHERE id='$UPKID';" | tr -d '\r')" "500"
# Tagihan > sisa paket: 500 dari paket + 1000 dari saldo.
B=$(saldo); chat "$TOK" >/dev/null
cek "sisa paket habis dulu, saldo menutup sisanya" "$(python -c "print(int(float('$B')-float('$(saldo)')))")" "1000"
cek "paket remaining 0" "$(dc "SELECT remaining FROM user_token_package WHERE id='$UPKID';" | tr -d '\r')" "0"
dc "DELETE FROM user_token_package WHERE id='$UPKID'; DELETE FROM token_package WHERE name='PkgUji';" >/dev/null
B=$(saldo); chat "$TOK" >/dev/null
cek "tanpa paket: tagihan penuh ke saldo" "$(python -c "print(int(float('$B')-float('$(saldo)')))")" "1500"

echo "== $( [ $FAIL = 0 ] && echo 'SEMUA PASS' || echo "$FAIL FAIL" ) =="
exit $FAIL
