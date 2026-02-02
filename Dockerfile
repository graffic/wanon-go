# Build stage
FROM golang:1.25 AS builder

# Set working directory
WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o /build/wanon \
    ./cmd/wanon

RUN CGO_ENABLED=0 go install -v github.com/jackc/tern/v2@latest

# Runtime stage - distroless
FROM gcr.io/distroless/static-debian12

# Set working directory
WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/wanon /app/wanon
COPY --from=builder /go/bin/tern /usr/bin/tern

# Copy migrations
COPY migrations /app/migrations

# Expose port (if needed for health checks or metrics)
EXPOSE 8080

# Run the binary directly
ENTRYPOINT ["/app/wanon"]
