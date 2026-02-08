-- Create quote table
CREATE TABLE IF NOT EXISTS quote (
    id BIGSERIAL PRIMARY KEY,
    creator JSONB NOT NULL,
    chat_id BIGINT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create index for chat lookups
CREATE INDEX idx_quote_chat_id ON quote(chat_id);

-- Create quote_entry table
CREATE TABLE IF NOT EXISTS quote_entry (
    id BIGSERIAL PRIMARY KEY,
    quote_id BIGINT NOT NULL REFERENCES quote(id) ON DELETE CASCADE,
    "order" INT NOT NULL,
    message JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

-- Create index for quote lookups
CREATE INDEX idx_quote_entry_quote_id ON quote_entry(quote_id);

-- Create index for soft deletes
CREATE INDEX idx_quote_entry_deleted_at ON quote_entry(deleted_at) WHERE deleted_at IS NOT NULL;

---- create above / drop below ----

DROP TABLE IF EXISTS quote_entry;
DROP TABLE IF EXISTS quote;
