# Wanon - Telegram Quote Bot

[![CI](https://github.com/graffic/wanon-go/actions/workflows/ci.yml/badge.svg)](https://github.com/graffic/wanon-go/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/graffic/wanon-go/branch/main/graph/badge.svg)](https://codecov.io/gh/graffic/wanon-go)
[![Go Report Card](https://goreportcard.com/badge/github.com/graffic/wanon-go)](https://goreportcard.com/report/github.com/graffic/wanon-go)

Wanon is a Telegram bot that allows users to save and retrieve quotes from group chats. It's a Go port of the original Elixir implementation.

## Features

- **Quote Storage**: Save memorable messages with `/addquote`
- **Random Quotes**: Retrieve random quotes with `/rquote`
- **Message Caching**: Automatically caches messages for building quote threads
- **Reply Chains**: Supports multi-message quote threads via reply chains
- **Periodic Cleanup**: Automatically cleans old cache entries
- **Chat Whitelist**: Restrict bot to specific chats

## Installation

### Using Go Install

```bash
go install github.com/graffic/wanon-go/cmd/wanon@latest
```

### From Source

```bash
git clone https://github.com/graffic/wanon-go.git
cd wanon-go
go build -o wanon ./cmd/wanon
```

### Using Docker

```bash
docker pull graffic/wanon:latest
```

## Configuration

Wanon can be configured using environment variables or YAML configuration files.

### Environment Variables

| Variable | Description | Required | Default |
|----------|-------------|----------|---------|
| `WANON_TELEGRAM_TOKEN` | Telegram Bot API token | Yes | - |
| `WANON_TELEGRAM_WEBHOOK` | Webhook URL (optional) | No | - |
| `WANON_DATABASE_HOST` | PostgreSQL host | No | `localhost` |
| `WANON_DATABASE_PORT` | PostgreSQL port | No | `5432` |
| `WANON_DATABASE_USER` | PostgreSQL user | No | `wanon` |
| `WANON_DATABASE_PASSWORD` | PostgreSQL password | No | `wanon` |
| `WANON_DATABASE_DATABASE` | PostgreSQL database name | No | `wanon` |
| `WANON_DATABASE_SSLMODE` | PostgreSQL SSL mode | No | `disable` |
| `WANON_CACHE_MAX_AGE` | Cache retention in seconds | No | `86400` (24h) |
| `WANON_ALLOWED_CHAT_IDS` | Comma-separated list of allowed chat IDs | Yes | - |

### Configuration Files

Create a configuration file in `config/` directory:

**config/production.yaml:**
```yaml
environment: production
telegram:
  token: ${WANON_TELEGRAM_TOKEN}
database:
  host: localhost
  port: 5432
  user: wanon
  password: wanon
  database: wanon
cache:
  max_age: 86400
```

## Development Setup

### Prerequisites

- Go 1.21 or later
- PostgreSQL 14+
- Docker and Docker Compose (optional)

### Using Docker Compose

```bash
# Start PostgreSQL
docker-compose up -d postgres

# Run migrations
go run ./cmd/wanon migrate

# Run the bot
go run ./cmd/wanon server
```

### Local Development

1. **Install dependencies:**
   ```bash
   go mod download
   ```

2. **Set up the database:**
   ```bash
   createdb wanon_development
   go run ./cmd/wanon migrate
   ```

3. **Set environment variables:**
   ```bash
   export WANON_TELEGRAM_TOKEN="your-bot-token"
   export WANON_ALLOWED_CHAT_IDS="-1001234567890"
   export WANON_DATABASE_DATABASE="wanon_development"
   ```

4. **Run the bot:**
   ```bash
   go run ./cmd/wanon
   ```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run tests with race detection
go test -race ./...
```

### Test Database Setup

Tests require a PostgreSQL database. By default, tests use:

```bash
export TEST_DB_HOST=localhost
export TEST_DB_PORT=5432
export TEST_DB_USER=wanon_test
export TEST_DB_PASSWORD=wanon_test
export TEST_DB_NAME=wanon_test
```

Create the test database:
```bash
createdb wanon_test
```

## Usage

### Bot Commands

| Command | Description |
|---------|-------------|
| `/addquote` | Reply to a message to save it as a quote |
| `/rquote` | Get a random quote from the chat |

### Example Usage

1. **Adding a quote:**
   - Reply to any message with `/addquote`
   - The bot saves the message and any messages in the reply chain

2. **Getting a random quote:**
   - Send `/rquote` in the chat
   - The bot sends a random previously saved quote

## Architecture

```
wanon-go/
├── cmd/wanon/           # Application entry point
├── internal/
│   ├── bot/            # Telegram bot logic
│   │   ├── bot.go      # Bot client and dispatcher
│   │   └── bot_test.go # Bot tests
│   ├── cache/          # Message caching system
│   │   ├── cache.go    # Cache operations
│   │   └── *_test.go   # Cache tests
│   ├── quotes/         # Quote management
│   │   ├── quotes.go   # Quote operations
│   │   └── *_test.go   # Quote tests
│   ├── telegram/       # Telegram API client
│   ├── config/         # Configuration management
│   └── storage/        # Database and migrations
├── testdata/           # Test fixtures
├── docker-compose.yml  # Docker Compose configuration
├── Dockerfile          # Docker image definition
└── README.md           # This file
```

## Deployment

### Docker Deployment

```bash
# Build image
docker build -t wanon:latest .

# Run with environment variables
docker run -d \
  -e WANON_TELEGRAM_TOKEN="your-token" \
  -e WANON_DATABASE_HOST="postgres" \
  -e WANON_ALLOWED_CHAT_IDS="-1001234567890" \
  wanon:latest
```

### Docker Compose

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f wanon

# Stop services
docker-compose down
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- Original Elixir implementation by [graffic](https://github.com/graffic)
- [Telegram Bot API](https://core.telegram.org/bots/api)
- [GORM](https://gorm.io/) - The fantastic ORM library for Golang
