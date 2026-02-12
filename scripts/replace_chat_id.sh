#!/bin/bash
# Script to replace chat_id in backup.sql with a new chat_id
# Usage: ./replace_chat_id.sh backup.sql [new_chat_id] > backup_new.sql

INPUT_FILE="${1:-backup.sql}"
NEW_CHAT_ID="${2:--1001549724404}"

if [ ! -f "$INPUT_FILE" ]; then
    echo "Error: File '$INPUT_FILE' not found" >&2
    exit 1
fi

# Replace chat_id values in the COPY section for the quote table
# The pattern matches: number<TAB>json<TAB>chat_id<TAB>timestamp
# and replaces chat_id with the new chat_id

sed -E "s/^([0-9]+\t\{[^}]+\}\t)-?[0-9]+(\t[0-9]{4}-[0-9]{2}-[0-9]{2})/\1${NEW_CHAT_ID}\2/" "$INPUT_FILE"
