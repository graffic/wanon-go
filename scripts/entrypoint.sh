#!/bin/sh
set -e

# Entrypoint script for Wanon Telegram Bot
# Runs migrations and then starts the server

echo "Running database migrations..."
if [ -n "$DATABASE_URL" ]; then
    tern migrate --conn-string "$DATABASE_URL" --migrations /app/migrations
else
    echo "ERROR: DATABASE_URL not set
    exit 1
fi

echo "Starting Wanon server..."
exec /app/wanon server "$@"
