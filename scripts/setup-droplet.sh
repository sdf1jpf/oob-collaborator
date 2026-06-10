#!/usr/bin/env bash
#
# Prepares a fresh Ubuntu droplet to run the OOB Collaborator server.
#
#   1. Frees up port 53 by disabling the systemd-resolved stub listener
#      (it collides with this server's authoritative DNS engine).
#   2. Installs Docker Engine + the Compose plugin.
#
# Usage (as root):
#   sudo bash scripts/setup-droplet.sh
#
# Or piped from the repo:
#   curl -fsSL https://raw.githubusercontent.com/<org>/oob-collaborator/main/scripts/setup-droplet.sh | bash
#
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "This script must run as root. Re-run with: sudo bash $0" >&2
  exit 1
fi

log() { printf '\n\033[1;32m==>\033[0m %s\n' "$*"; }

# ---------------------------------------------------------------------------
# 1. Free port 53 (disable systemd-resolved stub listener)
# ---------------------------------------------------------------------------
if systemctl is-active --quiet systemd-resolved; then
  log "Disabling systemd-resolved stub listener on :53"
  mkdir -p /etc/systemd/resolved.conf.d
  cat >/etc/systemd/resolved.conf.d/disable-stub.conf <<'EOF'
[Resolve]
DNSStubListener=no
EOF
  # Point the host at a real upstream resolver instead of the stub.
  ln -sf /run/systemd/resolve/resolv.conf /etc/resolv.conf
  systemctl restart systemd-resolved
  log "Port 53 is now free for the collaborator DNS engine"
else
  log "systemd-resolved not active; skipping :53 stub disable"
fi

# ---------------------------------------------------------------------------
# 2. Install Docker Engine + Compose plugin
# ---------------------------------------------------------------------------
if command -v docker >/dev/null 2>&1; then
  log "Docker already installed: $(docker --version)"
else
  log "Installing Docker Engine + Compose plugin"
  curl -fsSL https://get.docker.com | sh
fi

if docker compose version >/dev/null 2>&1; then
  log "Docker Compose plugin present: $(docker compose version | head -n1)"
else
  echo "WARNING: 'docker compose' plugin not found after install." >&2
  echo "Install it manually: https://docs.docker.com/compose/install/" >&2
fi

log "Host preparation complete."
cat <<'EOF'

Next steps:
  1. git clone <your-repo-url> oob-collaborator && cd oob-collaborator
  2. cp .env.example .env   # then edit DOMAIN, PUBLIC_IP, secrets, ACME_EMAIL
  3. docker compose up --build -d
  4. docker compose logs -f collaborator

See docs/DEPLOY_DIGITALOCEAN.md for the full guide.
EOF
