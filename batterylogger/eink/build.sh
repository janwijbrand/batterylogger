#!/bin/sh
# Cross-compile batteryeink for the Pi Zero (ARMv6). Pure Go, no cgo, no toolchain.
set -e
cd "$(dirname "$0")"
CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build -trimpath -ldflags='-s -w' -o batteryeink .
ls -lh batteryeink
