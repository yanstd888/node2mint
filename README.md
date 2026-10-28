<div align="center">

  <!-- DYNAMIC TYPING HEADER -->
  <a href="https://github.com/yanstd888/node2mint">
    <img src="https://readme-typing-svg.demolab.com?font=JetBrains+Mono&weight=700&size=20&duration=3000&pause=1000&color=10B981&center=true&vCenter=true&width=800&lines=Node2Mint+%E2%80%94+AI+Sell+Modern+%26+API+Token+Engine;Self-Hosted+AI+Marketplace+%C2%B7+OpenAI-Compatible+Go+Gateway;Atomic+PostgreSQL+Ledger+%C2%B7+Escrow+Commerce+%C2%B7+Multi-Channel+Payments" alt="Node2Mint — AI Sell Modern" />
  </a>

  <h1>🪙 Node2Mint — AI Sell Modern</h1>

  <p align="center">
    <img src="./docs/images/landing-hero.png" alt="Node2Mint - Self-Hosted Digital Marketplace, Token Billing Engine & OpenAI API Gateway" width="100%" />
  </p>

  <p align="center">
    <strong>Self-hosted AI marketplace, token billing engine, and OpenAI-compatible API gateway with atomic ledger balances, provider failover, and affiliate distribution.</strong>
  </p>

  <p align="center">
    A production-ready full-stack platform built with <strong>Next.js 15, Go 1.22, PostgreSQL 16, Redis 7, and Caddy 2</strong> for selling digital products, token packs, and routing AI model access.
  </p>

  <!-- TECH STACK & LICENSE BADGES -->
  <p align="center">
    <a href="LICENSE"><img alt="License MIT" src="https://img.shields.io/badge/License-MIT-2563EB?style=for-the-badge"></a>
    <img alt="Go 1.22+" src="https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=for-the-badge&logo=go&logoColor=white">
    <img alt="Next.js 15" src="https://img.shields.io/badge/Next.js-15-0D1117?style=for-the-badge&logo=next.js&logoColor=white">
    <img alt="PostgreSQL 16" src="https://img.shields.io/badge/PostgreSQL-16-336791?style=for-the-badge&logo=postgresql&logoColor=white">
    <img alt="Redis 7" src="https://img.shields.io/badge/Redis-7-DC382D?style=for-the-badge&logo=redis&logoColor=white">
    <img alt="Docker Compose" src="https://img.shields.io/badge/Docker-Compose-2496ED?style=for-the-badge&logo=docker&logoColor=white">
  </p>

  <!-- ECOSYSTEM & PAYMENT BADGES -->
  <p align="center">
    <img alt="OpenAI Compatible" src="https://img.shields.io/badge/API-OpenAI_v1_Compatible-10B981?style=flat-square&logo=openai&logoColor=white">
    <img alt="Payments" src="https://img.shields.io/badge/Payments-Stripe_%C2%B7_DOKU_%C2%B7_QRIS_%C2%B7_USDT-8B5CF6?style=flat-square">
    <a href="https://aiclub.ai.id/"><img alt="AICLUB Ecosystem" src="https://img.shields.io/badge/Ecosystem-AICLUB.AI.ID-FFD23F?style=flat-square&labelColor=111111"></a>
    <a href="https://github.com/sponsors/yanstd888"><img alt="Sponsor Yanstd" src="https://img.shields.io/badge/Sponsor-GitHub_Sponsors-EA4AAA?style=flat-square&logo=githubsponsors&logoColor=white"></a>
  </p>

  <!-- QUICK NAVIGATION -->
  <p align="center">
    <a href="#-why-node2mint">Why Node2Mint</a> ·
    <a href="#-features--core-modules">Features</a> ·
    <a href="#-system-architecture">Architecture</a> ·
    <a href="#-quick-start">Quick Start</a> ·
    <a href="#-api-reference">API Reference</a> ·
    <a href="#-configuration">Configuration</a> ·
    <a href="#-deployment">Deployment</a> ·
    <a href="#-safety-and-security">Security</a> ·
    <a href="#-bahasa-indonesia">Bahasa Indonesia</a>
  </p>

</div>

---

## ⚡ Why Node2Mint?

Running an AI model reselling service or token marketplace requires strict ledger safety. Distributed billing systems frequently suffer from race conditions, double-spending, and upstream provider outages. **Node2Mint** solves these engineering challenges by combining a high-concurrency **Go 1.22 API gateway** with an **atomic PostgreSQL 16 transaction model**:

* 🔒 **Atomic Money & Token Balances:** All balance debits and credits use row-level database locking (`SELECT FOR UPDATE`) within strict transaction boundaries—eliminating negative-balance race conditions.
* 🔄 **Automated Provider Failover:** Outbound AI chat completions automatically route through active upstream providers (OpenAI, Anthropic, custom endpoints) with health checks and circuit breaker fallback.
* 🔑 **Hashed Credentials (Zero Plaintext):** User API keys are stored exclusively as `SHA-256` hashes. Full keys are displayed only once upon creation and never stored in plaintext.
* 💳 **Multi-Channel Global & Local Payments:** Native support for card/e-wallet gateways (**Stripe, DOKU**), manual **QRIS / Bank Transfers** with admin verification workflows, and optional **TRC-20 USDT** crypto settlement.
* 🚀 **Built-In Growth & Affiliate Engine:** Native referral system (bonus token grants for inviters and invitees) and automated affiliate wallet commissions on completed marketplace orders.
* 🖥️ **50+ Production Web Templates Included:** Complete responsive UI across customer marketplace, seller dashboard, interactive AI playground/chat, and administrative consoles.

### How Node2Mint Compares (LLM Gateway & Marketplace Matrix)

| Capability | **Node2Mint** | LiteLLM / One-API | OpenRouter (Cloud) |
| :--- | :---: | :---: | :---: |
| **OpenAI-Compatible Gateway (`/v1/chat/completions`)** | ✅ Yes | ✅ Yes | ✅ Yes |
| **Upstream Health Checks & Circuit Breaker Failover** | ✅ Yes | ✅ Yes | ✅ Yes |
| **Row-Locked Atomic Ledger (`SELECT FOR UPDATE`)** | ✅ **Yes** | ⚠️ Varies | ✅ Yes |
| **Multi-Vendor Digital Marketplace & Escrow Wallet** | ✅ **Yes (Built-in)** | ❌ No | ❌ No |
| **Affiliate Commission & Referral Quota Engine** | ✅ **Yes (Built-in)** | ❌ No | ❌ No |
| **Stripe, DOKU, QRIS & TRC-20 USDT Payments** | ✅ **Yes (Native)** | ❌ External Plugin | ❌ Stripe/Crypto Only |
| **100% Self-Hosted & Full Source Ownership** | ✅ **MIT License** | ✅ Open Source | ❌ Proprietary SaaS |

---

## 🧩 Features & Core Modules

<table width="100%">
  <tr>
    <td width="50%" valign="top">
      <h3>🔀 API Gateway & Token Billing</h3>
      <ul>
        <li><strong>OpenAI-Compatible <code>/v1/chat/completions</code>:</strong> Serves upstream models to client apps, SDKs, Cursor, and Cline using standardized schema headers.</li>
        <li><strong>Dynamic Token Metering:</strong> Deducts user balances based on exact model input/output rates, including cached prompt discount calculations.</li>
        <li><strong>Fixed-Window Rate Limiting:</strong> Per-IP and per-account request throttling enforced via <strong>Redis 7</strong>.</li>
        <li><strong>Telegram Notification Gateway:</strong> Real-time alerts via Telegram Bot API for order placements, dispute escalations, and group membership verification.</li>
      </ul>
    </td>
    <td width="50%" valign="top">
      <h3>🛒 Marketplace & Escrow Commerce</h3>
      <ul>
        <li><strong>Storefront & Product Catalog:</strong> Sell digital prompts, fine-tuned models, datasets, software tools, and subscription access packs.</li>
        <li><strong>Escrow Settlement:</strong> Buyer funds are safely held in platform escrow until manual or automated confirmation releases payout to the seller's store wallet.</li>
        <li><strong>Order Cancellation & Refunds:</strong> Safe rollback paths with immutable, atomic transaction log entries for every money movement.</li>
        <li><strong>Seller Store Management:</strong> Product publishing, GMV/sales analytics, dispute handling, and withdrawal requests.</li>
      </ul>
    </td>
  </tr>
  <tr>
    <td width="50%" valign="top">
      <h3>🤝 Affiliate & Referral Program</h3>
      <ul>
        <li><strong>Referral Rewards:</strong> Automatic token quota grants for new signups and referral code owners.</li>
        <li><strong>Affiliate Commission:</strong> Configurable percentage payouts deposited directly into affiliator wallets upon order settlement.</li>
        <li><strong>Anti-Fraud Controls:</strong> IP hashing, Cloudflare Turnstile, and duplicate signup checks protect referral distributions.</li>
      </ul>
    </td>
    <td width="50%" valign="top">
      <h3>🎨 Frontend & UI/UX (Next.js 15)</h3>
      <ul>
        <li><strong>Dual-Mode Shell:</strong> Customer portal with sleek dark-mode aesthetics paired with a high-contrast amber administrative shell.</li>
        <li><strong>Multilingual (i18n):</strong> Built-in <strong>English (`en`)</strong> and <strong>Bahasa Indonesia (`id`)</strong> localization dictionaries.</li>
        <li><strong>Client-Side Image Optimization:</strong> Browser canvas WebP image compression (<code>&lt;256 KB</code>) prior to server upload.</li>
      </ul>
    </td>
  </tr>
</table>

---

## 🏗️ System Architecture

```text
                Client (Web / Mobile / CLI / OpenAI SDK)
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
      PostgreSQL 16                Redis 7               Upstream AI APIs
   (Ledger / Orders / Auth)     (Rate Limiting)       (OpenAI / Anthropic / Custom)
```

### Directory Structure

```text
node2mint/
├── gateway/                  # Go backend gateway (Chi router, pgxpool, redis)
│   ├── cmd/gateway/          # Application entrypoint
│   ├── internal/
│   │   ├── auth/             # JWT Bearer tokens, password hashing
│   │   ├── billing/          # Atomic charges, balance validation
│   │   ├── handler/          # HTTP controllers (admin, market, tokens, chat)
│   │   ├── provider/         # Upstream AI model client & failover
│   │   ├── rate/             # Redis-backed fixed-window rate limiter
│   │   └── store/            # PostgreSQL queries and migrations
│   ├── sql/                  # Schema definitions and base seed
│   └── .env.example          # Backend environment template
├── web/                      # Next.js 15 frontend (Tailwind CSS, shadcn/ui)
│   ├── app/                  # App router (marketplace, dashboard, admin, auth)
│   ├── components/           # Reusable UI components & layout shells
│   ├── lib/                  # API clients, auth context, i18n dictionaries
│   └── .env.example          # Frontend environment template
├── deploy/                   # Production deployment configurations
│   ├── docker-compose.prod.yml
│   ├── Caddyfile
│   └── site-allowlist.env.example
├── docs/                     # Design system and architecture documentation
├── docker-compose.yml        # Development stack orchestration
└── smoke-test.sh             # End-to-end billing and order regression check
```

---

## 🚀 Quick Start

### 1. Prerequisites
* **Docker Engine** & **Docker Compose v2+**
* **Go 1.22+** (for local backend development)
* **Node.js 18+** (for local frontend development)

### 2. Clone the Repository
```bash
git clone [https://github.com/yanstd888/node2mint.git](https://github.com/yanstd888/node2mint.git)
cd node2mint
```

### 3. Configure Environments
```bash
cp gateway/.env.example gateway/.env
cp web/.env.example web/.env.local
cp -r gateway/.secrets.example gateway/.secrets
```
> 💡 *The example templates contain pre-configured dev credentials. For production, generate unique 32+ character secrets (`JWT_SECRET`, `SECRET_KEY`).*

### 4. Run Development Stack

Start backend services (PostgreSQL 16, Redis 7, Go Gateway, Uptime Kuma):
```bash
docker compose up -d --build
```

Verify all containers are healthy:
```bash
docker compose ps
curl http://localhost:8080/healthz   # Returns "ok"
```

Start Next.js 15 frontend (in a separate terminal):
```bash
cd web
npm install
npm run dev
```

Access the local services:
* 🌐 **Web Marketplace & App:** `http://localhost:3000`
* ⚡ **API Gateway:** `http://localhost:8080`
* 📊 **Uptime Kuma Health Dashboard:** `http://localhost:3001`

#### Default Seed Accounts (Local Development Only)
The local compose setup automatically mounts `gateway/sql/99-local-dev-seed.sql` with two test accounts:

| Role | Email | Password | Pre-seeded Balance |
| :--- | :--- | :--- | :--- |
| **Admin** | `admin@test.local` | `Test1234!` | `50,000 IDR` |
| **Seller** | `seller@test.local` | `Test1234!` | `10,000 IDR` (Demo Store with 13 products) |

*(Seed accounts are strictly excluded in production deployments).*

### 5. Automated End-to-End Verification
Run the money and order validation regression smoke test:
```bash
bash smoke-test.sh
```

---

## 🔌 API Reference

The Go gateway exposes REST endpoints alongside OpenAI-compatible completions:

| Method | Endpoint | Auth | Description |
| :--- | :--- | :--- | :--- |
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

---

## ⚙️ Configuration

Neutral configuration templates are provided in `.env.example` files. **Never commit `.env` or `.env.local` files to version control.**

### Gateway (`gateway/.env`)
| Variable | Description | Default |
| :--- | :--- | :--- |
| `DATABASE_URL` | PostgreSQL connection string | `postgresql://aiuser:***@localhost:5432/aisell` |
| `REDIS_ADDR` | Redis host and port | `localhost:6379` |
| `JWT_SECRET` | 32+ character random string for auth tokens | `CHANGE_ME_RANDOM_SECRET` |
| `SECRET_KEY` | 32+ character random string for data encryption | `CHANGE_ME_ENCRYPTION_KEY` |
| `PORT` | HTTP listener port | `8080` |
| `RATE_LIMIT_DISABLE` | Disable rate limiter (local dev only) | `false` |
| `STRIPE_SECRET_KEY` | Optional Stripe API secret key | *(empty)* |
| `DOKU_SANDBOX` | Toggle DOKU payment simulator | `true` |
| `TELEGRAM_BOT_TOKEN` | Optional bot token for group membership gate | *(empty)* |

### Frontend (`web/.env.local`)
| Variable | Description | Default |
| :--- | :--- | :--- |
| `NEXT_PUBLIC_API_URL` | Gateway API URL | `http://localhost:8080` |
| `NEXT_PUBLIC_DEFAULT_LOCALE` | Default language (`id` or `en`) | `id` |
| `NEXT_PUBLIC_SITE_NAME` | Branding title | `Node2Mint` |

---

## 🚢 Deployment

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

---

## 🛡️ Safety and Security

* **Zero Plaintext Secrets in Repository:** No production keys, database passwords, or operational secrets are stored in Git.
* **Hashed API Keys:** Gateway API keys are hashed with `SHA-256` before insertion. Only the prefix (e.g., `n2m_live_...`) is queryable by users.
* **Race Condition Prevention:** Balances, order stock, and withdrawal flows run in isolated transactions with `SELECT ... FOR UPDATE` row locks.
* **Bot & Abuse Mitigation:** Optional Cloudflare Turnstile verification guards public registration and authentication endpoints.
* **Structured Audit Logs:** Every administrative balance adjustment, top-up approval, escrow release, and withdrawal records an immutable audit log row with actor ID, IP address, and timestamp.

### Pre-Commit & CI Verification
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

---

## 🇮🇩 Bahasa Indonesia — Ringkasan Node2Mint

**Node2Mint** adalah platform *full-stack open-source* (**Next.js 15 + Go 1.22 + PostgreSQL 16 + Redis 7**) yang menggabungkan **Marketplace Produk Digital AI**, **Token Billing Engine**, dan **OpenAI-Compatible API Gateway**.

Dirancang khusus untuk menjawab kebutuhan ekosistem bisnis digital modern (baik global maupun Indonesia):
* **Keamanan Saldo Anti-Minus (Atomic Ledger):** Menggunakan *row-level locking* (`SELECT FOR UPDATE`) di PostgreSQL untuk mencegah *race condition* atau *double-spending* saat ribuan request API berjalan bersamaan.
* **Dukungan Pembayaran Lokal & Global:** Mendukung integrasi **DOKU, QRIS, Transfer Bank Manual**, **Stripe**, hingga **USDT (TRC-20)** serta sistem rekber (*Escrow*) otomatis untuk melindungi pembeli dan penjual di marketplace.
* **Kompatibel Penuh dengan Ekosistem OpenAI:** Endpoint `/v1/chat/completions` dilengkapi *automatic provider failover* dan *circuit breaker*, siap dihubungkan ke aplikasi AI, bot, maupun IDE seperti Cursor dan Cline.

---

## 📚 Documentation & Ecosystem

| Document / Resource | Purpose |
| :--- | :--- |
| [**LICENSE**](LICENSE) | MIT Open Source License terms |
| [**CONTRIBUTING.md**](CONTRIBUTING.md) | Branching guidelines, coding standards, and PR workflows |
| [**SECURITY.md**](SECURITY.md) | Vulnerability disclosure policy and reporting contacts |
| [**CODE_OF_CONDUCT.md**](CODE_OF_CONDUCT.md) | Contributor Covenant v2.1 community standards |
| [**Design System**](docs/DESIGN-SYSTEM.md) | Frontend component specs, color palette, and layout rules |
| [**AICLUB Indonesia (`aiclub.ai.id`)**](https://aiclub.ai.id/) | Official AI tools directory, free `.ai.id` subdomains, and community hub |
| [**Toko AICLUB (`toko.aiclub.ai.id`)**](https://toko.aiclub.ai.id) | Live digital product & application marketplace |

---

## 🤝 Contributing & License

Contributions, bug reports, and pull requests are welcome! Please ensure changes include a minimal reproducible check or automated unit test where applicable. Read [CONTRIBUTING.md](CONTRIBUTING.md) for details.

This project is licensed under the [MIT License](LICENSE) — Built and maintained by [**Yanstd (@yanstd888)**](https://github.com/yanstd888).
