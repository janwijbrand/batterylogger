#!/usr/bin/env python3
"""Build a fake-but-plausible battery.db for the README shots.

A 120 Ah LiFePO4 bank in a van on modest-solar days: draws down overnight,
recovers through the day, currently charging in the late afternoon. The numbers
are invented — they exist to show the layout, and the README captions say so.

Regenerate the shots with:

    python3 docs/demo-data.py /tmp/demo.db
    cd batterylogger/eink && go build -o /tmp/be .
    /tmp/be -db /tmp/demo.db -nopaint -mono -png docs/dashboard-render.png -scale 8
    /tmp/be -db /tmp/demo.db -screen sysinfo -nopaint -mono -png docs/sysinfo-render.png -scale 8

and to put the same frame on the real panel for a photo (stop the daemon first,
start it again afterwards):

    sudo systemctl stop batteryeink
    ./batteryeink -db /tmp/demo.db -mono
"""
import sqlite3, sys, os, time

path = sys.argv[1]
BANK = 120.0

# SoC keypoints across a local day (hour -> %). Deliberately not a perfect
# sine: the overnight slope is steady (fridge), the daytime one is solar.
KEY = [(0, 74), (6, 58), (9, 61), (16, 80), (20, 90), (24, 74)]

# Per-day solar multiplier, most recent last: a bright day, then a duller one,
# so the two visible cycles aren't identical. Real data is never symmetric.
SUN = [1.0, 0.95, 1.05, 0.9, 1.0, 1.1, 0.72, 0.95, 0.88]

def soc_at(h, sun):
    for (h0, s0), (h1, s1) in zip(KEY, KEY[1:]):
        if h0 <= h <= h1:
            v = s0 + (s1 - s0) * (h - h0) / (h1 - h0)
            if s1 > s0:                      # a charging leg scales with the sun
                v = s0 + (v - s0) * sun
            return v
    return KEY[-1][1]

if os.path.exists(path):
    os.remove(path)
con = sqlite3.connect(path)
con.execute("""CREATE TABLE samples(ts INTEGER PRIMARY KEY,
  voltage REAL, current REAL, power INTEGER, soc INTEGER, remaining_ah INTEGER,
  direction INTEGER, rt_hours INTEGER, rt_min INTEGER, raw TEXT, synced INTEGER)""")

now = int(time.time())
now -= now % 60                      # tidy timestamp for the header
rows = []
for i in range(8 * 24 * 60, -1, -1):  # 8 days of one-minute samples
    ts = now - i * 60
    lt = time.localtime(ts)
    h = lt.tm_hour + lt.tm_min / 60
    sun = SUN[(lt.tm_yday) % len(SUN)]
    soc = soc_at(h, sun)
    rising = soc_at(min(h + 0.5, 24), sun) > soc
    socI = int(round(soc))
    rem = int(round(BANK * soc / 100))
    if rising:                        # solar in
        cur, volt, direction = 4.35, 13.6, 0
        left = BANK - rem
        mins = int(left / cur * 60)
    else:                             # fridge + lights out
        cur, volt, direction = -1.72, 12.9, 1
        mins = int(rem / abs(cur) * 60)
    rows.append((ts, volt, cur, int(round(volt * cur)), socI, rem, direction,
                 mins // 60, mins % 60, "", 1))
con.executemany("INSERT INTO samples VALUES (?,?,?,?,?,?,?,?,?,?,?)", rows)
con.commit()
last = rows[-1]
print(f"{path}: {len(rows)} rows; now soc={last[4]}% rem={last[5]}Ah "
      f"{'charging' if last[6]==0 else 'discharging'} {last[2]}A {last[1]}V "
      f"rt={last[7]}h{last[8]:02d}")
