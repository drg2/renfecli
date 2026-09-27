package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/seifreed/renfecli/internal/client"
)

// The human-readable views. They are kept apart from the commands, and write to
// an io.Writer rather than straight to os.Stdout, so the formatting is testable
// on its own — a command's job is to fetch and filter, not to lay out a table.

// printer is a sticky-error writer: it keeps the first write failure and skips
// the rest, so a view reads as the layout it is rather than as a page of
// two-value assignments. Callers check p.err once, at the end — which is more
// than fmt.Printf gave us, since that discarded the error on every line.
type printer struct {
	w   io.Writer
	err error
}

// printf writes one line of a view. Every string argument is a value the
// server chose, so it is stripped of control characters here — centrally,
// rather than at each of the thirty call sites. The format strings are the
// CLI's own, so its newlines and padding are untouched.
func (p *printer) printf(format string, args ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintf(p.w, format, safeArgs(args)...)
}

func (p *printer) println(args ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintln(p.w, safeArgs(args)...)
}

func safeArgs(args []any) []any {
	for i, a := range args {
		if s, ok := a.(string); ok {
			args[i] = safeField(s)
		}
	}
	return args
}

func printResults(w io.Writer, res *client.Results, q client.Query, withFares bool) error {
	p := &printer{w: w}
	p.printf("%s → %s   %s   %s\n", res.From, res.To, res.Date, passengers(q))
	// Renfe quotes per passenger, not per booking. With one traveller that is
	// the same number; with more it is off by a factor of however many, which is
	// exactly the mistake worth spending a line to prevent.
	if q.Seated() > 1 {
		p.printf("(prices are PER PERSON — multiply by %d for the party)\n", q.Seated())
	}
	// Two sentences rather than one clause, so neither has to agree with a
	// count that may be one or many.
	if q.Infants > 0 {
		p.println("(under-4s travel on a lap: they are not in the price above, " +
			"and Renfe quotes no fare for them here)")
	}
	p.println()
	if len(res.Journeys) == 0 {
		p.println("no journeys found")
		return p.err
	}
	for _, j := range res.Journeys {
		depTime := j.Departure
		if res.IsRange && j.Date != "" {
			depTime = j.Date + " " + j.Departure
		}
		p.printf("%-9s %s → %s  %-9s  %-22s  %s%s\n",
			trainType(j.TrainType), depTime, j.Arrival, duration(j.Minutes),
			trainLabel(j), price(j), tags(j))
		indent := "           "
		if res.IsRange && j.Date != "" {
			indent += strings.Repeat(" ", len(j.Date)+1)
		}
		if j.StationChange != "" {
			p.printf("%s⚠ %s\n", indent, j.StationChange)
		}
		if withFares {
			for _, f := range j.Fares {
				p.printf("%s%-16s %8.2f €  %s\n", indent, f.Name, f.Price, fareClass(f.Class))
			}
		}
	}
	if len(res.Calendar) > 0 {
		p.printf("\ncheapest nearby: %s  (renfe calendar for the full strip)\n", calendarLine(res.Calendar))
	}
	return p.err
}

// printCalendar prints the cheapest-fare-per-day strip, marking the best day.
// days is what the user asked to see; best is computed over the whole strip, so
// --cheapest still labels the one row it prints.
func printCalendar(w io.Writer, res *client.Results, days []client.PriceDay, best client.PriceDay) error {
	p := &printer{w: w}
	p.printf("%s → %s   cheapest fare per day\n\n", res.From, res.To)
	for _, d := range days {
		switch {
		case !d.Available || d.MinPrice <= 0:
			p.printf("%s  %10s\n", d.Date, "—")
		case d.Date == best.Date:
			p.printf("%s  %8.0f €  ← cheapest\n", d.Date, d.MinPrice)
		default:
			p.printf("%s  %8.0f €\n", d.Date, d.MinPrice)
		}
	}
	return p.err
}

func printItinerary(w io.Writer, it *client.Itinerary) error {
	p := &printer{w: w}
	p.printf("%s → %s   %s → %s", it.From, it.To, it.Departure, it.Arrival)
	if it.TransferTime != "" {
		p.printf("   (enlace: %s)", it.TransferTime)
	}
	p.printf("\n")
	if it.StationChange != "" {
		p.printf("⚠ %s\n", it.StationChange)
	}
	for _, leg := range it.Legs {
		p.printf("\n%s %s   %s → %s\n", leg.TrainType, leg.Train, leg.From, leg.To)
		p.printf("  %-6s %s  (salida)\n", leg.Departure, leg.From)
		for _, s := range leg.Stops {
			p.printf("  %-6s %s\n", s.Arrival, s.Name)
		}
		p.printf("  %-6s %s  (llegada)\n", leg.Arrival, leg.To)
		if len(leg.Services) > 0 {
			p.println("  prestaciones: " + strings.Join(leg.Services, "; "))
		}
	}
	return p.err
}

func printTrips(w io.Writer, trips []client.Trip) error {
	p := &printer{w: w}
	if len(trips) == 0 {
		p.println("no upcoming trips")
		return p.err
	}
	for _, t := range trips {
		p.printf("%s  %s → %s  %s %s  %s\n",
			t.Date, t.Departure, t.Arrival, t.TrainType, t.Train, t.Locator)
	}
	return p.err
}

func passengers(q client.Query) string {
	parts := []string{plural(q.Adults, "adult", "adults")}
	if q.Children > 0 {
		parts = append(parts, plural(q.Children, "child", "children"))
	}
	if q.Infants > 0 {
		parts = append(parts, plural(q.Infants, "infant", "infants"))
	}
	return strings.Join(parts, ", ")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// duration renders the API's minute count as "3h 23m"; the API also sends a
// localised sentence, but a fixed-width form keeps the table aligned.
func duration(minutes int) string {
	if minutes <= 0 {
		return ""
	}
	if minutes < 60 {
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%dh %02dm", minutes/60, minutes%60)
}

func trainLabel(j client.Journey) string {
	if len(j.Trains) == 0 {
		return ""
	}
	if j.Direct {
		return "tren " + j.Trains[0]
	}
	return fmt.Sprintf("%d legs: %s", len(j.Trains), strings.Join(j.Trains, "+"))
}

func price(j client.Journey) string {
	if j.SoldOut {
		return "sold out"
	}
	if j.Price == 0 {
		return "no fares"
	}
	if j.WheelchairOnly {
		// The fare is real but only buyable with --wheelchair, so showing the
		// bare price would promise a seat that is not for sale.
		return fmt.Sprintf("%.2f € — wheelchair spaces only", j.Price)
	}
	return fmt.Sprintf("from %.2f €", j.Price)
}

func tags(j client.Journey) string {
	var t []string
	if j.Cheapest {
		t = append(t, "cheapest")
	}
	if j.Fastest {
		t = append(t, "fastest")
	}
	if len(t) == 0 {
		return ""
	}
	return "  [" + strings.Join(t, ", ") + "]"
}

func fareClass(code string) string {
	switch code {
	case "T":
		return "turista"
	case "P":
		return "preferente"
	default:
		return code
	}
}

// calendarLine summarises the price strip Renfe returns with every search: the
// cheapest day on offer around the searched date.
func calendarLine(days []client.PriceDay) string {
	best := cheapestDay(days)
	if best.Date == "" {
		return "no priced days nearby"
	}
	return fmt.Sprintf("%s from %.0f €", best.Date, best.MinPrice)
}

// trainType caps the type column so a long name (Renfe sends "Intercit",
// "R.EXPRES") cannot push the rest of the row out of alignment.
func trainType(s string) string {
	const width = 9
	r := []rune(s)
	if len(r) > width {
		return string(r[:width])
	}
	return s
}
