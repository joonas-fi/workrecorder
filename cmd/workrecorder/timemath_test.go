package main

import (
	"strings"
	"testing"
	"time"

	"github.com/function61/gokit/testing/assert"
)

func TestTicksBetween(t *testing.T) {
	t13 := func(minute int, second int) time.Time { // t13(20,0) => 13:20:00
		return time.Date(2020, 2, 26, 13, minute, second, 0, time.UTC)
	}

	serializeTicksBetween := func(now time.Time, periodMins int, dur time.Duration) string {
		items := []string{""} // start with empty line
		for _, x := range ticksBetween(now, periodMins, dur) {
			items = append(items, x.Format("15:04:05"))
		}
		return strings.Join(items, "\n")
	}

	assert.EqualString(t, serializeTicksBetween(t13(20, 0), 5, 30*time.Second), `
13:20:00
13:20:30
13:21:00
13:21:30
13:22:00
13:22:30
13:23:00
13:23:30
13:24:00
13:24:30`)

	// 13:20:15 should miss tick 13:20:00 and start from the next
	assert.EqualString(t, serializeTicksBetween(t13(20, 15), 5, 30*time.Second), `
13:20:30
13:21:00
13:21:30
13:22:00
13:22:30
13:23:00
13:23:30
13:24:00
13:24:30`)
}

func TestFlooring(t *testing.T) {
	t13 := func(n int) time.Time { // t13(20) => 13:20
		return time.Date(2020, 2, 26, 13, n, 0, 0, time.UTC)
	}

	floor5 := func(t time.Time) time.Time {
		return timeFloorMinutes(t, 5)
	}

	str := func(t time.Time) string {
		return t.Format("15:04")
	}

	assert.EqualString(t, str(floor5(t13(20))), "13:20")
	assert.EqualString(t, str(floor5(t13(23))), "13:20")
	assert.EqualString(t, str(floor5(t13(24))), "13:20")
	assert.EqualString(t, str(floor5(t13(25))), "13:25")

	assert.EqualString(t, str(timeFloorMinutesAddNPeriod(t13(20), 5, 1)), "13:25")
	assert.EqualString(t, str(timeFloorMinutesAddNPeriod(t13(24), 5, 1)), "13:25")
	assert.EqualString(t, str(timeFloorMinutesAddNPeriod(t13(25), 5, 1)), "13:30")
}
