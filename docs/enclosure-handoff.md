<!-- SPDX-FileCopyrightText: 2026 Jan-Wijbrand Kolman -->
<!-- SPDX-License-Identifier: MIT -->

# Enclosure design handoff — batterijtje (OpenSCAD)

**Purpose:** bootstrap a focused session to design a 3D-printable **enclosure in
OpenSCAD** for the batterijtje device. The user has OpenSCAD experience and will
render / print / test-fit / iterate; your job is to write idiomatic **parametric**
`.scad`, explain the parametrisation, and help with the fiddly geometry.

You (the assistant) **cannot measure the hardware** — treat every dimension below
as either a *known nominal to verify* or a *must-measure*. Ask the user for
caliper readings and photos early.

---

## What batterijtje is (context)

An always-on **Raspberry Pi Zero W v1.1** that logs a campervan LiFePO4 battery
(ECTIVE AccuBox 120S) over BLE into SQLite and renders a dashboard onto a **2.7″
e-Paper HAT**, driven by a pure-Go daemon (`batteryeink`). A **DS3231 RTC** keeps
time across power-cuts. The device firmware is **done**; the enclosure is the last
hardware task before a real off-grid trip (→ 1.0). Full context in the repo
`README.md` and `CLAUDE.md`.

Repo: `github.com/janwijbrand/batterylogger`.

---

## The assembly to enclose

Stack, bottom → top:

1. **Raspberry Pi Zero W v1.1** — the base board.
2. **Waveshare 2.7″ e-Paper HAT (V2, 264×176)** — seated on the Pi's **40-pin
   header**, display facing **up/out**. Four tactile buttons **KEY1–KEY4** along
   one short edge of the HAT.
3. **DS3231 RTC (Adafruit PiRTC)** — *not* on the header. It's a separate small
   board joined by **4 short flying leads soldered to the Pi's underside**
   (header pins 1/3/5/6 = 3V3/SDA/SCL/GND), secured with a dab of **hot-glue**.
   It needs its own home in the enclosure, and the leads are **fragile** — the
   design must not stress them.

Note: an earlier 8-pin ribbon is **no longer used** (the HAT is on the header now)
— no cable to route, just the RTC's short leads.

### Openings / access needed
- **Display window** — the e-ink active area, on the top (HAT) face.
- **4 buttons** — access on the HAT's edge (cutouts over the caps, or printed plungers).
- **Micro-USB power** — on the Pi edge (the **PWR** port, not the data one).
- *(optional)* green **ACT LED** visible / light-pipe.
- *(optional)* **microSD** access (short edge) for reflashing.

---

## Dimensions

### Known nominals — VERIFY against the actual boards
- **Pi Zero W:** 65.0 × 30.0 mm; ~5 mm tall with components (more with the header).
  4× **M2.5** mounting holes on a **58 × 23 mm** rectangle, centres **3.5 mm** in
  from each edge.
- **40-pin header** raises the HAT roughly **11 mm** above the Pi (2×20 @ 2.54 mm
  pitch; solderless hammer header + the HAT's female socket).
- **2.7″ e-ink active area ≈ 57.3 × 38.2 mm** (Waveshare spec — the visible
  window). Its **offset within the HAT PCB must be measured**.
- Antenna: the Zero's onboard antenna is at **one corner** (silk-marked, near the
  microSD/PWR end). See the keep-out note below.

### MUST MEASURE (user, with calipers)
- HAT **PCB outline** (L × W × thickness) and how far it **overhangs** the Pi.
- **Display window** position/offset within the HAT outline (all four margins).
- The **4 button** centre positions (from a reference corner) + button-cap height.
- HAT **mounting-hole** positions (if used for standoffs).
- **Micro-USB PWR** port position + height on the Pi edge.
- **Antenna corner** location + how much clear space to reserve.
- **DS3231 board** dimensions, where its leads exit, and slack length.
- **Total stack height**: Pi underside → top of buttons / display surface.

---

## Design goals & constraints

- **⚠️ Antenna keep-out (learned the hard way):** with the HAT sitting over the
  Zero, its PCB **shadows the Zero's onboard WiFi antenna** and WiFi degrades
  badly. Reserve a **pocket / avoid dense plastic (and any metal) over the antenna
  corner**; vent or thin that region if possible. (The firmware now defaults WiFi
  **off** on boot and only enables it via a button, which mitigates the need — but
  don't make it worse.)
- **Display window** clear and flush; a small inner **lip** can retain the panel
  edge if the front frame holds the stack.
- **Buttons** reachable — cutouts over the tactile switches, or printed plungers.
- **Strain relief** at the micro-USB power lead.
- **RTC + leads**: a pocket/clip; do **not** tension the fragile solder joints.
- **Mounting in the van**: undecided — make it a parameter (screw tabs / flat back
  for adhesive-or-VESA / bracket). **Ask the user** which they want.
- **Ventilation**: the Pi runs ~40 °C but the van can be warm — a few slots.
- **Print-friendly (FDM):** ~2.5 mm walls, ~0.3 mm clearances, design to avoid
  supports (e.g. keep the display window on the top face; no steep overhangs).

---

## Suggested OpenSCAD structure

- **Fully parametric** — one variables block at the top; every feature derives
  from it. Two printed parts:
  - a **base tray** that locates the Pi+HAT stack (standoffs to the Pi's or HAT's
    mount holes) with the micro-USB and RTC pockets, and
  - a **front bezel/lid** with the display window + button openings that clips or
    screws on.
- Modules per feature: `pi_stack()`, `display_window()`, `button_holes()`,
  `usb_cutout()`, `antenna_pocket()`, `rtc_pocket()`, `mount_tabs()`.
- Fix an **orientation convention once** (which edge is +X, where the display/
  buttons/USB/antenna live) and comment it — most bugs come from losing that.
- Export an STL per part; expect **2–3 fit iterations** (clearances, button reach,
  window position) as the user test-prints.

### Starter skeleton (fill dims after measuring)
```scad
// batterijtje enclosure — parametric. Units: mm. VERIFY all dims.
$fn = 48;

// --- boards ---
pi_l = 65; pi_w = 30; pi_h = 5;         // Pi Zero
hat_l = 0; hat_w = 0; hat_h = 0;        // MEASURE: HAT PCB outline
stack_h = 11;                           // header height Pi->HAT (verify)
disp_w = 57.3; disp_h = 38.2;           // e-ink active area (verify)
disp_off_x = 0; disp_off_y = 0;         // MEASURE: window offset in HAT
// button centres (MEASURE), micro-USB pos (MEASURE), antenna corner (MEASURE)

// --- shell ---
wall = 2.5; clr = 0.3; lid_h = 0;

module base_tray() { /* standoffs, walls, usb + rtc pockets, antenna pocket */ }
module front_bezel() { /* display window + button openings */ }

base_tray();
// front_bezel();
```

---

## How to work with the user

- Write **idiomatic parametric** OpenSCAD and explain the variables so the user
  can tweak. They render (F5/F6) and print; iterate on their fit reports.
- **Ask for photos early** — the Pi+HAT on the header, the RTC on its flying
  leads, and the underside solder. (They exist in the originating session's
  history; the user can re-share, or take fresh ones.)
- Put the `.scad` + exported `.stl` under a new **`enclosure/`** dir in the repo;
  commit iterations.

## First steps for the new session
1. Ask for the caliper measurements listed above (or start from the known nominals
   + Waveshare 2.7″ HAT reference and refine).
2. Confirm the **mounting method**, and whether **microSD/LED** access is wanted.
3. Scaffold the **base tray** first; fit-check standoffs + USB cutout + antenna
   pocket; then add the bezel with the display window + button openings.
