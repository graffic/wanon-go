package cache

import (
	"context"
	"log/slog"
	"time"
)

// Config holds cache cleaner configuration
type Config struct {
	CleanInterval time.Duration
	KeepDuration  time.Duration
}

// Cleaner periodically cleans old cache entries
type Cleaner struct {
	service *Service
	config  Config
	logger  *slog.Logger
}

// NewCleaner creates a new cache cleaner
func NewCleaner(service *Service, config Config, logger *slog.Logger) *Cleaner {
	return &Cleaner{
		service: service,
		config:  config,
		logger:  logger,
	}
}

// Start begins the periodic cleanup process
func (c *Cleaner) Start(ctx context.Context) error {
	c.logger.Info("starting cache cleaner",
		"clean_interval", c.config.CleanInterval,
		"keep_duration", c.config.KeepDuration,
	)

	// Perform initial cleanup
	if err := c.clean(ctx); err != nil {
		c.logger.Error("initial cache cleanup failed", "error", err)
	}

	// Create ticker for periodic cleanup
	ticker := time.NewTicker(c.config.CleanInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("stopping cache cleaner")
			return ctx.Err()
		case <-ticker.C:
			if err := c.clean(ctx); err != nil {
				c.logger.Error("cache cleanup failed", "error", err)
			}
		}
	}
}

// clean removes old cache entries
func (c *Cleaner) clean(ctx context.Context) error {
	c.logger.Debug("running cache cleanup")

	cutoff := time.Now().Add(-c.config.KeepDuration).Unix()

	result := c.service.db.WithContext(ctx).
		Where("date < ?", cutoff).
		Delete(&CacheEntry{})

	if result.Error != nil {
		return result.Error
	}

	c.logger.Info("cache cleanup completed",
		"deleted", result.RowsAffected,
		"cutoff_unix", cutoff,
	)

	return nil
}

// CleanOnce performs a single cleanup operation (useful for testing or manual cleanup)
func (c *Cleaner) CleanOnce(ctx context.Context) error {
	return c.clean(ctx)
}
