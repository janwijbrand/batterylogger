# Hand-off 02 — The Pi Zero has no clock (RTC handling)

**Project:** AccuBox battery logger (`batterijtje`). Affects the **logger** (timestamps written to the DB) and, downstream, the dashboard's day bucketing.

## The problem

The Pi Zero W has **no RTC** — no battery-backed timekeeping. Time comes from NTP the moment it has network. At home that's instant. **Off-grid** (in the van, in Värmland), a reboot or a yanked USB cable — and remember the supply is the AccuBox's USB port, which has a user-facing off switch — brings the Zero up either at **epoch 0 (1970)** or, if `fake-hwclock` is installed, at the **last saved time, frozen** until network returns.

This matters more than a cosmetic clock error, because the **logger writes `ts` from `time.time()` at sample time**. If the clock is wrong when a sample is written, the **database itself gets bad timestamps** — not just the display. Downstream, `datetime.fromtimestamp()` day bucketing then smears samples into the wrong local day or into a phantom 1970 bucket, and the "48 h" window and "samples since" become misleading.

## Requested changes

**Tier A — minimum, do this now (software only):**

1. Confirm `fake-hwclock` is installed and enabled so timestamps stay monotonic and roughly right across a reboot:
   ```bash
   systemctl status fake-hwclock
   cat /etc/fake-hwclock.data
   ```
   If missing: `sudo apt install fake-hwclock`.

2. **Guard the data pipeline against pre-sync timestamps.** In the logger, treat any sample whose `ts` is implausibly old (e.g. `ts < 1_700_000_000`, ~Nov 2023) as **clock-not-yet-synced**. Options, in order of preference:
   - Hold samples in memory and don't commit until the clock looks sane (NTP synced), **or**
   - Write them but mark them: add an `unsynced` boolean column (or a `clock_ok` flag), so the dashboard can exclude/annotate them.
   In `api_data()`, filter out or visually flag unsynced rows so a 1970 bucket can never appear in the daily bars.

3. Optionally check sync state directly: `timedatectl show -p NTPSynchronized`.

**Tier B — proper fix (~€3 of hardware):**

Add a **DS3231 RTC** on the I²C header — battery-backed, ±2 min/year, the natural companion to a logger that lives off-grid.
- Wiring: SDA, SCL, 3V3, GND on the 40-pin header (4 wires). Add a footprint in the FreeCAD enclosure.
- Enable: `raspi-config` → enable I²C; add `dtoverlay=i2c-rtc,ds3231` to `/boot/firmware/config.txt`; **disable** `fake-hwclock` (`sudo apt remove fake-hwclock`) so `hwclock` reads the DS3231 on boot.
- Verify: `sudo hwclock -r` returns correct time with no network; reboot with WiFi off and confirm the clock survives.

## Acceptance

- Reboot the Zero **off-grid** (WiFi unreachable). No sample with a 1970/pre-2023 timestamp ever lands in the daily bars.
- With the DS3231 fitted: `hwclock -r` is correct after a network-less reboot, and DB timestamps are continuous across the reboot.
