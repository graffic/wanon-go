# Agent Instructions

This project uses **bd** (beads) for issue tracking. Run `bd onboard` to get started.

## Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --status in_progress  # Claim work
bd close <id>         # Complete work
bd sync               # Sync with git
```

## Development Commands

This project uses **mise** for tool management. All Go commands should be run via mise:

```bash
# Build the application
mise exec -- go build ./cmd/wanon

# Run tests (non-database tests)
mise exec -- go test ./internal/config/... ./internal/telegram/...

# Run all tests (requires PostgreSQL)
mise exec -- go test ./...

# Run tests with verbose output
mise exec -- go test -v ./internal/telegram/...

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

## Landing the Plane (Session Completion)

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd sync
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds

