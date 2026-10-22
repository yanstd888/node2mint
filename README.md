<h1 align="center">Node2Mint — AI Sell Modern</h1>

<p align="center">
  <strong>Self-hosted AI marketplace, token billing engine, and OpenAI-compatible API gateway with atomic ledger balances, provider failover, and affiliate distribution.</strong>
</p>

<p align="center">
  A production-ready full-stack platform built with Next.js 15, Go 1.22, PostgreSQL 16, Redis 7, and Caddy 2 for selling digital products, token packs, and routing AI model access.
</p>

<p align="center">
  <a href="#features">Features</a> ·
  <a href="#architecture">Architecture</a> ·
  <a href="#quick-start">Quick start</a> ·
  <a href="#api-reference">API reference</a> ·
  <a href="#configuration">Configuration</a> ·
  <a href="#deployment">Deployment</a> ·
  <a href="#safety-and-security">Safety</a> ·
  <a href="#documentation">Documentation</a>
</p>

<p align="center">
  <img alt="License MIT" src="https://img.shields.io/badge/license-MIT-blue.svg">
  <img alt="Go 1.22" src="https://img.shields.io/badge/go-1.22%2B-00ADD8.svg?logo=go">
  <img alt="Next.js 15" src="https://img.shields.io/badge/next.js-15-black.svg?logo=next.js">
  <img alt="PostgreSQL 16" src="https://img.shields.io/badge/storage-PostgreSQL%2016-336791.svg?logo=postgresql">
  <img alt="Redis 7" src="https://img.shields.io/badge/cache-Redis%207-DC382D.svg?logo=redis">
  <img alt="Docker" src="https://img.shields.io/badge/deployment-Docker%20Compose-2496ED.svg?logo=docker">
</p>

<p align="center">
  <img src="./docs/images/landing-hero.png" alt="Node2Mint - Digital Marketplace & AI API Tokens" width="100%" />
</p>

---

## Why Node2Mint?

Running an AI model reselling service or token marketplace requires robust ledger safety. Distributed billing often suffers from race conditions, double-spending, and provider outages. **Node2Mint** addresses these challenges by combining a high-performance Go API gateway with an atomic database transaction model:

- **Atomic money & token balances**: All balance debits and credits use row-level database locking (`SELECT FOR UPDATE`) within strict transaction boundaries.
- **Provider failover**: Outbound AI chat completions automatically route through active upstream providers with health checks and circuit breaker fallback.
- **Hashed credentials**: User API keys are stored exclusively as SHA-256 hashes. Full keys are displayed once upon creation and never stored in plaintext.
- **Multi-channel payments**: Native support for card/e-wallet gateways (Stripe, DOKU), manual QRIS/bank transfers with verification workflows, and optional TRC-20 USDT settlement.
- **Growth engine**: Built-in referral system (bonus token grants for inviters and invitees) and affiliate wallet commissions on completed orders.
- **Complete web templates included**: 50+ responsive pages across customer marketplace, seller dashboard, AI playground/chat, and administrative consoles.

## Architecture

```text
               Client (Web / Mobile / CLI)
                           │
                           ▼
                  Caddy 2 Reverse Proxy
                 (HTTPS / TLS Termination)
                ┌──────────┴──────────┐
                ▼                     ▼
       Next.js 15 Frontend       Go API Gateway
       (Port 3000 / SSR)        (Port 8080 / REST)
                                      │
            ┌─────────────────────────┼─────────────────────────┐
            ▼                         ▼                         ▼
      PostgreSQL 16                 Redis 7               Upstream AI APIs
   (Ledger / Orders / Auth)     (Rate Limiting)       (OpenAI / Anthropic / custom)
```

### Directory Structure

```text
node2mint/
├── gateway/              # Go backend gateway (Chi router, pgxpool, redis)
│   ├── cmd/gateway/      # Application entrypoint
│   ├── internal/
│   │   ├── auth/         # JWT Bearer tokens, password hashing
│   │   ├── billing/      # Atomic charges, balance validation
│   │   ├── handler/      # HTTP controllers (admin, market, tokens, chat)
│   │   ├── provider/     # Upstream AI model client & failover
│   │   ├── rate/         # Redis-backed fixed-window rate limiter
│   │   └── store/        # PostgreSQL queries and migrations
│   ├── sql/              # Schema definitions and base seed
│   └── .env.example      # Backend environment template
├── web/                  # Next.js 15 frontend (Tailwind CSS, shadcn/ui)
│   ├── app/              # App router (marketplace, dashboard, admin, auth)
│   ├── components/       # Reusable UI components & layout shells
│   ├── lib/              # API clients, auth context, i18n dictionaries
│   └── .env.example      # Frontend environment template
├── deploy/               # Production deployment configurations
│   ├── docker-compose.prod.yml
│   ├── Caddyfile
│   └── site-allowlist.env.example
├── docs/                 # Design system and architecture documentation
├── docker-compose.yml    # Development stack orchestration
└── smoke-test.sh         # End-to-end billing and order regression check
```

## Features

### API Gateway & Billing
- **OpenAI-Compatible `/v1/chat/completions`**: Serves upstream models to client applications using standardized schema headers.
- **Dynamic Token Metering**: Deducts user balances according to model input/output rates, with cached prompt discount calculations.
- **Fixed-Window Rate Limiting**: Per-IP and per-account request throttling enforced via Redis.
- **Telegram Notification Gateway**: Real-time alerts via Telegram Bot API for order placements, dispute escalations, and membership verification.

### Marketplace & Digital Commerce
- **Storefront & Product Catalog**: Digital prompts, models, datasets, and subscription access packs.
- **Escrow Settlement**: Buyer funds are held in platform escrow until manual or automated confirmation releases payout to the seller's store wallet.
- **Order Cancellation & Refunds**: Safe rollback paths with atomic transaction log entries for every money movement.
- **Seller Store Management**: Product publishing, sales analytics, dispute handling, and withdrawal requests.

### Affiliate & Referral Program
- **Referral Rewards**: Automatic token quota grants for new signups and referral code owners.
- **Affiliate Commission**: Configurable percentage payouts deposited directly into affiliator wallets upon order settlement.
- **Anti-Fraud Controls**: IP hashing and duplicate signup checks protect referral distributions.

### Frontend & UI/UX
- **Dual-Mode Shell**: Customer portal with dark mode aesthetics and high-contrast amber admin shell.
- **Multilingual (i18n)**: Built-in English and Bahasa Indonesia localization dictionaries.
- **Client-Side Image Optimization**: Browser canvas WebP image compression (<256 KB) before upload.

## Quick Start

### 1. Prerequisites
- Docker Engine & Docker Compose v2+
- Go 1.22+ (for local backend development)
- Node.js 18+ (for local frontend development)

### 2. Clone the Repository
```bash
git clone https://github.com/yanstd888/node2mint.git
cd node2mint
```

### 3. Configure Environments
```bash
cp gateway/.env.example gateway/.env
cp web/.env.example web/.env.local
```
Update `gateway/.env` with your secure credentials (`JWT_SECRET`, database passwords).

### 4. Run Development Stack
```bash
docker compose up -d --build
```
Verify all services are healthy:
```bash
docker compose ps
curl http://localhost:8080/healthz   # Returns "ok"
```

Open the web frontend:
- Web App: `http://localhost:3000`
- API Gateway: `http://localhost:8080`
- Default local admin: `admin@test.local` / `Test1234!` (only when seeded with test config)

### 5. Automated Verification
Run the end-to-end money and order validation smoke test:
```bash
bash smoke-test.sh
```

## API Reference

The gateway exposes REST endpoints and OpenAI-compatible completions:

| Method | Endpoint | Auth | Description |
| --- | --- | --- | --- |
| `POST` | `/v1/chat/completions` | Bearer API Key | OpenAI-compatible chat completions with failover |
| `POST` | `/api/auth/register` | Public / Turnstile | User registration with signup token grants |
| `POST` | `/api/auth/login` | Public / Turnstile | User authentication returning JWT token |
| `GET` | `/api/dashboard` | JWT | User profile, quotas, and wallet balance |
| `GET` | `/api/keys` | JWT | List active API keys (prefix only) |
| `POST` | `/api/keys` | JWT | Generate new API key (raw secret shown once) |
| `DELETE` | `/api/keys/{id}` | JWT | Revoke existing API key |
| `GET` | `/api/market/products` | Public | List published digital goods and access packs |
| `POST` | `/api/market/orders` | JWT | Place marketplace order with atomic balance escrow |
| `POST` | `/api/market/orders/{id}/confirm` | JWT | Buyer release of escrow to seller store wallet |
| `GET` | `/api/packages` | Public | Available token top-up and signup packages |
| `POST` | `/api/payment/checkout` | JWT | Create payment invoice (Stripe, DOKU, QRIS) |
| `GET` | `/api/admin/summary` | Admin | High-level platform GMV and user analytics |
| `POST` | `/api/admin/withdrawal/{id}/decide` | Admin | Approve or reject seller withdrawal with audit log |

## Configuration

Neutral configuration templates are provided in `.env.example` files:

### Gateway (`gateway/.env`)
| Variable | Description | Default |
| --- | --- | --- |
| `DATABASE_URL` | PostgreSQL connection string | `postgresql://aiuser:***@localhost:5432/aisell` |
| `REDIS_ADDR` | Redis host and port | `localhost:6379` |
| `JWT_SECRET` | 32+ character random string for auth tokens | `CHANGE_ME_RANDOM_SECRET` |
| `SECRET_KEY` | 32+ character random string for data encryption | `CHANGE_ME_ENCRYPTION_KEY` |
| `PORT` | HTTP listener port | `8080` |
| `RATE_LIMIT_DISABLE` | Disable rate limiter (local dev only) | `false` |
| `STRIPE_SECRET_KEY` | Optional Stripe API secret key | (empty) |
| `DOKU_SANDBOX` | Toggle DOKU payment simulator | `true` |
| `TELEGRAM_BOT_TOKEN` | Optional bot token for group membership gate | (empty) |

### Frontend (`web/.env.local`)
| Variable | Description | Default |
| --- | --- | --- |
| `NEXT_PUBLIC_API_URL` | Gateway API URL | `http://localhost:8080` |
| `NEXT_PUBLIC_DEFAULT_LOCALE` | Default language (`id` or `en`) | `id` |
| `NEXT_PUBLIC_SITE_NAME` | Branding title | `Node2Mint` |

Never commit `.env` or `.env.local` files to version control.

## Deployment

### Docker Compose (Production)
For production environments, mount secrets via Docker secrets or protected environment files:

```bash
cd deploy
cp site-allowlist.env.example site-allowlist.env
docker compose --env-file site-allowlist.env -f docker-compose.prod.yml up -d
```

### Cloudflare Tunnel Setup
To expose Node2Mint securely without opening public inbound firewall ports:
```bash
cloudflared tunnel run --protocol http2 --token YOUR_CLOUDFLARE_TUNNEL_TOKEN
```

## Safety and Security

- **Zero Plaintext Secrets in Repository**: No production keys, database passwords, or operational secrets are stored in Git.
- **Hashed API Keys**: Gateway API keys are hashed with SHA-256 before insertion. Only the prefix (e.g. `n2m_live_...`) is queryable by users.
- **Race Condition Prevention**: Balances, order stock, and withdrawal flows run in isolated transactions with `SELECT ... FOR UPDATE` locks.
- **Bot & Abuse Mitigation**: Optional Cloudflare Turnstile verification guards public registration and authentication endpoints.
- **Structured Audit Logs**: Every administrative balance adjustment, top-up approval, escrow release, and withdrawal records an immutable audit log row with actor ID, IP address, and timestamp.

## Verification

Run local checks before staging any pull request:

```bash
# Go gateway verification
cd gateway
go vet ./...
go test ./...

# Frontend type checking and build
cd ../web
npx tsc --noEmit --pretty false
npm run build

# Git cleanliness check
git diff --check
```

## Documentation

| Document | Purpose |
| --- | --- |
| [LICENSE](LICENSE) | MIT Open Source License terms |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Branching guidelines, coding standards, and PR workflows |
| [SECURITY.md](SECURITY.md) | Vulnerability disclosure policy and reporting contacts |
| [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) | Contributor Covenant v2.1 community standards |
| [Design System](docs/DESIGN-SYSTEM.md) | Frontend component specs, color palette, and layout rules |

## Contributing

Contributions, bug reports, and pull requests are welcome. Please ensure changes include a minimal reproducible check or automated unit test where applicable. Read [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

This project is licensed under the [MIT License](LICENSE).
