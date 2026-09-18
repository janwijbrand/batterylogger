#!/usr/bin/env bash
# Deploy the Python logger to the Pi and restart it.
# Usage: ./deploy.sh [user@host]     (or export DEPLOY_TARGET=user@host)
#
# The Go binary has its own script, which handles the running daemon holding
# the file open ("text file busy" on a plain scp):
#   ./eink/deploy.sh "$HOST"
set -euo pipefail

HOST="${1:-${DEPLOY_TARGET:-user@hostname.local}}"
REMOTE_USER="${HOST%@*}"
DEST="/home/${REMOTE_USER}/batterylogger"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Deploying logger to ${HOST} ..."
scp "$DIR/logger.py" "${HOST}:${DEST}/"
ssh "$HOST" "sudo systemctl restart batterylogger.service"
echo "Done."
