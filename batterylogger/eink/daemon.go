// SPDX-FileCopyrightText: 2026 Jan-Wijbrand Kolman
// SPDX-License-Identifier: MIT

// Daemon mode: paint the panel periodically and on button presses.
//
//	KEY1 force refresh · KEY2 cycle 24h/48h/7d · KEY3 WiFi on/off
//	KEY4 tap = system info, hold 3s = power off
package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

// windows is the KEY2 cycle (hours): 24h, 48h, 7d.
var windows = []int{24, 48, 168}

const (
	periodic   = 10 * time.Minute
	longPress  = 3 * time.Second
	coalesceMs = 180 * time.Millisecond
)

// RunDaemon owns the panel + buttons until the process exits.
func RunDaemon(dbFile string, mode PaintMode) error {
	epd, err := NewEPD()
	if err != nil {
		return fmt.Errorf("epd: %w", err)
	}
	defer epd.Close()

	winIdx := 1 // default 48h
	infoView := false

	// Default WiFi off on a fresh boot; the user enables it deliberately with
	// KEY3. Keyed on boot (not daemon restart) so redeploys don't cut access.
	if freshBoot() {
		log.Println("fresh boot: defaulting WiFi off")
		setWifi(false)
	}
	wifiOff := !wifiEnabled()

	events := make(chan KeyEvent, 8)
	go func() {
		if err := WatchKeys(events, longPress); err != nil {
			log.Printf("buttons disabled: %v", err)
		}
	}()

	pnt := &painter{epd: epd, mode: mode}

	paint := func() {
		if infoView {
			pnt.show(RenderSysInfo(sysInfo(wifiOff)), kindSysInfo)
			return
		}
		d, ferr := LoadData(dbFile, windows[winIdx])
		if ferr != nil {
			log.Printf("db: %v", ferr)
		}
		if d != nil {
			d.WifiOff = wifiOff
		}
		pnt.show(RenderDashboard(d, ferr), kindDashboard)
	}

	// apply mutates state / does actions; returns true if a repaint is wanted.
	apply := func(ev KeyEvent) bool {
		switch {
		case ev.Name == "KEY4" && ev.Long:
			log.Println("KEY4 held: powering off")
			// A kind change, so this is a full refresh — right for a frame that
			// then sits on unpowered glass for however long the van is parked.
			pnt.show(RenderMessage("bye", "safe to unplug when the LED is dark"), kindMessage)
			exec.Command("sudo", "poweroff").Run()
			select {} // wait for shutdown
		case ev.Long:
			return false // long-press only means something on KEY4
		case ev.Name == "KEY1":
			infoView = false
			pnt.forceBase() // a real flash, not a maybe-nothing partial
			return true
		case ev.Name == "KEY2":
			infoView = false
			winIdx = (winIdx + 1) % len(windows)
			return true
		case ev.Name == "KEY3":
			setWifi(wifiOff)         // currently off -> turn on, and vice-versa
			wifiOff = !wifiEnabled() // reflect what actually happened
			return true
		case ev.Name == "KEY4":
			infoView = !infoView
			return true
		}
		return false
	}

	log.Printf("daemon up: window=%dh wifiOff=%v mode=%+v", windows[winIdx], wifiOff, mode)
	paint()
	ticker := time.NewTicker(periodic)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			paint()
		case ev := <-events:
			if !apply(ev) {
				continue
			}
			// coalesce quick follow-up presses (e.g. cycling KEY2) into one paint
			deadline := time.After(coalesceMs)
		drain:
			for {
				select {
				case ev2 := <-events:
					apply(ev2)
				case <-deadline:
					break drain
				}
			}
			paint()
		}
	}
}

// --- WiFi (radio only; Bluetooth/BLE untouched) ---

// freshBoot reports whether we're within a few minutes of a boot (so the WiFi
// default-off should apply). Uses uptime so a mid-session daemon restart is safe.
func freshBoot() bool {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return false
	}
	var up float64
	fmt.Sscanf(string(b), "%f", &up)
	return up < 180
}

func wifiEnabled() bool {
	out, err := exec.Command("nmcli", "radio", "wifi").Output()
	if err != nil {
		return true // assume on if we can't tell
	}
	return strings.TrimSpace(string(out)) == "enabled"
}

// setWifi toggles the WiFi radio and the wifi-watchdog together, so the watchdog
// never fights a manual off (its brcmfmac reload would also disrupt Bluetooth).
func setWifi(on bool) {
	if on {
		exec.Command("sudo", "nmcli", "radio", "wifi", "on").Run()
		exec.Command("sudo", "systemctl", "start", "wifi-watchdog.service").Run()
	} else {
		exec.Command("sudo", "systemctl", "stop", "wifi-watchdog.service").Run()
		exec.Command("sudo", "nmcli", "radio", "wifi", "off").Run()
	}
}

// --- KEY4 system-info screen ---

func sysInfo(wifiOff bool) []string {
	var out []string
	if b, err := os.ReadFile("/proc/uptime"); err == nil {
		var up float64
		fmt.Sscanf(string(b), "%f", &up)
		s := int(up)
		out = append(out, fmt.Sprintf("up   %dd %dh %dm", s/86400, (s%86400)/3600, (s%3600)/60))
	}
	ip := "wifi off"
	if !wifiOff {
		ip = "-"
		if addrs, err := net.InterfaceAddrs(); err == nil {
			for _, a := range addrs {
				if n, ok := a.(*net.IPNet); ok && !n.IP.IsLoopback() && n.IP.To4() != nil {
					ip = n.IP.String()
					break
				}
			}
		}
	}
	out = append(out, "ip   "+ip)
	if b, err := os.ReadFile("/sys/class/thermal/thermal_zone0/temp"); err == nil {
		var t int
		fmt.Sscanf(string(b), "%d", &t)
		out = append(out, fmt.Sprintf("cpu  %.0f C", float64(t)/1000))
	}
	var st syscall.Statfs_t
	if syscall.Statfs("/", &st) == nil {
		free := float64(st.Bavail) * float64(st.Bsize) / 1e9
		tot := float64(st.Blocks) * float64(st.Bsize) / 1e9
		out = append(out, fmt.Sprintf("disk %.1f / %.0f GB free", free, tot))
	}
	if _, err := os.Stat("/dev/rtc0"); err == nil {
		out = append(out, "rtc  ok (ds3231)")
	} else {
		out = append(out, "rtc  none")
	}
	return out
}
