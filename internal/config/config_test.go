package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_WithAllowedChatIDs(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected []int64
		wantErr  bool
	}{
		{
			name:     "no env var set",
			envValue: "",
			expected: nil,
			wantErr:  false,
		},
		{
			name:     "single chat ID",
			envValue: "123456789",
			expected: []int64{123456789},
			wantErr:  false,
		},
		{
			name:     "multiple chat IDs",
			envValue: "123456789,-987654321,555555555",
			expected: []int64{123456789, -987654321, 555555555},
			wantErr:  false,
		},
		{
			name:     "invalid chat ID",
			envValue: "not-a-number",
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing env var
			os.Unsetenv("WANON_ALLOWED_CHAT_IDS")

			if tt.envValue != "" {
				os.Setenv("WANON_ALLOWED_CHAT_IDS", tt.envValue)
				defer os.Unsetenv("WANON_ALLOWED_CHAT_IDS")
			}

			cfg, err := Load("test")
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, cfg.AllowedChatIDs)
			}
		})
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Clean up any existing env vars
	os.Unsetenv("WANON_ALLOWED_CHAT_IDS")
	os.Unsetenv("WANON_DATABASE__PORT")
	os.Unsetenv("WANON_DATABASE__SSLMODE")
	os.Unsetenv("WANON_CACHE__CLEAN_INTERVAL")
	os.Unsetenv("WANON_CACHE__KEEP_DURATION")

	cfg, err := Load("test")
	require.NoError(t, err)

	// Check defaults
	assert.Equal(t, 5432, cfg.Database.Port)
	assert.Equal(t, "disable", cfg.Database.SSLMode)
	assert.NotZero(t, cfg.Cache.CleanInterval)
	assert.NotZero(t, cfg.Cache.KeepDuration)
}

func TestDSN(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected string
	}{
		{
			name: "all fields set",
			config: Config{
				Database: DatabaseConfig{
					Host:     "localhost",
					Port:     5432,
					User:     "testuser",
					Password: "testpass",
					Database: "testdb",
					SSLMode:  "disable",
				},
			},
			expected: "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=disable",
		},
		{
			name: "different port",
			config: Config{
				Database: DatabaseConfig{
					Host:     "db.example.com",
					Port:     5433,
					User:     "admin",
					Password: "secret",
					Database: "production",
					SSLMode:  "require",
				},
			},
			expected: "host=db.example.com port=5433 user=admin password=secret dbname=production sslmode=require",
		},
		{
			name: "empty password",
			config: Config{
				Database: DatabaseConfig{
					Host:     "localhost",
					Port:     5432,
					User:     "postgres",
					Password: "",
					Database: "mydb",
					SSLMode:  "disable",
				},
			},
			expected: "host=localhost port=5432 user=postgres password= dbname=mydb sslmode=disable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := tt.config.Database.DSN()
			assert.Equal(t, tt.expected, dsn)
		})
	}
}

func TestDSN_WithLoadedConfig(t *testing.T) {
	// Set up environment variables for database config
	os.Setenv("WANON_DATABASE__HOST", "testhost")
	os.Setenv("WANON_DATABASE__PORT", "5433")
	os.Setenv("WANON_DATABASE__USER", "testuser")
	os.Setenv("WANON_DATABASE__PASSWORD", "testpassword")
	os.Setenv("WANON_DATABASE__DATABASE", "testdatabase")
	os.Setenv("WANON_DATABASE__SSLMODE", "require")
	defer func() {
		os.Unsetenv("WANON_DATABASE__HOST")
		os.Unsetenv("WANON_DATABASE__PORT")
		os.Unsetenv("WANON_DATABASE__USER")
		os.Unsetenv("WANON_DATABASE__PASSWORD")
		os.Unsetenv("WANON_DATABASE__DATABASE")
		os.Unsetenv("WANON_DATABASE__SSLMODE")
	}()

	cfg, err := Load("test")
	require.NoError(t, err)

	dsn := cfg.Database.DSN()
	assert.Equal(t, "host=testhost port=5433 user=testuser password=testpassword dbname=testdatabase sslmode=require", dsn)
}
