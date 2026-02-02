package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-telegram/bot/models"
	"github.com/graffic/wanon-go/internal/bot"
	"github.com/graffic/wanon-go/internal/cache"
	"github.com/graffic/wanon-go/internal/config"
	"github.com/graffic/wanon-go/internal/quotes"
	"github.com/graffic/wanon-go/internal/storage"
	"github.com/graffic/wanon-go/internal/telegram"
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

	// Execute command
	switch cmd {
	case "migrate":
		return runMigrations(cfg)
	case "server":
		return runServer(cfg)
	default:
		// Default: auto-migrate then run server
		if err := runMigrations(cfg); err != nil {
			return fmt.Errorf("migration failed: %w", err)
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

func runMigrations(cfg *config.Config) error {
	slog.Info("running database migrations")

	db, err := storage.New(&cfg.Database)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	if err := storage.RunMigrations(db.DB); err != nil {
		return fmt.Errorf("migration failed: %w", err)
	}

	slog.Info("migrations completed successfully")
	return nil
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

	// Initialize Telegram client
	telegramClient, err := telegram.NewHTTPClient(cfg.Telegram.Token)
	if err != nil {
		return fmt.Errorf("failed to create Telegram client: %w", err)
	}

	// Create channel for updates (buffered for backpressure handling)
	updatesCh := make(chan []models.Update, 100)

	// Create errgroup for concurrent component management
	g, ctx := errgroup.WithContext(ctx)

	// Component 1: Updates poller
	updates := bot.NewUpdates(telegramClient, updatesCh)
	g.Go(func() error {
		return updates.Start(ctx)
	})

	// Component 2: Dispatcher
	dispatcher := bot.NewDispatcher(updatesCh, cfg.AllowedChatIDs)

	// Register cache middleware to process all messages through cache
	cacheService := cache.NewService(db.DB)
	cacheMiddleware := cache.NewMiddleware(cacheService, slog.Default())
	dispatcher.AddUpdateHandler(cacheMiddleware.HandleUpdate)

	// Register quote command handlers
	// Create adapter for telegram client to match quotes.TelegramClient interface
	quotesClient := quotes.NewTelegramClientAdapter(telegramClient)
	addQuoteHandler := quotes.NewAddQuoteHandler(db.DB, quotesClient)
	rquoteHandler := quotes.NewRQuoteHandler(db.DB, quotesClient)

	dispatcher.Register("addquote", quotes.NewCommandAdapter(addQuoteHandler))
	dispatcher.Register("rquote", quotes.NewCommandAdapter(rquoteHandler))

	g.Go(func() error {
		return dispatcher.Start(ctx)
	})

	// Component 3: Cache cleaner
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
