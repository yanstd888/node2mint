# Node2Mint production deployment

Cloudflare Tunnel calls `http://localhost:80`; Caddy is bound only to `127.0.0.1:80` and redirects `node2mint.com` to `www.node2mint.com`.

```sh
cd /root/node2mint/deploy
cp gateway.env.example gateway.env
cp site-allowlist.env.example site-allowlist.env
# Set SITE_IP_ALLOWLIST to the approved public IP/CIDR(s).
# Create gateway/.secrets/DB_PASSWORD and REDIS_PASSWORD with no newline.
docker compose --env-file site-allowlist.env -f docker-compose.prod.yml up -d --build
docker compose --env-file site-allowlist.env -f docker-compose.prod.yml ps
./migrate.sh
```

Validate a backup without restoring it:

```sh
BACKUP_DIR=/root/node2mint/backups ./backup.sh
./restore-check.sh /root/node2mint/backups/aisell-YYYYMMDDTHHMMSSZ.dump
```

USDT TRC-20 credentials stay unset and the on-chain watcher must remain disabled until its address, contract, TronGrid key, and exact-match settlement have been configured.

## DOKU Checkout

Set DOKU through the masked helper on the Ubuntu host; never commit or paste the values into chat:

```sh
./set-doku.sh
```

Configure the DOKU HTTP Notification URL as:

```text
https://www.node2mint.com/api/webhooks/doku
```

The application sends the same URL as an `override_notification_url` on every Checkout invoice. DOKU redirect/return pages do not credit the Wallet; only a signed notification or signed Check Status response matched to the original request ID, invoice, currency, and amount can settle the top-up.

## Midtrans: dihapus total

Midtrans dihentikan pada cutover DOKU (9 Oktober 2026): tabel `midtrans_payment`, handler webhook, reconciler, endpoint status, env `MIDTRANS_*`, dan helper `set-midtrans.sh` sudah dibuang. Tidak ada lagi jalur Midtrans di kode maupun deployment. Semua top-up eksternal lewat DOKU.
