#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
env_file=deploy/gateway.env
allowlist_file=deploy/site-allowlist.env
[ -f "$env_file" ] || { echo "gateway env tidak ditemukan: $env_file" >&2; exit 1; }
[ -f "$allowlist_file" ] || { echo "site allowlist tidak ditemukan: $allowlist_file" >&2; exit 1; }

read -rp 'DOKU environment [sandbox/production] (sandbox): ' doku_env
doku_env=${doku_env:-sandbox}
case "$doku_env" in sandbox|production) ;; *) echo 'Environment harus sandbox atau production.' >&2; exit 1;; esac
read -rp 'DOKU Client ID: ' client_id
read -rsp 'DOKU Secret Key: ' secret_key; echo
[ -n "$client_id" ] && [ -n "$secret_key" ] || { echo 'Client ID dan Secret Key DOKU wajib diisi.' >&2; exit 1; }

tmp=$(mktemp "deploy/.gateway.env.XXXXXX")
trap 'rm -f "$tmp"' EXIT
chmod 600 "$tmp"
{ grep -vE '^(DOKU_ENV|DOKU_CLIENT_ID|DOKU_SECRET_KEY)=' "$env_file" || true; } > "$tmp"
printf 'DOKU_ENV=%s\nDOKU_CLIENT_ID=%s\nDOKU_SECRET_KEY=%s\n' "$doku_env" "$client_id" "$secret_key" >> "$tmp"
mv "$tmp" "$env_file"
chmod 600 "$env_file"
docker compose --env-file "$allowlist_file" -f deploy/docker-compose.prod.yml config --quiet
docker compose --env-file "$allowlist_file" -f deploy/docker-compose.prod.yml up -d --force-recreate --no-deps gateway
docker compose --env-file "$allowlist_file" -f deploy/docker-compose.prod.yml exec -T gateway sh -lc '[ -n "${DOKU_CLIENT_ID:-}" ] && [ -n "${DOKU_SECRET_KEY:-}" ]'
printf 'Credential DOKU tersimpan, terdeteksi di gateway, dan gateway direstart. Nilai tidak ditampilkan.\n'