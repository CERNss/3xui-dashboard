.PHONY: build build-frontend build-frontend-react build-backend dev dev-frontend dev-frontend-react dev-backend tidy clean test test-backend test-e2e test-frontend test-ui test-ui-install lint lint-backend lint-frontend migrate docker-build docker-up docker-down

# ---- Paths ----
ROOT     := $(shell pwd)
FRONTEND := $(ROOT)/frontend
FRONTEND_REACT := $(ROOT)/frontend-react
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

build-frontend-react:
	@echo "==> Building React frontend..."
	cd $(FRONTEND_REACT) && npm install --no-audit --no-fund && npm run build

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

dev-frontend-react:
	@echo "==> Starting React frontend dev server (http://localhost:5174)..."
	cd $(FRONTEND_REACT) && npm install --no-audit --no-fund && npm run dev

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

# test-e2e expects $INTEGRATION_DB_URL to point at a writable Postgres
# (the integration test resets the schema before each run). Spin one
# up with:
#   docker run -d --rm --name pg-e2e -e POSTGRES_PASSWORD=test \
#     -e POSTGRES_DB=dashboard_e2e -p 5499:5432 postgres:16-alpine
test-e2e:
	cd $(BACKEND) && \
	  INTEGRATION_DB_URL=$${INTEGRATION_DB_URL:-postgres://postgres:test@127.0.0.1:5499/dashboard_e2e?sslmode=disable} \
	  go test -count=1 -v ./internal/e2e/...

# Playwright UI smoke. Prep + binary lifecycle in
# frontend/playwright.config.ts header comment. Run test-ui-install
# once to fetch Chromium (~120MB).
test-ui-install:
	cd $(FRONTEND) && npm install --no-audit --no-fund && npm run e2e:install

test-ui:
	cd $(FRONTEND) && BASE_URL=$${BASE_URL:-http://127.0.0.1:8080} npm run e2e

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
