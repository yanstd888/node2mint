#!/usr/bin/env sh
set -eu

# Run from deploy/: stream host schema because sync can replace its inode.
cat ../gateway/sql/schema.sql | docker compose --env-file site-allowlist.env -f docker-compose.prod.yml exec -T db psql -v ON_ERROR_STOP=1 -U aiuser -d aisell
