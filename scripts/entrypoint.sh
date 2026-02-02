#!/bin/sh
set -e

# Entrypoint script for Wanon Telegram Bot
# Handles migration and server startup

# Default to "server" if no command provided
if [ $# -eq 0 ]; then
    set -- server
fi

COMMAND=$1
shift

case "$COMMAND" in
    migrate)
        echo "Running database migrations..."
        exec /app/wanon migrate "$@"
        ;;
    server)
        echo "Starting Wanon server..."
        exec /app/wanon server "$@"
        ;;
    *)
        # Pass through any other command
        exec /app/wanon "$COMMAND" "$@"
        ;;
esac
