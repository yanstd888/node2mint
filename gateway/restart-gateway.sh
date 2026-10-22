#!/usr/bin/env bash
# Jalankan gateway lokal tanpa Docker (untuk verifikasi cepat) atau via Docker.
#   bash gateway/restart-gateway.sh          # Docker (default)
#   bash gateway/restart-gateway.sh local    # proses Go lokal + rebuild
set -uo pipefail
cd "$(dirname "$0")"
MODE="${1:-docker}"
if [ "$MODE" = "local" ]; then
  export PATH="/c/Program Files/Go/bin:$PATH"
  export $(grep -v '^#' .env | xargs -d '\n') 2>/dev/null || true
  go build -o "$LOCALAPPDATA/Temp/gateway.exe" ./cmd/gateway && exec "$LOCALAPPDATA/Temp/gateway.exe"
fi
cd ..
export PATH="$HOME/AppData/Local/Programs/DockerDesktop/resources/bin:$PATH"
docker compose build gateway >/dev/null 2>&1 || { echo "build gagal"; exit 1; }
docker rm -f ai-sell-modern-gateway-1 >/dev/null 2>&1
docker compose up -d --no-deps gateway >/dev/null 2>&1
for _ in $(seq 1 30); do
  [ "$(curl -s -o /dev/null -w '%{http_code}' http://localhost:8080/healthz 2>/dev/null)" = "200" ] && {
    echo "gateway siap (healthz 200)"; docker ps --filter name=gateway --format '{{.Names}} {{.Status}}'; exit 0; }
  sleep 1
done
echo "gateway TIDAK siap"; docker logs --tail 15 ai-sell-modern-gateway-1 2>&1; exit 1
