// SPDX-FileCopyrightText: 2026 Jan-Wijbrand Kolman
// SPDX-License-Identifier: MIT

package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (cgo-free, runs on ARMv6)
	_ "time/tzdata"        // embed the tz database so LoadLocation works anywhere
)

const (
	minValidTS = 1_700_000_000 // rows below this had an unset clock
	nightStart = 22            // local-time overnight window
	nightEnd   = 6
	seriesMax  = 300 // downsample the 48h SoC series to at most this many points
)

var loc = mustLoad("Europe/Amsterdam")

func mustLoad(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return l
}

// Latest is the freshest sample.
type Latest struct {
	TS          int64
	Voltage     float64
	Current     float64
	Power       int
	SoC         int
	RemainingAh int
	Direction   int // 0 = charging(+), else discharging(-)
	RtHours     int
	RtMin       int
	Synced      int
}

type SeriesPt struct {
	TS  int64
	SoC int
}

type Night struct {
	Night string
	OutAh int
}

// APIData is what the renderer needs (kept as a struct so render.go is unchanged).
type APIData struct {
	Now       int64
	Latest    *Latest
	Series    []SeriesPt
	LastNight *Night
}

// defaultDBPath looks for battery.db next to the binary (matches the deploy).
func defaultDBPath() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "battery.db")
	}
	return "battery.db"
}

// LoadData reads the dashboard figures straight from the logger's SQLite DB
// (read-only, WAL-safe) — no HTTP, no webserver. Mirrors the subset of
// batteryweb-go's /api/data that the e-ink layout uses.
func LoadData(path string) (*APIData, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	defer db.Close()

	now := time.Now().Unix()
	d := &APIData{Now: now}

	// latest sample
	var l Latest
	err = db.QueryRow(
		`SELECT ts,voltage,current,power,soc,remaining_ah,direction,rt_hours,rt_min,synced `+
			`FROM samples WHERE ts>=? ORDER BY ts DESC LIMIT 1`, minValidTS).
		Scan(&l.TS, &l.Voltage, &l.Current, &l.Power, &l.SoC, &l.RemainingAh,
			&l.Direction, &l.RtHours, &l.RtMin, &l.Synced)
	if err == nil {
		d.Latest = &l
	} else if err != sql.ErrNoRows {
		return nil, err
	}

	// 48h SoC series for the sparkline, downsampled
	sr, err := db.Query(`SELECT ts,soc FROM samples WHERE ts>=? ORDER BY ts`, now-48*3600)
	if err != nil {
		return nil, err
	}
	var series []SeriesPt
	for sr.Next() {
		var p SeriesPt
		if err := sr.Scan(&p.TS, &p.SoC); err != nil {
			sr.Close()
			return nil, err
		}
		series = append(series, p)
	}
	sr.Close()
	if len(series) > seriesMax {
		step := len(series)/seriesMax + 1
		ds := make([]SeriesPt, 0, seriesMax+1)
		for i := 0; i < len(series); i += step {
			ds = append(ds, series[i])
		}
		ds = append(ds, series[len(series)-1]) // always keep the last point
		series = ds
	}
	d.Series = series

	// last_night: overnight remaining_ah delta (22:00–06:00 local) for the most
	// recent night whose 06:00 has already passed.
	nr, err := db.Query(`SELECT ts,remaining_ah FROM samples WHERE ts>=? ORDER BY ts`, now-7*86400)
	if err != nil {
		return nil, err
	}
	nights := map[string]*[2]int{} // [first,last] remaining_ah
	for nr.Next() {
		var ts int64
		var rem int
		if err := nr.Scan(&ts, &rem); err != nil {
			nr.Close()
			return nil, err
		}
		lt := time.Unix(ts, 0).In(loc)
		hh := lt.Hour()
		if hh >= nightStart || hh < nightEnd {
			nk := lt.Format("2006-01-02")
			if hh < nightEnd {
				nk = lt.AddDate(0, 0, -1).Format("2006-01-02")
			}
			if n, ok := nights[nk]; ok {
				n[1] = rem
			} else {
				nights[nk] = &[2]int{rem, rem}
			}
		}
	}
	nr.Close()
	keys := make([]string, 0, len(nights))
	for k := range nights {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	nowLocal := time.Unix(now, 0).In(loc)
	for i := len(keys) - 1; i >= 0; i-- {
		k := keys[i]
		dt, _ := time.ParseInLocation("2006-01-02", k, loc)
		morning := time.Date(dt.Year(), dt.Month(), dt.Day()+1, nightEnd, 0, 0, 0, loc)
		if !nowLocal.Before(morning) {
			n := nights[k]
			out := n[0] - n[1]
			if out < 0 {
				out = 0
			}
			d.LastNight = &Night{Night: k, OutAh: out}
			break
		}
	}
	return d, nil
}
