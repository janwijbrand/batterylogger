// SPDX-FileCopyrightText: 2026 Jan-Wijbrand Kolman
// SPDX-License-Identifier: MIT

// batteryeink — renders the AccuBox dashboard from /api/data to the 2.7" e-Paper
// HAT (V2). One-shot: fetch, draw a 264x176 1-bit frame, push it, sleep. Meant
// to be run periodically by a systemd timer.
package main

import (
	"flag"
	"image/png"
	"log"
	"os"
	"time"

	"periph.io/x/host/v3"
)

func main() {
	apiURL := flag.String("api", "http://localhost:8080/api/data", "dashboard JSON endpoint")
	pngPath := flag.String("png", "", "also write the rendered frame to this PNG (debug)")
	scale := flag.Int("scale", 1, "nearest-neighbor upscale factor for the debug PNG")
	noPaint := flag.Bool("nopaint", false, "render/preview only; don't touch the panel")
	flashMode := flag.Bool("flash", false, "run a b/w flash test instead of the dashboard")
	flag.Parse()

	// Render the dashboard image first (needs no hardware).
	data, ferr := FetchData(*apiURL)
	if ferr != nil {
		log.Printf("fetch: %v (rendering offline frame)", ferr)
	}
	img := RenderDashboard(data, ferr)

	if *pngPath != "" {
		f, err := os.Create(*pngPath)
		if err != nil {
			log.Fatalf("png: %v", err)
		}
		if err := png.Encode(f, ScaleNN(Bilevel(img), *scale)); err != nil {
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
	e.Init()

	if *flashMode {
		flashTest(e)
		return
	}

	t := time.Now()
	e.Display(GetBufferFromImage(img))
	e.Sleep()
	log.Printf("dashboard painted (%.2fs)", time.Since(t).Seconds())
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
