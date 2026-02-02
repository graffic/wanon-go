package storage

import (
	"fmt"
	"os"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"gorm.io/gorm"
)

// Migrator handles database migrations
type Migrator struct {
	m *migrate.Migrate
}

// NewMigrator creates a new migrator instance
func NewMigrator(db *gorm.DB, migrationsPath string) (*Migrator, error) {
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		fmt.Sprintf("file://%s", migrationsPath),
		"postgres", driver)
	if err != nil {
		return nil, fmt.Errorf("failed to create migrator: %w", err)
	}

	return &Migrator{m: m}, nil
}

// Up runs all pending migrations
func (m *Migrator) Up() error {
	if err := m.m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}

// Down rolls back all migrations
func (m *Migrator) Down() error {
	if err := m.m.Down(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to rollback migrations: %w", err)
	}
	return nil
}

// Steps runs migrations up or down by the specified number of steps
func (m *Migrator) Steps(n int) error {
	if err := m.m.Steps(n); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migration steps: %w", err)
	}
	return nil
}

// Version returns the current migration version
func (m *Migrator) Version() (uint, bool, error) {
	version, dirty, err := m.m.Version()
	if err != nil {
		return 0, false, fmt.Errorf("failed to get migration version: %w", err)
	}
	return version, dirty, nil
}

// Force forces a migration version
func (m *Migrator) Force(version int) error {
	if err := m.m.Force(version); err != nil {
		return fmt.Errorf("failed to force migration version: %w", err)
	}
	return nil
}

// Close closes the migrator
func (m *Migrator) Close() error {
	mSrcErr, mDbErr := m.m.Close()
	if mSrcErr != nil {
		return fmt.Errorf("failed to close migration source: %w", mSrcErr)
	}
	if mDbErr != nil {
		return fmt.Errorf("failed to close migration database: %w", mDbErr)
	}
	return nil
}

// RunMigrations is a helper function to run migrations from the default path
func RunMigrations(db *gorm.DB) error {
	// Get the migrations path from environment or use default
	migrationsPath := os.Getenv("WANON_MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "internal/storage/migrations"
	}

	migrator, err := NewMigrator(db, migrationsPath)
	if err != nil {
		return err
	}
	defer migrator.Close()

	return migrator.Up()
}
