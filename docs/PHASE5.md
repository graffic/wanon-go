# Phase 5: Application Bootstrap

This phase implements the application entry point and deployment infrastructure for the Wanon Telegram bot.

## Files Created

### cmd/wanon/main.go
The main application entry point that:
- Parses CLI commands (`migrate`, `server`, or default)
- Loads configuration from files and environment variables
- Manages concurrent components using `errgroup`
- Handles graceful shutdown on SIGTERM/SIGINT signals
- Runs database migrations before starting the server

**Components managed by errgroup:**
1. **Updates Poller** - Polls Telegram Bot API for new messages
2. **Dispatcher** - Routes commands to appropriate handlers
3. **Cache Cleaner** - Periodically cleans old cache entries

### Dockerfile
Multi-stage Docker build:
- **Build stage**: Uses `golang:1.21-alpine` to compile the binary
- **Runtime stage**: Uses `alpine:latest` for a minimal production image
- Runs as non-root user (`wanon:wanon`)
- Includes health check and proper signal handling

### docker-compose.yml
Local development setup with:
- PostgreSQL 15 (Alpine) with persistent volume
- App service with environment variable configuration
- Health checks and service dependencies

### scripts/entrypoint.sh
Docker entrypoint script that:
- Handles `migrate` and `server` commands
- Defaults to running migrations then starting the server
- Passes through other commands to the binary

### config/development.yaml & config/production.yaml
Sample configuration files for different environments.

## Usage

### Local Development

```bash
# Start all services
docker-compose up

# Run migrations only
docker-compose run app migrate

# Start server only (assuming migrations are done)
docker-compose run app server
```

### Production Deployment

```bash
# Build the Docker image
docker build -t wanon:latest .

# Run migrations
docker run --rm -e WANON_DATABASE_HOST=... wanon:latest migrate

# Start server
docker run -d -e WANON_TELEGRAM_TOKEN=... wanon:latest server
```

### CLI Commands

```bash
# Build the binary
go build -o wanon ./cmd/wanon

# Run migrations
./wanon migrate

# Start server
./wanon server

# Default: migrate + server
./wanon
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ENV` | Environment name (development/production) | `development` |
| `WANON_TELEGRAM_TOKEN` | Telegram Bot API token | *required* |
| `WANON_DATABASE_HOST` | PostgreSQL host | `localhost` |
| `WANON_DATABASE_PORT` | PostgreSQL port | `5432` |
| `WANON_DATABASE_USER` | PostgreSQL user | `wanon` |
| `WANON_DATABASE_PASSWORD` | PostgreSQL password | `wanon` |
| `WANON_DATABASE_DATABASE` | PostgreSQL database name | `wanon` |
| `WANON_DATABASE_SSLMODE` | PostgreSQL SSL mode | `disable` |
| `WANON_CACHE_CLEAN_INTERVAL` | Cache cleanup interval | `10m` |
| `WANON_CACHE_KEEP_DURATION` | Cache entry retention | `48h` |

## Graceful Shutdown

The application handles the following signals for graceful shutdown:
- `SIGINT` (Ctrl+C)
- `SIGTERM` (Docker stop, Kubernetes)

When a signal is received:
1. Context is cancelled
2. All components receive the cancellation signal
3. Components finish their current work and exit
4. Application exits cleanly
