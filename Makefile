.PHONY: build dev-frontend dev-backend tidy clean

# ---- Paths ----
ROOT     := $(shell pwd)
FRONTEND := $(ROOT)/frontend
BACKEND  := $(ROOT)/backend

# ---- Build both frontend and backend ----
build: build-frontend build-backend

build-frontend:
	@echo "==> Building frontend..."
	cd $(FRONTEND) && npm install && npm run build
	@echo "==> Frontend build complete -> backend/internal/web/dist"

build-backend:
	@echo "==> Building backend..."
	cd $(BACKEND) && go build -o $(ROOT)/3xui-dashboard ./...
	@echo "==> Backend binary -> $(ROOT)/3xui-dashboard"

# ---- Development ----
dev-frontend:
	@echo "==> Starting frontend dev server (http://localhost:5173)..."
	cd $(FRONTEND) && npm install && npm run dev

dev-backend:
	@echo "==> Starting backend dev server (http://localhost:9090)..."
	cd $(BACKEND) && go run ./main.go

# ---- Dependency management ----
tidy:
	cd $(BACKEND) && go mod tidy

install-frontend:
	cd $(FRONTEND) && npm install

# ---- Cleanup ----
clean:
	rm -rf $(ROOT)/3xui-dashboard
	rm -rf $(BACKEND)/internal/web/dist
	rm -rf $(BACKEND)/data

# ---- Run both concurrently (requires make -j2) ----
dev:
	$(MAKE) -j2 dev-frontend dev-backend
