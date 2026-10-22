#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."
: "${HOME:?HOME is required}"

read -rp 'Google Client ID: ' client_id
read -rsp 'Google Client Secret: ' client_secret
echo
[ -n "$client_id" ] && [ -n "$client_secret" ] || { echo 'Client ID dan Client Secret wajib diisi.' >&2; exit 1; }

env_file=deploy/gateway.env
tmp=$(mktemp "deploy/.gateway.env.XXXXXX")
trap 'rm -f "$tmp"' EXIT
chmod 600 "$tmp"
{ grep -vE '^(GOOGLE_CLIENT_ID|GOOGLE_CLIENT_SECRET)=' "$env_file" || true; } > "$tmp"
printf 'GOOGLE_CLIENT_ID=%s\nGOOGLE_CLIENT_SECRET=%s\n' "$client_id" "$client_secret" >> "$tmp"
mv "$tmp" "$env_file"
chmod 600 "$env_file"
printf 'Google OAuth credential tersimpan. Nilainya tidak ditampilkan.\n'
