// SPDX-FileCopyrightText: 2026 Jan-Wijbrand Kolman
// SPDX-License-Identifier: MIT

// The paint policy: when to spend a full (flashing) refresh and when a partial
// one. Shared by the daemon and the -partialtest harness so the experiment
// exercises exactly the code the daemon runs.
package main

import (
	"bytes"
	"image"
	"log"
	"time"
)

const (
	// maxPartials: a full refresh at least hourly at the 10-min tick. E-ink
	// ghosts a little more with every differential update; the base refresh is
	// what clears it.
	maxPartials = 6
	// fullEvery: and once a day regardless, even if nothing on screen changed.
	// That rule is about the ink, not the content, so it outranks the
	// nothing-moved shortcut below.
	fullEvery = 24 * time.Hour
)

// screenKind identifies which frame is on the panel. A partial update between
// two different kinds is the worst case for the DU waveform — most of the panel
// differs, and the leftovers of the old layout smear — so every kind change
// forces a base refresh.
type screenKind int

const (
	kindNone screenKind = iota
	kindDashboard
	kindSysInfo
	kindMessage
)

func (k screenKind) String() string {
	switch k {
	case kindDashboard:
		return "dashboard"
	case kindSysInfo:
		return "sysinfo"
	case kindMessage:
		return "message"
	}
	return "none"
}

// PaintMode selects how the panel is driven. Zero value = mono with partial
// refresh, which is the point of all this.
type PaintMode struct {
	FourGray bool // old path: 4-grey, always a full flashing refresh
	FastBase bool // use the fast (0xC7) waveform for base refreshes
}

// painter owns what the panel is currently showing. The previous frame is kept
// host-side because the panel's copy of it does not survive deep sleep — see
// DisplayPartial.
type painter struct {
	epd  *EPD
	mode PaintMode

	lastBuf  []byte
	lastKind screenKind
	partials int
	lastFull time.Time
	force    bool
}

// forceBase makes the next paint a full refresh even if nothing changed. KEY1
// ("force refresh") means this: with partial updates the panel can legitimately
// decide a repaint is unnecessary, so the button would otherwise look dead — and
// a deliberate flash is exactly what you want when ghosting is bugging you.
func (p *painter) forceBase() { p.force = true }

// show paints img, choosing full or partial. Every path ends in Sleep(), so the
// panel is always left in deep sleep between updates.
func (p *painter) show(img *image.Gray, kind screenKind) {
	t := time.Now()

	if p.mode.FourGray {
		plane1, plane2 := GetBuffers4Gray(img)
		p.epd.Display4Gray(plane1, plane2)
		d := time.Since(t)
		p.epd.Sleep()
		log.Printf("paint %s: 4-grey in %.2fs", kind, d.Seconds())
		return
	}

	buf := GetBufferFromImage(img)
	due := p.force || p.lastFull.IsZero() || time.Since(p.lastFull) > fullEvery
	p.force = false

	// Nothing moved: don't touch the panel at all — unless a full refresh is
	// due, in which case repaint the same frame to keep the ink honest.
	if p.lastBuf != nil && !due && bytes.Equal(buf, p.lastBuf) {
		return
	}

	mode := "partial"
	switch {
	case p.lastBuf == nil || kind != p.lastKind || p.partials >= maxPartials || due:
		if p.mode.FastBase {
			p.epd.DisplayBaseFast(buf)
			mode = "base/fast"
		} else {
			p.epd.DisplayBase(buf)
			mode = "base"
		}
		p.partials, p.lastFull = 0, time.Now()
	default:
		p.epd.DisplayPartial(p.lastBuf, buf)
		p.partials++
	}
	d := time.Since(t)
	p.epd.Sleep()
	p.lastBuf, p.lastKind = buf, kind
	log.Printf("paint %s: %s in %.2fs (partials since base: %d)",
		kind, mode, d.Seconds(), p.partials)
}
