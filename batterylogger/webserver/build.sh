#!/bin/bash
# Cross-compile the dashboard for the Pi Zero W (ARMv6) from any machine with Go.
# No Docker, no cross C toolchain — pure Go (modernc.org/sqlite) makes this a one-liner.
set -e
cd "$(dirname "$0")"
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build -trimpath -ldflags='-s -w' -o batteryweb-go .
echo "built: $(file batteryweb-go | cut -d, -f1-3)"
echo "deploy: scp batteryweb-go <user>@<host>:~/batterylogger/ && sudo systemctl restart batteryweb"
