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

# 5. Stop old services if they exist
systemctl stop stremio-subdivx-tunnel 2>/dev/null || true
systemctl disable stremio-subdivx-tunnel 2>/dev/null || true
systemctl stop stremio-subdivx 2>/dev/null || true

# 6. Install systemd service
echo "==> Installing systemd service..."
cp "$SCRIPT_DIR/stremio-subdivx.service" /etc/systemd/system/
systemctl daemon-reload

# 7. Enable and start
systemctl enable stremio-subdivx
systemctl start stremio-subdivx

echo ""
echo "==> Waiting for tunnel URL..."
sleep 15

TUNNEL_URL=$(journalctl -u stremio-subdivx --no-pager | grep -o 'https://[a-z0-9-]*\.trycloudflare\.com' | tail -1 || true)

echo ""
echo "==> Installation complete!"
echo ""
if [ -n "$TUNNEL_URL" ]; then
  echo "Your addon URL: $TUNNEL_URL"
else
  echo "To get your HTTPS URL, run:"
  echo "  sudo journalctl -u stremio-subdivx -f"
  echo "  Look for: https://xxx.trycloudflare.com"
fi
echo ""
echo "Paste that URL in Stremio to install the addon."
