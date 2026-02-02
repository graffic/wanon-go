# Wanon: Elixir → Go Migration Plan

## Project Overview

**Wanon** is a Telegram quote bot that:
- Polls Telegram Bot API for updates
- Stores chat messages in a cache (for quote thread building)
- Saves quotes to PostgreSQL
- Supports commands: `/addquote`, `/rquote`
- Periodically cleans old cache entries

---

## Architecture Comparison

| Component | Elixir (Source) | Go (Target) |
|-----------|----------------|-------------|
| **Concurrency** | GenStage + OTP Supervisors | Goroutines + Channels + errgroup |
| **Database** | Ecto (ORM) | GORM or sqlx |
| **HTTP Client** | HTTPoison | net/http |
| **JSON** | Poison/Jason | encoding/json |
| **Scheduling** | GenServer.send_after | time.Ticker |
| **Configuration** | Mix config | envconfig / koanf |
| **CLI/Release** | Distillery | Go native binary |

---

## Directory Structure (Go)

```
wanon-go/
├── cmd/
│   └── wanon/
│       └── main.go              # Application entry point
├── internal/
│   ├── bot/
│   │   ├── bot.go               # Main bot orchestrator
│   │   ├── commands.go          # Command interface & registry
│   │   ├── dispatcher.go        # Update dispatcher
│   │   └── updates.go           # Telegram polling worker
│   ├── cache/
│   │   ├── cache.go             # Cache service interface
│   │   ├── add.go               # Add message to cache
│   │   ├── edit.go              # Edit cached message
│   │   └── clean.go             # Periodic cache cleanup
│   ├── quotes/
│   │   ├── models.go            # Quote & QuoteEntry structs
│   │   ├── builder.go           # Build quote from cache
│   │   ├── store.go             # Store quote to DB
│   │   ├── render.go            # Render quote as text
│   │   ├── addquote.go          # /addquote command
│   │   └── rquote.go            # /rquote command
│   ├── telegram/
│   │   ├── client.go            # Telegram client interface
│   │   └── http.go              # HTTP implementation
│   ├── config/
│   │   └── config.go            # Configuration struct & loading
│   └── storage/
│       ├── db.go                # Database connection
│       └── migrations/          # SQL migration files
├── docker-compose.yml           # For local development
├── Dockerfile                   # Multi-stage build
├── go.mod
└── README.md
```

---

## Phase 1: Foundation (Day 1-2)

| Task | Description |
|------|-------------|
| 1.1 Initialize Go module | `go mod init github.com/graffic/wanon-go` |
| 1.2 Setup configuration | Use `koanf` for env-based config (matches Elixir's Mix.Config) |
| 1.3 Database models | Define `Quote`, `QuoteEntry`, `CacheEntry` structs |
| 1.4 Migrations | Create SQL files matching Elixir's Ecto migrations |
| 1.5 Telegram client interface | Define Go interface matching `Wanon.Telegram.Client` |

---

## Phase 2: Telegram Integration (Day 2-3)

| Task | Description |
|------|-------------|
| 2.1 HTTP client | Implement `telegram.HTTP` using `net/http` + `encoding/json` |
| 2.2 Update polling | Replace `GenStage` producer with goroutine + channel |
| 2.3 Update dispatcher | Replace `GenStage` consumer with goroutine reading from channel |
| 2.4 Chat filtering | Port whitelist filter from `Dispatcher.filter_chat` |

### Key Translation: GenStage Producer → Go Channel

**Elixir:**
```elixir
def init(:ok) do
  {:producer, {0, 0}, dispatcher: GenStage.BroadcastDispatcher}
end

def handle_demand(demand, {offset, pending_demand}) when demand > 0 do
  produce_updates(offset, demand + pending_demand)
end
```

**Go:**
```go
func (u *Updates) Start(ctx context.Context) error {
    offset := 0
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case demand := <-u.demandCh:
            updates, err := u.client.GetUpdates(ctx, offset)
            if err != nil {
                log.Printf("get updates error: %v", err)
                continue
            }
            u.outCh <- updates
            offset = lastOffset(updates) + 1
        }
    }
}
```

---

## Phase 3: Cache System (Day 3-4)

| Task | Description |
|------|-------------|
| 3.1 CacheEntry model | Port Ecto schema to GORM model |
| 3.2 Cache.Add | Port `Cache.Add.execute` - store incoming messages |
| 3.3 Cache.Edit | Port `Cache.Edit.execute` - update edited messages |
| 3.4 Cache.Clean | Replace `GenServer` with `time.Ticker` |

### Key Translation: GenServer Scheduling → time.Ticker

**Elixir:**
```elixir
def init(state) do
  schedule(state.every)
  {:ok, state}
end

def handle_info(:clean, state) do
  clean_cache()
  schedule(state.every)
  {:noreply, state}
end

defp schedule(every) do
  Process.send_after(self(), :clean, every)
end
```

**Go:**
```go
func (c *Cleaner) Start(ctx context.Context) error {
    ticker := time.NewTicker(c.every)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        case <-ticker.C:
            if err := c.clean(ctx); err != nil {
                log.Printf("cache clean error: %v", err)
            }
        }
    }
}
```

### CacheEntry Schema Translation

**Elixir (Ecto):**
```elixir
schema "cache_entry" do
  field(:chat_id, :integer)
  field(:message_id, :integer)
  field(:reply_id, :integer, default: nil)
  field(:date, :integer)
  field(:message, :map)
end
```

**Go (GORM):**
```go
type CacheEntry struct {
    ID        uint `gorm:"primarykey"`
    ChatID    int64
    MessageID int64
    ReplyID   *int64
    Date      int64
    Message   datatypes.JSON `gorm:"type:jsonb"`
}
```

---

## Phase 4: Quotes System (Day 4-5)

| Task | Description |
|------|-------------|
| 4.1 Quote models | Port `Quote`, `QuoteEntry` Ecto schemas |
| 4.2 Builder | Port `Quotes.Builder.build_from` with recursive query |
| 4.3 Store | Port `Quotes.Store.store` |
| 4.4 Render | Port `Quotes.Render.render` |
| 4.5 AddQuote command | Port `/addquote` command handler |
| 4.6 RQuote command | Port `/rquote` command handler |

### Quote Schema Translation

**Elixir:**
```elixir
schema "quote" do
  field(:creator, :map)
  field(:chat_id, :integer)
  has_many(:entries, Wanon.Quotes.QuoteEntry)
  timestamps(updated_at: false)
end

schema "quote_entry" do
  field(:order, :integer)
  field(:message, :map)
  belongs_to(:quote, Wanon.Quotes.Quote)
end
```

**Go:**
```go
type Quote struct {
    ID        uint      `gorm:"primarykey"`
    Creator   datatypes.JSON
    ChatID    int64
    Entries   []QuoteEntry
    CreatedAt time.Time
}

type QuoteEntry struct {
    ID        uint           `gorm:"primarykey"`
    Order     int
    Message   datatypes.JSON `gorm:"type:jsonb"`
    QuoteID   uint
}
```

---

## Phase 5: Application Bootstrap (Day 5)

| Task | Description |
|------|-------------|
| 5.1 Main function | Replace `Application.start` with `main()` |
| 5.2 Graceful shutdown | Handle SIGTERM/SIGINT, cancel context |
| 5.3 Docker | Port multi-stage Dockerfile |
| 5.4 Docker Compose | Port `docker-compose.yml` for local dev |

### Key Translation: OTP Supervisor → errgroup

**Elixir:**
```elixir
def start(_type, _args) do
  children = [
    worker(Wanon.Repo, []),
    worker(Wanon.Telegram.Updates, []),
    worker(Wanon.Dispatcher, []),
    worker(Wanon.Cache.Clean, [])
  ]
  opts = [strategy: :one_for_one, name: Wanon.Supervisor]
  Supervisor.start_link(children, opts)
end
```

**Go:**
```go
func main() {
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()
    
    cfg := config.Load()
    db := storage.NewDB(cfg.Database)
    telegram := telegram.NewHTTPClient(cfg.Telegram)
    
    g, ctx := errgroup.WithContext(ctx)
    
    g.Go(func() error { return db.Start(ctx) })
    g.Go(func() error { 
        updates := bot.NewUpdates(db, telegram)
        return updates.Start(ctx) 
    })
    g.Go(func() error { 
        dispatcher := bot.NewDispatcher(db, telegram)
        return dispatcher.Start(ctx) 
    })
    g.Go(func() error { 
        cleaner := cache.NewCleaner(db, cfg.Cache)
        return cleaner.Start(ctx) 
    })
    
    if err := g.Wait(); err != nil {
        log.Fatalf("application error: %v", err)
    }
}
```

---

## Phase 6: Testing & Deployment (Day 6-7)

| Task | Description |
|------|-------------|
| 6.1 Unit tests | Port existing tests (Mox → go mock or testify) |
| 6.2 Integration tests | Port integration tests |
| 6.3 CI/CD | Port `.travis.yml` or use GitHub Actions |
| 6.4 Documentation | Update README with Go instructions |

---

## Missing Areas Found During Verification

After thorough review of the Elixir source code, the following areas were identified as missing or needing additional detail in the migration:

### 1. Database Migration Runner

**Source:** `lib/wanon/migrations.ex`

The Elixir project has a custom migration runner for distillery releases that runs migrations before starting the app. In Go, we can use:
- **golang-migrate/migrate** - Popular migration tool
- **GORM AutoMigrate** - For development only, not recommended for production

**Go Implementation:**
```go
// cmd/migrate/main.go or embedded in main app
import "github.com/golang-migrate/migrate/v4"

func RunMigrations(dbURL string) error {
    m, err := migrate.New(
        "file://internal/storage/migrations",
        dbURL)
    if err != nil {
        return err
    }
    return m.Up()
}
```

**Action:** Add migration tool to Phase 1.

---

### 2. getMe API Endpoint

**Source:** `lib/wanon/telegram/client.ex`

The Telegram client behaviour includes `get_me()` callback, but it's not used in the current codebase. However, it should be implemented for completeness.

**Action:** Add to Phase 2 as optional/low priority.

---

### 3. Quotes.Consumer (Unused Component)

**Source:** `lib/wanon/quotes/consumer.ex`

An unused GenStage producer-consumer that was likely intended for a different architecture. It references `Wanon.Telegram.GetUpdates` (which doesn't exist - should be `Updates`).

**Decision:** Skip this component as it's not used in production.

---

### 4. Test Infrastructure

#### 4.1 Mox Mocking → Go Mock

**Source:** `test/support/mocks.ex`

Elixir uses Mox for mocking. In Go, options include:
- **gomock** - Official Go mocking framework
- **testify/mock** - Part of testify package
- **Manual interfaces** - Most idiomatic Go approach

**Recommended:** Use manual interfaces for unit tests, httptest for HTTP mocking.

#### 4.2 Ecto SQL Sandbox → Test Transactions

**Source:** `test/test_helper.exs`

Elixir uses `Ecto.Adapters.SQL.Sandbox` for test isolation. In Go with GORM:
```go
func setupTestDB(t *testing.T) *gorm.DB {
    db := testutils.OpenTestDB()
    t.Cleanup(func() {
        // Clean up test data or use transactions
        db.Exec("TRUNCATE cache_entries, quotes, quote_entries CASCADE")
    })
    return db
}
```

#### 4.3 Integration Test Server

**Source:** `integration/support/telegram_api.ex`

Custom Plug-based mock server for integration tests. In Go:
```go
func setupMockTelegramAPI(t *testing.T) *httptest.Server {
    handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Return fixtures based on request path
    })
    return httptest.NewServer(handler)
}
```

**Action:** Add detailed test infrastructure to Phase 6.

---

### 5. CI/CD Pipeline Details

**Source:** `.old_travis.yml`

The Travis CI pipeline has three stages:
1. **Test** - Runs unit and integration tests with coverage
2. **Docker** - Builds and pushes Docker image on master
3. **Deploy** - Deploys to production using docker stack

**GitHub Actions equivalent needed:**
```yaml
# .github/workflows/ci.yml
name: CI
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:alpine
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
      - run: go test -coverprofile=coverage.out ./...
      - uses: codecov/codecov-action@v3
```

**Action:** Add CI/CD template to Phase 6.

---

### 6. Migration Script (migrate_n_run)

**Source:** `rel/commands/migrate_n_run.sh`

Distillery custom command that runs migrations then starts the app. For Go:
```dockerfile
# Dockerfile ENTRYPOINT script
#!/bin/sh
set -e
/wanon migrate  # Run migrations
exec /wanon server  # Start server
```

Or use a Go-based approach with subcommands:
```go
// cmd/wanon/main.go
func main() {
    switch os.Args[1] {
    case "migrate":
        runMigrations()
    case "server":
        runServer()
    default:
        runMigrations() // Auto-migrate on start
        runServer()
    }
}
```

**Action:** Add to Phase 5.

---

### 7. Test Fixtures

**Source:** `integration/fixture.*.json`, `integration/sendMessage.response.json`

JSON fixtures used in integration tests need to be ported to Go testdata:
```
testdata/
├── fixture.1.json
├── fixture.2.edit.json
└── sendMessage.response.json
```

**Action:** Copy fixtures to Phase 6.

---

### 8. Specific Test Cases to Port

#### Cache Clean Tests
- Deletes old cache entries based on timestamp
- Schedules next cleaning cycle correctly
- Initializes with correct parameters

#### Builder Tests
- Build without cache entries (fallback to backup)
- One entry in cache
- Multiple entries (reply chain)
- Missing entries in chain (partial cache hit)

#### Store Tests
- Stores quote entries with correct order (0, 1, 2...)

#### Edit Tests
- Select edit messages (edited_message field)
- Edit existing message in cache
- Edit missing message (no-op)

#### AddQuote Tests
- Selector filters unrecognized messages
- Selector accepts `/addquote`
- Add quote without reply (error message)
- Adds quote with reply (success message)

#### RQuote Tests
- Selector filters wrong messages
- Selector accepts `/rquote` (case insensitive)
- Handle rquote with no quotes (empty message)
- Get one quote (random selection)

**Action:** Ensure all test cases are covered in Phase 6.

---

### 9. Configuration Per Environment

**Source:** `config/dev.exs`, `config/test.exs`, `config/prod.exs`

Different config files per environment. In Go, use env vars or config files:
```go
// config.go
type Config struct {
    Environment string `koanf:"environment"`
    Database    DatabaseConfig
    Telegram    TelegramConfig
    Cache       CacheConfig
}

func Load() (*Config, error) {
    k := koanf.New(".")
    
    // Load from file based on ENV
    env := os.Getenv("ENV")
    if env == "" {
        env = "development"
    }
    k.Load(file.Provider(fmt.Sprintf("config/%s.yaml", env)), yaml.Parser())
    
    // Override with env vars
    k.Load(env.Provider("WANON_", ".", func(s string) string {
        return strings.ToLower(strings.TrimPrefix(s, "WANON_"))
    }), nil)
    
    var cfg Config
    k.Unmarshal("", &cfg)
    return &cfg, nil
}
```

**Action:** Add to Phase 1.

---

### 10. Code Formatting

**Source:** `.formatter.exs`

Elixir has built-in formatting. Go uses `gofmt` (standard) and optionally `golangci-lint`.

**Action:** Add to project setup (Makefile or scripts).

---

## Updated Migration Checklist

### Phase 1: Foundation
- [ ] Initialize Go module
- [ ] Create config package with koanf (support env vars and config files)
- [ ] Define all database models
- [ ] Create SQL migration files
- [ ] **Add golang-migrate for migration runner**
- [ ] Define Telegram client interface

### Phase 2: Telegram
- [ ] Implement HTTP client with retries
- [ ] **Implement getMe endpoint (optional)**
- [ ] Implement update polling worker
- [ ] Implement dispatcher with command registry
- [ ] Port chat whitelist filtering

### Phase 3: Cache
- [ ] Implement cache add command
- [ ] Implement cache edit command
- [ ] Implement cache clean worker with ticker

### Phase 4: Quotes
- [ ] Implement quote builder with recursive lookup
- [ ] Implement quote store
- [ ] Implement quote render
- [ ] Implement /addquote command
- [ ] Implement /rquote command

### Phase 5: Bootstrap
- [ ] Create main.go with errgroup
- [ ] Implement graceful shutdown
- [ ] **Add subcommands (migrate, server, or auto-migrate)**
- [ ] Create Dockerfile
- [ ] Create docker-compose.yml
- [ ] **Create entrypoint script for migrate+run**

### Phase 6: Testing & Deployment
- [ ] **Set up test infrastructure (test DB, fixtures)**
- [ ] Port unit tests (all test cases listed above)
- [ ] Port integration tests (mock Telegram API server)
- [ ] **Add code coverage reporting**
- [ ] **Setup GitHub Actions CI/CD (test, docker build, deploy)**
- [ ] Update README

### Final Verification
- [ ] Feature parity with Elixir version
- [ ] Load testing comparison
- [ ] Documentation complete

---

## Dependencies

| Purpose | Library | Import Path |
|---------|---------|-------------|
| Database ORM | GORM | `gorm.io/gorm` |
| Postgres driver | pgx | `gorm.io/driver/postgres` |
| JSON types | GORM datatypes | `gorm.io/datatypes` |
| **Migrations** | **golang-migrate** | **`github.com/golang-migrate/migrate/v4`** |
| Environment config | koanf | `github.com/knadh/koanf` |
| Error group | sync | `golang.org/x/sync/errgroup` |
| Structured logging | slog | `log/slog` (Go 1.21+) |
| Testing | testify | `github.com/stretchr/testify` |
| **CLI** | **urfave/cli** (optional) | **`github.com/urfave/cli`** |

---

## Configuration Mapping

| Elixir Config | Go Env Var | Default |
|---------------|------------|---------|
| `WANON_TELEGRAM_TOKEN` | `WANON_TELEGRAM_TOKEN` | *required* |
| `timeout` | `WANON_POLL_TIMEOUT` | `10` |
| `base_url` | `WANON_TELEGRAM_BASE_URL` | `https://api.telegram.org/bot` |
| `every` (cache clean) | `WANON_CACHE_CLEAN_INTERVAL` | `10m` |
| `keep` (cache retention) | `WANON_CACHE_KEEP_DURATION` | `48h` |
| Database host | `WANON_DB_HOST` | `localhost` |
| Database port | `WANON_DB_PORT` | `5432` |
| Database name | `WANON_DB_NAME` | `wanon` |
| Database user | `WANON_DB_USER` | `wanon` |
| Database password | `WANON_DB_PASSWORD` | `wanon` |
| Allowed chat IDs | `WANON_ALLOWED_CHAT_IDS` | *required* (comma-separated) |

---

## Database Migrations

### Migration 1: Create Cache Entry

```sql
-- 001_create_cache_entry.sql
CREATE TABLE cache_entries (
    id SERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    message_id BIGINT NOT NULL,
    reply_id BIGINT,
    date BIGINT NOT NULL,
    message JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (chat_id, message_id)
);

CREATE INDEX idx_cache_entries_date ON cache_entries(date);
```

### Migration 2: Create Quotes

```sql
-- 002_create_quotes.sql
CREATE TABLE quotes (
    id SERIAL PRIMARY KEY,
    creator JSONB NOT NULL,
    chat_id BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE quote_entries (
    id SERIAL PRIMARY KEY,
    "order" INTEGER NOT NULL,
    message JSONB NOT NULL,
    quote_id INTEGER NOT NULL REFERENCES quotes(id) ON DELETE CASCADE
);

CREATE INDEX idx_quotes_chat_id ON quotes(chat_id);
```

---

## Risk Areas & Considerations

| Risk | Mitigation |
|------|------------|
| **GenStage backpressure** | Use buffered channels with size limits |
| **Database connections** | Use connection pooling (`pgx` or GORM's pool) |
| **Error handling** | Go's explicit errors vs Elixir's "let it crash" - implement proper retry logic |
| **State recovery** | Implement retry logic for Telegram API failures |
| **Timezone handling** | Ensure Unix timestamp math matches Elixir behavior |
| **Message ordering** | Ensure quote entries maintain order (GORM Preload with Order clause) |
| **Migration versioning** | golang-migrate uses schema_migrations table vs Ecto's schema_migrations |

---

## Estimated Timeline

**Total: 7-10 days** (for a working port with tests)

| Days | Phase | Deliverables |
|------|-------|--------------|
| 1-2 | Foundation + Telegram | Config, models, HTTP client, polling |
| 3-4 | Cache system | Add, edit, clean functionality |
| 4-5 | Quotes system | Commands, builder, render |
| Day 6 | Bootstrap + Docker | Working binary, containers |
| 7-10 | Testing + Polish | Tests, CI/CD, bug fixes |

---

## Notes

### Elixir → Go Semantic Differences

1. **Concurrency Model**: Elixir uses lightweight processes with message passing; Go uses goroutines with shared memory (channels). Both are efficient, but Go's model is closer to traditional threading.

2. **Error Handling**: Elixir uses "let it crash" philosophy with supervisors; Go uses explicit error returns. Need to implement proper error handling and retries.

3. **Hot Reloading**: Elixir supports hot code reloading; Go requires restart. Use Docker or systemd for process management.

4. **Pattern Matching**: Elixir's powerful pattern matching can be approximated in Go with type switches and early returns.

5. **Pipelines**: Elixir's `|>` operator can be replaced with method chaining or functional options in Go.

### Original Elixir Project Structure Reference

```
wanon-elixir/
├── lib/
│   └── wanon/
│       ├── application.ex       # OTP Application
│       ├── command.ex           # Command behaviour
│       ├── dispatcher.ex        # GenStage consumer
│       ├── migrations.ex        # Ecto migrations runner ⚠️ MISSING IN PLAN
│       ├── repo.ex              # Ecto Repo
│       ├── cache/
│       │   ├── add.ex           # Add to cache command
│       │   ├── cache_entry.ex   # Ecto schema
│       │   ├── clean.ex         # GenServer cache cleaner
│       │   └── edit.ex          # Edit cache command
│       ├── quotes/
│       │   ├── add_quote.ex     # /addquote command
│       │   ├── builder.ex       # Quote builder from cache
│       │   ├── consumer.ex      # Quotes consumer (unused, skip)
│       │   ├── quote.ex         # Quote schema
│       │   ├── quote_entry.ex   # QuoteEntry schema
│       │   ├── render.ex        # Quote renderer
│       │   ├── rquote.ex        # /rquote command
│       │   └── store.ex         # Quote storage
│       └── telegram/
│           ├── client.ex        # Client behaviour (includes getMe)
│           ├── http.ex          # HTTP implementation
│           └── updates.ex       # GenStage producer
├── priv/repo/migrations/        # Ecto migrations
├── config/                      # Mix config files
├── rel/
│   ├── commands/
│   │   └── migrate_n_run.sh     # Release command script ⚠️ MISSING IN PLAN
│   └── config.exs               # Distillery config
├── integration/                 # Integration tests with fixtures ⚠️ MISSING IN PLAN
│   ├── fixture.*.json
│   ├── smoke_test.exs
│   └── support/
│       └── telegram_api.ex      # Mock Telegram API
└── test/                        # ExUnit tests with Mox ⚠️ MISSING DETAIL IN PLAN
    ├── support/
    │   └── mocks.ex
    └── wanon/
        ├── cache/
        └── quotes/
```
