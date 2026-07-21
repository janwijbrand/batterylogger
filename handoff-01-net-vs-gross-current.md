# Hand-off 01 — Daily bars are *net*, not gross (labeling + overnight metric)

**Project:** AccuBox battery logger (`batterijtje`). Touches `api_data()` in the Flask backend and the web/e-ink labels.

## The problem

The AccuBox exposes **one** current reading, measured at the battery terminal. It is the *net* of everything happening at once:

```
I_net = I_charge − I_load        (positive = into battery)
```

When solar/shore charging and a load (fridge, the Zero itself) run **at the same time**, the load is silently netted against the charge. Consequences for the current daily algorithm:

- **Daily "in"** = ∫ positive net current = gross charge **minus** whatever load ran during charging → **undercounts true harvest**.
- **Daily "out"** = ∫ |negative net current| = load only during **non-charging** periods → **undercounts true consumption**.

Both errors equal the same hidden quantity: energy drawn by loads *while charging was active*. This is inherent to single-point measurement — it **cannot** be fixed in software. Splitting true gross solar from gross load would need a second shunt on the charge/solar line (e.g. a Victron SmartShunt does **not** help — still one measurement point).

## What *is* clean

Whenever there is **zero charge source** (overnight, panels shaded, no shore power), net current = pure load. So the overnight portion of "out" is a trustworthy measure of nighttime consumption — usually the number that actually answers "will we make it to morning?"

## Requested changes

1. **Relabel so the bars don't over-claim.** They are net flows, not gross solar/load. Change the legend from `▲ IN (SOLAR/CHARGE)  ▼ OUT (USED)` to something honest like `▲ NET IN  ▼ NET OUT`, and add a one-line tooltip/footnote: "net at battery terminal; loads during charging are not separated."

2. **Add an overnight-load metric** (the clean number). Integrate `out` Ah only within a configurable local-time night window (default e.g. 22:00–06:00), reported per day as `overnight_out_ah`. State the assumption in a comment: *valid only if nothing charges overnight* (true unless shore-powered while parked). Surface it on the dashboard as e.g. "overnight load: 24 Ah".

   Sketch (inside the existing daily loop, reusing `a`, `b`, `dt`, `ah`):
   ```python
   local = datetime.datetime.fromtimestamp(a["ts"])
   is_night = local.hour >= 22 or local.hour < 6
   if ah < 0 and is_night:
       d["overnight_out"] += -ah
   ```
   Add `overnight_out` to the `days.setdefault(...)` dict and to the emitted `daily` records.

3. **Document the limitation** in `CLAUDE.md` / README so future-you (and future sessions) don't try to "fix" the netting.

## Out of scope

True gross in/out split — requires additional hardware (second shunt on the solar/charge feed). Note it as a possible future upgrade, don't attempt it in code.

## Acceptance

- Legend no longer implies gross solar/load.
- On a mixed sunny+load day, `overnight_out_ah` is present and matches expectation (roughly the fridge + Zero draw over the night hours).
