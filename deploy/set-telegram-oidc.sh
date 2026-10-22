#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

read -rp 'Telegram OIDC Client ID: ' client_id
read -rsp 'Telegram OIDC Client Secret: ' client_secret
echo

[ -n "$client_id" ] && [ -n "$client_secret" ] || {
  echo 'Client ID dan Client Secret wajib diisi.' >&2
  exit 1
}

env_file=deploy/gateway.env
tmp=$(mktemp "deploy/.gateway.env.XXXXXX")
trap 'rm -f "$tmp"' EXIT
chmod 600 "$tmp"

{ grep -vE '^(TELEGRAM_CLIENT_ID|TELEGRAM_CLIENT_SECRET)=' "$env_file" || true; } > "$tmp"
printf 'TELEGRAM_CLIENT_ID=%s\nTELEGRAM_CLIENT_SECRET=%s\n' "$client_id" "$client_secret" >> "$tmp"
mv "$tmp" "$env_file"
chmod 600 "$env_file"
printf '%s\n' 'Telegram OIDC credential tersimpan. Nilainya tidak ditampilkan.'
