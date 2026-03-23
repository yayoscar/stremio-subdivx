#!/bin/bash
set -e

# Start FlareSolverr in background
python -u /app/flaresolverr.py &
FLARESOLVERR_PID=$!

# Wait for FlareSolverr to be ready (up to 30s)
for i in $(seq 1 30); do
  if curl -s http://127.0.0.1:8191 >/dev/null 2>&1; then
    echo "FlareSolverr is ready"
    break
  fi
  sleep 1
done

# Start addon in foreground
exec /usr/local/bin/stremio-subdivx
