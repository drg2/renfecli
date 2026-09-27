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

func TestParseDateSpec(t *testing.T) {
	fixClock(t)

	// Single date test
	dates, display, isRange, err := parseDateSpec("today")
	if err != nil {
		t.Fatalf("unexpected error for single date: %v", err)
	}
	if isRange || len(dates) != 1 || display != "2026-09-22" {
		t.Errorf("parseDateSpec(\"today\") = %v, %q, %v; want [2026-09-22], \"2026-09-22\", false", dates, display, isRange)
	}

	// Date range test
	dates, display, isRange, err = parseDateSpec("[today +3]")
	if err != nil {
		t.Fatalf("unexpected error for range: %v", err)
	}
	if !isRange || len(dates) != 4 || display != "[2026-09-22 2026-09-25]" {
		t.Errorf("parseDateSpec(\"[today +3]\") = %v, %q, %v; want 4 dates, \"[2026-09-22 2026-09-25]\", true", dates, display, isRange)
	}

	// Exceeds 90 days test
	_, _, _, err = parseDateSpec("[today +95]")
	if err == nil {
		t.Errorf("expected error for date range > 90 days, got nil")
	}

	// End before start test
	_, _, _, err = parseDateSpec("[tomorrow today]")
	if err == nil {
		t.Errorf("expected error for end date before start date, got nil")
	}

	// Unclosed bracket test
	_, _, _, err = parseDateSpec("[today tomorrow")
	if err == nil {
		t.Errorf("expected error for unclosed bracket, got nil")
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
