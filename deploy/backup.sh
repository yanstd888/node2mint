#!/usr/bin/env sh
set -eu

# Run from deploy/: BACKUP_DIR=/srv/node2mint/backups ./backup.sh
backup_dir=${BACKUP_DIR:-./backups}
stamp=$(date -u +%Y%m%dT%H%M%SZ)
umask 077
mkdir -p "$backup_dir"
docker compose -f docker-compose.prod.yml exec -T db \
  pg_dump -U aiuser -d aisell -Fc > "$backup_dir/aisell-$stamp.dump"
find "$backup_dir" -type f -name 'aisell-*.dump' -mtime +14 -delete
