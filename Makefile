APP = stremio-subdivx

test:
	test -z "$(shell gofmt -l .)"
	go vet ./...
	go install honnef.co/go/tools/cmd/staticcheck@latest
	staticcheck ./...
	go test -timeout 10s -race ./...

run:
	cd frontend && npm run build:dev
	go run cmd/addon/*

build:
	mkdir -p frontend/dist && touch frontend/dist/index.html
	go build -o .bin/$(APP) cmd/addon/*

docker-build:
	@docker build . --tag $(APP)

docker-run: docker-build
	docker network inspect subdivx-net >/dev/null 2>&1 || docker network create subdivx-net
	-docker rm -f flaresolverr 2>/dev/null
	docker run -d --rm --name flaresolverr --network subdivx-net \
		-p 8191:8191 \
		ghcr.io/flaresolverr/flaresolverr:latest
	docker run --rm --name $(APP) --network subdivx-net \
		-e SERVICE_ENVIRONMENT='dk' \
		-e FLARESOLVERR_URL='http://flaresolverr:8191' \
		-p 3593:3593 \
		-v "./.cache:/app/.cache" \
		$(APP)

docker-build-allinone:
	@DOCKER_BUILDKIT=1 docker build -f Dockerfile.allinone . --tag $(APP)-allinone

docker-run-allinone: docker-build-allinone
	docker run --rm --name $(APP)-allinone \
		-e SERVICE_ENVIRONMENT='dk' \
		-p 3593:3593 \
		-v "./.cache:/app/.cache" \
		$(APP)-allinone