// SPDX-FileCopyrightText: 2026 Jan-Wijbrand Kolman
// SPDX-License-Identifier: MIT

// Partial (differential, non-flashing) refresh for the 2.7" V2 panel, mono only.
//
// The controller keeps two RAM planes: 0x24 = the new image, 0x26 = the image
// currently on the glass. A partial update (0x22 <- 0xFF) runs the DU waveform,
// which only moves pixels where the two planes differ — so it doesn't flash and
// takes a fraction of a full refresh.
//
// Mono only, and that's structural: the 4-grey path uses 0x26 as its second
// bit-plane rather than as "what's on the glass", so the two modes can't share
// a panel state. See epd4gray.go.
package main

// setFullWindow points the RAM window and both address counters at the whole
// panel. Init() leaves 0x44/0x4E at their post-SWRESET defaults, which happen to
// be right; being explicit costs six bytes and removes the assumption. It also
// has to be repeated between two plane writes, since writing a full frame walks
// the address counter to the end.
func (e *EPD) setFullWindow() {
	e.cmd(0x44) // RAM-X start/end: (0x15+1)*8 = 176
	e.data(0x00, 0x15)
	e.cmd(0x45) // RAM-Y start/end: 0..263
	e.data(0x00, 0x00, 0x07, 0x01)
	e.cmd(0x4E) // RAM-X counter
	e.data(0x00)
	e.cmd(0x4F) // RAM-Y counter
	e.data(0x00, 0x00)
}

// turnOnPartial runs the update sequence with the partial (DU) waveform.
// 0xF7 (turnOn) loads the full waveform from OTP; 0xC7 (4-grey, fast) loads no
// LUT at all and runs whatever init installed.
func (e *EPD) turnOnPartial() {
	e.cmd(0x22)
	e.data(0xFF)
	e.cmd(0x20)
	e.readBusy()
}

// DisplayBase paints a full flashing refresh and leaves *both* RAM planes
// holding the frame — the baseline that later partials diff against. Use it on
// the first paint, on every screen change, and periodically to clear ghosting.
// Equivalent to Waveshare's display_Base().
func (e *EPD) DisplayBase(buf []byte) {
	e.Init()
	e.setFullWindow()
	e.cmd(0x24) // new image
	e.data(buf...)
	e.setFullWindow()
	e.cmd(0x26) // previous image
	e.data(buf...)
	e.turnOn()
}

// DisplayPartial updates the panel from prev to next without flashing. prev must
// be the frame actually on the glass — pass the buffer last handed to
// DisplayBase or DisplayPartial.
//
// Writing prev to 0x26 is the whole trick and must not be optimised away.
// Waveshare's display_Partial() writes only 0x24 and trusts whatever is left in
// 0x26; that holds in their demo loop because it never sleeps. We call Sleep()
// after every paint, and deep sleep stops refreshing that RAM, so it decays into
// noise (the corruption Hugo Chargois traced on the 7.5" V2,
// thoughts.gohu.org/posts/2025/epaper-partial-updates/). Re-sending the last
// frame from host memory means we never depend on panel RAM surviving anything —
// which also makes it moot whether SWRESET clears RAM.
//
// Both buffers are full frames: the DU waveform only moves differing pixels, so
// there's no bounding box to compute and none of Waveshare's x/8 window
// arithmetic. A plane is 5808 bytes ≈ 12 ms at 4 MHz — noise next to the refresh.
func (e *EPD) DisplayPartial(prev, next []byte) {
	e.Init()    // we always wake from deep sleep, so the config is gone
	e.cmd(0x3C) // border waveform: follow the partial LUT, don't flash the border
	e.data(0x80)
	e.setFullWindow()
	e.cmd(0x26) // what's on the glass — the write Waveshare omits
	e.data(prev...)
	e.setFullWindow()
	e.cmd(0x24) // what we want on the glass
	e.data(next...)
	e.turnOnPartial()
}

// initFast is Waveshare's init_Fast(): a full-refresh init that deliberately
// lies about the temperature to get a shorter waveform. It reads the built-in
// sensor (0x18, 0x22 <- 0xB1) and then overwrites the result via 0x1A with
// 0x64 0x00 before loading the LUT (0x22 <- 0x91). On the byte pattern that
// looks like 12-bit, 1/16 °C units — 0x640 = 1600/16 = 100 °C — i.e. "the panel
// is scorching, use the quick waveform". Hot ink needs less drive; a cold panel
// told it is hot gets under-driven, which is the risk this trades on.
func (e *EPD) initFast() {
	e.reset()
	e.readBusy()
	e.cmd(0x12) // SWRESET
	e.readBusy()
	e.cmd(0x12) // SWRESET (twice, as the vendor does)
	e.readBusy()
	e.cmd(0x18) // read built-in temperature sensor
	e.data(0x80)
	e.cmd(0x22) // load temperature value
	e.data(0xB1)
	e.cmd(0x20)
	e.readBusy()
	e.cmd(0x1A) // ...then overwrite it
	e.data(0x64, 0x00)
	e.cmd(0x45) // RAM-Y start/end: 0..263
	e.data(0x00, 0x00, 0x07, 0x01)
	e.cmd(0x4F) // RAM-Y counter
	e.data(0x00, 0x00)
	e.cmd(0x11) // data entry mode
	e.data(0x03)
	e.cmd(0x22) // load LUT for the (faked) temperature
	e.data(0x91)
	e.cmd(0x20)
	e.readBusy()
}

// turnOnFast runs the update sequence with no LUT load — initFast already
// installed one.
func (e *EPD) turnOnFast() {
	e.cmd(0x22)
	e.data(0xC7)
	e.cmd(0x20)
	e.readBusy()
}

// DisplayBaseFast is DisplayBase on the fast waveform: same both-planes
// contract, shorter (and shorter-settling) flash.
//
// Measured on this panel 2026-09-03, and it does not pay off: 2.30 s against
// 1.88 s for the plain 0xF7 base. initFast spends two SWRESETs and two extra
// activate-and-wait cycles (load temperature, then load LUT) before it paints,
// and that costs more than the shorter waveform saves. Kept behind -fastbase
// because the finding is worth being able to re-check — not because it is
// recommended. If anyone revisits it, the thing to try is trimming initFast
// (the second SWRESET, the sensor read it immediately overwrites) rather than
// the waveform itself.
func (e *EPD) DisplayBaseFast(buf []byte) {
	e.initFast()
	e.setFullWindow()
	e.cmd(0x24)
	e.data(buf...)
	e.setFullWindow()
	e.cmd(0x26)
	e.data(buf...)
	e.turnOnFast()
}
