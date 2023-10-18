#!/bin/sh
set -e

# Set environment variables
export USER=git
export GITEA_CUSTOM=/data/gitea

# Start Gitea in the background
/bin/s6-svscan /etc/s6 &

# Wait for Gitea to start
while ! nc -z localhost 3000; do
  echo "Waiting for Gitea to start..."
  sleep 1
done

# Create admin user
su-exec git /app/gitea/gitea admin user create --admin --access-token --username=gigo-dev --password=gigo-dev --email=dev@gigo.dev --must-change-password=false

# Keep the container running
wait %1
