#!/usr/bin/env bash
# Build batteryeink for the Pi and install it over the running daemon.
# Usage: ./deploy.sh [user@host]     (or export DEPLOY_TARGET=user@host)
#
# Why not a plain scp: while batteryeink.service is running, the Pi holds the
# binary open and scp'ing onto it fails with "text file busy". Landing it as
# .new and renaming works because the rename only swaps the directory entry —
# the running process keeps the old inode until systemd restarts it.
set -euo pipefail

HOST="${1:-${DEPLOY_TARGET:-user@hostname.local}}"
REMOTE_USER="${HOST%@*}"
DEST="/home/${REMOTE_USER}/batterylogger"
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "Building..."
"$DIR/build.sh"

echo "Deploying to ${HOST} ..."
scp "$DIR/batteryeink" "${HOST}:${DEST}/batteryeink.new"
ssh "$HOST" "mv ${DEST}/batteryeink.new ${DEST}/batteryeink \
  && chmod +x ${DEST}/batteryeink \
  && sudo systemctl restart batteryeink.service"

echo "Restarted; waiting for the first paint..."
ssh "$HOST" "sleep 14; journalctl -u batteryeink.service -n 3 --no-pager"
