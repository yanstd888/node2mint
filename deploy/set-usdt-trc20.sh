#!/usr/bin/env bash
set -euo pipefail

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$root/deploy"
[ -f gateway.env ] || { echo 'gateway.env tidak ditemukan'; exit 1; }

read -r -p 'Alamat penerima USDT TRC-20: ' address
read -r -p 'Kontrak USDT TRC-20 [TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t]: ' contract
contract=${contract:-TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t}
read -r -p 'Kurs Rupiah per 1 USDT (mis. 16000): ' rate
read -r -s -p 'TronGrid API key: ' key; printf '\n'

[[ $address =~ ^T[1-9A-HJ-NP-Za-km-z]{33}$ ]] || { echo 'Alamat TRON tidak valid'; exit 1; }
[[ $contract =~ ^T[1-9A-HJ-NP-Za-km-z]{33}$ ]] || { echo 'Kontrak TRC-20 tidak valid'; exit 1; }
[[ $rate =~ ^[0-9]+([.][0-9]+)?$ ]] && awk "BEGIN { exit !($rate > 0) }" || { echo 'Kurs harus angka positif'; exit 1; }
[ -n "$key" ] || { echo 'TronGrid API key wajib diisi'; exit 1; }

# ponytail: replace only these four keys; all other deployment secrets remain untouched.
tmp=$(mktemp gateway.env.XXXXXX)
chmod 600 "$tmp"
grep -vE '^(CRYPTO_TRON_ADDRESS|CRYPTO_USDT_CONTRACT|CRYPTO_IDR_PER_USDT|CRYPTO_TRONGRID_API_KEY)=' gateway.env > "$tmp" || true
{
  printf 'CRYPTO_TRON_ADDRESS=%s\n' "$address"
  printf 'CRYPTO_USDT_CONTRACT=%s\n' "$contract"
  printf 'CRYPTO_IDR_PER_USDT=%s\n' "$rate"
  printf 'CRYPTO_TRONGRID_API_KEY=%s\n' "$key"
} >> "$tmp"
mv "$tmp" gateway.env
chmod 600 gateway.env
docker compose -f docker-compose.prod.yml up -d --force-recreate --no-deps gateway >/dev/null
printf 'USDT TRC-20 environment saved; gateway restarted.\n'
