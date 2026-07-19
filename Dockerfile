# syntax=docker/dockerfile:1
#
# Single-image build for pocket-money (V3-6.1). Build context = repo ROOT so
# the Go build can embed the Expo web export produced in stage 1.
#   docker build -t pocket-money .

# ── Stage 1: export the Expo web bundle ───────────────────────────────
FROM node:22-alpine AS web
WORKDIR /app
# Deps first for layer caching.
COPY app/package.json app/package-lock.json ./
RUN npm ci
COPY app/ ./
# RELATIVE api base → same-origin, CORS moot. Baked into the JS bundle at export.
ENV EXPO_PUBLIC_API_URL=/api/v1
ENV NODE_ENV=production
# --clear busts any stale Metro transform cache so EXPO_PUBLIC_API_URL is always
# re-inlined (a cached transform could otherwise bake a wrong/empty API base).
RUN npx expo export --platform web --clear
# Output: /app/dist  (index.html + _expo/… + assets/…)

# ── Stage 2: build the Go binary with embedded dist + migrations ──────
FROM golang:1.24-alpine AS build
RUN apk add --no-cache git
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
# Drop the committed stub, then embed the real web export in its place.
RUN rm -rf internal/web/dist
COPY --from=web /app/dist ./internal/web/dist
# Embeds internal/web/dist (web SPA) + migrations/*.sql (iofs) into /server.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /server ./cmd/server

# ── Stage 3: slim runtime — a single static binary + certs ────────────
FROM alpine:3.19
RUN apk --no-cache add ca-certificates tzdata wget
WORKDIR /app
COPY --from=build /server .
RUN adduser -D -g '' appuser
USER appuser
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1
CMD ["./server"]
