// SPDX-FileCopyrightText: 2026 Jan-Wijbrand Kolman
// SPDX-License-Identifier: MIT

// 1-bit dashboard renderer for the 264x176 e-Paper panel. Draws into an
// *image.Gray (0=black, 255=white); GetBufferFromImage packs it for the panel.
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	W = 264
	H = 176
)

var (
	fTitle = mustFace(gobold.TTF, 15)
	fBig   = mustFace(gobold.TTF, 40)
	fPct   = mustFace(gobold.TTF, 18)
	fSmall = mustFace(goregular.TTF, 11)
	fTiny  = mustFace(goregular.TTF, 9)
	fTileV = mustFace(gobold.TTF, 16)
)

func mustFace(ttf []byte, px float64) font.Face {
	f, err := opentype.Parse(ttf)
	if err != nil {
		panic(err)
	}
	fc, err := opentype.NewFace(f, &opentype.FaceOptions{Size: px, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		panic(err)
	}
	return fc
}

// --- primitive drawing helpers (on image.Gray, black = true) ---

func px(img *image.Gray, x, y int, on bool) {
	if x < 0 || y < 0 || x >= W || y >= H {
		return
	}
	if on {
		img.SetGray(x, y, color.Gray{Y: 0})
	} else {
		img.SetGray(x, y, color.Gray{Y: 255})
	}
}

func hline(img *image.Gray, x0, x1, y int) {
	for x := x0; x <= x1; x++ {
		px(img, x, y, true)
	}
}

func vline(img *image.Gray, x, y0, y1 int) {
	for y := y0; y <= y1; y++ {
		px(img, x, y, true)
	}
}

func rectOutline(img *image.Gray, x0, y0, x1, y1 int) {
	hline(img, x0, x1, y0)
	hline(img, x0, x1, y1)
	vline(img, x0, y0, y1)
	vline(img, x1, y0, y1)
}

func fillRect(img *image.Gray, x0, y0, x1, y1 int) {
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			px(img, x, y, true)
		}
	}
}

func dottedHLine(img *image.Gray, x0, x1, y int) {
	for x := x0; x <= x1; x += 2 {
		px(img, x, y, true)
	}
}

func line(img *image.Gray, x0, y0, x1, y1 int) {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		px(img, x0, y0, true)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

// triangle marks for charge (up) / discharge (down); apex height s, drawn with
// its top-left of the bounding box at (x, y).
func triUp(img *image.Gray, x, y, s int) {
	for i := 0; i <= s; i++ {
		half := i / 2
		hline(img, x+s/2-half, x+s/2+half, y+i)
	}
}

func triDown(img *image.Gray, x, y, s int) {
	for i := 0; i <= s; i++ {
		half := (s - i) / 2
		hline(img, x+s/2-half, x+s/2+half, y+i)
	}
}

// --- text helpers ---

func text(img *image.Gray, fc font.Face, x, baseline int, s string) {
	d := &font.Drawer{Dst: img, Src: image.NewUniform(color.Gray{Y: 0}), Face: fc, Dot: fixed.P(x, baseline)}
	d.DrawString(s)
}

func textW(fc font.Face, s string) int {
	d := &font.Drawer{Face: fc}
	return d.MeasureString(s).Round()
}

// textTop draws with the string's top at `top`; returns the baseline y used.
func textTop(img *image.Gray, fc font.Face, x, top int, s string) int {
	base := top + fc.Metrics().Ascent.Round()
	text(img, fc, x, base, s)
	return base
}

func textRight(img *image.Gray, fc font.Face, xr, top int, s string) {
	textTop(img, fc, xr-textW(fc, s), top, s)
}

// --- formatting ---

func ageStr(sec int64) string {
	switch {
	case sec < 90:
		return fmt.Sprintf("%ds", sec)
	case sec < 5400:
		return fmt.Sprintf("%dm", sec/60)
	default:
		return fmt.Sprintf("%.0fh", float64(sec)/3600)
	}
}

func durShort(h, m int) string {
	switch {
	case h >= 48:
		return fmt.Sprintf("~%dd", (h+12)/24)
	case h >= 1:
		return fmt.Sprintf("%dh%02d", h, m)
	default:
		return fmt.Sprintf("%dm", m)
	}
}

// --- the dashboard ---

func RenderDashboard(d *APIData, ferr error) *image.Gray {
	img := image.NewGray(image.Rect(0, 0, W, H))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.Gray{Y: 255}), image.Point{}, draw.Src)
	rectOutline(img, 0, 0, W-1, H-1)

	// header
	textTop(img, fTitle, 5, 3, "batterijtje")

	if ferr != nil || d == nil || d.Latest == nil {
		msg := "no data"
		if ferr != nil {
			msg = ferr.Error()
		}
		textRight(img, fTiny, W-5, 6, "offline")
		hline(img, 3, W-4, 22)
		textTop(img, fBig, 20, 60, "—")
		textTop(img, fSmall, 20, 110, msg)
		return img
	}

	L := d.Latest
	chg := L.Direction == 0
	age := d.Now - L.TS
	upd := "upd " + ageStr(age)
	if age > 180 {
		upd = "STALE " + ageStr(age)
	}
	if L.Synced == 0 {
		upd += " !clk"
	}
	textRight(img, fTiny, W-5, 6, upd)
	hline(img, 3, W-4, 22)

	// ---- left column: SoC hero ----
	textTop(img, fBig, 6, 24, fmt.Sprintf("%d", L.SoC))
	socW := textW(fBig, fmt.Sprintf("%d", L.SoC))
	textTop(img, fPct, 6+socW+2, 44, "%")
	textTop(img, fSmall, 7, 65, fmt.Sprintf("%d Ah", L.RemainingAh))

	// battery bar
	bx0, by0, bx1, by1 := 7, 79, 90, 91
	rectOutline(img, bx0, by0, bx1, by1)
	fillRect(img, bx1+1, by0+3, bx1+3, by1-3) // nub
	fw := int(float64(bx1-bx0-2) * clampF(float64(L.SoC)/100, 0, 1))
	if fw > 0 {
		fillRect(img, bx0+1, by0+1, bx0+1+fw, by1-1)
	}

	// state line
	if chg {
		triUp(img, 7, 95, 8)
		textTop(img, fSmall, 19, 94, "charging")
	} else {
		triDown(img, 7, 95, 8)
		textTop(img, fSmall, 19, 94, "consuming")
	}

	// ---- right column: 2x2 tiles ----
	type tile struct {
		label, val string
		arrow      int // 0 none, 1 up, 2 down
	}
	cur := fmt.Sprintf("%.2fA", absF(L.Current))
	volt := fmt.Sprintf("%.1fV", L.Voltage)
	powr := fmt.Sprintf("%dW", absI(L.Power))
	rtLabel := "TO EMPTY"
	if chg {
		rtLabel = "TO FULL"
	}
	rt := durShort(L.RtHours, L.RtMin)
	arr := 2
	if chg {
		arr = 1
	}
	tiles := [4]tile{
		{"CURRENT", cur, arr},
		{"VOLTAGE", volt, 0},
		{"POWER", powr, arr},
		{rtLabel, rt, 0},
	}
	tx := [2]int{100, 182}
	ty := [2]int{25, 64}
	tw, th := 78, 37
	for i, t := range tiles {
		x0 := tx[i%2]
		y0 := ty[i/2]
		rectOutline(img, x0, y0, x0+tw, y0+th)
		textTop(img, fTiny, x0+4, y0+3, t.label)
		vx := x0 + 5
		if t.arrow != 0 {
			if t.arrow == 1 {
				triUp(img, vx, y0+19, 8)
			} else {
				triDown(img, vx, y0+19, 8)
			}
			vx += 11
		}
		textTop(img, fTileV, vx, y0+16, t.val)
	}

	// ---- sparkline: 48h SoC ----
	textTop(img, fTiny, 6, 107, "48h SoC")
	night := "night –"
	if d.LastNight != nil {
		night = fmt.Sprintf("night %d Ah", d.LastNight.OutAh)
	}
	textRight(img, fTiny, W-6, 107, night)

	px0, py0, px1, py1 := 6, 120, W-6, 172 // plot box (top=100%, bottom=0%)
	// gridlines 0/50/100
	dottedHLine(img, px0, px1, py0)
	dottedHLine(img, px0, px1, (py0+py1)/2)
	dottedHLine(img, px0, px1, py1)
	if len(d.Series) > 0 {
		t0 := d.Now - 48*3600
		t1 := d.Now
		span := float64(t1 - t0)
		if span < 1 {
			span = 1
		}
		xat := func(ts int64) int {
			f := float64(ts-t0) / span
			return px0 + int(clampF(f, 0, 1)*float64(px1-px0))
		}
		yat := func(soc int) int {
			return py1 - int(clampF(float64(soc)/100, 0, 1)*float64(py1-py0))
		}
		prevx, prevy := xat(d.Series[0].TS), yat(d.Series[0].SoC)
		for _, p := range d.Series[1:] {
			cx, cy := xat(p.TS), yat(p.SoC)
			line(img, prevx, prevy, cx, cy)
			prevx, prevy = cx, cy
		}
		// dot on the last point
		fillRect(img, prevx-1, prevy-1, prevx+1, prevy+1)
	} else {
		textTop(img, fTiny, px0+4, (py0+py1)/2-4, "collecting...")
	}

	return img
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
func absF(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}
func absI(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// blackThreshold: a rendered pixel darker than this becomes a black panel pixel.
// Set high so anti-aliased text strokes stay solid instead of breaking up.
const blackThreshold = 176

// GetBufferFromImage packs the rendered Gray image into the panel's 1-bit buffer.
func GetBufferFromImage(img *image.Gray) []byte {
	return GetBuffer(func(x, y int) bool {
		return img.GrayAt(x, y).Y < blackThreshold
	})
}

// Bilevel returns a pure black/white copy using blackThreshold — i.e. exactly
// what the panel will show. Handy for debug PNGs.
func Bilevel(img *image.Gray) *image.Gray {
	b := img.Bounds()
	out := image.NewGray(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if img.GrayAt(x, y).Y < blackThreshold {
				out.SetGray(x, y, color.Gray{Y: 0})
			} else {
				out.SetGray(x, y, color.Gray{Y: 255})
			}
		}
	}
	return out
}

// grayLevel quantizes an 8-bit gray to one of 4 panel levels:
// 0=black, 1=dark grey, 2=light grey, 3=white. Boundaries are the midpoints
// between the canonical values 0x00 / 0x80 / 0xC0 / 0xFF.
func grayLevel(y uint8) int {
	switch {
	case y < 64:
		return 0
	case y < 160:
		return 1
	case y < 224:
		return 2
	default:
		return 3
	}
}

var levelValue = [4]uint8{0x00, 0x80, 0xC0, 0xFF}

// GetBuffers4Gray packs the rendered image into the two 1-bit RAM planes the
// 4-grey mode needs. plane1 bit set for black|light-grey; plane2 for
// black|dark-grey. Same rotation/bit-order as the mono GetBuffer.
func GetBuffers4Gray(img *image.Gray) (p1, p2 []byte) {
	p1 = make([]byte, bufLen)
	p2 = make([]byte, bufLen)
	for y := 0; y < epdWidth; y++ { // 0..175
		for x := 0; x < epdHeight; x++ { // 0..263
			lvl := grayLevel(img.GrayAt(x, y).Y)
			newy := epdHeight - x - 1
			idx := (y + newy*epdWidth) / 8
			mask := byte(0x80 >> uint(y%8))
			if lvl == 0 || lvl == 2 { // black or light grey
				p1[idx] |= mask
			}
			if lvl == 0 || lvl == 1 { // black or dark grey
				p2[idx] |= mask
			}
		}
	}
	return
}

// Quantize4 maps every pixel to its 4-level canonical grey — i.e. exactly what
// the 4-grey panel will show. Handy for debug PNGs.
func Quantize4(img *image.Gray) *image.Gray {
	b := img.Bounds()
	out := image.NewGray(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			out.SetGray(x, y, color.Gray{Y: levelValue[grayLevel(img.GrayAt(x, y).Y)]})
		}
	}
	return out
}

// ScaleNN nearest-neighbor upscales (so debug PNGs show real pixels, unsmoothed).
func ScaleNN(img *image.Gray, n int) *image.Gray {
	if n <= 1 {
		return img
	}
	b := img.Bounds()
	out := image.NewGray(image.Rect(0, 0, b.Dx()*n, b.Dy()*n))
	for y := 0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			v := img.GrayAt(b.Min.X+x, b.Min.Y+y)
			for dy := 0; dy < n; dy++ {
				for dx := 0; dx < n; dx++ {
					out.SetGray(x*n+dx, y*n+dy, v)
				}
			}
		}
	}
	return out
}
