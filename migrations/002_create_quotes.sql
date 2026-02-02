-- Create quotes table
CREATE TABLE IF NOT EXISTS quotes (
    id BIGSERIAL PRIMARY KEY,
    creator JSONB NOT NULL,
    chat_id BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create index for chat lookups
CREATE INDEX idx_quotes_chat_id ON quotes(chat_id);

-- Create quote_entries table
CREATE TABLE IF NOT EXISTS quote_entries (
    id BIGSERIAL PRIMARY KEY,
    quote_id BIGINT NOT NULL REFERENCES quotes(id) ON DELETE CASCADE,
    "order" INT NOT NULL,
    message JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create index for quote lookups
CREATE INDEX idx_quote_entries_quote_id ON quote_entries(quote_id);

-- Create index for soft deletes
CREATE INDEX idx_quote_entries_deleted_at ON quote_entries(deleted_at) WHERE deleted_at IS NOT NULL;

---- create above / drop below ----

DROP TABLE IF EXISTS quote_entries;
DROP TABLE IF EXISTS quotes;
