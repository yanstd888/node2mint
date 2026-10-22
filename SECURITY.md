# Security Policy

## Reporting a Vulnerability

**Do NOT open a public GitHub issue for security vulnerabilities.**

Email: `security@node2mint.com` (PGP key: [link-to-key])

Please include:
- Description of the issue
- Reproduction steps
- Your contact info / PGP key (optional)

We acknowledge within 24 hours and aim to fix within 7 days.

## Supported Versions

| Version | Supported |
|---------|-----------|
| main    | ✅         |
| 1.x     | ✅         |
| <1.0    | ❌         |

## Security Practices
- API keys are stored hashed (SHA-256), never plaintext
- All money flows use atomic DB transactions (SELECT FOR UPDATE + txn)
- Rate limiting via Redis fixed-window limiter
- JWT with `Bearer` auth
- Cloudflare Tunnel (no public ports for SSH)

## Out of Scope
- DoS vectors without concrete repro
- Issues requiring physical/server access
- Social engineering
- Vulnerabilities in third-party deps (use dependabot)
