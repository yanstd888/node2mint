#!/usr/bin/env sh
set -eu

# Non-destructive: validates that pg_restore can read a backup archive.
test $# -eq 1 || { echo "usage: $0 backups/aisell-YYYYMMDDTHHMMSSZ.dump" >&2; exit 64; }
docker compose -f docker-compose.prod.yml exec -T db \
  pg_restore --list < "$1" >/dev/null
echo "backup archive is readable"
