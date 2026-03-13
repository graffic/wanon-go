# Agent Instructions


## Development Commands

This project uses **mise** for tool management. All Go commands should be run via mise:

```bash
# Build the application
mise exec -- go build ./cmd/wanon

# Run tests (non-database tests)
mise exec -- go test ./internal/config/... ./internal/telegram/...

# Run all tests (requires PostgreSQL)
mise exec -- go test --count=1 ./...

# Run tests with verbose output
mise exec -- go test --count=1 -v ./internal/telegram/...

# Run tests with coverage
mise exec -- go test -coverprofile=coverage.out ./...
mise exec -- go tool cover -html=coverage.out -o coverage.html

# Get dependencies
mise exec -- go mod download

# Tidy dependencies
mise exec -- go mod tidy

# Run linter (requires golangci-lint)
mise exec -- golangci-lint run ./...
```

### Database Setup for Tests

Tests in `internal/quotes/` and `internal/cache/` use **testcontainers-go** to automatically spin up PostgreSQL containers. No manual database setup is required!

The test helper in `internal/testutils/db.go` will:
1. Start a PostgreSQL container automatically
2. Run migrations
3. Clean up the container after tests complete

**Requirements:**
- Docker must be running
- No environment variables needed for database tests

**Legacy manual setup (if needed):**
```bash
# Only needed if you want to run tests against a manually managed database
export TEST_DB_HOST=localhost
export TEST_DB_PORT=5432
export TEST_DB_USER=wanon_test
export TEST_DB_PASSWORD=wanon_test
export TEST_DB_NAME=wanon_test
```

### Environment Variables

```bash
# Application configuration
export WANON_TELEGRAM_TOKEN=your_bot_token
export WANON_ALLOWED_CHAT_IDS=-1001234567890,-1009876543210
```

## Code notes
* Always run tests at the end of a tasks that modifies code
* Always review tests if they need to be updated or added.

