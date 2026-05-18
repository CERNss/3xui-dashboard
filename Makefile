.PHONY: build build-frontend build-backend dev dev-frontend dev-backend tidy clean test test-backend test-frontend lint lint-backend lint-frontend migrate docker-build docker-up docker-down

# ---- Paths ----
ROOT     := $(shell pwd)
FRONTEND := $(ROOT)/frontend
BACKEND  := $(ROOT)/backend
BIN      := $(ROOT)/3xui-dashboard

# ============================================================================
# Build
# ============================================================================

# Builds frontend and backend; embedded SPA goes into the binary.
build: build-frontend build-backend

build-frontend:
	@echo "==> Building frontend..."
	cd $(FRONTEND) && npm install --no-audit --no-fund && npm run build

build-backend:
	@echo "==> Building backend..."
	cd $(BACKEND) && go build -o $(BIN) ./cmd/dashboard
	@echo "==> Binary -> $(BIN)"

# ============================================================================
# Development
# ============================================================================

dev-frontend:
	@echo "==> Starting frontend dev server (http://localhost:5173)..."
	cd $(FRONTEND) && npm install --no-audit --no-fund && npm run dev

dev-backend:
	@echo "==> Starting backend (http://localhost:8080)..."
	cd $(BACKEND) && go run ./cmd/dashboard -env $(ROOT)/deploy/.env

# Run both concurrently (requires make -j2).
dev:
	$(MAKE) -j2 dev-frontend dev-backend

# ============================================================================
# Test / Lint
# ============================================================================

test: test-backend test-frontend

test-backend:
	cd $(BACKEND) && go test ./...

test-frontend:
	cd $(FRONTEND) && npm run typecheck

lint: lint-backend lint-frontend

lint-backend:
	cd $(BACKEND) && go vet ./...

lint-frontend:
	cd $(FRONTEND) && npm run lint

# ============================================================================
# Migrations
# ============================================================================
#
# The binary runs migrations on boot unless DB_MIGRATE_ON_BOOT=false.
# This target launches the binary in migrate-and-exit mode by setting
# the listen address to a closed port and timing out — useful for CI
# pipelines that want migrations applied as a separate step.
migrate:
	@echo "==> Running migrations (binary boot path, will exit after migrate)..."
	cd $(BACKEND) && DB_MIGRATE_ON_BOOT=true go run ./cmd/dashboard -env $(ROOT)/deploy/.env || true

# ============================================================================
# Docker
# ============================================================================

docker-build:
	docker build -t 3xui-dashboard:dev -f deploy/Dockerfile .

docker-up:
	docker compose -f deploy/docker-compose.yml up -d

docker-down:
	docker compose -f deploy/docker-compose.yml down

# ============================================================================
# Cleanup
# ============================================================================

tidy:
	cd $(BACKEND) && go mod tidy

clean:
	rm -f $(BIN)
	# Preserve .gitkeep so //go:embed compiles before the next build.
	find $(BACKEND)/internal/web/dist -mindepth 1 ! -name '.gitkeep' -delete 2>/dev/null || true
	rm -rf $(FRONTEND)/dist
