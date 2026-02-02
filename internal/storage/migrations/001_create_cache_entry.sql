-- +migrate Up
-- Create cache_entries table
CREATE TABLE IF NOT EXISTS cache_entries (
    id BIGSERIAL PRIMARY KEY,
    key VARCHAR(255) UNIQUE NOT NULL,
    value JSONB NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create index for key lookups
CREATE INDEX idx_cache_entries_key ON cache_entries(key);

-- Create index for expiration cleanup
CREATE INDEX idx_cache_entries_expires_at ON cache_entries(expires_at) WHERE expires_at IS NOT NULL;

-- Create index for soft deletes
CREATE INDEX idx_cache_entries_deleted_at ON cache_entries(deleted_at) WHERE deleted_at IS NOT NULL;

-- +migrate Down
-- Drop cache_entries table
DROP TABLE IF EXISTS cache_entries;
