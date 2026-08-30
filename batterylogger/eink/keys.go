// SPDX-FileCopyrightText: 2026 Jan-Wijbrand Kolman
// SPDX-License-Identifier: MIT

// The 4 buttons on the Waveshare 2.7" e-Paper HAT. Active-low with a pull-up
// (pressed = falling edge). KEY1..4 = BCM 5/6/13/19 (verified via -keytest).
package main

import (
	"fmt"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
)

type key struct {
	name string
	pin  gpio.PinIO
}

var keyPins = []struct{ name, bcm string }{
	{"KEY1", "GPIO5"},
	{"KEY2", "GPIO6"},
	{"KEY3", "GPIO13"},
	{"KEY4", "GPIO19"},
}

func setupKeys() ([]*key, error) {
	var keys []*key
	for _, d := range keyPins {
		p := gpioreg.ByName(d.bcm)
		if p == nil {
			return nil, fmt.Errorf("gpio %s (%s) not found", d.bcm, d.name)
		}
		if err := p.In(gpio.PullUp, gpio.NoEdge); err != nil {
			return nil, fmt.Errorf("%s: %w", d.name, err)
		}
		keys = append(keys, &key{name: d.name, pin: p})
	}
	return keys, nil
}

// KeyEvent is a debounced button event. Long is true once a button has been
// held for the long-press threshold (used by KEY4 to power off).
type KeyEvent struct {
	Name string
	Long bool
}

// WatchKeys polls the 4 buttons and emits KeyEvents. A short press fires on
// release (if it wasn't held long); a long press fires once, at longAfter,
// while still held. Runs until the process exits.
func WatchKeys(ch chan<- KeyEvent, longAfter time.Duration) error {
	keys, err := setupKeys()
	if err != nil {
		return err
	}
	n := len(keys)
	down := make([]bool, n)
	start := make([]time.Time, n)
	longFired := make([]bool, n)
	const debounce = 30 * time.Millisecond
	for {
		now := time.Now()
		for i, k := range keys {
			low := k.pin.Read() == gpio.Low // pressed
			switch {
			case low && !down[i]:
				down[i] = true
				start[i] = now
				longFired[i] = false
			case low && down[i]:
				if !longFired[i] && now.Sub(start[i]) >= longAfter {
					longFired[i] = true
					ch <- KeyEvent{Name: keys[i].name, Long: true}
				}
			case !low && down[i]:
				down[i] = false
				if !longFired[i] && now.Sub(start[i]) >= debounce {
					ch <- KeyEvent{Name: keys[i].name, Long: false}
				}
			}
		}
		time.Sleep(15 * time.Millisecond)
	}
}

// scanPins: general-purpose GPIOs to watch when hunting for the button pins
// (avoids the SPI display pins 8/10/11/17/24/25/18 and I²C RTC pins 2/3).
var scanPins = []string{
	"GPIO4", "GPIO5", "GPIO6", "GPIO12", "GPIO13", "GPIO16",
	"GPIO19", "GPIO20", "GPIO21", "GPIO22", "GPIO23", "GPIO26", "GPIO27",
}

// KeyScan watches every candidate GPIO and reports which drops LOW on a press —
// finds the button pins regardless of the assumed mapping.
func KeyScan() error {
	type p struct {
		name string
		pin  gpio.PinIO
		last gpio.Level
	}
	var ps []*p
	fmt.Print("watching: ")
	for _, n := range scanPins {
		pin := gpioreg.ByName(n)
		if pin == nil {
			continue
		}
		if err := pin.In(gpio.PullUp, gpio.NoEdge); err != nil {
			continue
		}
		l := pin.Read()
		ps = append(ps, &p{name: n, pin: pin, last: l})
		fmt.Printf("%s=%v ", n, l)
	}
	fmt.Println("\npress any KEY now — I report the pin that drops LOW...")
	for {
		for _, x := range ps {
			cur := x.pin.Read()
			if x.last == gpio.High && cur == gpio.Low {
				fmt.Printf(">>> %s -> LOW (a button is here)\n", x.name)
			}
			x.last = cur
		}
		time.Sleep(15 * time.Millisecond)
	}
}

// KeyTest polls the buttons and prints presses — used to confirm the BCM
// mapping and that the pull-ups read HIGH at rest, LOW when pressed.
func KeyTest() error {
	keys, err := setupKeys()
	if err != nil {
		return err
	}
	last := make([]gpio.Level, len(keys))
	fmt.Print("resting levels: ")
	for i, k := range keys {
		last[i] = k.pin.Read()
		fmt.Printf("%s=%v ", k.name, last[i])
	}
	fmt.Println("\n(High=idle, Low=pressed) — press KEY1..KEY4 now...")
	for {
		for i, k := range keys {
			cur := k.pin.Read()
			if last[i] == gpio.High && cur == gpio.Low {
				fmt.Printf("%s pressed (pin %s)\n", k.name, k.pin.Name())
			}
			last[i] = cur
		}
		time.Sleep(15 * time.Millisecond)
	}
}
