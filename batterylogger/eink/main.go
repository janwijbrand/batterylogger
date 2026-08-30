// SPDX-FileCopyrightText: 2026 Jan-Wijbrand Kolman
// SPDX-License-Identifier: MIT

// batteryeink — renders the AccuBox dashboard from /api/data to the 2.7" e-Paper
// HAT (V2). One-shot: fetch, draw a 264x176 frame, push it, sleep. Meant to be
// run periodically by a systemd timer. Default is 4-grey; -mono forces 1-bit.
package main

import (
	"flag"
	"image"
	"image/png"
	"log"
	"os"
	"time"

	"periph.io/x/host/v3"
)

func main() {
	dbFlag := flag.String("db", "", "path to battery.db (default: alongside the binary)")
	pngPath := flag.String("png", "", "also write the rendered frame to this PNG (debug)")
	scale := flag.Int("scale", 1, "nearest-neighbor upscale factor for the debug PNG")
	noPaint := flag.Bool("nopaint", false, "render/preview only; don't touch the panel")
	flashMode := flag.Bool("flash", false, "run a b/w flash test instead of the dashboard")
	mono := flag.Bool("mono", false, "1-bit black/white instead of 4-grey")
	flag.Parse()

	dbFile := *dbFlag
	if dbFile == "" {
		dbFile = defaultDBPath()
	}

	// Render the dashboard image first (needs no hardware) — read the DB directly.
	data, ferr := LoadData(dbFile)
	if ferr != nil {
		log.Printf("db: %v (rendering offline frame)", ferr)
	}
	img := RenderDashboard(data, ferr)

	if *pngPath != "" {
		var preview *image.Gray
		if *mono {
			preview = Bilevel(img)
		} else {
			preview = Quantize4(img)
		}
		f, err := os.Create(*pngPath)
		if err != nil {
			log.Fatalf("png: %v", err)
		}
		if err := png.Encode(f, ScaleNN(preview, *scale)); err != nil {
			log.Fatalf("png encode: %v", err)
		}
		f.Close()
		log.Printf("wrote %s", *pngPath)
	}

	if *noPaint {
		return
	}

	// Hardware.
	if _, err := host.Init(); err != nil {
		log.Fatalf("host init: %v", err)
	}
	e, err := NewEPD()
	if err != nil {
		log.Fatalf("epd: %v", err)
	}
	defer e.Close()

	t := time.Now()
	switch {
	case *flashMode:
		e.Init()
		flashTest(e)
	case *mono:
		e.Init()
		e.Display(GetBufferFromImage(img))
		e.Sleep()
		log.Printf("dashboard painted (mono, %.2fs)", time.Since(t).Seconds())
	default:
		e.Init4Gray()
		p1, p2 := GetBuffers4Gray(img)
		e.Display4Gray(p1, p2)
		e.Sleep()
		log.Printf("dashboard painted (4-grey, %.2fs)", time.Since(t).Seconds())
	}
}

func flashTest(e *EPD) {
	e.Clear()
	black := make([]byte, bufLen)
	white := make([]byte, bufLen)
	for i := range white {
		white[i] = 0xFF
	}
	for i := 0; i < 3; i++ {
		e.Display(black)
		e.Display(white)
	}
	e.Sleep()
	log.Println("flash done")
}
