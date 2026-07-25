// SPDX-FileCopyrightText: 2026 Jan-Wijbrand Kolman
// SPDX-License-Identifier: MIT

// batteryweb-go — a single static binary that replaces webapp.py: it serves the
// e-ink dashboard (web/index.html) and the /api/data JSON, reading the logger's
// SQLite DB read-only. Feature-parity with webapp.py, including the 30s cache,
// downsampled 48h series, and local-time day/night bucketing.
//
// Pure Go (modernc.org/sqlite, no cgo) so it cross-compiles to the Pi Zero's
// ARMv6 with just: CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=6 go build
package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	_ "modernc.org/sqlite" // pure-Go SQLite driver
	_ "time/tzdata"        // embed the tz database so LoadLocation works anywhere
)

const (
	minValidTS = 1_700_000_000 // rows below this had an unset clock
	gapCap     = 300           // seconds; don't integrate across longer gaps
	nightStart = 22
	nightEnd   = 6
	cacheTTL   = 30  // seconds; cache the heavy aggregation
	seriesMax  = 480 // downsample the 48h series to at most this many points
)

var (
	baseDir = execDir()
	dbPath  = filepath.Join(baseDir, "battery.db")
	webDir  = filepath.Join(baseDir, "web")
	loc     = mustLoc("Europe/Amsterdam")
	db      *sql.DB

	cacheMu  sync.Mutex
	cacheKey int64 = -1
	cacheVal *heavy
)

func execDir() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Dir(exe)
	}
	return "."
}

func mustLoc(name string) *time.Location {
	l, err := time.LoadLocation(name)
	if err != nil {
		log.Fatalf("timezone %s: %v", name, err)
	}
	return l
}

// --- JSON shapes: identical keys to webapp.py's /api/data ---

type Sample struct {
	TS          int64   `json:"ts"`
	Voltage     float64 `json:"voltage"`
	Current     float64 `json:"current"`
	Power       int     `json:"power"`
	SoC         int     `json:"soc"`
	RemainingAh int     `json:"remaining_ah"`
}

type Latest struct {
	TS          int64   `json:"ts"`
	Voltage     float64 `json:"voltage"`
	Current     float64 `json:"current"`
	Power       int     `json:"power"`
	SoC         int     `json:"soc"`
	RemainingAh int     `json:"remaining_ah"`
	Direction   int     `json:"direction"`
	RtHours     int     `json:"rt_hours"`
	RtMin       int     `json:"rt_min"`
	Raw         string  `json:"raw"`
	Synced      int     `json:"synced"`
}

type Daily struct {
	Day        string  `json:"day"`
	In         float64 `json:"in"`
	Out        float64 `json:"out"`
	Gaps       int     `json:"gaps"`
	GapSeconds int     `json:"gap_seconds"`
}

type Night struct {
	Night string `json:"night"`
	OutAh int    `json:"out_ah"`
}

type heavy struct {
	Series         []Sample
	Daily          []Daily
	Overnight      []Night
	LastNight      *Night
	TotalRows      int
	Since          *int64
	UnsyncedRecent int
}

func round2(x float64) float64 { return math.Round(x*100) / 100 }

// computeHeavy runs the expensive part (series + 7-day aggregation), mirroring
// webapp.py's _compute_heavy. Cached by the caller.
func computeHeavy(now int64) (*heavy, error) {
	h := &heavy{Series: []Sample{}, Daily: []Daily{}, Overnight: []Night{}}

	// 48h series, downsampled for the sparkline.
	sr, err := db.Query(`SELECT ts,voltage,current,power,soc,remaining_ah FROM samples WHERE ts>=? ORDER BY ts`, now-48*3600)
	if err != nil {
		return nil, err
	}
	var series []Sample
	for sr.Next() {
		var s Sample
		if err := sr.Scan(&s.TS, &s.Voltage, &s.Current, &s.Power, &s.SoC, &s.RemainingAh); err != nil {
			sr.Close()
			return nil, err
		}
		series = append(series, s)
	}
	sr.Close()
	if len(series) > seriesMax {
		step := len(series)/seriesMax + 1
		ds := make([]Sample, 0, seriesMax+1)
		for i := 0; i < len(series); i += step {
			ds = append(ds, series[i])
		}
		ds = append(ds, series[len(series)-1]) // always keep the last point
		series = ds
	}
	if series != nil {
		h.Series = series
	}

	// 7-day rows for daily integration + overnight delta.
	type rrow struct {
		ts  int64
		cur float64
		rem int
	}
	rr, err := db.Query(`SELECT ts,current,remaining_ah FROM samples WHERE ts>=? ORDER BY ts`, now-7*86400)
	if err != nil {
		return nil, err
	}
	var rows []rrow
	for rr.Next() {
		var r rrow
		if err := rr.Scan(&r.ts, &r.cur, &r.rem); err != nil {
			rr.Close()
			return nil, err
		}
		rows = append(rows, r)
	}
	rr.Close()

	days := map[string]*Daily{}
	nights := map[string]*[2]int{} // [first,last] remaining_ah in the night window
	for i := range rows {
		a := rows[i]
		lt := time.Unix(a.ts, 0).In(loc)
		hh := lt.Hour()
		if hh >= nightStart || hh < nightEnd {
			nk := lt.Format("2006-01-02")
			if hh < nightEnd {
				nk = lt.AddDate(0, 0, -1).Format("2006-01-02")
			}
			if n, ok := nights[nk]; ok {
				n[1] = a.rem
			} else {
				nights[nk] = &[2]int{a.rem, a.rem}
			}
		}
		if i+1 >= len(rows) {
			continue
		}
		b := rows[i+1]
		day := lt.Format("2006-01-02")
		d, ok := days[day]
		if !ok {
			d = &Daily{Day: day}
			days[day] = d
		}
		dt := b.ts - a.ts
		if dt <= 0 {
			continue
		}
		if dt > gapCap {
			d.Gaps++
			d.GapSeconds += int(dt)
			continue
		}
		ah := (a.cur + b.cur) / 2 * float64(dt) / 3600
		if ah >= 0 {
			d.In += ah
		} else {
			d.Out += -ah
		}
	}

	dayKeys := make([]string, 0, len(days))
	for k := range days {
		dayKeys = append(dayKeys, k)
	}
	sort.Strings(dayKeys)
	for _, k := range dayKeys {
		d := days[k]
		d.In, d.Out = round2(d.In), round2(d.Out)
		h.Daily = append(h.Daily, *d)
	}

	nightKeys := make([]string, 0, len(nights))
	for k := range nights {
		nightKeys = append(nightKeys, k)
	}
	sort.Strings(nightKeys)
	for _, k := range nightKeys {
		n := nights[k]
		out := n[0] - n[1]
		if out < 0 {
			out = 0
		}
		h.Overnight = append(h.Overnight, Night{Night: k, OutAh: out})
	}

	// Most recent night whose 06:00 the following day has already passed.
	nowLocal := time.Unix(now, 0).In(loc)
	for i := len(nightKeys) - 1; i >= 0; i-- {
		k := nightKeys[i]
		d, _ := time.ParseInLocation("2006-01-02", k, loc)
		morning := time.Date(d.Year(), d.Month(), d.Day()+1, nightEnd, 0, 0, 0, loc)
		if !nowLocal.Before(morning) {
			n := nights[k]
			out := n[0] - n[1]
			if out < 0 {
				out = 0
			}
			h.LastNight = &Night{Night: k, OutAh: out}
			break
		}
	}

	var cnt int
	var mn sql.NullInt64
	if err := db.QueryRow(`SELECT COUNT(*), MIN(ts) FROM samples WHERE ts>=?`, minValidTS).Scan(&cnt, &mn); err != nil {
		return nil, err
	}
	h.TotalRows = cnt
	if mn.Valid {
		v := mn.Int64
		h.Since = &v
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM samples WHERE ts>=? AND synced=0`, now-48*3600).Scan(&h.UnsyncedRecent); err != nil {
		return nil, err
	}
	return h, nil
}

func apiData(w http.ResponseWriter, r *http.Request) {
	now := time.Now().Unix()

	// latest is cheap and stays fresh (not cached)
	var l Latest
	err := db.QueryRow(
		`SELECT ts,voltage,current,power,soc,remaining_ah,direction,rt_hours,rt_min,raw,synced `+
			`FROM samples WHERE ts>=? ORDER BY ts DESC LIMIT 1`, minValidTS).
		Scan(&l.TS, &l.Voltage, &l.Current, &l.Power, &l.SoC, &l.RemainingAh,
			&l.Direction, &l.RtHours, &l.RtMin, &l.Raw, &l.Synced)
	var latest *Latest
	if err == nil {
		latest = &l
	} else if err != sql.ErrNoRows {
		log.Println("latest:", err)
	}

	key := now / cacheTTL
	cacheMu.Lock()
	if cacheKey != key || cacheVal == nil {
		h, e := computeHeavy(now)
		if e != nil {
			cacheMu.Unlock()
			http.Error(w, e.Error(), http.StatusInternalServerError)
			return
		}
		cacheVal, cacheKey = h, key
	}
	h := cacheVal
	cacheMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"now":             now,
		"latest":          latest,
		"series":          h.Series,
		"daily":           h.Daily,
		"overnight":       h.Overnight,
		"last_night":      h.LastNight,
		"total_rows":      h.TotalRows,
		"since":           h.Since,
		"unsynced_recent": h.UnsyncedRecent,
	})
}

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	var err error
	// read-only, matching webapp.py; busy_timeout so a concurrent writer never blocks us hard
	db, err = sql.Open("sqlite", "file:"+dbPath+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxOpenConns(4)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/data", apiData)
	mux.Handle("/", http.FileServer(http.Dir(webDir))) // serves index.html at /

	log.Printf("batteryweb-go listening on %s (db=%s, web=%s)", addr, dbPath, webDir)
	log.Fatal(http.ListenAndServe(addr, mux))
}
