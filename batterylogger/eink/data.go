// SPDX-FileCopyrightText: 2026 Jan-Wijbrand Kolman
// SPDX-License-Identifier: MIT

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Latest mirrors the `latest` object from batteryweb-go's /api/data.
type Latest struct {
	TS          int64   `json:"ts"`
	Voltage     float64 `json:"voltage"`
	Current     float64 `json:"current"`
	Power       int     `json:"power"`
	SoC         int     `json:"soc"`
	RemainingAh int     `json:"remaining_ah"`
	Direction   int     `json:"direction"` // 0 = charging(+), else discharging(-)
	RtHours     int     `json:"rt_hours"`
	RtMin       int     `json:"rt_min"`
	Synced      int     `json:"synced"`
}

type SeriesPt struct {
	TS  int64 `json:"ts"`
	SoC int   `json:"soc"`
}

type Night struct {
	Night string `json:"night"`
	OutAh int    `json:"out_ah"`
}

// APIData is the subset of /api/data the display needs.
type APIData struct {
	Now       int64      `json:"now"`
	Latest    *Latest    `json:"latest"`
	Series    []SeriesPt `json:"series"`
	LastNight *Night     `json:"last_night"`
}

// FetchData pulls /api/data from the local dashboard server.
func FetchData(url string) (*APIData, error) {
	c := &http.Client{Timeout: 10 * time.Second}
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	var d APIData
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return nil, err
	}
	return &d, nil
}
