package storage

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	"github.com/graffic/wanon-go/internal/config"
)

func RunMigrations(cfg *config.DatabaseConfig) error {
	slog.Info("running database migrations")

	// Build connection string from config
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.Database,
		cfg.SSLMode,
	)

	// Run tern migrate using full path
	cmd := exec.Command("tern", "migrate", "--conn-string", connStr, "--migrations", "./migrations")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	slog.Info("migrations completed successfully")
	return nil
}
