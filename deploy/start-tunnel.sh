#!/bin/bash
set -euo pipefail

# Start cloudflared in background and capture the tunnel URL
LOGFILE="/tmp/cloudflared-tunnel.log"
/usr/local/bin/cloudflared tunnel --url http://localhost:3593 --no-autoupdate 2>&1 | tee "$LOGFILE" &
TUNNEL_PID=$!

# Wait for the tunnel URL to appear (up to 30s)
TUNNEL_URL=""
for i in $(seq 1 30); do
  TUNNEL_URL=$(grep -o 'https://[a-z0-9-]*\.trycloudflare\.com' "$LOGFILE" 2>/dev/null | head -1 || true)
  if [ -n "$TUNNEL_URL" ]; then
    break
  fi
  sleep 1
done

if [ -z "$TUNNEL_URL" ]; then
  echo "ERROR: Could not obtain tunnel URL after 30s"
  kill $TUNNEL_PID 2>/dev/null || true
  exit 1
fi

echo "Tunnel URL: $TUNNEL_URL"

# Restart the addon container with the correct ADDON_HOST
docker stop stremio-subdivx 2>/dev/null || true
docker rm -f stremio-subdivx 2>/dev/null || true
docker run -d --rm --name stremio-subdivx \
  -p 3593:3593 \
  -e "ADDON_HOST=$TUNNEL_URL" \
  -v /srv/apps/stremio-subdivx/.cache:/app/.cache \
  stremio-subdivx-allinone

echo "Addon running with ADDON_HOST=$TUNNEL_URL"

# Keep the script alive (follow tunnel process)
wait $TUNNEL_PID
