# Hand-off 03 — Daily-Ah integration refinements

**Project:** AccuBox battery logger (`batterijtje`). All changes live in the daily-Ah loop of `api_data()`.

Baseline is already correct in one important way: it integrates on **actual elapsed time** (`dt = b.ts - a.ts`) rather than assuming fixed spacing, and `GAP_CAP` prevents holding stale current across a long outage. These are refinements on top of that.

## 1. Trapezoidal integration (free accuracy)

Currently uses the left endpoint only:
```python
ah = a["current"] * dt / 3600.0
```
`b` is already in hand, so use the average of both endpoints:
```python
i_avg = (a["current"] + b["current"]) / 2.0
ah = i_avg * dt / 3600.0
```
Removes left-endpoint bias. Marginal at 30 s spacing, but it matters slightly more here than usual: near a **sign change** (charge↔discharge) the left endpoint can dump a whole interval into the wrong bucket. Trapezoidal splits it more fairly.

## 2. DC-offset deadband — but calibrate first (do NOT hardcode blindly)

Integration is unforgiving of any zero-offset in the current sensor: a steady phantom +0.03 A integrates to ~0.7 Ah/day of "in" **from nothing**. A small deadband (`|i| < threshold → 0`) cleans it up.

**Critical constraint:** the threshold must sit **below the genuine baseline load**. The Zero itself draws ~1 W from the AccuBox USB port ≈ **~0.09 A** at the battery — a *real* discharge that should be counted. A deadband set at 0.1 A would erase the logger's own load and make overnight consumption read zero when it shouldn't.

Because the Zero is always drawing, you can't observe a truly "idle" bus. Calibrate instead: during a period you're confident **only the Zero** is connected (no fridge, no other loads), the reading should equal the known Zero draw (~0.09 A discharge). Any deviation from that expected value **is** the offset.

Suggested, conservative:
```python
DEADBAND = 0.05  # A; MUST be below the real baseline load (~0.09 A for the Zero)
if abs(i_avg) < DEADBAND:
    i_avg = 0.0
```
Verify against reality before trusting it — don't deadband away genuine tiny loads.

## 3. Gap accounting (so totals are honestly a floor)

`GAP_CAP` drops any interval > 300 s entirely. This biases daily totals **low** (misses charge/discharge across the gap) — the *safe* direction (never invents energy), but it makes the totals a **floor**, not an exact figure. If BLE outages turn out frequent on the road, you'll want to know how much you're missing.

Add per-day gap bookkeeping in the same loop:
```python
if dt > GAP_CAP:
    d = days.setdefault(day, {...})
    d["gaps"] += 1
    d["gap_seconds"] += dt
    continue
```
Surface `gaps` / `gap_seconds` in the `daily` records and, optionally, a small "N gaps today" note on the dashboard so a suspiciously low bar has an explanation.

## Acceptance

- An idle day (only the Zero connected, no other loads) shows daily `in` ≈ 0 and `out` ≈ the Zero's own draw — **not** an inflated "in" from sensor offset.
- Daily totals sanity-check against a known event (e.g. a measured overnight fridge run).
- Days with BLE dropouts show a nonzero `gaps` count, so low bars are explainable rather than mysterious.
