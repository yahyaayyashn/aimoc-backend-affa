# ============================================
# aimoc-be — Go (Fiber + GORM/PostgreSQL)
# ============================================

FROM golang:1.22-alpine AS builder
WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build binary — main package ada di cmd/api/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server ./cmd/api/

# ─── Runner ───────────────────────────────
FROM alpine:3.20 AS runner
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata wget
ENV TZ=Asia/Jakarta

COPY --from=builder /app/server .
COPY --from=builder /app/migrations ./migrations

RUN mkdir -p uploads

EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

CMD ["./server"]
