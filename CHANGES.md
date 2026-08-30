# Changelog

All notable changes to **batterijtje** are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/); versioning will follow
[SemVer](https://semver.org/) once there's a tagged release. The project aims
for a **1.0** after it has logged a full multi-day off-grid trip — until then
everything lives under _Unreleased_.

Data timestamps are UTC; the dates below are local (Europe/Amsterdam).

## [Unreleased]

Working toward 1.0.

### 2026-08-30 — real hardware: RTC + e-ink dashboard

**Added**
- **DS3231 real-time clock** (Adafruit PiRTC) on I²C, via
  `dtoverlay=i2c-rtc,ds3231`. Verified it holds time on its coin cell across a
  full power-cut, so the clock is correct at boot before NTP — the proper fix
  for the Pi Zero's lack of an RTC. Retired `fake-hwclock` in its favour.
- **`batteryeink`** — a second single static ARMv6 Go binary that renders the
  dashboard to a **Waveshare 2.7″ e-Paper HAT (V2, 264×176)**. Pure Go: it
  fetches `/api/data` from the local server, draws a 1-bit frame, and drives the
  panel over SPI — its own ported mono driver (`periph.io` for SPI/GPIO, no cgo,
  no Python). Cross-compiles from a laptop like `batteryweb-go`.
- **`batteryeink.timer`** — repaints the panel every 10 min (one-shot service;
  ~3.4 s per refresh, zero idle footprint).

**Notes**
- The RTC sits on the header corner (pins 1–6); the display connects via its
  8-pin cable to the SPI + control pins, so the two coexist with no pin overlap.
- The e-ink layout mirrors the 264×176 web preview: big SoC, battery bar,
  charge/consume state, Current/Voltage/Power/Time tiles, and a 48 h SoC
  sparkline. Grayscale (the panel does 4 levels) is a planned refinement.

### 2026-07-25 — resilience & a Go rewrite

**Added**
- **WiFi watchdog** daemon: self-heals a wedged Broadcom (`brcmfmac`) radio on
  the Pi Zero without a physical power-cycle — escalates only on sustained loss
  (NetworkManager reconnect → driver reload → reboot, with a 30-minute reboot
  cooldown). Travel-safe: it won't reboot-loop when simply out of range of home
  WiFi. Logs events plus an hourly heartbeat.
- **Persistent journald** (size-capped) so the cause of a failure survives a
  reboot.
- **Multi-resolution e-ink size preview** as the root page: the dashboard
  rendered side-by-side at 800×480 / 400×300 / 250×122 in true 1-bit black &
  white, to judge how small a panel is still usable.
- **`batteryweb-go`** — a Go rewrite of the dashboard server: a single static
  ARMv6 binary, drop-in for the Flask app, reading SQLite read-only via the
  pure-Go `modernc.org/sqlite` driver. ~15 ms warm API responses (was seconds),
  ~18 MB RSS (was ~33 MB). Cross-compiles to the Pi Zero with no Docker and no
  cross toolchain (`webserver/build.sh`).

**Changed**
- The root page is now the size preview; the single-panel dashboard is retired
  for this development phase.
- Dashboard server rewritten Flask → Go. `webapp.py` is kept as a documented
  one-line fallback in the systemd unit.

**Fixed**
- **`/api/data` performance:** downsampled the 48 h series (~900 KB → ~43 KB),
  switched to integer-offset local day/night bucketing (no per-row timezone
  conversions), added a 30 s server-side cache, and made the client refresh
  self-scheduling. This removed a single-core thrash spiral that appeared as
  history grew past a few days.
- Documented the BMS "remaining time" register: it reports time-to-**empty**
  while discharging and time-to-**full** while charging.

### 2026-07-20 / 21 — initial build

**Added**
- **BLE acquisition** of the ECTIVE AccuBox 120S over a reverse-engineered
  Modbus-RTU-over-BLE protocol — the Modbus slave id is derived from the
  device's advertised name. Self-contained; contains none of the vendor app's
  code.
- **SQLite (WAL) logger**, poll-and-disconnect every 15 s, as a boot-persistent
  systemd service that survives WiFi drops and abrupt power cuts.
- **e-ink-style 800×480 monochrome dashboard** + a read-only JSON API (the web
  layer opens the DB read-only so it never blocks the logger).
- **Net daily-Ah bars** (trapezoidal integration of the net battery current)
  with gap accounting, and an **overnight-load metric** derived from the BMS's
  own remaining-Ah delta across the local night window — the trustworthy
  "will we make it to morning?" figure.
- **Clock resilience without an RTC:** `fake-hwclock` (15-minute saves), a
  per-sample `synced` flag, aggregates that ignore pre-2023 timestamps, and a
  "clock unsynced" badge.
- **WiFi tuning** for the shared WiFi/BT radio: power-save disabled, plus an
  rfkill-unblock on logger start (fixes Bluetooth being blocked after a cold
  boot).

**Changed**
- Store timestamps as **UTC epoch**; bucket days and nights in explicit
  Europe/Amsterdam local time so it doesn't depend on the Pi's system timezone.
- Dashboard visual language: ▲/▼ arrows for charging/consuming, a diverging
  daily-energy chart (in above the baseline, out below), streamlined tiles.
- Poll interval reduced from 60 s to 15 s.
