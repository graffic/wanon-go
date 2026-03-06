package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/graffic/wanon-go/internal/bot/middleware"
	"github.com/graffic/wanon-go/internal/cache"
	"github.com/graffic/wanon-go/internal/config"
	"github.com/graffic/wanon-go/internal/quotes"
	"github.com/graffic/wanon-go/internal/stats"
	"github.com/graffic/wanon-go/internal/storage"
	"golang.org/x/sync/errgroup"
)

func main() {
	if err := run(); err != nil {
		slog.Error("application error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	// Parse command/subcommand
	cmd := parseCommand()

	// Load configuration
	env := os.Getenv("ENV")
	if env == "" {
		env = "development"
	}

	cfg, err := config.Load(env)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Configure slog with configured level
	level := parseLogLevel(cfg.Logging.Level)
	opts := &slog.HandlerOptions{
		Level: level,
	}
	handler := slog.NewTextHandler(os.Stderr, opts)
	slog.SetDefault(slog.New(handler))

	// Execute command
	switch cmd {
	case "server":
		return runServer(cfg)
	case "import-stats":
		return runImportStats(cfg)
	default:
		// Default: run migrations and server
		if err := storage.RunMigrations(&cfg.Database); err != nil {
			return err
		}
		return runServer(cfg)
	}
}

// runImportStats processes a Telegram JSON export and imports message stats.
//
// Usage:
//
//	wanon import-stats <file> [--chat-id=<id>]
//
// The chat ID defaults to the one embedded in the export file.
func runImportStats(cfg *config.Config) error {
	args := os.Args[2:] // strip "import-stats"

	var filePath string
	var chatID int64

	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "--chat-id="):
			raw := strings.TrimPrefix(arg, "--chat-id=")
			n, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid --chat-id value %q: %w", raw, err)
			}
			chatID = n
		default:
			if filePath == "" {
				filePath = arg
			}
		}
	}

	if filePath == "" {
		return fmt.Errorf("usage: wanon import-stats <file> [--chat-id=<id>]")
	}

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("import-stats: open file: %w", err)
	}
	defer f.Close()

	db, err := storage.New(&cfg.Database)
	if err != nil {
		return fmt.Errorf("import-stats: connect to database: %w", err)
	}
	defer db.Close()

	svc := stats.NewService(db.DB)

	slog.Info("starting stats import", "file", filePath, "chat_id", chatID)
	result, err := svc.ImportFromExport(context.Background(), chatID, f)
	if err != nil {
		return fmt.Errorf("import-stats: %w", err)
	}

	slog.Info("import complete",
		"messages_processed", result.MessagesProcessed,
		"buckets_written", result.BucketsWritten,
		"users_updated", result.UsersUpdated,
		"cutoff_utc", result.CutoffUTC,
	)
	return nil
}

func parseCommand() string {
	if len(os.Args) < 2 {
		return "default"
	}
	return os.Args[1]
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "info", "":
		return slog.LevelInfo
	default:
		return slog.LevelInfo
	}
}

func runServer(cfg *config.Config) error {
	slog.Info("starting wanon server", "environment", cfg.Environment)

	// Create context with signal handling
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer cancel()

	// Initialize database
	db, err := storage.New(&cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Initialize cache service
	cacheService := cache.NewService(db.DB)
	statsService := stats.NewService(db.DB)

	// Create middlewares
	chatFilterMiddleware := middleware.ChatFilter(cfg.AllowedChatIDs, cfg.AutoLeaveUnauthorized, slog.Default())
	cacheMiddleware := createCacheMiddleware(cacheService)
	statsMiddleware := stats.NewMiddleware(statsService, slog.Default())

	// Create bot options
	opts := []bot.Option{
		bot.WithMiddlewares(chatFilterMiddleware, cacheMiddleware, statsMiddleware),
		bot.WithDefaultHandler(defaultHandler),
	}

	// Initialize Telegram bot
	b, err := bot.New(cfg.Telegram.Token, opts...)
	if err != nil {
		return fmt.Errorf("failed to create Telegram bot: %w", err)
	}

	// Register command handlers
	addQuoteHandler := quotes.NewAddQuoteHandler(b, db.DB)
	rquoteHandler := quotes.NewRQuoteHandler(b, db.DB)
	seenHandler := stats.NewSeenHandler(b, statsService, slog.Default())
	topHandler := stats.NewTopHandler(b, statsService, cfg.Stats, slog.Default())

	// Register handlers for specific commands
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/addquote`), addQuoteHandler.Handle)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/rquote`), rquoteHandler.Handle)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^(/|!)seen`), seenHandler.Handle)
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^(/|!)top`), topHandler.Handle)

	// Register bot commands with Telegram (shows in command menu)
	commands := []models.BotCommand{
		{Command: addQuoteHandler.Command(), Description: addQuoteHandler.Description()},
		{Command: rquoteHandler.Command(), Description: rquoteHandler.Description()},
		{Command: seenHandler.Command(), Description: seenHandler.Description()},
		{Command: topHandler.Command(), Description: topHandler.Description()},
	}
	if _, err := b.SetMyCommands(ctx, &bot.SetMyCommandsParams{Commands: commands}); err != nil {
		slog.Error("failed to set bot commands", "error", err)
	} else {
		slog.Info("bot commands registered", "count", len(commands))
	}

	// Create errgroup for concurrent component management
	g, ctx := errgroup.WithContext(ctx)

	// Verify bot
	user, err := b.GetMe(ctx)
	if err != nil {
		return ctx.Err()
	}

	// Component 1: Bot polling
	g.Go(func() error {
		slog.Info("starting bot polling", "firstName", user.FirstName, "lastName", user.LastName)
		b.Start(ctx)
		return ctx.Err()
	})

	// Component 2: Cache cleaner
	cleanerConfig := cache.Config{
		CleanInterval: cfg.Cache.CleanInterval,
		KeepDuration:  cfg.Cache.KeepDuration,
	}
	cleaner := cache.NewCleaner(cacheService, cleanerConfig, slog.Default())
	g.Go(func() error {
		return cleaner.Start(ctx)
	})

	slog.Info("all components started, waiting for shutdown signal")

	// Wait for all components to complete
	if err := g.Wait(); err != nil {
		if err == context.Canceled {
			slog.Info("graceful shutdown completed")
			return nil
		}
		return fmt.Errorf("component error: %w", err)
	}

	slog.Info("application stopped")
	return nil
}

// createCacheMiddleware creates a bot middleware that processes updates through cache
func createCacheMiddleware(cacheService *cache.Service) bot.Middleware {
	cacheMw := cache.NewMiddleware(cacheService, slog.Default())

	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, update *models.Update) {
			// Process through cache first
			if err := cacheMw.HandleUpdate(ctx, update); err != nil {
				slog.Error("cache middleware error", "error", err)
			}
			// Continue to next handler
			next(ctx, b, update)
		}
	}
}

// defaultHandler handles non-command messages
func defaultHandler(ctx context.Context, b *bot.Bot, update *models.Update) {
	// Extract message from update
	var msg *models.Message
	if update.Message != nil {
		msg = update.Message
	} else if update.EditedMessage != nil {
		msg = update.EditedMessage
	}

	if msg == nil {
		return
	}

	// Default handler - just log the message
	slog.Debug("received message", "chat_id", msg.Chat.ID, "text", msg.Text)
}
