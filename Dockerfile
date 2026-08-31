# Build Stage
FROM golang:alpine AS builder

ENV GOTOOLCHAIN=auto

WORKDIR /app

# Install build dependencies & certificates
RUN apk add --no-cache ca-certificates tzdata git

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Sync dependencies and build statically linked binary
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/remind-bot .

# Final Production Image
FROM alpine:3.21

WORKDIR /app

# Install timezone database and TLS certificates
RUN apk add --no-cache ca-certificates tzdata

# Create data directory for persistent SQLite storage
RUN mkdir -p /data

# Copy compiled binary from builder
COPY --from=builder /app/remind-bot /app/remind-bot

# Set default runtime environment
ENV DB_PATH=/data/bot.db
ENV TIMEZONE=Asia/Jakarta
ENV LOG_LEVEL=INFO

# Expose HTTP port for Render / Docker health checks
EXPOSE 8080 10000

ENTRYPOINT ["/app/remind-bot"]
