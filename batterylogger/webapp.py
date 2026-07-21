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

app = Flask(__name__)


def q(sql, args=()):
    con = sqlite3.connect(f"file:{DB_PATH}?mode=ro", uri=True, timeout=5)
    con.row_factory = sqlite3.Row
    try:
        return [dict(r) for r in con.execute(sql, args)]
    finally:
        con.close()


@app.route("/api/data")
def api_data():
    now = int(time.time())
    latest = q("SELECT * FROM samples WHERE ts>=? ORDER BY ts DESC LIMIT 1", (MIN_VALID_TS,))
    series = q(
        "SELECT ts,voltage,current,power,soc,remaining_ah FROM samples "
        "WHERE ts>=? ORDER BY ts",
        (now - 48 * 3600,),
    )
    rows = q("SELECT ts,current,remaining_ah FROM samples WHERE ts>=? ORDER BY ts",
             (now - 7 * 86400,))

    # Per-day Ah, trapezoidally integrated from the *net* battery current, with
    # gap bookkeeping so a low bar is explainable (totals are a floor, not exact).
    days = {}
    for a, b in zip(rows, rows[1:]):
        day = datetime.datetime.fromtimestamp(a["ts"], LOCAL_TZ).strftime("%Y-%m-%d")
        d = days.setdefault(day, {"in": 0.0, "out": 0.0, "gaps": 0, "gap_seconds": 0})
        dt = b["ts"] - a["ts"]
        if dt <= 0:
            continue
        if dt > GAP_CAP:                       # missing data across a BLE/power outage
            d["gaps"] += 1
            d["gap_seconds"] += dt
            continue
        i_avg = (a["current"] + b["current"]) / 2.0    # trapezoidal: average both ends
        ah = i_avg * dt / 3600.0
        if ah >= 0:
            d["in"] += ah
        else:
            d["out"] += -ah
    daily = [
        {"day": k, "in": round(v["in"], 2), "out": round(v["out"], 2),
         "gaps": v["gaps"], "gap_seconds": v["gap_seconds"]}
        for k, v in sorted(days.items())
    ]

    # Overnight consumption from the BMS's own remaining-Ah delta across the local
    # night window — independent of current integration/offset. The clean "will we
    # make it to morning?" number. Valid only if nothing charges overnight
    # (true unless shore-powered while parked).
    nights = {}
    for r in rows:
        lt = datetime.datetime.fromtimestamp(r["ts"], LOCAL_TZ)
        if lt.hour >= NIGHT_START or lt.hour < NIGHT_END:
            key = (lt.date() if lt.hour >= NIGHT_START
                   else lt.date() - datetime.timedelta(days=1)).isoformat()
            n = nights.get(key)
            if n is None:
                nights[key] = {"first": r["remaining_ah"], "last": r["remaining_ah"]}
            else:
                n["last"] = r["remaining_ah"]
    overnight = [{"night": k, "out_ah": max(0, v["first"] - v["last"])}
                 for k, v in sorted(nights.items())]
    now_local = datetime.datetime.fromtimestamp(now, LOCAL_TZ)
    last_night = None
    for k in sorted(nights, reverse=True):      # most recent night whose 06:00 has passed
        morning = datetime.datetime.combine(
            datetime.date.fromisoformat(k) + datetime.timedelta(days=1),
            datetime.time(NIGHT_END, 0), LOCAL_TZ)
        if now_local >= morning:
            last_night = {"night": k,
                          "out_ah": max(0, nights[k]["first"] - nights[k]["last"])}
            break
    meta = q("SELECT COUNT(*) c, MIN(ts) mn FROM samples WHERE ts>=?", (MIN_VALID_TS,))[0]
    unsynced = q(
        "SELECT COUNT(*) c FROM samples WHERE ts>=? AND synced=0", (now - 48 * 3600,)
    )[0]["c"]
    return jsonify(
        {
            "now": now,
            "latest": latest[0] if latest else None,
            "series": series,
            "daily": daily,
            "overnight": overnight,
            "last_night": last_night,
            "total_rows": meta["c"],
            "since": meta["mn"],
            "unsynced_recent": unsynced,
        }
    )


@app.route("/")
def index():
    return send_from_directory(WEB, "index.html")


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=8080, threaded=True)
