-- Create user_message_stats table
CREATE TABLE IF NOT EXISTS user_message_stats (
    chat_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    user_name TEXT,
    last_message_at TIMESTAMP WITH TIME ZONE NOT NULL,
    total_messages BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (chat_id, user_id)
);

-- Create user_message_hourly table
CREATE TABLE IF NOT EXISTS user_message_hourly (
    chat_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    user_name TEXT,
    bucket_ts TIMESTAMP WITH TIME ZONE NOT NULL,
    message_count BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (chat_id, user_id, bucket_ts)
);

-- Create index for bucket time queries
CREATE INDEX idx_user_message_hourly_bucket_ts ON user_message_hourly(bucket_ts);

---- create above / drop below ----

DROP TABLE IF EXISTS user_message_hourly;
DROP TABLE IF EXISTS user_message_stats;
