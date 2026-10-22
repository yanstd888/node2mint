# Contributing to Node2Mint

## Dev Environment (Windows)
1. Install WSL2 + Ubuntu + Docker Desktop.
2. Clone:
   ```powershell
   git clone https://github.com/node2mint/ai-sell-modern.git
   cd ai-sell-modern
   ```
3. Copy env templates (never commit real `.env`):
   ```bash
   cp gateway/.env.example gateway/.env
   cp web/.env.example web/.env.local
   ```
4. Start:
   ```bash
   docker compose up -d --build
   cd web && npm install && npm run dev
   ```

## Branch / PR Flow
- Branch: `feat/<short>`, `fix/<short>`, `docs/<short>`
- Commit message: conventional format — `feat:`, `fix:`, `chore:`
- PR: one logical change, self-reviewed, CI green
- Rebase before merge

## Tests
```bash
# Go backend
cd gateway
go vet ./...
go test ./...

# Frontend
cd ../web
npm run build
```

## What NOT to Commit
- Any `.env` file, real API keys, or credentials
- Build artifacts (`node_modules/`, `.next/`, `dist/`)
- Database dumps or logs

## Reporting Bugs / Features
- Use GitHub Issues with the template
- Security → [SECURITY.md](SECURITY.md)
