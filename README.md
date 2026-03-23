# stremio-subdivx

## Description

Stremio addon for getting subtitles from Subdivx.

### Website & Install

https://stremio-subdivx.xor.ar/

## Configuration

The following environment variables can be used to configure the addon:

*   `ADDON_HOST`: Public URL where the addon is accessible (default: `http://127.0.0.1:3593`)
*   `SERVER_LISTEN_ADDR`: Network address the HTTP server listens on (default: `:3593`)
*   `FLARESOLVERR_URL`: URL of a FlareSolverr instance for Cloudflare bypass (default: `http://127.0.0.1:8191`)
*   `OTEL_ENABLED`: Enable OpenTelemetry instrumentation (default: `false`)
*   `OTEL_EXPORTER_ENDPOINT`: OpenTelemetry gRPC endpoint (default: `127.0.0.1:4317`)
*   `LOKI_HOST`: Loki URL for stats polling (default: empty, disabled)

## Build

```bash
make build
```

## Run

```bash
make run
```

## Docker

### All-in-one image (recommended)

Bundles the addon and FlareSolverr in a single container:

```bash
make docker-build-allinone
make docker-run-allinone
```

### Separate containers

Runs the addon and FlareSolverr as separate containers:

```bash
make docker-build
make docker-run
```

## Deploy

### Option 1: Any Docker platform (Koyeb, Render, Railway, etc.)

The all-in-one image runs everything in a single container — works on any platform that supports Docker with just one service:

```bash
docker build -f Dockerfile.allinone -t stremio-subdivx .
```

Then push to your platform's registry and deploy with:
*   **Port:** 3593
*   **Min RAM:** 512MB
*   **Env var:** `ADDON_HOST=https://your-app-url`

### Option 2: Fly.io (separate containers)

Install the [Fly CLI](https://fly.io/docs/flyctl/install/) and authenticate:

```bash
fly auth login
```

Deploy FlareSolverr (internal only):

```bash
fly apps create stremio-subdivx-flaresolverr
fly deploy --config fly.flaresolverr.toml
```

Deploy the addon:

```bash
fly apps create stremio-subdivx
fly volumes create subdivx_cache --region eze --size 1 --app stremio-subdivx
fly deploy
fly secrets set ADDON_HOST=https://stremio-subdivx.fly.dev --app stremio-subdivx
```
