# OOB Collaborator Server & Engagement Dashboard

Self-hosted multi-protocol Out-of-Band (OOB) interaction server with a PostgreSQL-backed engagement dashboard.

## Features

- **DNS** (UDP/TCP :53) — wildcard A records + interaction logging
- **HTTP/HTTPS** (:80/:443) — request trapping with full header/body capture
- **SMTP** (:25/:587) — mail interaction capture
- **Burp-style polling API** (`GET /bi/b`) — documented custom protocol
- **Web dashboard** — TanStack Router/Query/Table, live WebSocket ticker, split-screen inspector
- **Wildcard TLS** — Let's Encrypt via ACME DNS-01 (self-answered by the DNS engine)

## Quick Start (Docker)

```bash
cp .env.example .env
# Edit .env with your domain, public IP, passwords, and ACME email

docker compose up --build -d
```

Dashboard: `https://yourdomain.com/dashboard` (or `http://localhost:443` in dev mode)

Default admin password: value of `ADMIN_PASSWORD` in `.env`

### Dev Mode (no TLS)

Set in `.env`:
```
DOMAIN=localhost
SKIP_TLS=true
HTTPS_PORT=8080
```

## Deployment

For a step-by-step DigitalOcean Droplet deployment (DNS delegation, firewall,
host prep, and TLS), see [docs/DEPLOY_DIGITALOCEAN.md](docs/DEPLOY_DIGITALOCEAN.md).
Automation helpers live in [`scripts/`](scripts/): `setup-droplet.sh` for host
prep and `cloud-init.yaml` to bootstrap the droplet at creation time.

Pushes to `main` build/push the image to GHCR and deploy it to the droplet over
SSH via [`.github/workflows/deploy.yml`](.github/workflows/deploy.yml) (uses the
[`docker-compose.prod.yml`](docker-compose.prod.yml) override). See the
[CI/CD section](docs/DEPLOY_DIGITALOCEAN.md#continuous-deployment-github-actions)
for the required repository secrets.

## DNS Setup (Production)

Point your domain's NS record to the droplet:

| Record | Value |
|--------|-------|
| A | `ns1.yourdomain.com` → Droplet IP |
| NS | `yourdomain.com` → `ns1.yourdomain.com` |

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/login` | — | Admin login → JWT |
| GET | `/api/engagements` | JWT | List engagements |
| POST | `/api/engagements` | JWT | Create engagement |
| GET | `/api/engagements/:id/interactions` | JWT | List interactions |
| GET | `/api/engagements/:id/payloads` | JWT | List payloads |
| POST | `/api/payloads/generate` | JWT | Generate payload token |
| GET | `/bi/b` | Poll token | Fetch undelivered interactions |
| GET | `/bi/health` | Poll token | Health check |
| GET | `/ws` | — | WebSocket live ticker |

See [docs/POLLING_API.md](docs/POLLING_API.md) for Burp-style polling details.

## Local Development

### Backend
```bash
cd backend
export DATABASE_URL=postgres://collaborator:collaborator@localhost:5432/collaborator?sslmode=disable
export SKIP_TLS=true DOMAIN=localhost HTTPS_PORT=8080 ADMIN_PASSWORD=changeme JWT_SECRET=dev
go run ./cmd/server
```

### Frontend
```bash
cd frontend
npm install
npm run dev
```

### Database only
```bash
docker compose up postgres migrate -d
```

## Project Structure

```
backend/          Go server (DNS, HTTP, SMTP, API, poll, WS)
frontend/         React + TanStack dashboard
docker-compose.yml
Dockerfile        Multi-stage build (frontend + backend)
```

## Burp Suite Compatibility

This server implements a **custom documented polling API** (`/bi/b`) with Collaborator-like JSON. Full native Burp private Collaborator protocol compatibility is a future phase.
