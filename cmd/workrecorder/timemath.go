package main

import (
	"time"
)

// (13:00:00.75, 2min, 30sec) => 13:00:00, 13:00:30, 13:01:00, 13:01:30
// (13:00:01.00, 2min, 30sec) =>           13:00:30, 13:01:00, 13:01:30
func ticksBetween(now time.Time, periodMins int, dur time.Duration) []time.Time {
	min := timeFloorMinutes(now, periodMins)
	max := timeFloorMinutesAddNPeriod(now, periodMins, 1)

	return filterNotBefore(floorSecond(now), timestampsBetween(min, max, dur))
}

// discards fractional seconds
func timestampsBetween(start time.Time, stopExclusive time.Time, interval time.Duration) []time.Time {
	ns := []time.Time{}
	current := start
	for current.Before(stopExclusive) {
		ns = append(ns, current)
		current = current.Add(interval)
	}

	return ns
}

func timeFloorMinutes(t time.Time, minutes int) time.Time {
	return timeFloorMinutesAddNPeriod(t, minutes, 0)
}

func timeFloorMinutesAddNPeriod(t time.Time, minutes int, n int) time.Time {
	return time.Date(
		t.Year(),
		t.Month(),
		t.Day(),
		t.Hour(),
		t.Minute()/minutes*minutes+n*minutes,
		0,
		0,
		time.UTC)
}

func floorSecond(input time.Time) time.Time {
	return time.Date(
		input.Year(),
		input.Month(),
		input.Day(),
		input.Hour(),
		input.Minute(),
		input.Second(),
		0,
		input.Location())
}

// timestamps >= perspective
func filterNotBefore(than time.Time, items []time.Time) []time.Time {
	match := []time.Time{}

	for _, item := range items {
		if !item.Before(than) {
			match = append(match, item)
		}
	}

	return match
}
