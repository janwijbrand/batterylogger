// SPDX-FileCopyrightText: 2026 Jan-Wijbrand Kolman
// SPDX-License-Identifier: MIT

// -partialtest: drive a scripted sequence of frames through the real paint
// policy so partial refresh can be watched on the panel without waiting on the
// 10-minute tick. -dumpcmds does the off-hardware half: print the exact command
// stream a paint would emit, since ordering is what makes or breaks the partial
// path and it can't be seen from a photo.
package main

import (
	"fmt"
	"image"
	"io"
	"log"
	"math"
	"time"
)

// synthData builds a plausible dashboard at time `at`, so a run can show the
// clock ticking and the sparkline sliding without touching the real DB.
func synthData(at time.Time, soc int, charging bool) *APIData {
	now := at.Unix()
	d := &APIData{Now: now, WindowHours: 48}
	cur, volt := -1.82, 12.7
	dir := 1
	if charging {
		cur, volt, dir = 4.35, 13.4, 0
	}
	d.Latest = &Latest{
		TS: now, Voltage: volt, Current: cur, Power: int(volt * cur),
		SoC: soc, RemainingAh: soc * 2, Direction: dir,
		RtHours: 14, RtMin: 5, Synced: 1,
	}
	for i := 0; i <= 240; i++ { // one point per 12 min over the 48 h window
		ts := now - int64(48*3600) + int64(i)*12*60
		f := float64(ts) / float64(24*3600)
		d.Series = append(d.Series, SeriesPt{
			TS:  ts,
			SoC: clampI(soc+int(22*math.Sin(f*2*math.Pi)), 5, 100),
		})
	}
	d.LastNight = &Night{Night: at.Format("2006-01-02"), OutAh: 18}
	return d
}

func clampI(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// PartialTest paints a scripted sequence: a base, then the cheap case (only the
// clock moves), then a bigger diff, then a screen change, then enough partials
// to trip the base counter. It calls the real Sleep() between frames — skipping
// that is the one shortcut that would hide the very bug DisplayPartial exists to
// prevent.
//
// Note what this can't compress: RAM decay needs wall-clock time, so a run at
// -interval 15s proves the sequencing works and proves nothing about the sleep
// problem. That still wants the daemon idling for half an hour.
func PartialTest(epd *EPD, interval time.Duration, mode PaintMode) {
	pnt := &painter{epd: epd, mode: mode}
	base := time.Now()
	soc := 87

	step := func(what string, img *image.Gray, kind screenKind) {
		log.Printf("--- %s", what)
		pnt.show(img, kind)
		time.Sleep(interval)
	}

	// 1. baseline
	step("base: first dashboard", RenderDashboard(synthData(base, soc, false), nil), kindDashboard)

	// 2. the cheap case: 10 minutes pass, so the clock ticks and the sparkline
	//    slides by about a pixel. This is what the periodic repaint looks like.
	for i := 1; i <= 3; i++ {
		at := base.Add(time.Duration(i) * 10 * time.Minute)
		step(fmt.Sprintf("partial: +%dm (clock + sparkline)", i*10),
			RenderDashboard(synthData(at, soc, false), nil), kindDashboard)
	}

	// 3. a bigger diff: SoC hero, battery bar, tiles and arrows all change.
	step("partial: charging, SoC 87 -> 42 (big diff)",
		RenderDashboard(synthData(base.Add(40*time.Minute), 42, true), nil), kindDashboard)

	// 4. screen change -> must be a base refresh, not a partial.
	step("kind change: sysinfo", RenderSysInfo([]string{
		"up   12d 4h 37m", "ip   192.168.178.42", "cpu  46 C",
		"disk 21.4 / 30 GB free", "rtc  ok (ds3231)",
	}), kindSysInfo)
	step("kind change: back to dashboard",
		RenderDashboard(synthData(base.Add(50*time.Minute), 42, true), nil), kindDashboard)

	// 5. run past maxPartials so the periodic base refresh shows itself.
	for i := 1; i <= maxPartials+1; i++ {
		at := base.Add(time.Duration(50+i*10) * time.Minute)
		step(fmt.Sprintf("partial run %d/%d", i, maxPartials+1),
			RenderDashboard(synthData(at, 42-i, true), nil), kindDashboard)
	}

	// 6. identical frame: should be skipped entirely (no log line, no flash).
	last := RenderDashboard(synthData(base.Add(time.Duration(50+(maxPartials+1)*10)*time.Minute), 42-(maxPartials+1), true), nil)
	log.Printf("--- identical frame (expect no paint at all)")
	pnt.show(last, kindDashboard)

	log.Printf("partialtest done")
}

// DumpCommands prints the command stream for a base paint and a partial paint,
// without any hardware. Long payloads are summarised — what matters is the
// order: that 0x3C lands after the reset, that 0x26 is written before 0x24, and
// that the window is re-pointed between the two plane writes.
func DumpCommands(w io.Writer) {
	n := 0
	epd := newTraceEPD(func(op string, b []byte) {
		n++
		switch {
		case op == "data" && len(b) > 8:
			fmt.Fprintf(w, "  %-5s %d bytes (%02x %02x ...)\n", op, len(b), b[0], b[1])
		case op == "data":
			fmt.Fprintf(w, "  %-5s % 02x\n", op, b)
		case op == "cmd":
			fmt.Fprintf(w, "  %-5s %02x%s\n", op, b[0], cmdName(b[0]))
		default:
			fmt.Fprintf(w, "  %s\n", op)
		}
	})

	a := GetBufferFromImage(RenderDashboard(synthData(time.Now(), 87, false), nil))
	b := GetBufferFromImage(RenderDashboard(synthData(time.Now().Add(10*time.Minute), 87, false), nil))

	diff := 0
	for i := range a {
		if a[i] != b[i] {
			diff++
		}
	}

	fmt.Fprintln(w, "DisplayBase:")
	epd.DisplayBase(a)
	epd.Sleep()
	fmt.Fprintln(w, "\nDisplayPartial:")
	epd.DisplayPartial(a, b)
	epd.Sleep()
	fmt.Fprintln(w, "\nDisplayBaseFast:")
	epd.DisplayBaseFast(a)
	epd.Sleep()
	fmt.Fprintf(w, "\n%d operations; frames differ in %d/%d bytes (%.1f%%) over a 10-minute tick\n",
		n, diff, len(a), 100*float64(diff)/float64(len(a)))
}

// cmdName annotates the opcodes that matter for reading a dump.
func cmdName(c byte) string {
	switch c {
	case 0x11:
		return " data-entry"
	case 0x12:
		return " SWRESET"
	case 0x18:
		return " temp-sensor"
	case 0x1A:
		return " temp-write"
	case 0x20:
		return " ACTIVATE"
	case 0x22:
		return " update-ctrl"
	case 0x24:
		return " RAM(new)"
	case 0x26:
		return " RAM(prev)"
	case 0x3C:
		return " border"
	case 0x44, 0x45:
		return " window"
	case 0x4E, 0x4F:
		return " counter"
	case 0x10:
		return " deep-sleep"
	}
	return ""
}
