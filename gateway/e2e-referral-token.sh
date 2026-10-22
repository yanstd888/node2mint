#!/usr/bin/env bash
# E2E referral: reward = PAKET TOKEN (bukan wallet). Token admin dipakai untuk approve topup.
set -u
API=http://localhost:8080
export PATH="$HOME/AppData/Local/Programs/DockerDesktop/resources/bin:$PATH"
cd "$(dirname "$0")"
dc() { docker compose exec -T db psql -U aiuser -d aisell -Atc "$1" | tr -d '\r'; }

TOK=$(curl -s -X POST "$API/api/auth/login" -H 'Content-Type: application/json' \
  -d '{"email":"admin@test.local","password":"Test1234!"}' | python -c "import sys,json;print(json.load(sys.stdin)['token'])")
AUTH="Authorization: Bearer $TOK"

REF=$(curl -s -H "$AUTH" "$API/api/referral")
CODE=$(printf '%s' "$REF" | python -c "import sys,json;d=json.load(sys.stdin);print(d.get('code') or d.get('myCode') or d.get('referral_code') or '')")
echo "kode referral admin: $CODE"

EM="refe2e-$$@test.local"
curl -s -X POST "$API/api/auth/register" -H 'Content-Type: application/json' \
  -d "{\"first_name\":\"Referral\",\"last_name\":\"E2E\",\"email\":\"$EM\",\"password\":\"password123\",\"password_confirm\":\"password123\",\"aff_code\":\"$CODE\"}" >/dev/null
INV_ID=$(dc "SELECT id FROM \"user\" WHERE email='$EM';")
BAL0=$(dc "SELECT wallet_balance FROM \"user\" WHERE email='admin@test.local';")
ITOK=$(curl -s -X POST "$API/api/auth/login" -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EM\",\"password\":\"password123\"}" | python -c "import sys,json;print(json.load(sys.stdin)['token'])")
curl -s -X POST -H "Authorization: Bearer $ITOK" -H 'Content-Type: application/json' \
  -d '{"amount":10000}' "$API/api/wallet/topup" >/dev/null
TTX=$(dc "SELECT id FROM credit_transaction WHERE user_id='$INV_ID' AND type='TOPUP' AND status='PENDING' LIMIT 1;")
curl -s -X POST -H "$AUTH" "$API/api/admin/topup/$TTX/complete" >/dev/null

QS_INVITEE="SELECT COUNT(*)||' baris / '||COALESCE(SUM(remaining),0)||' token' FROM user_token_package utp JOIN token_package tp ON tp.id=utp.token_package_id WHERE tp.name='Reward Referral' AND utp.user_id='$INV_ID';"
QS_INVITER="SELECT COUNT(*)||' baris / '||COALESCE(SUM(remaining),0)||' token' FROM user_token_package utp JOIN token_package tp ON tp.id=utp.token_package_id WHERE tp.name='Reward Referral' AND utp.user_id=(SELECT created_by FROM referral_code WHERE code='$CODE');"
BAL1=$(dc "SELECT wallet_balance FROM \"user\" WHERE email='admin@test.local';")
WBAL_INVITEE=$(dc "SELECT wallet_balance::int FROM \"user\" WHERE id='$INV_ID';")

echo "invitee paket reward : $(dc "$QS_INVITEE")"
echo "inviter paket reward : $(dc "$QS_INVITER")"
echo "wallet admin         : $BAL0 -> $BAL1 (komisi topup 30% = 3000 diharapkan)"
echo "wallet invitee       : $WBAL_INVITEE (= nominal topupnya sendiri; bonus TIDAK menambah wallet)"

# Bersihkan artefak uji (paket reward invitee ikut terhapus ON DELETE CASCADE).
dc "DELETE FROM \"user\" WHERE email LIKE 'refe2e-%@test.local';" >/dev/null
echo "cleanup: user uji dihapus"
