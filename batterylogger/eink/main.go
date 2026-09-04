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
	keytest := flag.Bool("keytest", false, "monitor the 4 HAT buttons and print presses")
	daemon := flag.Bool("daemon", false, "run continuously: paint periodically + on button presses")
	screen := flag.String("screen", "dashboard", "which frame to render: dashboard | sysinfo | message")
	wifiOff := flag.Bool("wifioff", false, "preview the dashboard as it looks with WiFi off")
	fourGray := flag.Bool("4gray", false, "daemon: use the old 4-grey full-refresh path")
	fastBase := flag.Bool("fastbase", false, "use the fast (0xC7) waveform for base refreshes")
	partialTest := flag.Bool("partialtest", false, "paint a scripted sequence through the real paint policy")
	interval := flag.Duration("interval", 15*time.Second, "-partialtest: pause between frames")
	dumpCmds := flag.Bool("dumpcmds", false, "print the command stream a paint would emit; no hardware")
	flag.Parse()
	mode := PaintMode{FourGray: *fourGray, FastBase: *fastBase}

	if *dumpCmds {
		DumpCommands(os.Stdout)
		return
	}

	if *keytest {
		if _, err := host.Init(); err != nil {
			log.Fatalf("host init: %v", err)
		}
		if err := KeyTest(); err != nil {
			log.Fatalf("keytest: %v", err)
		}
		return
	}

	dbFile := *dbFlag
	if dbFile == "" {
		dbFile = defaultDBPath()
	}

	if *daemon {
		if _, err := host.Init(); err != nil {
			log.Fatalf("host init: %v", err)
		}
		log.Fatal(RunDaemon(dbFile, mode))
	}

	if *partialTest {
		if _, err := host.Init(); err != nil {
			log.Fatalf("host init: %v", err)
		}
		e, err := NewEPD()
		if err != nil {
			log.Fatalf("epd: %v", err)
		}
		defer e.Close()
		PartialTest(e, *interval, mode)
		return
	}

	// One-shot: render the frame first (needs no hardware).
	var img *image.Gray
	switch *screen {
	case "sysinfo": // the KEY4 screen, with stand-in values so it can be checked off-box
		img = RenderSysInfo([]string{
			"up   12d 4h 37m", "ip   192.168.178.42", "cpu  46 C",
			"disk 21.4 / 30 GB free", "rtc  ok (ds3231)",
		})
	case "message":
		img = RenderMessage("bye", "safe to unplug when the LED is dark")
	default:
		data, ferr := LoadData(dbFile, 48)
		if ferr != nil {
			log.Printf("db: %v (rendering offline frame)", ferr)
		}
		if data != nil {
			data.WifiOff = *wifiOff
		}
		img = RenderDashboard(data, ferr)
	}

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
		flashTest(e)
	case *mono:
		e.Display(GetBufferFromImage(img))
		e.Sleep()
		log.Printf("dashboard painted (mono, %.2fs)", time.Since(t).Seconds())
	default:
		p1, p2 := GetBuffers4Gray(img)
		e.Display4Gray(p1, p2)
		e.Sleep()
		log.Printf("dashboard painted (4-grey, %.2fs)", time.Since(t).Seconds())
	}
}

func flashTest(e *EPD) {
	e.Clear()
	black := make([]byte, bufLen)
	white := whiteBuf()
	for i := 0; i < 3; i++ {
		e.Display(black)
		e.Display(white)
	}
	e.Sleep()
	log.Println("flash done")
}
