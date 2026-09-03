# Hand-off 05 — Mono paint path + partial (non-flashing) refresh

**Project:** AccuBox battery logger (`batterijtje`). Touches `batterylogger/eink/epd.go` (small additions), a new `epdpartial.go`, and `RunDaemon`'s `show()` in `daemon.go`. Unlike 04, this one **needs the hardware** — the interesting failure modes only appear on glass.

**Why:** hand-off 04 made the 1-bit render legible, which was the prerequisite. Now the paint path can leave 4-grey, and once it's mono the controller's differential refresh becomes available: updates that don't flash and take a fraction of a full refresh.

The two are mutually exclusive. 4-grey uses RAM `0x26` as its second bit-plane; partial refresh needs `0x26` to hold the image currently on the glass so the DU waveform can diff against it. Picking mono is what buys the partial path.

## 1. Add the partial-refresh primitives

New file `epdpartial.go`, alongside the existing `epd.go` / `epd4gray.go` split. Three additions:

- `turnOnPartial()` — `0x22` with `0xFF`, then `0x20`. (`0xF7` is the full waveform, `0xC7` loads no LUT at all — that's the 4-grey path with its own uploaded table.)
- `setFullWindow()` — sets `0x44`/`0x45` ranges and `0x4E`/`0x4F` counters to the whole panel. `Init()` currently relies on post-SWRESET defaults for `0x44`/`0x4E`; being explicit here costs six bytes and removes an assumption.
- `DisplayBase(buf)` — writes `buf` to **both** `0x24` and `0x26`, then a normal full refresh. This is the flashing baseline paint.
- `DisplayPartial(prev, next)` — `Init()`, border waveform `0x3C` ← `0x80`, write `prev` to `0x26`, write `next` to `0x24`, then `turnOnPartial()`.

**The `prev` write is the whole trick and must not be optimised away.** Waveshare's `display_Partial()` writes only `0x24` and trusts whatever is left in `0x26`. That works in their demo loop because it never sleeps. We call `Sleep()` (`0x10`/`0x01`) after every paint, and deep-sleep stops refreshing that RAM, so it decays into noise — the corruption Hugo Chargois traced on the 7.5" V2 (`thoughts.gohu.org/posts/2025/epaper-partial-updates/`). Keeping the last frame host-side and re-sending it means we never depend on panel RAM surviving anything.

Both buffers are full frames. The DU waveform only moves pixels that differ, so there's no need to compute a bounding box — skip the window arithmetic in Waveshare's version entirely.

**Deliberate deviation, worth a comment in the code:** the vendor's `display_Partial()` opens with a bare `reset()`, where `DisplayPartial` calls the full `Init()` (reset + SWRESET + Y-range + data entry). That isn't gratuitous defensiveness — their demo never sleeps, so the panel is still configured from the last `init()`. We deep-sleep after every paint, so the configuration has to be re-established regardless. SWRESET also clears the border waveform and data-entry registers, which is why `0x3C` and `setFullWindow()` come after it and not before. Per the SSD1680 datasheet SWRESET leaves RAM alone, but since both planes get rewritten on every call it doesn't matter either way — that's the property worth preserving if anyone refactors this later.

## 2. Switch `show()` to mono and track the previous frame

In `RunDaemon`:

```go
const (
    maxPartials = 6              // -> a full refresh at least hourly at the 10-min tick
    fullEvery   = 24 * time.Hour // and once a day regardless
)

type screenKind int // dashboard, sysinfo, message

var (
    lastBuf  []byte
    lastKind screenKind
    partials int
    lastFull time.Time
)

show := func(img *image.Gray, kind screenKind) {
    buf := GetBufferFromImage(img)
    if lastBuf != nil && bytes.Equal(buf, lastBuf) {
        return // nothing moved; don't touch the panel
    }
    needFull := lastBuf == nil ||
        kind != lastKind ||
        partials >= maxPartials ||
        time.Since(lastFull) > fullEvery
    if needFull {
        epd.DisplayBase(buf)
        partials, lastFull = 0, time.Now()
    } else {
        epd.DisplayPartial(lastBuf, buf)
        partials++
    }
    epd.Sleep()
    lastBuf, lastKind = buf, kind
}
```

Notes:

- **The `kind != lastKind` rule matters more than the counter.** A DU update between two near-identical dashboards moves a handful of pixels; a DU between a dashboard and the sysinfo screen moves most of the panel, and that's where partial refresh looks worst — smeared, half-erased remnants of the previous layout. Every screen transition gets a clean full refresh.
- **`RenderSysInfo` and `RenderMessage` go through the same `show()`**, so they inherit the mono path and participate in the base chain automatically. Don't special-case them beyond passing the right `kind` — a 4-grey info screen would silently break the next partial.
- The KEY4-hold `bye` message is the last thing painted before shutdown, and it then sits on the glass unpowered for however long the van is parked. The kind change forces it to be a full refresh, which is what you want: nothing half-driven baking in for a month.
- The identical-frame guard buys nothing today (the header timestamp changes every tick) but costs nothing and covers the button-press paths, where repeated KEY1 presses currently repaint unconditionally.
- Log the elapsed time per paint and which mode was used, the way the one-shot path in `main.go` already does. Quantifying full-vs-partial is half the point of the exercise.

## 3. Try a fast base refresh (the middle gear)

Partials are cheap; the every-`maxPartials` base refresh is the expensive step. There's a third gear worth measuring: `init_Fast` + `0x22` `0xC7`. Still a flashing full refresh, but a markedly quicker one.

How it works is worth understanding before using it. `init_Fast` reads the built-in sensor (`0x18` ← `0x80`, then `0x22` ← `0xB1`, `0x20`) and then **overwrites the result** with `0x1A` ← `0x64 0x00` — forcing the temperature register to 100 °C — before loading the LUT with `0x22` ← `0x91`. Hotter ink needs a shorter waveform, so claiming the panel is scorching is how you get a fast refresh out of it. `TurnOnDisplay_Fast`'s `0xC7` then loads neither temperature nor LUT; it just runs what init already installed.

So implement `DisplayBaseFast(buf)` as: `init_Fast`, `setFullWindow`, write `0x24`, write `0x26`, `0x22` `0xC7` + `0x20`. Same both-planes contract as `DisplayBase` — the base's job is establishing `0x26`, and that doesn't change.

Put it behind a `-fastbase` flag (default off) and compare against the `0xF7` base on the same panel: specifically, whether it clears accumulated ghosting as thoroughly. A short waveform has less time to settle the ink, and clearing ghosts is the entire reason the base refresh exists. If it clears as well, it's free; if it doesn't, the slow base is doing real work.

The catch is that a forced 100 °C waveform is exactly the under-drive risk when the panel is actually cold. Summer van, no issue. It's the one place where "the built-in compensation is doing something for you" stops being abstract.

## 4. Keep 4-grey reachable

Don't delete `epd4gray.go` or the 4-grey branch in `main.go`. Add a `-4gray` daemon flag (default off) so the modes can be compared on the same panel minutes apart.

## 5. Temperature gating is out of scope

The controller compensates on its own for every `0xF7` refresh, so a host-side cold check would be policy layered on top, not a substitute — and with sub-zero touring off the list there's nothing for that policy to do yet. Deliberately not implemented.

Recorded for whenever it becomes interesting: the DS3231's sensor is the one to read, since it shares a box with the panel, and the stable path is `/sys/class/rtc/rtc0/device/hwmon/hwmon*/temp1_input`. Do **not** glob `/sys/class/hwmon/hwmon*` and take the first match — hwmon numbering isn't stable, and on this Pi the first match is `cpu_thermal`, which reads ~16 °C hotter than the RTC and would gate on entirely the wrong thing.

## 6. Build a `-partialtest` harness

04 was verifiable on the couch; this isn't, which is exactly why it needs tooling. Two pieces, and the cheap one is the off-hardware one.

**Off-hardware: golden-file the command stream.** The waveform can't be simulated, but the bytes going out over SPI can be captured. If the SPI handle isn't already behind an interface, extract one, then add a recording implementation that logs `cmd`/`data` calls to a text file. A test then asserts that `DisplayBase` and `DisplayPartial` emit the expected sequence — that `0x26` is written before `0x24`, that `setFullWindow()` reappears between the two plane writes, that `0x3C` lands after SWRESET rather than before, that the byte counts are 5808 each. All the ordering constraints in section 1 become regressions that fail in CI instead of manifesting as speckle on a panel three weeks later. Diff the recorded stream against the vendor Python once by hand to confirm the golden file is right in the first place.

**On-hardware: a scripted sequence.** `-partialtest` should paint a series of synthetic frames — a ticking clock, an SoC ramp, a forced screen-kind change, a forced base refresh — through the real `show()` policy, with a `-interval` flag rather than waiting on the 10-minute tick. It must call the real `Sleep()` between frames; skipping that is the one shortcut that would hide the bug this design exists to prevent.

Be aware of what the harness can't compress: RAM decay needs wall-clock time, so a run at `-interval 10s` proves the sequencing works and proves nothing about the sleep problem. Keep the long idle run in the acceptance list below regardless of how clean the fast run looks.

## 7. Optional, measure first

`Sleep()` blocks for 2 s after every paint, mirroring Waveshare's `sleep()`. With a partial refresh at a few hundred ms, that delay dominates the button-press latency. The next operation does a hardware reset anyway, so a shorter delay is probably safe — but treat it as a separate experiment, changed on its own, and back it out at the first sign of flakiness. Panel-longevity behaviour is not worth guessing at to save a second.

## Acceptance

On hardware:

- A periodic tick updates the timestamp **without flashing**; the rest of the frame is visibly untouched.
- KEY2 cycling 24h/48h/7d feels responsive rather than ceremonial. This is the real win — the tick is unattended, the buttons are not.
- **The sleep test:** let the daemon idle through at least three periodic ticks (30+ min, so several sleep/wake cycles) and confirm partials are still clean. Speckle, inverted patches, or ghosts of much older frames mean the `0x26` restore isn't landing — that's the failure this design exists to prevent, and it will not show up in a fast manual test.
- After `maxPartials` updates a full refresh visibly flashes and the panel comes back clean; no ghost accumulation across a full day.
- `-fastbase` measured against the default base with timings logged for both, and a note in the PR on whether the fast base clears ghosting as completely. "Faster" isn't the criterion — "faster and still clean" is.
- The KEY4 info screen and the KEY4-hold `bye` message both render correctly, and every transition between screen kinds is a full refresh with no remnant of the previous layout.
- The golden-file test passes and fails loudly if the plane write order is swapped — verify by deliberately breaking it once.
- Refresh intervals stay ≥180 s and the panel gets at least one full refresh per 24 h, per Waveshare's guidance. The 10-minute tick and hourly base satisfy both — don't tighten the tick in this hand-off.
- `CHANGES.md` updated under _Unreleased_.
