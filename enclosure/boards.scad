// SPDX-FileCopyrightText: 2026 Jan-Wijbrand Kolman
// SPDX-License-Identifier: MIT
//
// Mock volumes for the real hardware, in interior coordinates (see params.scad).
// These are NOT printed — they exist so the shell can be checked against them,
// visually (views) and numerically (tests.scad).
//
// Every mock takes `g` = grow: inflate the volume by g mm on all sides to bake
// in a clearance.  tests.scad intersects the grown mocks with the printed parts;
// any intersection at all is a fit bug.

include <params.scad>

module rrect(l, w, r) { translate([r, r]) offset(r = r) square([l - 2*r, w - 2*r]); }

// ---- the HAT PCB, its panel, and the key caps -------------------------------
module mock_hat(g = 0) {
    difference() {
        translate([ring - g, ring - g, hat_z - g])
            linear_extrude(hat_t + 2*g) rrect(hat_l + 2*g, hat_w + 2*g, hat_corner_r);
        for (h = hat_holes) {
            p = hat_to_int(h);
            translate([p[0], p[1], hat_z - g - 1]) cylinder(d = hat_hole_d, h = hat_t + 2*g + 2);
        }
    }
    // e-ink glass (approximated as the active area plus the glass border)
    translate([ring + disp_x - 6.6 - g, ring + disp_y - 3.8 - g, hat_top - g])
        cube([disp_w + 13.2 + 2*g, disp_h + 7.6 + 2*g, panel_t + 2*g]);
    // key caps
    for (i = [0 : btn_n - 1])
        translate([ring + btn_x, ring + btn_y0 + i*btn_pitch, hat_top - g])
            cylinder(d = 4.5 + 2*g, h = btn_cap_h + 2*g);
    // 40-pin female socket hanging under the HAT
    translate([ring + pi_off_x + 32.5 - 25.4 - g, ring + pi_off_y + 3.5 - 2.54 - g, hat_z - header_h - g])
        cube([50.8 + 2*g, 5.08 + 2*g, header_h + 2*g]);
}

// ---- the Pi Zero, hanging below on the header -------------------------------
module mock_pi(g = 0) {
    px = ring + pi_off_x;  py = ring + pi_off_y;  pz = pi_under;
    translate([px - g, py - g, pz - g])
        linear_extrude(pi_t + 2*g) rrect(pi_l + 2*g, pi_w + 2*g, 3);
    // components on top (SoC, RAM) and the taller stuff underneath (SD holder)
    translate([px + 8 - g, py + 6 - g, pz + pi_t - g]) cube([40 + 2*g, 18 + 2*g, 1.5 + 2*g]);
    translate([px - g, py - g, pz - 2.0 - g])          cube([18 + 2*g, pi_w + 2*g, 2.0 + 2*g]);
    // micro-USB PWR receptacle, facing +Y into the box
    translate([px + usb_x() - pi_usb_w/2 - g, py + pi_w - 2 - g, pz + pi_t - g])
        cube([pi_usb_w + 2*g, 5.5 + 2*g, pi_usb_h + 2*g]);
}

// x of the micro-USB PWR centre in Pi-local coords, honouring which end the SD is on
function usb_x() = pi_sd_low_x ? pi_usb_pwr_x : pi_l - pi_usb_pwr_x;

// the Pi's onboard antenna corner (keep plastic away, and never metal)
function antenna_xy() = [ring + pi_off_x + (pi_sd_low_x ? 6 : pi_l - 6),
                         ring + pi_off_y + pi_w - 6];

// ---- the DS3231 on its flying leads -----------------------------------------
module mock_rtc(g = 0) {
    p = rtc_xy();
    translate([p[0] - g, p[1] - g, rtc_z - g]) cube([rtc_l + 2*g, rtc_w + 2*g, rtc_h + 2*g]);
    // the lead connector, standing off the PCB towards the Pi
    translate([p[0] + (rtc_l - rtc_conn_l)/2 - g, p[1] - g, rtc_z - g])
        cube([rtc_conn_l + 2*g, rtc_conn_w + 2*g, rtc_conn_h + 2*g]);
}
// lives in the free floor strip beyond the Pi's connector edge
// clear of the cable run, which now leaves the Pi at the button end
function rtc_xy() = [ring + 45, ring + pi_off_y + pi_w + 8];

module mock_stack(g = 0) { mock_hat(g); mock_pi(g); if (show_rtc) mock_rtc(g); }
