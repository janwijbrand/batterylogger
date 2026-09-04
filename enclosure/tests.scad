// SPDX-FileCopyrightText: 2026 Jan-Wijbrand Kolman
// SPDX-License-Identifier: MIT
//
// Clearance assertions.  Each case renders the OVERLAP between two solids that
// must never touch; an empty result (0 facets) is a pass.  `make test` runs
// them all.  This is the cheap half of the fit check — it cannot know whether
// the numbers in params.scad match the real hardware, only whether the design
// is self-consistent with them.

include <boards.scad>
use <enclosure.scad>

// a part with its intended board contacts removed — what must never touch
module tray_clr()  { difference() { tray();  contact_features(); } }
module bezel_clr() { difference() { bezel(); contact_features(); } }

test = 0;
name_only = false;

g = 0.3;   // required air gap between plastic and board

names = [
    "tray vs HAT",              // 0
    "tray vs Pi Zero",          // 1
    "tray vs DS3231",           // 2
    "bezel vs HAT",             // 3
    "bezel vs Pi Zero",         // 4
    "tray vs bezel",            // 5
    "display window unblocked", // 6
    "RTC leads vs HAT underside",// 7
];

module case(n) {
    if      (n == 0) intersection() { tray_clr();  mock_hat(g); }
    else if (n == 1) intersection() { tray_clr();  mock_pi(g); }
    else if (n == 2) intersection() { tray_clr();  mock_rtc(g); }
    else if (n == 3) intersection() { bezel_clr(); mock_hat(g); }
    else if (n == 4) intersection() { bezel_clr(); mock_pi(g); }
    else if (n == 5) intersection() { tray();  bezel(); }
    // board-vs-board: the RTC's lead connector must clear the HAT above it
    else if (n == 7) intersection() { mock_rtc(g); mock_hat(0); }
    // the whole active area must have clear line of sight through the bezel
    else if (n == 6) intersection() {
        bezel();
        translate([ring + disp_x, ring + disp_y, split_z - 1])
            cube([disp_w, disp_h, front_t + 2]);
    }
}

// `debug=true` paints the offending overlap red inside a ghost of the parts,
// so a failing case can be LOOKED at instead of guessed at.
debug = false;

if (name_only) echo(names[test]);
else if (debug) { %tray(); %bezel(); %mock_stack(0); color("red") case(test); }
else case(test);
