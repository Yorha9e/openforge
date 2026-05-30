#!/bin/bash
set -e

echo "=== OpenForge Docker Entrypoint ==="
echo "Profile: ${OPENFORGE_PROFILE:-docker-dev}"
echo "Config: ${CONFIG_FILE:-config/profiles/docker-dev.yaml}"

# Run database migrations
echo "Running database migrations..."
./openforge-cli -config "${CONFIG_FILE:-config/profiles/docker-dev.yaml}" migrate up

# Start the server
echo "Starting server..."
exec ./openforge -addr ":8080" -config "${CONFIG_FILE:-config/profiles/docker-dev.yaml}"
