# Campervan battery logger

See @van-battery-logger-brief.md.

## Where it runs
Everything lives on the Pi Zero (`username@hostname.local`) under `~/batterylogger/`:
`logger.py` (BLE poll → SQLite), `batteryeink` (Go: reads SQLite → paints the 2.7"
e-Paper HAT), `battery.db` (SQLite/WAL). systemd units: `batterylogger.service`,
`batteryeink.service` (a daemon: 10-min repaint + the 4 HAT buttons),
`wifi-watchdog.service`. A DS3231 RTC keeps the clock across
power-cuts. (An earlier Flask/Go web dashboard on :8080 was retired — the panel is
the display now; `batteryeink -nopaint -png` gives a remote look.) Dev happens
off-box; deploy the logger with `scp`, the Go binary via `eink/build.sh` + `scp`.

## Render notes
- **The small text is a bitmap font** (`basicfont.Face7x13`), the big text an outline
  face. That's deliberate: outline glyphs at 9-11 px only looked right thanks to
  4-grey antialiasing, and needing that antialiasing is what blocks partial refresh
  (4-grey uses RAM `0x26` as its second bit-plane; partial needs it for the previous
  image). **Keep every `fTiny`/`fSmall` string ASCII** — the face has no glyph outside
  0x20-0x7e and silently paints U+FFFD boxes. Layout is 7 px per character; check new
  strings against `-nopaint -mono -png x.png -scale 4` before believing they fit.

## Data notes (read before "fixing" these)
- **Net measurement, by design.** The AccuBox exposes **one** current at the battery
  terminal = charge − load. Gross solar and gross load **cannot** be separated in
  software (needs a second shunt on the charge line). Daily bars are therefore *net*
  in/out — labelled as such. Don't try to "fix" the netting in code.
- **Overnight load is the clean number.** With no charge source overnight, net = pure
  load, so `last_night` (from the BMS `remaining_ah` delta over 22:00–06:00 local) is a
  trustworthy "will we make it to morning?" figure. Assumes nothing charges overnight.
- **Timestamps are UTC epoch** in the DB (`ts`); the web layer localises with
  `LOCAL_TZ = Europe/Amsterdam` for day/night bucketing. Matches the `../energy` / `../zonde`
  convention. Don't store local time.
- **RTC fitted.** A DS3231 on I²C (`dtoverlay=i2c-rtc,ds3231`) keeps the clock across
  power-cuts, replacing `fake-hwclock`; each sample still carries a `synced` flag
  (the header shows `?clk` when it's unset) and aggregates ignore pre-2023 timestamps.
