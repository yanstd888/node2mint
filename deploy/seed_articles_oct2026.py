#!/usr/bin/env python3
"""Publish the reviewed October 2026 editorial seed through production Postgres."""
from __future__ import annotations

import json
import re
import subprocess
import sys
import unicodedata
from pathlib import Path

HERE = Path(__file__).resolve().parent
DATA = HERE / "seed_articles_oct2026.json"
COMPOSE = ["docker", "compose", "-f", "docker-compose.prod.yml"]


def sql(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def slugify(value: str) -> str:
    value = unicodedata.normalize("NFKD", value).encode("ascii", "ignore").decode().lower()
    return re.sub(r"(^-|-$)", "", re.sub(r"[^a-z0-9]+", "-", value))


def main() -> int:
    articles = json.loads(DATA.read_text(encoding="utf-8"))
    values = []
    for article in articles:
        values.append("(" + ",".join([
            sql(article["title"]), sql(slugify(article["title"])), sql(article["content"]),
            sql(article["excerpt"]), sql(article["image"]),
        ]) + ")")
    query = """
WITH admin_author AS (
  SELECT id FROM \"user\" WHERE role IN ('ADMIN','SUPER_ADMIN') AND NOT is_banned ORDER BY created_at LIMIT 1
), seeded(title,slug,content,excerpt,image) AS (VALUES %s)
INSERT INTO article (author_id, author_role, title, slug, content, excerpt, image, status, is_admin_post, published_at)
SELECT admin_author.id, 'ADMIN', seeded.title, seeded.slug, seeded.content, seeded.excerpt, seeded.image, 'PUBLISHED', true, now()
FROM seeded CROSS JOIN admin_author
ON CONFLICT (slug) DO UPDATE SET
  title=EXCLUDED.title, content=EXCLUDED.content, excerpt=EXCLUDED.excerpt, image=EXCLUDED.image,
  status='PUBLISHED', is_admin_post=true, published_at=COALESCE(article.published_at, now()), updated_at=now()
RETURNING slug, title;
""" % ",".join(values)
    result = subprocess.run(COMPOSE + ["exec", "-T", "db", "psql", "-v", "ON_ERROR_STOP=1", "-U", "aiuser", "-d", "aisell", "-At", "-F", "|"], input=query, text=True, capture_output=True)
    if result.returncode:
        sys.stderr.write(result.stderr)
        return result.returncode
    lines = [line for line in result.stdout.splitlines() if "|" in line]
    if len(lines) != len(articles):
        sys.stderr.write(f"published {len(lines)} of {len(articles)} articles\n")
        return 1
    print("\n".join(lines))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
