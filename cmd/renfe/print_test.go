package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/seifreed/renfecli/internal/client"
)

// The views are what a person actually reads, so the things worth locking down
// are the ones that would mislead: a per-person price shown as a party total, a
// wheelchair-only train shown as simply available, a connection whose station
// change goes unmentioned.

func TestPrintResultsWarnsThatPricesArePerPerson(t *testing.T) {
	res := &client.Results{From: "MADRID", To: "BARCELONA", Date: "2026-09-22",
		Journeys: []client.Journey{{Departure: "06:00", Arrival: "09:00", Minutes: 180,
			Direct: true, Trains: []string{"3120"}, Price: 45, Available: true}}}

	one := render(t, res, client.Query{Adults: 1})
	if strings.Contains(one, "PER PERSON") {
		t.Errorf("a solo traveller does not need the multiply-by-one warning:\n%s", one)
	}
	three := render(t, res, client.Query{Adults: 2, Children: 1})
	if !strings.Contains(three, "multiply by 3") {
		t.Errorf("a party of three must be told the price is per person:\n%s", three)
	}
	if !strings.Contains(three, "2 adults, 1 child") {
		t.Errorf("the party should be spelled out:\n%s", three)
	}
}

func TestPrintResultsFlagsWhatCannotBeBought(t *testing.T) {
	res := &client.Results{From: "A", To: "B", Date: "2026-09-22", Journeys: []client.Journey{
		{Departure: "06:00", Arrival: "09:00", Price: 45, WheelchairOnly: true},
		{Departure: "07:00", Arrival: "10:00", SoldOut: true},
		{Departure: "08:00", Arrival: "11:00"},
		{Departure: "09:00", Arrival: "13:00", Trains: []string{"1", "2"},
			StationChange: "cambio de estación en Madrid"},
	}}
	out := render(t, res, client.Query{Adults: 1})
	for _, want := range []string{
		"wheelchair spaces only", // priced, but only for an H seat
		"sold out",
		"no fares",
		"2 legs: 1+2",
		"⚠ cambio de estación en Madrid",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

func TestPrintResultsDateRangeSearchFailed(t *testing.T) {
	res := &client.Results{
		From: "A", To: "B", Date: "[2026-09-22 2026-09-23]", IsRange: true,
		Journeys: []client.Journey{
			{Date: "2026-09-22", Departure: "06:00", Arrival: "09:00", Price: 45, Available: true},
			{Date: "2026-09-23", TrainType: "FAIL"},
		},
	}
	out := render(t, res, client.Query{Adults: 1})
	for _, want := range []string{
		"[2026-09-22 2026-09-23]",
		"2026-09-22 06:00 → 09:00",
		"2026-09-23  search failed",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in date range output with failed day:\n%s", want, out)
		}
	}
}

func TestPrintResultsDateRangeNoJourneys(t *testing.T) {
	res := &client.Results{
		From: "A", To: "B", Date: "[2026-09-22 2026-09-23]", IsRange: true,
		Journeys: []client.Journey{
			{Date: "2026-09-22", Departure: "06:00", Arrival: "09:00", Price: 45, Available: true},
			{Date: "2026-09-23", TrainType: "NONE"},
		},
	}
	out := render(t, res, client.Query{Adults: 1})
	for _, want := range []string{
		"[2026-09-22 2026-09-23]",
		"2026-09-22 06:00 → 09:00",
		"2026-09-23  no journeys found",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in date range output with empty day:\n%s", want, out)
		}
	}
}

func TestPrintResultsEmpty(t *testing.T) {
	out := render(t, &client.Results{From: "A", To: "B", Date: "2026-09-22"}, client.Query{Adults: 1})
	if !strings.Contains(out, "no journeys found") {
		t.Errorf("an empty result must say so:\n%s", out)
	}
}

func TestPrintResultsListsFares(t *testing.T) {
	res := &client.Results{From: "A", To: "B", Date: "2026-09-22", Journeys: []client.Journey{{
		Departure: "06:00", Arrival: "09:00", Price: 45, Available: true,
		Fares: []client.Fare{{Name: "Básico", Price: 45, Class: "T"}, {Name: "Prémium", Price: 120, Class: "P"}},
	}}}
	var sb strings.Builder
	if err := printResults(&sb, res, client.Query{Adults: 1}, true); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Básico", "turista", "Prémium", "preferente", "120.00"} {
		if !strings.Contains(sb.String(), want) {
			t.Errorf("missing %q in:\n%s", want, sb.String())
		}
	}
	if strings.Contains(render(t, res, client.Query{Adults: 1}), "Prémium") {
		t.Error("fare buckets must stay hidden without --fares")
	}
}

func TestPrintResultsDateRange(t *testing.T) {
	res := &client.Results{
		From: "A", To: "B", Date: "[2026-09-22 2026-09-23]", IsRange: true,
		Journeys: []client.Journey{
			{Date: "2026-09-22", Departure: "06:00", Arrival: "09:00", Price: 45, Available: true},
			{Date: "2026-09-23", Departure: "07:00", Arrival: "10:00", Price: 50, Available: true},
		},
	}
	out := render(t, res, client.Query{Adults: 1})
	for _, want := range []string{
		"[2026-09-22 2026-09-23]",
		"2026-09-22 06:00 → 09:00",
		"2026-09-23 07:00 → 10:00",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in date range output:\n%s", want, out)
		}
	}
}

func TestPrintCalendarMarksTheCheapestDay(t *testing.T) {
	days := []client.PriceDay{
		{Date: "2026-09-21", MinPrice: 60, Available: true},
		{Date: "2026-09-22", MinPrice: 30, Available: true},
		{Date: "2026-09-23", MinPrice: 0},
	}
	var sb strings.Builder
	if err := printCalendar(&sb, &client.Results{From: "A", To: "B"}, days, cheapestDay(days)); err != nil {
		t.Fatal(err)
	}
	out := sb.String()
	if !strings.Contains(out, "2026-09-22        30 €  ← cheapest") {
		t.Errorf("the cheapest day must be marked:\n%s", out)
	}
	if !strings.Contains(out, "2026-09-23           —") {
		t.Errorf("a day with nothing on sale must not read as free:\n%s", out)
	}
}

func TestPrintItinerary(t *testing.T) {
	it := &client.Itinerary{From: "MADRID", To: "CÁDIZ", Departure: "07:05", Arrival: "11:58",
		TransferTime: "1 horas 40 minutos", StationChange: "Atocha → Chamartín",
		Legs: []client.ItineraryLeg{{Train: "207", TrainType: "ALVIA", From: "MADRID", To: "SEVILLA",
			Departure: "07:05", Arrival: "09:40",
			Stops:    []client.Stop{{Name: "CÓRDOBA", Arrival: "08:45", Departure: "08:47"}},
			Services: []string{"Cafetería"}}}}
	var sb strings.Builder
	if err := printItinerary(&sb, it); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"(enlace: 1 horas 40 minutos)", "⚠ Atocha → Chamartín",
		"ALVIA 207", "CÓRDOBA", "(salida)", "(llegada)", "prestaciones: Cafetería"} {
		if !strings.Contains(sb.String(), want) {
			t.Errorf("missing %q in:\n%s", want, sb.String())
		}
	}
}

func TestPrintTripsEmpty(t *testing.T) {
	var sb strings.Builder
	if err := printTrips(&sb, nil); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sb.String(), "no upcoming trips") {
		t.Errorf("got %q", sb.String())
	}
}

// A write that fails must reach the caller rather than being discarded line by
// line, which is what fmt.Printf did before the views took a Writer.
func TestPrintReportsAWriteFailure(t *testing.T) {
	res := &client.Results{From: "A", To: "B", Journeys: []client.Journey{{Departure: "06:00"}}}
	if err := printResults(brokenPipe{}, res, client.Query{Adults: 1}, false); err == nil {
		t.Fatal("a failed write must be reported")
	}
}

type brokenPipe struct{}

func (brokenPipe) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

// trainType keeps the type column aligned; cutting by byte would split a rune
// in an accented name and emit U+FFFD.
func TestTrainTypeTruncatesByRune(t *testing.T) {
	got := trainType("Regional Exprés")
	if got != "Regional " {
		t.Errorf("trainType = %q, want %q", got, "Regional ")
	}
	if strings.ContainsRune(trainType("AVE Íntercité"), '�') {
		t.Error("truncation split a rune")
	}
}

func render(t *testing.T, res *client.Results, q client.Query) string {
	t.Helper()
	var sb strings.Builder
	if err := printResults(&sb, res, q, false); err != nil {
		t.Fatal(err)
	}
	return sb.String()
}
