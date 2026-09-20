package main

import (
	"testing"
	"time"
)

// Pin the clock to a Tuesday so the relative forms are deterministic.
func fixClock(t *testing.T) {
	t.Helper()
	orig := now
	now = func() time.Time { return time.Date(2026, 9, 22, 14, 30, 0, 0, time.Local) }
	t.Cleanup(func() { now = orig })
}

func TestParseDate(t *testing.T) {
	fixClock(t)
	tests := map[string]string{
		"":           "2026-09-22",
		"today":      "2026-09-22",
		"hoy":        "2026-09-22",
		"tomorrow":   "2026-09-23",
		"mañana":     "2026-09-23",
		"+0":         "2026-09-22",
		"+10":        "2026-10-02",
		"2026-12-25": "2026-12-25",
		"25/12/2026": "2026-12-25",
		"25-12-2026": "2026-12-25",
		"25/12/26":   "2026-12-25",
		// A weekday means the *next* one, so asking on a Tuesday for "tuesday"
		// gets the week ahead rather than a date already past its trains.
		"tuesday": "2026-09-29",
		"friday":  "2026-09-25",
		"viernes": "2026-09-25",
		"domingo": "2026-09-27",
	}
	for in, want := range tests {
		got, err := parseDate(in)
		if err != nil {
			t.Errorf("parseDate(%q): %v", in, err)
			continue
		}
		if got.Format(time.DateOnly) != want {
			t.Errorf("parseDate(%q) = %s, want %s", in, got.Format(time.DateOnly), want)
		}
		if got.Hour() != 0 || got.Minute() != 0 {
			t.Errorf("parseDate(%q) kept a time component: %s", in, got)
		}
	}
}

func TestParseDateRejectsNonsense(t *testing.T) {
	fixClock(t)
	for _, in := range []string{"someday", "+", "+-3", "2026-13-45", "32/01/2026"} {
		if _, err := parseDate(in); err == nil {
			t.Errorf("parseDate(%q) = nil error, want failure", in)
		}
	}
}
