#!/usr/bin/env bash
set -euo pipefail

APP=/root/node2mint/deploy
LOG=/home/server/node2mint-autostart-wsl.log

mkdir -p "$(dirname "$LOG")"
exec >>"$LOG" 2>&1
printf '\n[%s] Node2Mint autostart begin\n' "$(date -Is)"
export PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

for _ in $(seq 1 60); do
  if docker info >/dev/null 2>&1; then
    break
  fi
  sleep 2
done
docker info >/dev/null 2>&1

cd "$APP"
docker compose --env-file site-allowlist.env -f docker-compose.prod.yml up -d

for _ in $(seq 1 60); do
  if curl -fsS --max-time 3 http://127.0.0.1/ >/dev/null 2>&1; then
    break
  fi
  sleep 2
done
curl -fsS --max-time 3 http://127.0.0.1/ >/dev/null

if pgrep -u server -f '[c]loudflared tunnel run' >/dev/null 2>&1; then
  printf '[%s] cloudflared already running\n' "$(date -Is)"
  exit 0
fi

runuser -u server -- env HOME=/home/server /bin/bash -lc '
  token=$(tr -d "\r\n" < "$HOME/.cf-token")
  test -n "$token"
  nohup cloudflared tunnel run --protocol http2 --token "$token" >> "$HOME/cloudflared.log" 2>&1 </dev/null &
  disown
'
printf '[%s] Node2Mint autostart requested cloudflared\n' "$(date -Is)"
