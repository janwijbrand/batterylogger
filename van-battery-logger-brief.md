# Van battery logger — project brief

## The idea

Build a small, always-on logger that records historical data from the LiFePO4
battery in my campervan (ECTIVE Accubox 120S power station), which exposes live
values — voltage, current, watts, state of charge, Ah remaining — over Bluetooth
LE but keeps no history. Goal: an intuition for real-world usage patterns (daily
Ah consumed, solar yield, overnight drain) during multi-day off-grid trips.

Final form: a Raspberry Pi Zero W v1.1 polling the battery BMS over BLE,
appending to SQLite, and rendering a dashboard to a Waveshare 7.5" e-ink HAT
(800×480) every ~10 minutes — big now-values, a 48 h SoC sparkline, and per-day
bars for Ah used / solar in. Powered from the Accubox's own USB-A port, so the
logger appears in its own data as a ~1 W baseline.

## What is known about the BLE protocol

- The battery inside is an ECTIVE LC-series LiFePO4 with a BLE BMS. It is
  **not** a mystery protocol: support for it was recently merged into
  `patman15/BMS_BLE-HA` (Home Assistant integration) and its underlying library
  **`aiobmsble`** (PyPI, built on `bleak`). See BMS_BLE-HA issue #572 for logs
  from a working ECTIVE LC 120.
- Observed characteristics from that issue: device advertises a name like
  `24-03-20-011`, service UUID `0xAF30`, communication on characteristic
  `0xFFE1`. Request frames look like Modbus RTU over BLE, e.g. `22 03 04 00 00
  08 42 6f` (addr 0x22, func 0x03 read registers, start, count, CRC16). No
  pairing/auth.
- Plan A: use `aiobmsble` directly and never parse a byte by hand. Plan B (if
  the Accubox's BMS generation differs): sniff with `bleak` notifications / nRF
  Connect and decode the register map manually.
- BLE peripherals like this accept one connection at a time — use a
  **poll-and-disconnect** pattern (connect, read, disconnect, sleep 30–60 s) so
  the vendor iPhone app still works between polls.

## Architecture (Tier: minimal, no Home Assistant)

One Python service (systemd unit):

```
poll BMS via BLE (every 30–60 s) → append SQLite (WAL mode, batched commits)
every 10 min → SQL aggregate → render 800×480 PNG with Pillow → push to e-ink
```

## Hardware & platform constraints

- **Pi Zero W v1.1 = ARMv6.** 32-bit Raspberry Pi OS Lite only. No Docker.
  Install Python packages from piwheels (default on Pi OS); get Pillow via `apt
  install python3-pil`, not pip. `bleak` works on ARMv6.
- Shared WiFi/BT radio on the Zero W: if BLE times out intermittently, disable
  WiFi power management (`iwconfig wlan0 power off`).
- SD card is a SanDisk High Endurance 32 GB. Still: WAL mode, minimal logging,
  tolerate abrupt power loss (the supply is a USB port someone can unplug).
- Development happens on a spare Pi 3 (ARMv7, faster); deploy to the Zero via
  rsync/SD swap. Confirm one full run on the Zero before trusting it —
  piwheels/apt packages should make ARMv6 a non-issue.
- E-ink HAT not yet purchased; display code should be cleanly separated so the
  logger runs headless first.

## Current status / immediate task

The Zero was just flashed (Pi OS Lite 32-bit, WiFi + SSH preconfigured in
Raspberry Pi Imager) and powered up for the first time in years. **First job:
verify the board is alive and sane before any project work.**

Checklist for this session:

1. Find it on the LAN (`ping hostname.local`, or check the router/`nmap` for
   a new host), SSH in.
2. Sanity: `uptime`, `vcgencmd measure_temp`, `vcgencmd get_throttled`
   (undervoltage check — relevant later on the van USB cable), free space on the
   card.
3. `sudo apt update && sudo apt full-upgrade` (slow on this board — let it run).
4. Enable SPI via `raspi-config` (for the future e-ink HAT).
5. Confirm BLE works: `bluetoothctl scan on` or a 10-line `bleak` discovery
   script — ideally spotting the Accubox advertising (name pattern
   `##-##-##-###`, service `af30`).
6. If all green: this board is the production target; development shifts to the
   Pi 3.
