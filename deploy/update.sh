#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

echo "==> Updating stremio-subdivx addon"

cd "$PROJECT_DIR"
git pull
make docker-build-allinone

echo "==> Restarting services..."
sudo systemctl restart stremio-subdivx

echo "==> Update complete!"
