#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

read -rp 'Telegram Bot Username (tanpa @): ' bot_username
read -rsp 'Telegram Bot Token: ' bot_token
echo

bot_username=${bot_username#@}
[ -n "$bot_username" ] && [ -n "$bot_token" ] || {
  echo 'Bot username dan bot token wajib diisi.' >&2
  exit 1
}

env_file=deploy/gateway.env
tmp=$(mktemp "deploy/.gateway.env.XXXXXX")
trap 'rm -f "$tmp"' EXIT
chmod 600 "$tmp"

{ grep -vE '^(TELEGRAM_BOT_USERNAME|TELEGRAM_BOT_TOKEN)=' "$env_file" || true; } > "$tmp"
printf 'TELEGRAM_BOT_USERNAME=%s\nTELEGRAM_BOT_TOKEN=%s\n' "$bot_username" "$bot_token" >> "$tmp"
mv "$tmp" "$env_file"
chmod 600 "$env_file"
printf '%s\n' 'Telegram credential tersimpan. Nilainya tidak ditampilkan.'
