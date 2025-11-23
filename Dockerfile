# Build stage
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Run tests before building
RUN go test ./... -v

# Build the application
RUN go build -o bin/prompt-registry ./cmd/server

# Runtime stage
FROM alpine:latest

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/bin/prompt-registry .

# Copy web assets (if not embedded)
COPY --from=builder /app/web ./web

# Expose port (Render sets PORT env var)
EXPOSE 8080

# Run the application
CMD ["./prompt-registry"]
