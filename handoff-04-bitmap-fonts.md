# Hand-off 04 — Bitmap fonts for the small text (1-bit render path)

**Project:** AccuBox battery logger (`batterijtje`). All changes live in `batterylogger/eink/render.go`. No hardware needed — the whole task is verifiable with `-nopaint -png`.

**Why:** the dashboard currently paints in 4-grey. The greys aren't decorative — nothing in `RenderDashboard` draws a deliberate mid-tone — they exist entirely to antialias small text that looked blocky in 1-bit. That's an outline font (`goregular`) being asked to resolve sub-pixel stems at 9 px, and antialiasing rescuing it.

A **bitmap font** removes the need for the rescue: glyphs are drawn on the pixel grid, so they're already 1-bit stencils and look sharp when thresholded. Fixing this unblocks a mono render path, which in turn unblocks **partial (non-flashing) refresh** — 4-grey and partial are mutually exclusive on this controller, because 4-grey uses RAM `0x26` as its second bit-plane whereas partial needs it to hold the previous image.

Partial refresh is a separate hand-off. This one only needs to make mono look good.

## 1. Swap the small faces to a bitmap font

Monospace is **preferred** — it suits the uppercase tile labels (`CURRENT`, `VOLTAGE`, `POWER`, `TO EMPTY` / `TO FULL`) and it makes the header timestamp stop reflowing as digits change width, which will matter later for partial-update diff regions.

Start with what's already in the dependency tree:

```go
import "golang.org/x/image/font/basicfont"

fTiny  = basicfont.Face7x13   // was mustFace(goregular.TTF, 9)
fSmall = basicfont.Face7x13   // was mustFace(goregular.TTF, 11)
```

`basicfont.Face7x13` is a `font.Face`, so `text`, `textW`, `textTop` and `textRight` need no changes. `golang.org/x/image/font/inconsolata` (`Regular8x16`, `Bold8x16`) is the alternative if 7x13 reads too small or too wide; both are true bitmap faces.

**Keep the outline fonts for the large text.** `fBig` (40 px), `fTitle` (15 px), `fPct` (18 px) and `fTileV` (16 px) threshold cleanly — glyph edges are thin relative to glyph mass. Do not replace them.

Scope check: `fTiny` and `fSmall` are used for the tile labels, the timestamp, the wifi tag, the sparkline labels, the `night N Ah` figure, `charging`/`consuming`, `collecting...`, the remaining-Ah line, the offline/error strings, and `RenderSysInfo` / `RenderMessage`. Sweep all of them.

## 2. Fix the layout where fixed-width breaks it

7 px per character is wider than 9 px `goregular` averages, so some strings will grow. Known pressure points:

- **Tile labels** at `x0+4` inside 78 px boxes. `CURRENT` = 7 chars = 49 px, fits. `TO EMPTY` = 8 chars = 56 px, fits. Verify against the box edge rather than trusting this.
- **Header**: `batterijtje` + wifi tag on the left, timestamp on the right, all on a 264 px row. The timestamp `2006/01/02 15:04` is 16 chars = 112 px, plus ` ?clk` when the clock is unsynced = 21 chars = 147 px. That is the tightest thing on the screen — check it with the `?clk` suffix present.
- **`RenderSysInfo`** lines are already written as aligned columns (`up   `, `ip   `, `cpu  `) — those were fudged for a proportional font and will now actually align. Tidy the padding.

If something genuinely won't fit, say so rather than silently shrinking: shortening the timestamp to `01/02 15:04`, or moving the wifi tag onto the sparkline row, are both acceptable and preferred over reintroducing a proportional face. Flag it in the PR description either way.

## 3. Retune `blackThreshold` afterwards

`blackThreshold = 176` biases generously toward black, which was fattening antialiased small glyphs. With the small faces now bitmap (unaffected by the threshold — they emit 0 or 255), this constant only governs the large outline text. Re-check the 40 px SoC number and the 16 px tile values at the current value and adjust for weight if they look thin or clogged.

## 4. Don't change the paint path yet

Leave `RunDaemon`'s `show()` on 4-grey. This hand-off is about making the mono render good enough to switch to; the switch itself, and partial refresh, come after.

## Acceptance

Verified entirely off-hardware:

```
./batteryeink -nopaint -mono -png mono.png -scale 4
```

- `mono.png` (which is `Bilevel()` — exactly what the panel would show) is legible at 4× with no glyph reduced to disconnected fragments.
- Every string stays inside its box or panel edge, including the timestamp with the ` ?clk` suffix and the longest window label (`7d SoC`).
- `RenderSysInfo` and `RenderMessage` also render cleanly in mono — they're painted by the same `show()` and will inherit whatever mode the dashboard uses.
- Side-by-side against `-png` without `-mono` (the 4-grey `Quantize4` preview) for a before/after in the PR.
- `CHANGES.md` updated under _Unreleased_.
