// SPDX-FileCopyrightText: 2026 Jan-Wijbrand Kolman
// SPDX-License-Identifier: MIT
//
// batterijtje enclosure — parameters.  Units: mm.  Everything derives from here.
//
// SOURCE TAGS — how much to trust each number:
//   [SPEC]   vendor specification
//   [NOM]    published nominal for the part family
//   [MEAS]   photogrammetry from ref/IMG_6563.jpeg.  The four dashboard tiles
//            are drawn at known panel pixels, so they calibrate the photo; the
//            header's 2.54 mm pitch then measured back to 2.542 (+0.09%), which
//            is the scale check.  Positions RELATIVE to the display are good to
//            ~0.3 mm; the board's own edges are only good to ~1 mm, so the
//            board-to-shell offset is the loose one — the 5 mm ring absorbs it.
//   [PHOTO]  eyeballed off docs/dashboard-eink.jpg  (±1 mm)
//   [GUESS]  plausible placeholder, MUST be measured before a final print
//   (untagged = a design choice, tune freely)
//
// ORIENTATION CONVENTION — fix it once, comment it, never argue with it again:
//
//        +Y  far edge (Pi's USB/HDMI edge points this way, into the box)
//         ^
//         |   +---------------------------------------+
//         |   | [K1]                                  |
//         |   | [K2]   +-------------------+          |   HAT PCB, 85 x 56
//         |   | [K3]   |  e-ink active     |          |   seen from the FRONT
//         |   | [K4]   +-------------------+          |   (display facing you)
//         |   |            <-- Pi Zero underneath -->  |
//         |   +---------------------------------------+
//         |    ^ x=0: button edge          header edge is y=0 (bottom)
//         +--------------------------------------------> +X
//
//   +Z is up, out of the display face.  z=0 is the INTERIOR FLOOR of the tray.
//   HAT coordinates are measured from the HAT PCB's (x=0,y=0) corner;
//   hat_to_int() converts them into interior/shell coordinates.

$fn = 64;

/* ---------------------------------------------------------------- boards -- */

// CONFIRMED against ref/IMG_6566.png: measured 84.85 x 55.44, i.e. the vendor
// figure within the ~0.3 mm the edge threshold eats.  (The earlier 82.7 x 57.0
// came from the dark-table photo and was wrong.)
hat_l        = 85.0;    // [SPEC/MEAS] Waveshare 2.7" e-Paper HAT PCB, long axis
hat_w        = 56.0;    // [SPEC/MEAS] short axis
hat_t        = 1.6;     // [GUESS] PCB thickness
hat_corner_r = 3.0;     // [GUESS] PCB corner radius

// HAT mounting holes, in HAT coords. Only THREE — the 4th corner is under the
// panel glass.  [PHOTO]
// [MEAS] three holes; the fourth corner is under the panel glass.
// These are the HAT's OWN holes on a 58 x 49 grid -- the full-size-Pi pattern,
// not the Zero's 58 x 23.  They do not line up with the Pi underneath.
hat_holes   = [[23.3, 2.7], [81.8, 2.8], [81.3, 51.7]];
hat_hole_d  = 2.8;      // [MEAS] 2.4-2.8 across the three
// Only the FIRST hole gets a close-fitting locating peg.  The other two are
// loose: three tight pegs would over-constrain the board, and my hole positions
// carry ~0.7 mm of measurement noise between them.
peg_fit     = 0.35;     // undersize of the locating peg vs the hole
peg_loose   = 1.3;      // undersize of the other two

pi_l  = 65.0;  pi_w = 30.0;  pi_t = 1.2;   // [NOM/MEAS] Raspberry Pi Zero W v1.1
pi_off_x = 17.2;   // [MEAS] ref/IMG_6567.png, +/-0.2.  The Pi is NOT flush with
                   // the HAT's far edge -- it sits ~2.8 mm inside it.
pi_off_y =  0.0;   // [MEAS] measured 0.5; flush within the threshold error

// The Pi's microSD end.  Determines where the antenna and the microSD sit.
// true  = microSD at LOW x  (the button/overhang end)
// false = microSD at HIGH x (the far short edge)
pi_sd_low_x = false;    // [GUESS] ASK THE USER — flips antenna + SD side

// [MEAS] the tallest thing under the Pi is the shrinkwrapped RTC lead bundle at
// 3.2, barely past the microSD holder's 2.5 -- so the hot-glued bundle costs
// 0.7 mm, not the 4-5 feared, and needs no relief pocket in the floor.
pi_under   = 3.2;
// [MEAS] 16.0 from the edge-on photo (HAT PCB back 2.9 -> Pi PCB front 18.6),
// 16.6 from the caliper sandwich once the bundle is subtracted.  That is 5 mm
// MORE than the canonical 11.0 HAT spacing -- if the HAT presses down further
// onto the header, drop this and the case gets 5 mm thinner.
header_h   = 16.3;

// Pi connector edge (y = pi_w), positions measured from the Pi's microSD end.
// [MEAS] PWR is the 3rd connector, 55.0 from the microSD short edge -- i.e.
// only 10 mm from the FAR end, so it sits at the button end of the HAT.
pi_usb_pwr_x = 55.0;
pi_usb_w     = 8.0;     // [MEAS] micro-B receptacle shell width
pi_usb_h     = 3.0;     // [MEAS] shell height above PCB

/* ------------------------------------------------------------- e-ink area -- */

disp_w = 57.288;  disp_h = 38.192;   // [SPEC] active area, 264x176 @ 0.217 mm
disp_x = 17.0;    disp_y = 11.6;     // [MEAS] active-area origin in HAT coords
panel_t = 1.3;                       // [MEAS] glass + adhesive above the PCB

/* ---------------------------------------------------------------- buttons -- */

btn_x     =  3.25;  // [MEAS] centre line of the KEY column, from the HAT edge
btn_y0    =  6.35;  // [MEAS] KEY4 (lowest) centre
// NOT 12.7 (5 x 0.1").  That was wishful reading of a blurry photo; all four
// caps measured on a calibrated frame give 13.40 / 13.53 / 13.44.
btn_pitch = 13.46;  // [MEAS]
btn_n     = 4;
// [MEAS] caliper 4.1 from the back face, minus hat_t.  The caps stand 1.2 mm
// proud of the GLASS, so the bezel must clear the caps, not the panel.
btn_cap_h = 2.5;

/* -------------------------------------------------------------------- RTC -- */

rtc_l = 21.2;  rtc_w = 20.0;  rtc_h = 7.6;   // [MEAS] battery clip to chip
// Whatever carries the four leads onto the RTC, measured from its PCB.
// [MEAS] 18.0 = the current vertical DuPont stack, the worst case.
//   right-angle DuPont ~6 | JST-PH ~8 (RA ~6) | soldered direct ~3
// The case height does NOT depend on this -- header_h sets it -- so lowering
// it buys vibration resistance, not volume.
rtc_conn_h = 18.0;
rtc_conn_l = 11.0;  rtc_conn_w = 5.5;
rtc_z = 0.8;                                 // rests on pads, clear of the floor

/* ------------------------------------------------------------------ shell -- */

wall      = 2.5;    // side walls
floor_t   = 2.5;    // tray floor (the back of the device)
front_t   = 2.5;    // bezel front plate
ring      = 5.0;    // interior gap around the HAT PCB — also hosts the corner bosses
clr       = 0.4;    // general print clearance

out_r     = 8.5;    // exterior corner radius
chamfer   = 1.5;    // top + bottom edge chamfer

// fasteners: 4 x M2.5 self-tapping, front to back, into corner bosses
scr_d      = 2.1;   // pilot bore for a self-tapping M2.5
scr_head_d = 5.0;   // counterbore in the bezel
scr_head_h = 1.6;
boss_d     = 5.0;
boss_c     = 3.8;   // boss centre, along the 45-degree diagonal from the interior corner

// van mounting — undecided, so it is a mode.  Ears are used for both screwed
// modes so no fastener head ever ends up inside the case.
//   "none"    flat back, for VHB / Command strips
//   "tabs"    two flanged ears, through-holes, screwed to a panel
//   "keyhole" two flanged ears with keyhole slots — hangs on two screws, lifts off
mount      = "none";
mount_scr  = 3.5;   // screw shank the ears take
mount_head = 6.5;   // head that must pass through the keyhole's round end

// features (flip these off to isolate geometry while iterating)
show_vents  = true;
show_rtc    = true;
show_usb    = true;

/* --------------------------------------------------------------- derived -- */

int_l = hat_l + 2*ring;              // interior footprint
int_w = hat_w + 2*ring;
ext_l = int_l + 2*wall;
ext_w = int_w + 2*wall;
in_r  = out_r - wall;

hat_z    = pi_under + pi_t + header_h;   // HAT PCB underside, above the floor
hat_top  = hat_z + hat_t;
panel_top= hat_top + panel_t;
split_z  = panel_top + 0.5;              // tray/bezel parting plane
ext_h    = floor_t + split_z + front_t;

// HAT coords -> interior coords
function hat_to_int(p) = [p[0] + ring, p[1] + ring];
