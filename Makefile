# Run `make` to see the commands.
.DEFAULT_GOAL := help
.PHONY: help setup dev check test build image smoke run clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X github.com/selfhostit/troye/app.Version=$(VERSION)

help: ## Show the commands
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  make %-8s %s\n", $$1, $$2}'

setup: ## Install the Go and web dependencies
	cd backend && go mod download
	cd web && npm ci
	cd workers && uv sync

dev: ## Run the backend and the web app with live reload (open http://localhost:5173)
	@trap 'kill 0' INT TERM EXIT; \
	(cd backend && go tool air) & \
	(cd web && npm run dev) & \
	wait

check: ## Format, lint and type-check everything
	cd backend && test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	cd backend && go vet ./...
	cd web && npm run lint && npm run check
	cd workers && uv run ruff check . && uv run ruff format --check .

test: ## Run all tests
	cd backend && go test ./...
	cd web && npm test
	cd workers && uv run pytest -q

build: ## Build one binary with the web app next to it, in ./dist
	cd web && npm run build
	cd backend && CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)" -o ../dist/troye .
	rm -rf dist/pb_public && cp -r web/dist dist/pb_public

image: ## Build the app and worker Docker images
	docker build --build-arg VERSION=$(VERSION) -t troye:dev .
	docker build -t troye-worker:dev workers

smoke: image ## Deploy both images with Docker Compose and check them end to end
	TROYE_IMAGE=troye:dev TROYE_WORKER_IMAGE=troye-worker:dev EXPECT_VERSION=$(VERSION) deploy/smoke-test.sh

run: image ## Build and run the Docker image on http://localhost:8090
	docker run --rm -p 127.0.0.1:8090:8090 -v $(CURDIR)/pb_data:/app/pb_data troye:dev

clean: ## Remove build output (keeps your data in pb_data)
	rm -rf dist web/dist backend/tmp
