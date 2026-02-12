package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"regexp"
	"syscall"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/graffic/wanon-go/internal/bot/middleware"
	"github.com/graffic/wanon-go/internal/cache"
	"github.com/graffic/wanon-go/internal/config"
	"github.com/graffic/wanon-go/internal/quotes"
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
	// Configure slog with debug level
	opts := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}
	handler := slog.NewTextHandler(os.Stderr, opts)
	slog.SetDefault(slog.New(handler))

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

	// Execute command
	switch cmd {
	case "server":
		return runServer(cfg)
	default:
		// Default: run migrations and server
		if err := storage.RunMigrations(&cfg.Database); err != nil {
			return err
		}
		return runServer(cfg)
	}
}

func parseCommand() string {
	if len(os.Args) < 2 {
		return "default"
	}
	return os.Args[1]
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

	// Create middlewares
	chatFilterMiddleware := middleware.ChatFilter(cfg.AllowedChatIDs, cfg.AutoLeaveUnauthorized, slog.Default())
	cacheMiddleware := createCacheMiddleware(cacheService)

	// Create bot options
	opts := []bot.Option{
		bot.WithMiddlewares(chatFilterMiddleware, cacheMiddleware),
		bot.WithDefaultHandler(defaultHandler),
	}

	// Initialize Telegram bot
	b, err := bot.New(cfg.Telegram.Token, opts...)
	if err != nil {
		return fmt.Errorf("failed to create Telegram bot: %w", err)
	}

	// Register command handlers
	addQuoteHandler := quotes.NewAddQuoteHandler(db.DB)
	rquoteHandler := quotes.NewRQuoteHandler(db.DB)

	// Register handlers for specific commands
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/addquote`), wrapHandler(addQuoteHandler))
	b.RegisterHandlerRegexp(bot.HandlerTypeMessageText, regexp.MustCompile(`^/rquote`), wrapHandler(rquoteHandler))

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

// wrapHandler wraps a command handler to match bot.HandlerFunc signature
func wrapHandler(handler interface {
	Handle(ctx context.Context, b *bot.Bot, update *models.Update) error
}) bot.HandlerFunc {
	return func(ctx context.Context, b *bot.Bot, update *models.Update) {
		if err := handler.Handle(ctx, b, update); err != nil {
			slog.Error("command handler error", "error", err)
		}
	}
}
