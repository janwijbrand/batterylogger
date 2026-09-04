<!-- SPDX-FileCopyrightText: 2026 Jan-Wijbrand Kolman -->
<!-- SPDX-License-Identifier: MIT -->

# batterijtje enclosure

A parametric OpenSCAD enclosure for the Pi Zero + 2.7" e-Paper HAT + DS3231
stack. Two printed parts: a **tray** (back, walls, board mounts) and a **bezel**
(front frame with the display window and the KEY column).

Status: **v0.1, proof of concept.** Every board dimension is either a vendor
spec, a published nominal, or scaled off a photo — see the `[SPEC]`/`[NOM]`/
`[PHOTO]`/`[GUESS]` tags in `params.scad`. Nothing here has been test-printed.

## Files

| file | what it is |
| --- | --- |
| `params.scad` | every dimension, one block. The orientation convention is documented here — read it first. |
| `boards.scad` | mock volumes of the real hardware. Not printed; they exist to be checked against. |
| `enclosure.scad` | `tray()`, `bezel()`, and the feature modules. |
| `tests.scad` | clearance assertions — the overlap between things that must not touch. |
| `Makefile` | the iteration loop. |

## The loop

```
make views          # 7 PNGs into build/ — the fast look-at-it pass
make test           # 7 clearance assertions; PASS means zero overlap
make dbg T=3        # render what test 3 is complaining about, in red, in context
make stl            # build/tray.stl, build/bezel.stl
```

`make test` intersects each printed part with each board mock grown by 0.3 mm.
Any resulting geometry at all is a fit bug. `contact_features()` in
`enclosure.scad` lists the only places plastic is *allowed* to touch a board
(standoff seats, locating pegs, the anti-flex rib, the RTC pads, the bezel's
limit stops); tests subtract it first. Keep that list short — if you find
yourself adding to it, something is probably wrong.

The tests only prove the design is self-consistent **with the numbers in
`params.scad`**. They cannot know whether those numbers match the hardware.
That is what the first test print is for.

## Design decisions

- **The HAT carries the stack.** Three pillars rise from the tray floor to the
  HAT's own mounting holes, with pegs that locate it laterally. The Pi hangs
  underneath on the 40-pin header, steadied by a low rib under its far edge.
  Assembly is: drop the stack in, fit the bezel, four screws.
- **Four M2.5 screws, front to back**, into corner bosses. No snap-fits: they
  need iterations to tune and they fatigue on a case that gets opened. Opening
  the case is the intended route to the microSD and the RTC battery, so there
  are no separate slots for either.
- **The Zero's USB edge faces into the middle of the box**, not at a wall — the
  HAT is 85x56 and the Zero is 65x30, so the Zero sits under one end. The power
  lead runs in a channel to the far wall and is pinched by two hooks at the exit.
- **Antenna:** the vent cluster in the floor sits under the Zero's antenna
  corner, so there is no solid plastic there. Nothing metal goes in the case.
- **Print orientation:** tray open-side-up, bezel face-down. No supports —
  every overhang is a chamfer or a hole that widens downward.

## Open questions

Only heights remain — photographs are poor at Z. See the caliper list in the
session notes; the critical one is `header_h` (Pi PCB top to HAT PCB underside,
still the published 11.0 nominal), because it sets the whole interior height.

## What the photographs settled

Two shots on white paper, measured rather than eyeballed:

**`ref/IMG_6566.png`** (front, shutdown screen). `RenderMessage` draws
`rectOutline(img, 0, 0, W-1, H-1)` — a line exactly on the active-area
boundary — so the panel calibrates the photograph with no free parameters.
Verified by re-projecting: the deduced board outline lands on the PCB edges and
every hole and key-cap marker hits its target.

- HAT PCB measures **84.85 x 55.44** against a spec of 85.0 x 56.0. Confirmed.
- `disp_x` 17.0, `disp_y` 11.6.
- The three `hat_holes` sit on a **58 x 49 grid — the full-size-Pi pattern**,
  not the Zero's 58 x 23, so they do *not* line up with the Pi underneath.
- `btn_pitch` is **13.46**, not the 12.7 ("5 x 0.1 inch") read off an earlier
  blurry photo. That was wishful thinking; all four caps now agree.

**`ref/IMG_6567.png`** (back). Calibrated on the Pi's own mounting holes, a
58 x 23 mm rectangle by spec; the two independent axes agreed to 0.8%.

- `pi_off_x` = **17.2** (was guessed at 20.0). The Pi sits ~2.8 mm inside the
  HAT's far edge, not flush with it.
- `pi_off_y` = 0: the header edges are flush, as the HAT standard implies.

**`ref/IMG_6573.png`** (edge-on, camera level with the board). Self-calibrating
off the key pitch: the caps stand ~1.1 mm proud of the glass, giving
`btn_cap_h` ~2.7 rather than the 2.0 guessed.

## Shot list for the next photo pass

The measurement pipeline calibrates on the dashboard tiles, not on a ruler, so
what limits it is edge contrast — a navy PCB edge against a dark table, in its
own shadow, is where the remaining ~1 mm of error comes from.

- Board on **white paper**. Biggest single win: every PCB edge gets a hard
  boundary, and the board-to-shell offset stops being the loose number.
- **Soft even light**, no hard shadow touching the board edge.
- **Stand back and zoom** rather than moving close — flattens the keystone and
  the ~0.3 mm offset between the glass plane and the PCB plane.
- **Leave the dashboard on screen**: the four tiles are the calibration target.
- Ruler optional now; if included, lay it *on* the board, not on the table.

Still entirely unphotographed, and currently guessed from published nominals:

- the **Pi side** — `pi_off_x`, the micro-USB PWR position, the antenna corner;
- an **edge-on view of the assembled stack** — total height, and `btn_cap_h`;
- the **DS3231 on its leads**, for the pocket.

`btn_cap_h` is better taken with calipers than from a photo: it is a height, so
a top-down image barely constrains it, and it decides whether the bezel clears
the key caps or lands on them.

Drop new photos in `enclosure/ref/` and re-run the measurement pass.
