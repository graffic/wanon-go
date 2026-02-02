package testutils

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	pgmigrate "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TestDBConfig holds configuration for test database
type TestDBConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	SSLMode  string
}

// DefaultTestDBConfig returns default test database configuration
func DefaultTestDBConfig() TestDBConfig {
	return TestDBConfig{
		Host:     getEnv("TEST_DB_HOST", "localhost"),
		Port:     getEnvInt("TEST_DB_PORT", 5432),
		User:     getEnv("TEST_DB_USER", "wanon_test"),
		Password: getEnv("TEST_DB_PASSWORD", "wanon_test"),
		Database: getEnv("TEST_DB_NAME", "wanon_test"),
		SSLMode:  getEnv("TEST_DB_SSLMODE", "disable"),
	}
}

// DSN returns the PostgreSQL connection string
func (c *TestDBConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.SSLMode)
}

// TestDB wraps a GORM database connection for testing
type TestDB struct {
	DB *gorm.DB
}

// NewTestDB creates a new test database connection
func NewTestDB(t *testing.T) *TestDB {
	cfg := DefaultTestDBConfig()
	
	db, err := gorm.Open(postgres.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	testDB := &TestDB{DB: db}
	
	// Run migrations
	if err := testDB.RunMigrations(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Clean up after test
	t.Cleanup(func() {
		testDB.Cleanup()
	})

	return testDB
}

// NewTestDBWithContext creates a new test database with context
func NewTestDBWithContext(ctx context.Context, t *testing.T) *TestDB {
	return NewTestDB(t)
}

// RunMigrations runs database migrations
func (tdb *TestDB) RunMigrations() error {
	sqlDB, err := tdb.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	driver, err := pgmigrate.WithInstance(sqlDB, &pgmigrate.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://../../internal/storage/migrations",
		"postgres", driver)
	if err != nil {
		// Try alternative path
		m, err = migrate.NewWithDatabaseInstance(
			"file://../storage/migrations",
			"postgres", driver)
		if err != nil {
			return fmt.Errorf("failed to create migration instance: %w", err)
		}
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// Cleanup truncates all tables
func (tdb *TestDB) Cleanup() {
	tables := []string{"quote_entries", "quotes", "cache_entries"}
	for _, table := range tables {
		tdb.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
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

// LoadFixture loads a JSON fixture file
func LoadFixture(t *testing.T, filename string) []byte {
	data, err := os.ReadFile(fmt.Sprintf("../../testdata/%s", filename))
	if err != nil {
		// Try alternative path
		data, err = os.ReadFile(fmt.Sprintf("../testdata/%s", filename))
		if err != nil {
			t.Fatalf("Failed to load fixture %s: %v", filename, err)
		}
	}
	return data
}

// WaitForCondition waits for a condition to be met or timeout
func WaitForCondition(t *testing.T, condition func() bool, timeout time.Duration, interval time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(interval)
	}
	t.Fatal("Condition not met within timeout")
}

// getEnv gets environment variable or returns default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt gets environment variable as int or returns default
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		var result int
		fmt.Sscanf(value, "%d", &result)
		return result
	}
	return defaultValue
}

// SetupTestLogger configures a test logger
func SetupTestLogger() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}
