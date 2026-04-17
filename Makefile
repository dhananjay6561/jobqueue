# Makefile for the jobqueue project.
# Run `make help` to list all available targets.

.PHONY: build test test-race lint fmt vet run run-race clean deps \
        docker-build docker-up docker-down docker-logs help

# ── Variables ────────────────────────────────────────────────────────────────
BINARY      := jobqueue
BUILD_DIR   := bin
CMD_DIR     := ./cmd/server
GO          := go
GOFLAGS     := -v

# ── Build ─────────────────────────────────────────────────────────────────────
## build: Compile the server binary into $(BUILD_DIR)/
build:
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -o $(BUILD_DIR)/$(BINARY) $(CMD_DIR)

## build-static: Produce a fully static binary suitable for distroless containers
build-static:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build \
		-ldflags="-w -s" -trimpath \
		-o $(BUILD_DIR)/$(BINARY)-linux-amd64 $(CMD_DIR)

# ── Run ───────────────────────────────────────────────────────────────────────
## run: Run the server with .env loaded (requires local postgres + redis)
run:
	$(GO) run $(CMD_DIR)

## run-race: Run with the race detector enabled
run-race:
	$(GO) run -race $(CMD_DIR)

# ── Test ──────────────────────────────────────────────────────────────────────
## test: Run all unit tests
test:
	$(GO) test ./... -count=1

## test-race: Run tests with the race detector
test-race:
	$(GO) test -race ./... -count=1

## test-cover: Run tests and generate an HTML coverage report
test-cover:
	$(GO) test -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

## test-integration: Run integration tests (requires running postgres + redis)
test-integration:
	$(GO) test -tags=integration -race ./... -count=1

# ── Code quality ──────────────────────────────────────────────────────────────
## vet: Run go vet
vet:
	$(GO) vet ./...

## lint: Run golangci-lint (install: brew install golangci-lint)
lint:
	golangci-lint run ./...

## fmt: Format all Go source files
fmt:
	$(GO) fmt ./...
	goimports -w . 2>/dev/null || true

# ── Dependencies ──────────────────────────────────────────────────────────────
## deps: Download and tidy module dependencies
deps:
	$(GO) mod download
	$(GO) mod tidy

# ── Docker ───────────────────────────────────────────────────────────────────
## docker-build: Build the Docker image
docker-build:
	docker build -t $(BINARY):latest .

## docker-up: Start all services with docker-compose
docker-up:
	docker compose up --build -d

## docker-down: Stop and remove all containers
docker-down:
	docker compose down

## docker-logs: Tail logs for all services
docker-logs:
	docker compose logs -f

## docker-ps: Show running containers
docker-ps:
	docker compose ps

# ── Cleanup ───────────────────────────────────────────────────────────────────
## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html

# ── Help ──────────────────────────────────────────────────────────────────────
## help: Print this help message
help:
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@grep -E '^## ' Makefile | sed 's/## /  /'
	@echo ""
