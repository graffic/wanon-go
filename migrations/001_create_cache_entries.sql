-- Create cache_entry table
CREATE TABLE IF NOT EXISTS cache_entry (
    id BIGSERIAL PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    message_id BIGINT NOT NULL,
    reply_id BIGINT,
    date BIGINT NOT NULL,
    message JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create index for chat_id + message_id lookups
CREATE UNIQUE INDEX idx_cache_entry_chat_message ON cache_entry(chat_id, message_id);

-- Create index for reply lookups
CREATE INDEX idx_cache_entry_reply ON cache_entry(chat_id, reply_id) WHERE reply_id IS NOT NULL;

-- Create index for date-based queries
CREATE INDEX idx_cache_entry_date ON cache_entry(date);

---- create above / drop below ----

DROP TABLE IF EXISTS cache_entry;
