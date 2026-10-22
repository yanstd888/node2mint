#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

read -rp 'Turnstile Site Key: ' site_key
read -rsp 'Turnstile Secret Key: ' secret_key
echo

[ -n "$site_key" ] && [ -n "$secret_key" ] || {
  echo 'Site Key dan Secret Key wajib diisi.' >&2
  exit 1
}

env_file=deploy/gateway.env
tmp=$(mktemp "deploy/.gateway.env.XXXXXX")
trap 'rm -f "$tmp"' EXIT
chmod 600 "$tmp"

{ grep -vE '^(TURNSTILE_SITE_KEY|TURNSTILE_SECRET_KEY|TURNSTILE_EXPECTED_HOSTNAME)=' "$env_file" || true; } > "$tmp"
printf 'TURNSTILE_SITE_KEY=%s\nTURNSTILE_SECRET_KEY=%s\nTURNSTILE_EXPECTED_HOSTNAME=www.node2mint.com\n' "$site_key" "$secret_key" >> "$tmp"
mv "$tmp" "$env_file"
chmod 600 "$env_file"
printf '%s\n' 'Turnstile tersimpan untuk www.node2mint.com. Nilainya tidak ditampilkan.'
