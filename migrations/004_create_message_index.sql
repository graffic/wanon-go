CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE INDEX idx_quote_entry_text_trgm ON quote_entry 
USING gin ((message->>'text') gin_trgm_ops);

---- create above / drop below ----

DROP INDEX idx_quote_entry_text_trgm;