# syntax=docker/dockerfile:1.7

# =============================================================================
# Stage 1 — build the Vue SPA. The build output (../backend/internal/web/dist)
#          is consumed by stage 2 via //go:embed.
# =============================================================================
FROM node:22-alpine AS frontend
WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY frontend/ ./
# vite outputs to ../backend/internal/web/dist; create the path on disk first.
RUN mkdir -p ../backend/internal/web/dist
RUN npm run build

# =============================================================================
# Stage 2 — build the Go binary. Embeds the bundle produced by stage 1.
# =============================================================================
FROM golang:1.26-alpine AS backend
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# Pull in the frontend assets from stage 1 into the embed path.
COPY --from=frontend /src/backend/internal/web/dist ./internal/web/dist
ENV CGO_ENABLED=0
RUN go build -trimpath -ldflags="-s -w" -o /out/dashboard ./cmd/dashboard

# =============================================================================
# Stage 3 — slim runtime. Statically linked binary + ca-certificates so HTTPS
#          calls to 3x-ui panels work out of the box.
# =============================================================================
FROM alpine:3.20 AS runtime
RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -u 10001 -g 'dashboard' dashboard
USER dashboard
WORKDIR /app
COPY --from=backend /out/dashboard /usr/local/bin/dashboard
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/dashboard"]
