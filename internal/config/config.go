package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/providers/structs"
	"github.com/knadh/koanf/v2"
)

// Config holds all application configuration
type Config struct {
	Environment           string         `koanf:"environment"`
	Telegram              TelegramConfig `koanf:"telegram"`
	Database              DatabaseConfig `koanf:"database"`
	Cache                 CacheConfig    `koanf:"cache"`
	AllowedChatIDs        []int64        `koanf:"allowed_chat_ids"`
	AutoLeaveUnauthorized bool           `koanf:"auto_leave_unauthorized"`
}

// TelegramConfig holds Telegram bot configuration
type TelegramConfig struct {
	Token   string `koanf:"token"`
	Webhook string `koanf:"webhook"`
}

// DatabaseConfig holds database connection configuration
type DatabaseConfig struct {
	Host       string `koanf:"host"`
	Port       int    `koanf:"port"`
	User       string `koanf:"user"`
	Password   string `koanf:"password"`
	Database   string `koanf:"database"`
	SSLMode    string `koanf:"sslmode"`
	Migrations string `koanf:"migrations"`
}

// CacheConfig holds cache-specific configuration
type CacheConfig struct {
	CleanInterval time.Duration `koanf:"clean_interval"` // e.g., "10m"
	KeepDuration  time.Duration `koanf:"keep_duration"`  // e.g., "48h"
}

// DSN returns the PostgreSQL connection string
func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host,
		c.Port,
		c.User,
		c.Password,
		c.Database,
		c.SSLMode,
	)
}

// Load loads configuration from environment variables and config files
func Load(environment string) (*Config, error) {
	k := koanf.New(".")
	// Load defaults first (lowest priority)
	if err := k.Load(structs.Provider(defaultConfig(), "koanf"), nil); err != nil {
		return nil, fmt.Errorf("error loading defaults: %w", err)
	}

	// Load from config file based on environment
	configFile := fmt.Sprintf("config/%s.yaml", environment)
	if err := k.Load(file.Provider(configFile), yaml.Parser()); err != nil {
		// Config file is optional, log but don't fail
		fmt.Printf("Warning: could not load config file %s: %v\n", configFile, err)
	}

	// Load from environment variables with WANON_ prefix
	// Environment variables override config file values
	if err := k.Load(env.ProviderWithValue("WANON_", "__", func(key string, value string) (string, interface{}) {
		finalKey := strings.TrimPrefix(strings.ToLower(key), "wanon_")

		// Check if the existing config has this key as a slice
		switch k.Get(finalKey).(type) {
		case []interface{}, []string, []int64:
			// It's a slice, split by comma
			parts := strings.Split(value, ",")
			for i := range parts {
				parts[i] = strings.TrimSpace(parts[i])
			}
			return finalKey, parts
		}

		return finalKey, value
	}), nil); err != nil {
		return nil, fmt.Errorf("error loading environment variables: %w", err)
	}

	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	cfg.Environment = environment

	return &cfg, nil
}

// defaultConfig returns the default configuration values
func defaultConfig() Config {
	return Config{
		Database: DatabaseConfig{
			Port:       5432,
			SSLMode:    "disable",
			Migrations: "./migrations",
		},
		Cache: CacheConfig{
			CleanInterval: 10 * time.Minute,
			KeepDuration:  48 * time.Hour,
		},
	}
}
