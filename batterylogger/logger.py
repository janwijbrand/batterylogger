#!/usr/bin/env python3
"""ECTIVE AccuBox BLE battery logger -> SQLite (poll-and-disconnect).

Protocol reverse-engineered from the ECTIVE 'BM X' Android app
(com.lipower.battery): Modbus-RTU over BLE char ffe1. The Modbus slave id
is the LAST segment of the advertised name (e.g. 23-03-28-076 -> slave 76).
Status = readHoldingRegisters(slave, 0x0400, 8); 21-byte big-endian reply.
"""
import asyncio
import logging
import os
import re
import sqlite3
import time

from bleak import BleakClient, BleakScanner

DB_PATH = os.path.join(os.path.dirname(os.path.abspath(__file__)), "battery.db")
CHAR_UUID = "0000ffe1-0000-1000-8000-00805f9b34fb"
SVC_AF30 = "0000af30-0000-1000-8000-00805f9b34fb"
NAME_RE = re.compile(r"^\d{2}-\d{2}-\d{2}-\d{1,3}$")

POLL_INTERVAL = 15.0   # seconds between the start of each poll
SCAN_TIMEOUT = 20.0
FRAME_TIMEOUT = 8.0
MIN_VALID_TS = 1_700_000_000            # ~2023-11-14; below this the clock is unset
SYNC_MARKER = "/run/systemd/timesync/synchronized"  # present once NTP has synced this boot

logging.basicConfig(level=logging.INFO, format="%(asctime)s %(levelname)s %(message)s")
log = logging.getLogger("batterylogger")


def clock_synced() -> bool:
    """True once systemd-timesyncd has completed at least one NTP sync this boot."""
    return os.path.exists(SYNC_MARKER)


def crc16_modbus(data: bytes) -> int:
    crc = 0xFFFF
    for b in data:
        crc ^= b
        for _ in range(8):
            crc = (crc >> 1) ^ 0xA001 if (crc & 1) else (crc >> 1)
    return crc


def build_status_cmd(slave: int) -> bytes:
    frame = bytes([slave, 0x03, 0x04, 0x00, 0x00, 0x08])
    crc = crc16_modbus(frame)
    return frame + bytes([crc & 0xFF, (crc >> 8) & 0xFF])  # CRC little-endian


def decode(d: bytes) -> dict:
    be = lambda i: int.from_bytes(d[i:i + 2], "big")  # Modbus registers are big-endian
    direction = be(11)                                # 0 = charging(+), else discharging(-)
    sign = 1 if direction == 0 else -1
    return {
        "remaining_ah": be(3),
        "soc": be(5),
        "rt_hours": be(7),
        "rt_min": be(9),
        "direction": direction,
        "current": round(sign * be(13) * 0.01, 2),
        "voltage": round(be(15) * 0.1, 2),
        "power": sign * be(17),
    }


def init_db() -> sqlite3.Connection:
    con = sqlite3.connect(DB_PATH)
    con.execute("PRAGMA journal_mode=WAL")
    con.execute("PRAGMA synchronous=NORMAL")
    con.execute(
        """CREATE TABLE IF NOT EXISTS samples(
             ts INTEGER PRIMARY KEY,
             voltage REAL, current REAL, power INTEGER,
             soc INTEGER, remaining_ah INTEGER, direction INTEGER,
             rt_hours INTEGER, rt_min INTEGER, raw TEXT, synced INTEGER)"""
    )
    # migrate DBs created before the `synced` column existed
    cols = [r[1] for r in con.execute("PRAGMA table_info(samples)")]
    if "synced" not in cols:
        con.execute("ALTER TABLE samples ADD COLUMN synced INTEGER")
    con.commit()
    return con


async def find_device():
    def match(d, adv):
        name = d.name or (adv.local_name or "")
        if NAME_RE.match(name):
            return True
        return SVC_AF30 in [u.lower() for u in (adv.service_uuids or [])]
    return await BleakScanner.find_device_by_filter(match, timeout=SCAN_TIMEOUT)


async def poll_once(con: sqlite3.Connection) -> bool:
    dev = await find_device()
    if dev is None:
        log.warning("device not found in scan")
        return False
    name = dev.name or ""
    try:
        slave = int(name.split("-")[-1])
    except ValueError:
        log.error("cannot derive slave id from name %r", name)
        return False

    cmd = build_status_cmd(slave)
    buf = bytearray()
    done = asyncio.Event()

    def cb(_sender, data):
        buf.extend(data)
        if len(buf) >= 21:
            done.set()

    async with BleakClient(dev, timeout=30.0) as cl:   # context mgr => poll-and-disconnect
        await cl.start_notify(CHAR_UUID, cb)
        await asyncio.sleep(0.3)
        await cl.write_gatt_char(CHAR_UUID, cmd, response=False)
        try:
            await asyncio.wait_for(done.wait(), FRAME_TIMEOUT)
        except asyncio.TimeoutError:
            log.warning("no reply within %ss (slave=%d)", FRAME_TIMEOUT, slave)
            return False

    frame = bytes(buf[:21])
    if frame[0] != slave or frame[1] != 0x03:
        log.warning("unexpected frame header: %s", frame.hex())
        return False

    v = decode(frame)
    ts = int(time.time())
    synced = 1 if (ts >= MIN_VALID_TS and clock_synced()) else 0
    if ts < MIN_VALID_TS:
        log.warning("system clock not set (ts=%d); sample flagged unsynced", ts)
    con.execute(
        "INSERT OR REPLACE INTO samples VALUES(?,?,?,?,?,?,?,?,?,?,?)",
        (ts, v["voltage"], v["current"], v["power"], v["soc"],
         v["remaining_ah"], v["direction"], v["rt_hours"], v["rt_min"], frame.hex(), synced),
    )
    con.commit()
    log.info("V=%.1f I=%.2f P=%d SoC=%d%% Ah=%d %s%s", v["voltage"], v["current"],
             v["power"], v["soc"], v["remaining_ah"],
             "chg" if v["direction"] == 0 else "dis",
             "" if synced else " [UNSYNCED]")
    return True


async def main() -> None:
    con = init_db()
    log.info("logger started, db=%s interval=%ss", DB_PATH, POLL_INTERVAL)
    while True:
        start = time.monotonic()
        try:
            await poll_once(con)
        except Exception as e:  # keep the loop alive through BLE/transient errors
            log.error("poll error: %s: %s", type(e).__name__, e)
        await asyncio.sleep(max(1.0, POLL_INTERVAL - (time.monotonic() - start)))


if __name__ == "__main__":
    try:
        asyncio.run(main())
    except KeyboardInterrupt:
        pass
