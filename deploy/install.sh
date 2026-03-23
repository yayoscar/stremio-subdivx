#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "==> Installing stremio-subdivx addon"

# 1. Install Docker if not present
if ! command -v docker &>/dev/null; then
  echo "==> Installing Docker..."
  apt-get update
  apt-get install -y ca-certificates curl
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc
  chmod a+r /etc/apt/keyrings/docker.asc
  echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu \
    $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
    tee /etc/apt/sources.list.d/docker.list > /dev/null
  apt-get update
  apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin
  systemctl enable docker
  systemctl start docker
  echo "==> Docker installed"
else
  echo "==> Docker already installed"
fi

# 2. Install cloudflared if not present
if ! command -v cloudflared &>/dev/null; then
  echo "==> Installing cloudflared..."
  curl -fsSL https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64 \
    -o /usr/local/bin/cloudflared
  chmod +x /usr/local/bin/cloudflared
  echo "==> cloudflared installed"
else
  echo "==> cloudflared already installed"
fi

# 3. Build Docker image
echo "==> Building Docker image..."
cd "$PROJECT_DIR"
make docker-build-allinone

# 4. Create cache directory
mkdir -p "$PROJECT_DIR/.cache"

# 5. Install systemd services
echo "==> Installing systemd services..."
cp "$SCRIPT_DIR/stremio-subdivx.service" /etc/systemd/system/
cp "$SCRIPT_DIR/stremio-subdivx-tunnel.service" /etc/systemd/system/
systemctl daemon-reload

# 6. Enable and start services
systemctl enable stremio-subdivx stremio-subdivx-tunnel
systemctl start stremio-subdivx
echo "==> Waiting for addon to start..."
sleep 10
systemctl start stremio-subdivx-tunnel

echo ""
echo "==> Installation complete!"
echo ""
echo "To get your HTTPS URL, run:"
echo "  journalctl -u stremio-subdivx-tunnel -f"
echo ""
echo "Look for a line like:"
echo '  https://xxx-yyy-zzz.trycloudflare.com'
echo ""
echo "Paste that URL in Stremio to install the addon."
