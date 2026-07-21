#!/usr/bin/env bash
# Deploy the batterijtje code to the Pi and restart the services.
# Usage: ./deploy.sh [user@host]     (or export DEPLOY_TARGET=user@host)
set -euo pipefail

HOST="${1:-${DEPLOY_TARGET:-user@hostname.local}}"
REMOTE_USER="${HOST%@*}"
DEST="/home/${REMOTE_USER}/batterylogger"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Deploying to ${HOST} ..."
ssh "$HOST" "mkdir -p ${DEST}/web"
scp "$DIR/logger.py" "$DIR/webapp.py" "${HOST}:${DEST}/"
scp "$DIR/web/index.html" "${HOST}:${DEST}/web/"
ssh "$HOST" "sudo systemctl restart batterylogger.service batteryweb.service"
echo "Done → dashboard on port 8080"
