// SPDX-FileCopyrightText: 2026 Jan-Wijbrand Kolman
// SPDX-License-Identifier: MIT
//
// batterijtje enclosure — two printed parts: a tray (back + walls) and a bezel
// (front frame).  See params.scad for every dimension and the orientation
// convention.  Render one part with -D 'part="tray"' etc.
//
//   part = "tray" | "bezel" | "assembly" | "section"
//
// Print notes: tray prints open-side-up, bezel prints face-down.  No supports
// in either orientation — every overhang is a chamfer or a downward-widening
// hole.

include <boards.scad>

part = "tray";

/* ------------------------------------------------------------- primitives -- */

// a rounded-rect prism with chamfered bottom/top edges, at the origin
module cbox(l, w, h, r, cb = 0, ct = 0) {
    eps = 0.01;
    hull() {
        if (cb > 0)
            translate([cb, cb, 0]) linear_extrude(eps) rrect(l - 2*cb, w - 2*cb, max(0.1, r - cb));
        translate([0, 0, cb]) linear_extrude(max(eps, h - cb - ct)) rrect(l, w, r);
        if (ct > 0)
            translate([ct, ct, h - eps]) linear_extrude(eps) rrect(l - 2*ct, w - 2*ct, max(0.1, r - ct));
    }
}

// a stadium slot of length l and width w, lying along +X, extruded h
module slot(l, w, h) {
    linear_extrude(h) hull() { circle(d = w); translate([l - w, 0]) circle(d = w); }
}

// the four bezel/tray screw positions, on the interior corner diagonals
function screw_pts() = [[boss_c, boss_c], [int_l - boss_c, boss_c],
                        [int_l - boss_c, int_w - boss_c], [boss_c, int_w - boss_c]];

/* -------------------------------------------------------------- contacts -- */

// The ONLY places plastic is allowed to touch a board.  tests.scad subtracts
// these before checking clearances, so this list is a design statement: keep it
// short, and be suspicious of anything you have to add to it.
module contact_features() {
    for (h = hat_holes) {                       // HAT seats + locating pegs
        p = hat_to_int(h);
        translate([p[0], p[1], 0]) {
            cylinder(d = 7.0, h = hat_z + 0.01);
            cylinder(d = hat_hole_d - peg_fit + 0.01, h = hat_z + hat_t + 0.7);
        }
    }
    pi_rib();                                   // anti-flex rib under the Pi
    rtc_pads();                                 // the DS3231 rests on these
    for (h = hat_holes) {                       // bezel limit-stop pads
        p = hat_to_int(h);
        translate([p[0], p[1], hat_top + 0.4]) cylinder(d = 6.6, h = split_z - hat_top);
    }
}

module rtc_pads() {
    p = rtc_xy();
    for (dx = [2, rtc_l - 5], dy = [2, rtc_w - 5])
        translate([p[0] + dx, p[1] + dy, 0]) cube([3, 3, rtc_z]);
}

/* -------------------------------------------------------------------- tray -- */

module tray_body() {
    difference() {
        translate([-wall, -wall, -floor_t]) cbox(ext_l, ext_w, floor_t + split_z, out_r, chamfer, 0);
        translate([0, 0, 0]) linear_extrude(split_z + 1) rrect(int_l, int_w, in_r);
    }
}

module screw_bosses() {
    for (p = screw_pts()) translate([p[0], p[1], 0]) cylinder(d = boss_d, h = split_z);
}

// pillars into the HAT's own mounting holes: a seat at PCB height, plus a peg
// that locates the board laterally.  The bezel clamps it down from above.
module hat_pillars() {
    for (i = [0 : len(hat_holes) - 1]) {
        p = hat_to_int(hat_holes[i]);
        translate([p[0], p[1], 0]) {
            cylinder(d = 6.5, h = hat_z);
            cylinder(d = hat_hole_d - (i == 0 ? peg_fit : peg_loose), h = hat_z + hat_t + 0.6);
        }
    }
}

// the Pi hangs off the header; a low rib under its far edge stops it flexing
module pi_rib() {
    px = ring + pi_off_x;  py = ring + pi_off_y;
    translate([px + 6, py + pi_w - 3.5, 0]) cube([pi_l - 12, 2.0, pi_under - 0.6]);
}

// channel from the micro-USB PWR port out through the +Y wall, with a pinch
// strain relief right at the wall
module usb_channel_pos() {
    cx = ring + pi_off_x + usb_x();
    y0 = ring + pi_off_y + pi_w;
    difference() {
        union() {
            // guide ribs either side of the cable run
            for (s = [-1, 1])
                translate([cx + s*6.0 - 1.0, y0 + 4.5, 0]) cube([2.0, int_w - y0 - 4.5, 5.5]);
            // pinch hooks at the wall
            for (s = [-1, 1])
                translate([cx + s*3.1 - 0.9, int_w - 6, 0]) cube([1.8, 3.0, 4.6]);
        }
        translate([cx - 3.0, int_w - 6.2, 3.4]) cube([6.0, 3.4, 3.0]);   // hook undercut
    }
}
module usb_channel_neg() {
    cx = ring + pi_off_x + usb_x();
    translate([cx, int_w - 1, 1.2]) rotate([-90, 0, 0])
        translate([0, 0, -1]) linear_extrude(wall + 3) hull() {
            translate([0,  1.3]) circle(d = 5.4);
            translate([0, -1.3]) circle(d = 5.4);
        }
}

// three-sided well for the DS3231, open toward the Pi so the leads stay slack
module rtc_pocket() {
    p = rtc_xy();  h = 4.5;  t = 1.6;
    difference() {
        // a wall around the board footprint...
        difference() {
            translate([p[0] - t - clr, p[1] - t - clr, 0]) cube([rtc_l + 2*t + 2*clr, rtc_w + 2*t + 2*clr, h]);
            translate([p[0] - clr, p[1] - clr, -1]) cube([rtc_l + 2*clr, rtc_w + 2*clr, h + 2]);
        }
        // ...notched on the Pi side so the fragile leads leave with slack...
        translate([p[0] + 4, p[1] - t - 1, 1.4]) cube([rtc_l - 8, t + 2, h]);
        // ...and opened at both ends so a fingernail can lift the board out.
        for (x = [p[0] - t - 1, p[0] + rtc_l - 5])
            translate([x, p[1] + rtc_w/2 - 4, 1.4]) cube([6, 8, h]);
    }
}

// vents: a fan in the floor over the antenna corner, a matching one at the
// other end.  Rounded ends, so they read as a motif rather than as damage.
module floor_vents() {
    a = antenna_xy();
    for (i = [0 : 4])
        translate([a[0] - 16, a[1] - 8 + i*3.6, -floor_t - 1]) slot(22, 2.2, floor_t + 2);
    for (i = [0 : 4])
        translate([ring + 6, ring + 8 + i*3.6, -floor_t - 1]) slot(14, 2.2, floor_t + 2);
}

module tray() {
    difference() {
        union() {
            tray_body();
            screw_bosses();
            hat_pillars();
            pi_rib();
            if (show_usb) usb_channel_pos();
            if (show_rtc) { rtc_pocket(); rtc_pads(); }
            mount_ears();
        }
        for (p = screw_pts()) translate([p[0], p[1], -1]) cylinder(d = scr_d, h = split_z + 2);
        if (show_usb)   usb_channel_neg();
        if (show_vents) floor_vents();
    }
}

// ears on the two short edges, so nothing intrudes into the case
module mount_ears() {
    if (mount != "none") {
        ew = 14;  el = 12;  et = 3.0;
        for (s = [0, 1]) {
            x = s ? int_l + wall : -wall - el;
            translate([x, int_w/2 - ew/2, -floor_t]) difference() {
                hull() {
                    cube([el, ew, et]);
                    translate([s ? -2 : el, 0, 0]) cube([2, ew, et]);
                }
                translate([el/2 + (s ? -2 : 2), ew/2, -1]) {
                    if (mount == "tabs") {
                        cylinder(d = mount_scr, h = et + 2);
                        translate([0, 0, 1 + et - 1.6]) cylinder(d1 = mount_scr, d2 = mount_head, h = 1.8);
                    } else {                                   // keyhole
                        cylinder(d = mount_head, h = et + 2);
                        linear_extrude(et + 2)
                            polygon([[-mount_scr/2, 0], [-mount_scr/2, 5.5],
                                     [ mount_scr/2, 5.5], [ mount_scr/2, 0]]);
                    }
                }
            }
        }
    }
}

/* ------------------------------------------------------------------ bezel -- */

// window with a reveal that widens toward the front face: deep-set look, and
// self-supporting when printed face-down
module display_window() {
    w = disp_w + 2*0.6;  h = disp_h + 2*0.6;
    x = ring + disp_x - 0.6;  y = ring + disp_y - 0.6;
    rev = 1.2;
    hull() {
        translate([x, y, split_z - 1]) cube([w, h, 1.2]);
        translate([x - rev, y - rev, split_z + front_t]) cube([w + 2*rev, h + 2*rev, 0.01]);
    }
}

module button_holes() {
    for (i = [0 : btn_n - 1])
        translate([ring + btn_x, ring + btn_y0 + i*btn_pitch, split_z - 1])
            cylinder(d1 = 6.2, d2 = 7.4, h = front_t + 1.5);
}

// shallow relief strip down the key column: a tactile landmark, and it balances
// the wide border on that side
module button_strip() {
    x = ring + btn_x;  y0 = ring + btn_y0;
    translate([x, y0, split_z + front_t - 1.0])
        rotate([0, 0, 90]) slot((btn_n - 1)*btn_pitch + 11, 11, 1.2);
}

// registration rib that drops into the tray, and pads that hold the HAT down
module bezel_underside() {
    difference() {
        translate([0, 0, split_z - 3.0]) linear_extrude(3.0)
            difference() { rrect(int_l, int_w, in_r); translate([1.2, 1.2]) rrect(int_l - 2.4, int_w - 2.4, max(0.1, in_r - 1.2)); }
        for (p = screw_pts()) translate([p[0], p[1], split_z - 4]) cylinder(d = boss_d + 2*clr, h = 6);
    }
    for (h = hat_holes) {
        p = hat_to_int(h);
        translate([p[0], p[1], hat_top + 0.4]) difference() {
            cylinder(d = 6.5, h = split_z - hat_top - 0.4);
            translate([0, 0, -1]) cylinder(d = hat_hole_d + 1.0, h = hat_t + 1.6);
        }
    }
}

module bezel() {
    difference() {
        union() {
            translate([-wall, -wall, split_z]) cbox(ext_l, ext_w, front_t, out_r, 0, chamfer);
            bezel_underside();
        }
        display_window();
        button_holes();
        button_strip();
        for (p = screw_pts()) translate([p[0], p[1], split_z - 4]) {
            cylinder(d = scr_d + 2*clr + 0.5, h = 12);
            translate([0, 0, 4 + front_t - scr_head_h]) cylinder(d = scr_head_d, h = 4);
        }
    }
}

/* ------------------------------------------------------------------ gauge -- */

// Two throwaway test coupons. They share every parameter with the real parts,
// so they cannot drift from the design they are checking.
//
//   gauge_front  lay it on the HAT's FRONT face, registered into the board's
//                x=0 / y=0 corner by the L-lip underneath.  Checks: does the
//                window frame the active area evenly, do the key caps enter
//                their holes, is disp_x / disp_y / btn_pitch right.
//
//   gauge_floor  sit the whole stack on its three pillars.  Checks: do the pegs
//                enter the HAT's holes, does the HAT land flat on all three,
//                and does the Pi's underside (bundle included) still clear the
//                plate.  That last one validates floor->HAT underside = hat_z,
//                which is the only form of header_h the tray actually needs.

gauge_t   = 2.0;
gauge_lip = 2.5;

module gauge_plate() { linear_extrude(gauge_t) rrect(int_l, int_w, in_r); }

// registration lip hugging two edges of the HAT PCB, so the coupon can only
// sit one way round
module gauge_lip_corner() {
    union() {
        {
            translate([ring - 1.6 - clr, ring - 1.6 - clr, -gauge_lip])
                cube([1.6, hat_w*0.45, gauge_lip + 0.01]);
            translate([ring - 1.6 - clr, ring - 1.6 - clr, -gauge_lip])
                cube([hat_l*0.45, 1.6, gauge_lip + 0.01]);
        }
    }
}

module gauge_front() {
    // cut the apertures straight through at their NARROWEST size: the bezel's
    // tapered reveal would leave a membrane in a 2 mm plate, and the minimum
    // opening is the thing being checked anyway.
    difference() {
        union() { gauge_plate(); gauge_lip_corner(); }
        translate([ring + disp_x - 0.6, ring + disp_y - 0.6, -1])
            cube([disp_w + 1.2, disp_h + 1.2, gauge_t + 2]);
        for (i = [0 : btn_n - 1])
            translate([ring + btn_x, ring + btn_y0 + i*btn_pitch, -1])
                cylinder(d = 6.2, h = gauge_t + 2);
        for (p = screw_pts()) translate([p[0], p[1], -1]) cylinder(d = scr_d, h = 9);
        translate([ring, ring, gauge_t - 0.6])
            linear_extrude(1) difference() {
                offset(0.4) rrect(hat_l, hat_w, hat_corner_r);
                offset(-0.4) rrect(hat_l, hat_w, hat_corner_r);
            }
    }
}

module gauge_floor() {
    difference() {
        union() {
            translate([0, 0, -gauge_t]) gauge_plate();
            hat_pillars();
            // a stub of wall on two sides: checks the ring clearance too
            for (a = [[0, 0, 6, int_w], [0, 0, int_l, 6]])
                translate([a[0], a[1], -gauge_t]) cube([a[2], a[3], 4]);
        }
        // lighten everywhere except under the Pi, whose underside clearance
        // is one of the things being measured
        translate([ring + 8, ring + pi_w + 6, -gauge_t - 1])
            cube([int_l - ring - 20, int_w - pi_w - ring - 12, gauge_t + 2]);
        if (show_usb) usb_channel_neg();
        for (p = screw_pts()) translate([p[0], p[1], -gauge_t - 1]) cylinder(d = scr_d, h = 9);
    }
}

/* ------------------------------------------------------------------ views -- */

module assembly() {
    color("#3c4550") tray();
    color("#2b323a", 0.55) bezel();
    color("#1f6f3f", 0.9) mock_pi();
    color("#12406e", 0.9) mock_hat();
    color("#7a4a1f", 0.9) if (show_rtc) mock_rtc();
}

render_top = true;
if (render_top) {
if      (part == "tray")     tray();
else if (part == "bezel")    bezel();
else if (part == "assembly") assembly();
else if (part == "gauge_front") gauge_front();
else if (part == "gauge_floor") gauge_floor();
else if (part == "section")
    difference() { assembly(); translate([-20, int_w/2 - 200, -20]) cube([200, 200, 200]); }
}
