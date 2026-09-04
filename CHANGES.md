# Changelog

All notable changes to **batterijtje** are recorded here. The format follows
[Keep a Changelog](https://keepachangelog.com/); versioning will follow
[SemVer](https://semver.org/) once there's a tagged release. The project aims
for a **1.0** after it has logged a full multi-day off-grid trip — until then
everything lives under _Unreleased_.

Data timestamps are UTC; the dates below are local (Europe/Amsterdam).

## [Unreleased]

Working toward 1.0.

### 2026-09-04 — new README shots

**Changed**
- **The README images are refreshed** for the bitmap font, the wifi icon and the
  partial-refresh work: a new photo of the panel, a matching pixel-for-pixel
  render of the same frame, and the KEY4 system screen alongside the buttons
  section.
- The frames show **representative data generated for the shot**, and the captions
  say so. The previous captions described real home-float readings; these numbers
  are invented, and a README shouldn't imply otherwise.

**Added**
- `batteryeink -wifioff` previews the dashboard as it looks with the wifi icon
  absent, which is otherwise only reachable by toggling WiFi on the real thing.

### 2026-09-04 — a wifi icon, back in the header

**Changed**
- **The wifi indicator is an icon again, next to the title.** The bitmap font had
  pushed it down to the sparkline label row, because at a fixed 7 px per character
  the words "wifi off" don't fit beside a 147 px `?clk` timestamp — but a 21x11
  icon does, with room to spare even in that worst case. The sparkline label row
  goes back to just the window label and the night figure.
- **Off is shown by the icon's absence**, not by a crossed-out icon. A
  strike-through needs a white gutter either side to read as crossing rather than
  merging, and at this size that gutter eats the arcs it crosses; every slashed
  variant came out looking like a smudge. Absence is unambiguous where a slash
  was not. (The attempts are in the history of `wifiOffArt` if anyone wants to
  re-litigate it on a bigger panel.)
- Icons are hand-placed 1-bit art (`wifiOnArt` in `render.go`), not rasterised
  geometry: at this size every pixel is a design decision and a generated arc is
  mush. Draw it, render it at `-scale 8`, look at it, adjust.

### 2026-09-04 — `?clk` stops crying wolf

**Fixed**
- **`clock_synced()` now trusts the DS3231**, not just NTP. It tested one thing —
  whether systemd-timesyncd had synced this boot — which stopped being the right
  question when the RTC went in. Since the daemon deliberately boots with WiFi
  **off** (KEY3 enables it), a power-cut on a trip meant every sample was flagged
  unsynced and the panel showed `?clk` indefinitely while the clock was in fact
  correct — a permanent warning, i.e. one you learn to ignore. A sample is now
  trusted if NTP has synced **or** `/dev/rtc0` is present and the clock reads
  past `MIN_VALID_TS`. A dead coin cell is still caught: the DS3231 then hands
  back an epoch-ish date, which fails that same test.

### 2026-09-03 — partial (non-flashing) refresh

**Changed**
- **The daemon paints mono with partial refresh.** A periodic repaint no longer
  flashes: the controller's DU waveform only moves pixels where the new frame
  differs from the one on the glass. Measured on the panel: **0.99 s for a
  partial vs 1.88 s for a full mono refresh vs 6.01 s for the old 4-grey path.**
  Button presses are the real win — KEY2 cycling the window now answers in about
  a second instead of six.
- **`Sleep()` no longer blocks for 2 s.** Waveshare spends the post-deep-sleep
  settle delay immediately; we record when sleep started and pay whatever is left
  of it at the next reset. The panel still gets its quiet 2 s, but an idle daemon
  spends them idling and a button press 10 minutes later waits for none of them.
  That delay was most of the wall-clock cost of a paint (2.98 s -> 0.99 s).
- **Every `Display*` call is now self-contained** — it runs its own init, so the
  caller only does `Display*(...)` then `Sleep()`. Previously `Display` needed a
  separate `Init()` while the new partial calls did their own; one rule is harder
  to get wrong, and a partial that skipped its reset would paint through whatever
  state the last call left behind.
- **KEY1 now forces a base (flashing) refresh.** With partial updates a repaint
  can legitimately be skipped when nothing changed, which would leave "force
  refresh" looking dead. It doubles as the on-demand way to clear ghosting.

**Added**
- `epdpartial.go`: `DisplayBase` (paints and seeds *both* RAM planes),
  `DisplayPartial` (restores the previous frame to `0x26` from host memory before
  writing `0x24`), `setFullWindow`, and the fast-waveform variants. Waveshare's
  `display_Partial()` writes only `0x24` and trusts the panel's `0x26`; we sleep
  after every paint and deep sleep stops refreshing that RAM, so the previous
  frame is always re-sent from our own copy.
- The paint policy (`paint.go`) spends a full refresh on: the first paint, any
  change of screen kind (dashboard / sysinfo / message — a partial across two
  different layouts is where DU looks worst), every 6 partials, once a day, and
  on KEY1. An unchanged frame is skipped entirely.
- `-partialtest` drives a scripted sequence (clock tick, big diff, screen change,
  a run past the base counter) through the real policy with `-interval`, and
  `-dumpcmds` prints the exact command stream a paint would emit without any
  hardware — ordering is what makes or breaks the partial path and it can't be
  seen from a photo.
- `-4gray` keeps the old path reachable for comparison on the same panel.

**Not adopted**
- `init_Fast` + `0xC7` for base refreshes (`-fastbase`, default off): measured
  **slower** at 2.30 s, because that init spends two SWRESETs and two extra
  activate-and-wait cycles before painting. Kept only so the finding can be
  re-checked.
- Host-side cold gating of partial refresh. The controller compensates for
  temperature itself; a host policy on top has nothing to do until sub-zero
  touring is on the table. Noted in `handoff-05-partial-refresh.md`, including
  the trap that `/sys/class/hwmon/hwmon*` is not stably ordered — the DS3231 is
  reachable at `/sys/class/rtc/rtc0/device/hwmon/hwmon*/temp1_input`.

### 2026-09-03 — a bitmap font for the small text

**Changed**
- **The small faces are now a bitmap font** (`basicfont.Face7x13`) instead of
  9 px / 11 px `goregular`. An outline font that small can't resolve its stems
  on the pixel grid, and the only reason it looked acceptable was the 4-grey
  antialiasing propping it up. A bitmap face is already a 1-bit stencil, so it
  stays sharp when thresholded — which is what unblocks a mono render path, and
  with it **partial (non-flashing) refresh**: 4-grey and partial are mutually
  exclusive on this controller, since 4-grey uses RAM `0x26` as its second
  bit-plane while partial needs it to hold the previous image. The large text
  (SoC number, tile values, headings) stays outline — it thresholds cleanly.
- **The wifi tag moved from the header to the sparkline label row.** At a fixed
  7 px per character the header can't hold `batterijtje` + the tag + a
  `2026/09/03 15:04 ?clk` timestamp (147 px). It sits at a fixed x so it doesn't
  shift between "wifi on" and "wifi off" — a field that moves when it changes is
  the enemy of a partial-update diff region.
- **`blackThreshold` 176 → 128.** The 176 bias existed to keep antialiased 9 px
  stems alive; with the small text now threshold-independent it only fattened
  the 16 px tile values (the counters of "8" and the apex of "A" filled in).
- The strings drawn in the small faces are ASCII now (`·` and `–` became `-`,
  the KEY4 hint got shorter) — a bitmap face has no glyph outside 0x20-0x7e and
  would otherwise paint U+FFFD boxes. The offline frame's error string is
  truncated to the panel width instead of running off the edge.
- The paint path is **unchanged**: the daemon still paints 4-grey. This change
  only makes mono good enough to switch to.

**Added**
- `batteryeink -screen sysinfo|message` renders the KEY4 and power-off frames
  off-hardware, so `-nopaint -png` can check all three screens.

### 2026-08-30 — the 4 HAT buttons; e-ink becomes a daemon

**Added**
- The **2.7″ HAT's 4 buttons** now do things (BCM 5/6/13/19, polled + debounced):
  - **KEY1** force-refresh
  - **KEY2** cycle the sparkline window: 24 h / 48 h / 7 d
  - **KEY3** WiFi on/off — **radio only**, so Bluetooth/BLE polling never stops;
    also pauses the `wifi-watchdog` while off, and shows a header indicator
  - **KEY4** tap = a system-info screen (uptime, IP, CPU temp, disk, RTC);
    hold 3 s = graceful power-off
- The header timestamp already shows wall-clock time; the sparkline label now
  reflects the selected window.

**Changed**
- **`batteryeink` is now a daemon** (`Type=simple`, `Restart=always`) instead of
  a timer-triggered one-shot — it holds button state, repaints every 10 min, and
  reacts to presses (rapid presses coalesce into one repaint). The
  `batteryeink.timer` is retired.

### 2026-08-30 — e-ink goes standalone; web dashboard retired

**Changed**
- **`batteryeink` now reads `battery.db` directly** (read-only, pure-Go
  `modernc.org/sqlite`) instead of fetching `/api/data` over HTTP — so the e-ink
  panel no longer needs a running server.

**Removed**
- The **web dashboard** (`webapp.py` / `batteryweb-go` / `web/index.html` and the
  `batteryweb` service) — no longer needed now that the panel is the display and
  the renderer is self-sufficient. For a remote look, `batteryeink -nopaint -png`
  dumps the current frame. (All recoverable from git history.)

The running setup is now just: `batterylogger` (BLE→SQLite), `batteryeink.timer`
(paint the panel), and `wifi-watchdog` (keep the Pi reachable).

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

- **4-grey rendering**: text is anti-aliased into the panel's 4 grey levels
  (ported `Init_4Gray` + its LUT + a dual-plane `0x24`/`0x26` write) for
  noticeably smoother glyphs; `-mono` forces 1-bit. 4-grey refresh ~8 s.

**Notes**
- The RTC sits on the header corner (pins 1–6); the display connects via its
  8-pin cable to the SPI + control pins, so the two coexist with no pin overlap.
- The e-ink layout mirrors the 264×176 web preview: big SoC, battery bar,
  charge/consume state, Current/Voltage/Power/Time tiles, and a 48 h SoC
  sparkline.

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
