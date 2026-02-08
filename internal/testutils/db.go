package testutils

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestDB wraps a GORM database connection for testing
type TestDB struct {
	DB        *gorm.DB
	container *postgres.PostgresContainer
}

// NewTestDB creates a new test database connection using testcontainers
func NewTestDB(t *testing.T) *TestDB {
	ctx := context.Background()

	// Start PostgreSQL container
	container, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("wanon_test"),
		postgres.WithUsername("wanon_test"),
		postgres.WithPassword("wanon_test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("Failed to start PostgreSQL container: %v", err)
	}

	// Get connection string
	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		container.Terminate(ctx)
		t.Fatalf("Failed to get connection string: %v", err)
	}

	// Connect to database
	db, err := gorm.Open(gormpostgres.Open(connStr), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		container.Terminate(ctx)
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	testDB := &TestDB{
		DB:        db,
		container: container,
	}

	// Run migrations using tern CLI
	if err := testDB.RunMigrations(connStr); err != nil {
		container.Terminate(ctx)
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Clean up after test
	t.Cleanup(func() {
		testDB.Cleanup()
	})

	return testDB
}

// RunMigrations runs database migrations using tern CLI
func (tdb *TestDB) RunMigrations(connStr string) error {
	// Get the directory of this file to find migrations
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	migrationsPath := filepath.Join(dir, "..", "..", "migrations")

	// Run tern migrate using the connection string and migrations path
	cmd := exec.Command("tern", "migrate", "--conn-string", connStr, "--migrations", migrationsPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("tern migrate failed: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// Cleanup truncates all tables and terminates the container
func (tdb *TestDB) Cleanup() {
	ctx := context.Background()

	// Truncate tables
	tables := []string{"quote_entry", "quote", "cache_entry"}
	for _, table := range tables {
		tdb.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
	}

	// Terminate container
	if tdb.container != nil {
		tdb.container.Terminate(ctx)
	}
}

// Transaction runs a function within a database transaction and rolls back after
func (tdb *TestDB) Transaction(t *testing.T, fn func(tx *gorm.DB)) {
	tx := tdb.DB.Begin()
	if tx.Error != nil {
		t.Fatalf("Failed to begin transaction: %v", tx.Error)
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	fn(tx)

	tx.Rollback()
}
