# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git

# Copy go mod files first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /aurum ./cmd/aurum

# Runtime stage
FROM alpine:3.21

WORKDIR /app

# Add ca-certificates for HTTPS and timezone data
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /aurum /app/aurum

# Run as non-root user
RUN adduser -D -g '' appuser
USER appuser

EXPOSE 8080

ENTRYPOINT ["/app/aurum"]
