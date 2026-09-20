package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// now is the clock, injectable so date parsing is testable.
var now = time.Now

// dateLayouts are the written forms accepted for --date, most specific first.
// Renfe itself speaks dd/MM/yyyy; ISO is accepted because that is what scripts
// and agents produce.
var dateLayouts = []string{"2006-01-02", "02/01/2006", "02-01-2006", "2006/01/02", "02/01/06"}

// parseDate turns a user-supplied date into a day. It accepts the written
// layouts above plus the relative forms people actually type at a terminal:
// "", "today", "tomorrow", weekday names, and "+N" for N days out.
func parseDate(s string) (time.Time, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	today := truncateDay(now())
	switch s {
	case "", "today", "hoy", "avui":
		return today, nil
	case "tomorrow", "manana", "mañana", "dema", "demà":
		return today.AddDate(0, 0, 1), nil
	}
	if after, ok := strings.CutPrefix(s, "+"); ok {
		n, err := strconv.Atoi(after)
		if err != nil || n < 0 {
			return time.Time{}, fmt.Errorf("bad relative date %q: use +N days", s)
		}
		return today.AddDate(0, 0, n), nil
	}
	if d, ok := nextWeekday(s, today); ok {
		return d, nil
	}
	for _, layout := range dateLayouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return truncateDay(t), nil
		}
	}
	return time.Time{}, fmt.Errorf("bad date %q: use YYYY-MM-DD, DD/MM/YYYY, today, tomorrow, a weekday, or +N", s)
}

// weekdays maps the English and Spanish day names to their time.Weekday.
var weekdays = map[string]time.Weekday{
	"monday": time.Monday, "lunes": time.Monday,
	"tuesday": time.Tuesday, "martes": time.Tuesday,
	"wednesday": time.Wednesday, "miercoles": time.Wednesday, "miércoles": time.Wednesday,
	"thursday": time.Thursday, "jueves": time.Thursday,
	"friday": time.Friday, "viernes": time.Friday,
	"saturday": time.Saturday, "sabado": time.Saturday, "sábado": time.Saturday,
	"sunday": time.Sunday, "domingo": time.Sunday,
}

// nextWeekday resolves a day name to the next such day strictly after today —
// "friday" typed on a Friday means the one coming, not the one you are on.
func nextWeekday(s string, today time.Time) (time.Time, bool) {
	wd, ok := weekdays[s]
	if !ok {
		return time.Time{}, false
	}
	delta := (int(wd) - int(today.Weekday()) + 7) % 7
	if delta == 0 {
		delta = 7
	}
	return today.AddDate(0, 0, delta), true
}

func truncateDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
