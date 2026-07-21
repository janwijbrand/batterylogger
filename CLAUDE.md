# Campervan battery logger

See @van-battery-logger-brief.md.

## Where it runs
Everything lives on the Pi Zero (`username@hostname.local`) under `~/batterylogger/`:
`logger.py` (BLE poll → SQLite), `webapp.py` + `web/index.html` (Flask dashboard, port 8080),
`battery.db` (SQLite/WAL). systemd units: `batterylogger.service`, `batteryweb.service`.
Dev happens off-box; deploy with `scp` then `systemctl restart`.

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
- **No RTC.** `fake-hwclock` (15-min saves) keeps the clock roughly right across
  power-cuts; each sample carries a `synced` flag; aggregates ignore pre-2023 timestamps.
  A DS3231 on I²C is the eventual proper fix.
