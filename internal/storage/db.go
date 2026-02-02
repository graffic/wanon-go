package storage

import (
	"fmt"

	"github.com/graffic/wanon-go/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB holds the database connection
type DB struct {
	*gorm.DB
}

// New creates a new database connection
func New(cfg *config.DatabaseConfig) (*DB, error) {
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN()), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &DB{db}, nil
}

// NewWithLogger creates a new database connection with custom logger level
func NewWithLogger(cfg *config.DatabaseConfig, logLevel logger.LogLevel) (*DB, error) {
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	}

	db, err := gorm.Open(postgres.Open(cfg.DSN()), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	return &DB{db}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	return sqlDB.Close()
}

// AutoMigrate runs auto-migration for the given models
func (db *DB) AutoMigrate(models ...interface{}) error {
	return db.DB.AutoMigrate(models...)
}
