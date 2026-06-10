# Deploying to DigitalOcean

This guide deploys the OOB Collaborator server to a single DigitalOcean Droplet.

Unlike a typical web app, this server is its **own authoritative nameserver**: it
answers wildcard A records for your domain *and* self-answers the Let's Encrypt
ACME DNS-01 challenge to obtain a wildcard TLS certificate. That means you delegate
DNS to the droplet at your registrar rather than hosting the zone in the
DigitalOcean DNS panel.

It also binds privileged ports `53` (DNS), `80`/`443` (HTTP/S), and `25`/`587`
(SMTP), so the firewall and host setup matter.

---

## 1. Prerequisites

- A **domain** you control (to delegate DNS to the droplet).
- A DigitalOcean account.
- The repo pushed somewhere the droplet can `git clone` (or you can `scp` it up).

## 2. Create the Droplet

| Setting | Recommendation |
|---------|----------------|
| Image | Ubuntu 24.04 LTS |
| Size | Basic, **2 GB RAM** minimum (Postgres + Go server + image builds) |
| IP | Note the public IPv4 — referred to below as `DROPLET_IP` |
| Auth | Your SSH key |

> The fastest path is to paste [`scripts/cloud-init.yaml`](../scripts/cloud-init.yaml)
> into the droplet's **User data** field at creation time — it performs all of the
> host prep in [section 5](#5-host-preparation-automated-by-cloud-init)
> automatically. You can also run [`scripts/setup-droplet.sh`](../scripts/setup-droplet.sh)
> manually after the droplet is up.

## 3. DNS delegation (the critical part)

The server *is* the DNS authority for your domain, so configure delegation at your
**registrar** (not in DigitalOcean's DNS panel).

Create these records at your registrar:

| Type | Host | Value |
|------|------|-------|
| A (glue) | `ns1.yourdomain.com` | `DROPLET_IP` |
| NS | `yourdomain.com` | `ns1.yourdomain.com` |

Some registrars require you to register `ns1.yourdomain.com` as a "private
nameserver" / "host record" / "glue record" before they will accept the NS record.
Do that first if prompted.

Verify once propagated:

```bash
dig +short NS yourdomain.com
dig @DROPLET_IP test.yourdomain.com A   # should answer with DROPLET_IP
```

## 4. DigitalOcean firewall

Create a Cloud Firewall (Networking → Firewalls), attach it to the droplet, and add
these **inbound** rules:

| Protocol | Ports | Purpose |
|----------|-------|---------|
| TCP | 22 | SSH |
| TCP | 80, 443 | HTTP / HTTPS trap + dashboard + ACME |
| UDP | 53 | DNS |
| TCP | 53 | DNS (large responses / zone transfers) |
| TCP | 25, 587 | SMTP interaction capture |

> ⚠️ **Port 25 caveat:** DigitalOcean blocks **outbound** port 25 by default to
> fight spam. Inbound port 25 (what this server uses to *receive* SMTP
> interactions) works fine. Only file a support ticket to unblock 25 if you later
> need the server to *send* mail.

## 5. Host preparation (automated by cloud-init)

If you did **not** use cloud-init, run [`scripts/setup-droplet.sh`](../scripts/setup-droplet.sh)
on the droplet as root. It performs the steps below:

1. Disables the `systemd-resolved` stub listener on `:53` (otherwise it collides
   with this server's DNS engine).
2. Installs Docker Engine + the Compose plugin.

```bash
ssh root@DROPLET_IP
curl -fsSL https://raw.githubusercontent.com/<your-org>/oob-collaborator/main/scripts/setup-droplet.sh | bash
# or, if the repo is already cloned:
sudo bash scripts/setup-droplet.sh
```

## 6. Deploy the application

```bash
ssh root@DROPLET_IP
git clone <your-repo-url> oob-collaborator
cd oob-collaborator

cp .env.example .env
```

Edit `.env` for production:

```bash
DOMAIN=yourdomain.com
PUBLIC_IP=DROPLET_IP          # the droplet's real public IP, NOT 0.0.0.0
NS_HOST=ns1

ADMIN_PASSWORD=<strong-password>
JWT_SECRET=<long-random-string>     # openssl rand -hex 32
POLL_TOKEN=<random-token>           # openssl rand -hex 24

ACME_EMAIL=you@example.com
ACME_STAGING=true             # start with staging, flip to false once it works
```

Build and start:

```bash
docker compose up --build -d
docker compose logs -f collaborator
```

Watch the logs for the DNS/HTTP/HTTPS listeners coming up and the ACME certificate
being obtained via the DNS-01 challenge (the DNS engine answers its own challenge —
no extra config needed).

## 7. Validate, then go to production TLS

```bash
# DNS trap
dig @DROPLET_IP anything.yourdomain.com    # answers DROPLET_IP

# SMTP trap
nc DROPLET_IP 25                           # should get an SMTP banner

# Dashboard (cert will be untrusted while ACME_STAGING=true)
curl -kI https://yourdomain.com/dashboard
```

Once staging works end-to-end, switch to real Let's Encrypt certs:

```bash
sed -i 's/^ACME_STAGING=.*/ACME_STAGING=false/' .env
docker compose up -d --build
```

Then open `https://yourdomain.com/dashboard` and log in with `ADMIN_PASSWORD`.

---

## Continuous deployment (GitHub Actions)

[`.github/workflows/deploy.yml`](../.github/workflows/deploy.yml) builds the image,
pushes it to the GitHub Container Registry (GHCR), and deploys it to the droplet
over SSH on every push to `main` (or via manual "Run workflow").

Instead of building on the server, the droplet pulls the prebuilt image using
[`docker-compose.prod.yml`](../docker-compose.prod.yml), which overrides the
`collaborator` service to use `${COLLABORATOR_IMAGE}` with `pull_policy: always`.

### One-time droplet setup

1. Complete the manual deploy ([sections 5–6](#6-deploy-the-application)) at least
   once so the repo is cloned (e.g. `/opt/oob-collaborator`) with a valid `.env`
   and a `postgres_data` volume. The workflow only updates the compose files and
   the running container — it never touches your `.env`.
2. Ensure the deploy user can run Docker without `sudo`
   (`usermod -aG docker <user>`).

### Required repository secrets

Set these under **Settings → Secrets and variables → Actions**:

| Secret | Purpose |
|--------|---------|
| `DROPLET_HOST` | Droplet public IP or hostname |
| `DROPLET_USER` | SSH user (e.g. `root` or a deploy user) |
| `DROPLET_SSH_KEY` | Private SSH key (PEM) authorized on the droplet |
| `DROPLET_SSH_PORT` | *(optional)* SSH port, defaults to `22` |
| `DEPLOY_PATH` | *(optional)* repo path on the droplet, defaults to `/opt/oob-collaborator` |
| `GHCR_PULL_TOKEN` | PAT with `read:packages` so the droplet can pull from GHCR (not needed if you make the GHCR package public) |

> The build job uses the built-in `GITHUB_TOKEN` to push to GHCR — no secret
> needed for the push. `GHCR_PULL_TOKEN` is only for the *pull* on the droplet.
> Alternatively, set the package visibility to public in GHCR and you can drop
> `GHCR_PULL_TOKEN` and the `docker login` step.

### What the deploy step runs on the droplet

```bash
cd "$DEPLOY_PATH"
git fetch --depth 1 origin main
git checkout origin/main -- docker-compose.yml docker-compose.prod.yml
echo "$GHCR_TOKEN" | docker login ghcr.io -u "$GHCR_USER" --password-stdin
export COLLABORATOR_IMAGE=ghcr.io/<owner>/oob-collaborator:<sha>
docker compose -f docker-compose.yml -f docker-compose.prod.yml pull collaborator
docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d
```

## Operations

| Task | Command |
|------|---------|
| View logs | `docker compose logs -f collaborator` |
| Restart | `docker compose restart collaborator` |
| Update code | `git pull && docker compose up -d --build` |
| Stop (keep data) | `docker compose down` |
| **Wipe everything** | `docker compose down -v` ⚠️ deletes the DB volume |

### Notes & recommendations

- **Test ACME against staging first** (`ACME_STAGING=true`). The cert will be
  untrusted, but it avoids hitting Let's Encrypt rate limits while you debug DNS
  delegation. Flip to `false` only once issuance succeeds.
- **Postgres credentials** default to `collaborator:collaborator` in
  `docker-compose.yml` / `.env.example`. The DB port is not published to the host
  (it's only reachable inside the Docker network), but change it for
  defense-in-depth if you like. If you do, update `DATABASE_URL` in both
  `.env` and the `collaborator` service `environment` block, plus the `migrate`
  service command, in `docker-compose.yml`.
- **Persistence:** interaction data lives in the `postgres_data` Docker volume and
  survives `docker compose down`. Only `down -v` destroys it.

## Troubleshooting

| Symptom | Likely cause / fix |
|---------|--------------------|
| `bind: address already in use` on `:53` | `systemd-resolved` stub still active — re-run `scripts/setup-droplet.sh` or section 5. |
| ACME cert never issues | DNS delegation not live yet. Confirm `dig @DROPLET_IP test.yourdomain.com` answers and the registrar NS/glue records are set. |
| Dashboard unreachable | Cloud Firewall missing 80/443 inbound, or `DOMAIN`/`PUBLIC_IP` wrong in `.env`. |
| No SMTP interactions | Inbound 25 blocked by firewall (inbound 25 is allowed by DO; only outbound is blocked). |
| Container exits citing database | Postgres still starting; the server waits up to 60s — check `docker compose logs postgres`. |
