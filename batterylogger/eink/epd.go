// SPDX-FileCopyrightText: 2026 Jan-Wijbrand Kolman
// SPDX-License-Identifier: MIT

// Minimal pure-Go driver for the Waveshare 2.7" e-Paper HAT (V2), mono/1-bit
// path only. Ported from Waveshare's epd2in7_V2.py + epdconfig.py — just the
// commands the full-refresh black/white flow uses (no 4-gray/partial/fast).
//
// Hardware (BCM pins): RST=17, DC=25, BUSY=24, PWR=18; CS is the SPI hardware
// CE0 (/dev/spidev0.0), not a GPIO. SPI 4 MHz, mode 0. BUSY is active-high.
package main

import (
	"fmt"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
)

const (
	epdWidth  = 176 // panel native width  (short axis)
	epdHeight = 264 // panel native height (long axis)
	rowBytes  = epdWidth / 8
	bufLen    = rowBytes * epdHeight // 5808
)

// EPD holds the SPI connection and control GPIO lines.
type EPD struct {
	port spi.PortCloser
	conn spi.Conn
	rst  gpio.PinIO
	dc   gpio.PinIO
	busy gpio.PinIO
	pwr  gpio.PinIO

	// trace, when non-nil, captures the command stream *instead* of touching
	// any hardware — so the exact byte sequence of a paint can be checked off
	// the Pi (see -dumpcmds). Ordering is the whole game in the partial path.
	trace func(op string, b []byte)

	// sleptAt is when deep sleep was last entered, so the settle delay can be
	// paid lazily — see Sleep.
	sleptAt time.Time
}

// newTraceEPD returns an EPD that drives nothing and records what it would have
// sent. It has no SPI port or GPIO pins, so never call Close on it.
func newTraceEPD(trace func(op string, b []byte)) *EPD {
	return &EPD{trace: trace}
}

// NewEPD opens SPI0.0 and the control pins. host.Init() must have run first.
func NewEPD() (*EPD, error) {
	p, err := spireg.Open("/dev/spidev0.0")
	if err != nil {
		return nil, fmt.Errorf("open spi: %w", err)
	}
	c, err := p.Connect(4*physic.MegaHertz, spi.Mode0, 8)
	if err != nil {
		p.Close()
		return nil, fmt.Errorf("connect spi: %w", err)
	}
	e := &EPD{
		port: p, conn: c,
		rst:  gpioreg.ByName("GPIO17"),
		dc:   gpioreg.ByName("GPIO25"),
		busy: gpioreg.ByName("GPIO24"),
		pwr:  gpioreg.ByName("GPIO18"),
	}
	if e.rst == nil || e.dc == nil || e.busy == nil || e.pwr == nil {
		p.Close()
		return nil, fmt.Errorf("gpio pin(s) not found (rst/dc/busy/pwr)")
	}
	if err := e.pwr.Out(gpio.High); err != nil {
		p.Close()
		return nil, fmt.Errorf("pwr high: %w", err)
	}
	if err := e.busy.In(gpio.PullNoChange, gpio.NoEdge); err != nil {
		p.Close()
		return nil, fmt.Errorf("busy in: %w", err)
	}
	return e, nil
}

func (e *EPD) cmd(b byte) {
	if e.trace != nil {
		e.trace("cmd", []byte{b})
		return
	}
	e.dc.Out(gpio.Low)
	e.conn.Tx([]byte{b}, nil)
}

func (e *EPD) data(b ...byte) {
	if len(b) == 0 {
		return
	}
	if e.trace != nil {
		e.trace("data", b)
		return
	}
	e.dc.Out(gpio.High)
	const max = 4096 // spidev per-transfer ceiling; CS toggling between chunks is fine
	for i := 0; i < len(b); i += max {
		j := i + max
		if j > len(b) {
			j = len(b)
		}
		e.conn.Tx(b[i:j], nil)
	}
}

func (e *EPD) readBusy() {
	if e.trace != nil {
		e.trace("busy", nil)
		return
	}
	for e.busy.Read() == gpio.High { // high = busy
		time.Sleep(20 * time.Millisecond)
	}
}

func (e *EPD) reset() {
	if e.trace != nil {
		e.trace("reset", nil)
		return
	}
	e.settle()
	e.rst.Out(gpio.High)
	time.Sleep(200 * time.Millisecond)
	e.rst.Out(gpio.Low)
	time.Sleep(2 * time.Millisecond)
	e.rst.Out(gpio.High)
	time.Sleep(200 * time.Millisecond)
}

// Init runs the full-refresh init sequence (mirrors epd2in7_V2.init()).
func (e *EPD) Init() {
	e.reset()
	e.readBusy()
	e.cmd(0x12) // SWRESET
	e.readBusy()
	e.cmd(0x45) // set RAM-Y start/end: 0..263
	e.data(0x00, 0x00, 0x07, 0x01)
	e.cmd(0x4F) // set RAM-Y counter to 0
	e.data(0x00, 0x00)
	e.cmd(0x11) // data entry mode
	e.data(0x03)
}

func (e *EPD) turnOn() {
	e.cmd(0x22)
	e.data(0xF7)
	e.cmd(0x20)
	e.readBusy()
}

// Display writes a full 5808-byte 1-bit frame and refreshes (flashing).
//
// Every Display* entry point is self-contained: it runs its own init, so the
// caller only ever does Display*(...) then Sleep(). That matters most in the
// partial path — a partial that skipped its reset would paint through whatever
// state the previous call left behind — so the mono and 4-grey paths follow the
// same rule rather than being the odd ones out.
func (e *EPD) Display(buf []byte) {
	e.Init()
	e.cmd(0x24)
	e.data(buf...)
	e.turnOn()
}

// Clear paints the panel all-white.
func (e *EPD) Clear() {
	e.Display(whiteBuf())
}

// whiteBuf is an all-white (all bits set) frame buffer.
func whiteBuf() []byte {
	b := make([]byte, bufLen)
	for i := range b {
		b[i] = 0xFF
	}
	return b
}

// Sleep puts the controller into deep sleep (image is retained on the glass).
func (e *EPD) Sleep() {
	e.cmd(0x10)
	e.data(0x01)
	e.sleptAt = time.Now()
}

// sleepSettle is Waveshare's 2 s delay after entering deep sleep. They spend it
// immediately; we only record when sleep started and pay whatever is left of it
// at the next reset (below). The panel still gets its quiet 2 s — but an idle
// daemon spends them idling, and a button press 10 minutes later waits for
// nothing. With a partial refresh at well under a second, that fixed 2 s was
// most of the wall-clock cost of a paint.
const sleepSettle = 2 * time.Second

// settle waits out whatever remains of the post-sleep quiet period.
func (e *EPD) settle() {
	if e.sleptAt.IsZero() {
		return
	}
	if d := sleepSettle - time.Since(e.sleptAt); d > 0 {
		time.Sleep(d)
	}
	e.sleptAt = time.Time{}
}

// Close drops the control lines and releases SPI. It pays any outstanding
// settle delay first: everywhere else the delay can be deferred because the next
// thing to touch the panel is a reset, but here the next thing is the power pin
// going low, and cutting power mid-transition is what the delay guards against.
func (e *EPD) Close() {
	e.settle()
	e.rst.Out(gpio.Low)
	e.dc.Out(gpio.Low)
	e.pwr.Out(gpio.Low)
	e.port.Close()
}

// GetBuffer packs a 264(w)x176(h) landscape frame into the panel's native
// portrait 1-bit buffer. black(x,y) reports whether image pixel (x,y) is black.
// Mirrors epd2in7_V2.getbuffer()'s "Horizontal" branch.
func GetBuffer(black func(x, y int) bool) []byte {
	buf := make([]byte, bufLen)
	for i := range buf {
		buf[i] = 0xFF
	}
	for y := 0; y < epdWidth; y++ { // 0..175
		for x := 0; x < epdHeight; x++ { // 0..263
			if black(x, y) {
				newy := epdHeight - x - 1
				idx := (y + newy*epdWidth) / 8
				buf[idx] &^= 0x80 >> uint(y%8)
			}
		}
	}
	return buf
}
