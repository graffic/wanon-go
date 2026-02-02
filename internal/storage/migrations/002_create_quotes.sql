-- +migrate Up
-- Create quotes table
CREATE TABLE IF NOT EXISTS quotes (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create index for chat lookups
CREATE INDEX idx_quotes_chat_id ON quotes(chat_id);

-- Create index for soft deletes
CREATE INDEX idx_quotes_deleted_at ON quotes(deleted_at) WHERE deleted_at IS NOT NULL;

-- Create unique constraint for chat_id + name combination (excluding soft deleted)
CREATE UNIQUE INDEX idx_quotes_chat_name ON quotes(chat_id, name) WHERE deleted_at IS NULL;

-- Create quote_entries table
CREATE TABLE IF NOT EXISTS quote_entries (
    id BIGSERIAL PRIMARY KEY,
    quote_id BIGINT NOT NULL REFERENCES quotes(id) ON DELETE CASCADE,
    text TEXT NOT NULL,
    author VARCHAR(255),
    author_id BIGINT,
    message_id BIGINT,
    date TIMESTAMP WITH TIME ZONE,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create index for quote lookups
CREATE INDEX idx_quote_entries_quote_id ON quote_entries(quote_id);

-- Create index for author lookups
CREATE INDEX idx_quote_entries_author_id ON quote_entries(author_id) WHERE author_id IS NOT NULL;

-- Create index for soft deletes
CREATE INDEX idx_quote_entries_deleted_at ON quote_entries(deleted_at) WHERE deleted_at IS NOT NULL;

-- +migrate Down
-- Drop quote_entries table first (foreign key constraint)
DROP TABLE IF EXISTS quote_entries;

-- Drop quotes table
DROP TABLE IF EXISTS quotes;
