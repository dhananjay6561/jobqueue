# ─── Stage 1: Frontend builder ───────────────────────────────────────────────
FROM node:22-alpine AS frontend-builder

WORKDIR /frontend

COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci

COPY frontend/ .
RUN npm run build

# ─── Stage 2: Go builder ─────────────────────────────────────────────────────
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -trimpath \
    -o /jobqueue \
    ./cmd/server

# ─── Stage 2: Final image ────────────────────────────────────────────────────
# Distroless gives us the smallest possible attack surface — no shell, no
# package manager, just the binary and TLS certificates.
FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /

# Copy the compiled binary from the builder stage.
COPY --from=builder /jobqueue /jobqueue

# Copy migrations so RunMigrations can find them at runtime.
COPY --from=builder /build/migrations /migrations

# Copy the built React frontend.
COPY --from=frontend-builder /frontend/dist /frontend/dist

# Copy TLS certificates for outgoing HTTPS calls.
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy timezone data.
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

# Run as non-root (uid 65532 = nonroot in distroless).
USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/jobqueue"]
