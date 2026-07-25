#!/usr/bin/env python3
"""Tiny read-only dashboard for the AccuBox logger DB.

Serves an e-ink-style dashboard (prototype for the future Waveshare display)
and a JSON API. Opens the SQLite DB read-only so it never blocks the logger
(WAL mode allows concurrent reader + writer).
"""
import datetime
import os
import sqlite3
import time
from zoneinfo import ZoneInfo

from flask import Flask, jsonify, send_from_directory

BASE = os.path.dirname(os.path.abspath(__file__))
DB_PATH = os.path.join(BASE, "battery.db")
WEB = os.path.join(BASE, "web")
GAP_CAP = 300  # seconds; ignore integration across gaps longer than this
MIN_VALID_TS = 1_700_000_000  # ~2023-11-14; rows below this had an unset clock
# Timestamps are stored UTC (epoch seconds); a "day" is defined in the van's local
# time. Explicit here so bucketing does not depend on the Pi's system tz setting.
LOCAL_TZ = ZoneInfo("Europe/Amsterdam")
NIGHT_START, NIGHT_END = 22, 6  # local-time window for the overnight-load metric
CACHE_TTL = 30    # seconds; the heavy 7-day aggregation changes slowly, so cache it
SERIES_MAX = 480  # downsample the 48h series to at most this many points (sparkline)

app = Flask(__name__)

# In-process cache for the expensive part of /api/data (everything except `latest`,
# which stays fresh). Keyed on now // CACHE_TTL so it recomputes at most every TTL.
_heavy = {"key": None, "val": None}


def q(sql, args=()):
    con = sqlite3.connect(f"file:{DB_PATH}?mode=ro", uri=True, timeout=5)
    con.row_factory = sqlite3.Row
    try:
        return [dict(r) for r in con.execute(sql, args)]
    finally:
        con.close()


def _compute_heavy(now):
    """The expensive part of the API response (series + 7-day aggregation).

    Buckets days/nights in local time using a single fixed UTC offset instead of a
    per-row ZoneInfo conversion (~24k rows on the Zero). The offset can differ across
    a DST switch inside the 7-day window — twice a year one night's bucket shifts an
    hour, acceptable for this metric. Result is cached by the caller (CACHE_TTL).
    """
    off = int(LOCAL_TZ.utcoffset(
        datetime.datetime.fromtimestamp(now, LOCAL_TZ)).total_seconds())
    _daystr = {}

    def day_num(ts):                    # local day number (days since 1970-01-01)
        return (ts + off) // 86400

    def day_str(n):                     # cached; only ~7 distinct days in the window
        s = _daystr.get(n)
        if s is None:
            s = (datetime.date(1970, 1, 1) + datetime.timedelta(days=n)).isoformat()
            _daystr[n] = s
        return s

    series = q(
        "SELECT ts,voltage,current,power,soc,remaining_ah FROM samples "
        "WHERE ts>=? ORDER BY ts",
        (now - 48 * 3600,),
    )
    if len(series) > SERIES_MAX:        # thin for the sparkline, always keep the last point
        step = len(series) // SERIES_MAX + 1
        series = series[::step] + [series[-1]]

    rows = q("SELECT ts,current,remaining_ah FROM samples WHERE ts>=? ORDER BY ts",
             (now - 7 * 86400,))

    # Per-day Ah, trapezoidally integrated from the *net* battery current, with gap
    # bookkeeping so a low bar is explainable (totals are a floor, not exact). Same
    # pass tracks the overnight remaining-Ah delta (the clean "will we make it?" figure).
    days, nights = {}, {}
    for i, a in enumerate(rows):
        h = ((a["ts"] + off) % 86400) // 3600
        if h >= NIGHT_START or h < NIGHT_END:              # in the night window
            nk = day_str(day_num(a["ts"]) - (0 if h >= NIGHT_START else 1))
            n = nights.get(nk)
            if n is None:
                nights[nk] = {"first": a["remaining_ah"], "last": a["remaining_ah"]}
            else:
                n["last"] = a["remaining_ah"]
        if i + 1 >= len(rows):
            continue
        b = rows[i + 1]
        d = days.setdefault(day_str(day_num(a["ts"])),
                            {"in": 0.0, "out": 0.0, "gaps": 0, "gap_seconds": 0})
        dt = b["ts"] - a["ts"]
        if dt <= 0:
            continue
        if dt > GAP_CAP:                                    # missing data across an outage
            d["gaps"] += 1
            d["gap_seconds"] += dt
            continue
        ah = (a["current"] + b["current"]) / 2.0 * dt / 3600.0   # trapezoidal
        if ah >= 0:
            d["in"] += ah
        else:
            d["out"] += -ah

    daily = [
        {"day": k, "in": round(v["in"], 2), "out": round(v["out"], 2),
         "gaps": v["gaps"], "gap_seconds": v["gap_seconds"]}
        for k, v in sorted(days.items())
    ]
    overnight = [{"night": k, "out_ah": max(0, v["first"] - v["last"])}
                 for k, v in sorted(nights.items())]
    now_local = datetime.datetime(1970, 1, 1) + datetime.timedelta(seconds=now + off)
    last_night = None
    for k in sorted(nights, reverse=True):      # most recent night whose 06:00 has passed
        morning = datetime.datetime.combine(
            datetime.date.fromisoformat(k) + datetime.timedelta(days=1),
            datetime.time(NIGHT_END, 0))
        if now_local >= morning:
            last_night = {"night": k,
                          "out_ah": max(0, nights[k]["first"] - nights[k]["last"])}
            break
    meta = q("SELECT COUNT(*) c, MIN(ts) mn FROM samples WHERE ts>=?", (MIN_VALID_TS,))[0]
    unsynced = q(
        "SELECT COUNT(*) c FROM samples WHERE ts>=? AND synced=0", (now - 48 * 3600,)
    )[0]["c"]
    return {
        "series": series,
        "daily": daily,
        "overnight": overnight,
        "last_night": last_night,
        "total_rows": meta["c"],
        "since": meta["mn"],
        "unsynced_recent": unsynced,
    }


@app.route("/api/data")
def api_data():
    now = int(time.time())
    latest = q("SELECT * FROM samples WHERE ts>=? ORDER BY ts DESC LIMIT 1", (MIN_VALID_TS,))
    key = now // CACHE_TTL
    if _heavy["key"] != key or _heavy["val"] is None:
        _heavy["val"] = _compute_heavy(now)
        _heavy["key"] = key
    return jsonify({"now": now, "latest": latest[0] if latest else None, **_heavy["val"]})


@app.route("/")
def index():
    # During this dev phase the root page is the multi-resolution e-ink preview
    # (800x480 / 400x300 / 250x122 side by side), rendered 1-bit from live data.
    return send_from_directory(WEB, "index.html")


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8080, threaded=True)
