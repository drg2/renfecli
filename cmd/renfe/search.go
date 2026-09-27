package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/seifreed/renfecli/internal/client"
	"github.com/seifreed/renfecli/internal/config"
)

// searchFlags are the journey options shared by `search` and `calendar`.
type searchFlags struct {
	date     string
	ret      string
	adults   int
	children int
	infants  int
	pet      bool
	bike     bool
	plazaH   bool
	direct   bool
}

func addSearchFlags(fs *flag.FlagSet) *searchFlags {
	s := &searchFlags{}
	fs.StringVar(&s.date, "date", "", "departure date: YYYY-MM-DD, DD/MM/YYYY, 'today', 'tomorrow', +N days, or range '[from until]' (default today)")
	fs.StringVar(&s.ret, "return", "", "return date: prints the return journeys too and prices the outbound under round-trip rules")
	fs.IntVar(&s.adults, "adults", 0, "adult passengers (default 1, or [defaults] adults)")
	fs.IntVar(&s.children, "children", 0, "children aged 4-13")
	fs.IntVar(&s.infants, "infants", 0, "infants under 4 (no seat)")
	fs.BoolVar(&s.pet, "pet", false, "travelling with a pet")
	fs.BoolVar(&s.bike, "bike", false, "travelling with a bicycle")
	fs.BoolVar(&s.plazaH, "wheelchair", false, "request a wheelchair (H) space")
	fs.BoolVar(&s.direct, "direct", false, "exclude journeys with a connection")
	return s
}

// searchSpec holds the parsed parameters for search, supporting single dates or ranges.
type searchSpec struct {
	origin      client.Station
	dest        client.Station
	outDates    []time.Time
	outDisplay  string
	outIsRange  bool
	retDates    []time.Time
	retDisplay  string
	retIsRange  bool
	hasReturn   bool
	adults      int
	children    int
	infants     int
	pet         bool
	bike        bool
	wheelchairH bool
	direct      bool
}

func buildSearchSpec(cmd string, cl *client.Client, cfg config.Config, sf *searchFlags, args []string) (searchSpec, error) {
	var spec searchSpec
	from, to := cfg.Defaults.Origin, cfg.Defaults.Destination
	switch len(args) {
	case 0:
	case 1:
		to = args[0]
	default:
		from, to = args[0], strings.Join(args[1:], " ")
	}
	if from == "" || to == "" {
		return spec, fmt.Errorf("need an origin and a destination: renfe %s <origin> <destination> [--date …]", cmd)
	}
	stations, err := loadStations(cl, false)
	if err != nil {
		return spec, err
	}
	origin, err := client.ResolveStation(stations, from)
	if err != nil {
		return spec, err
	}
	dest, err := client.ResolveStation(stations, to)
	if err != nil {
		return spec, err
	}

	outDates, outDisp, outIsRange, err := parseDateSpec(sf.date)
	if err != nil {
		return spec, err
	}

	today := truncateDay(now())
	if outDates[0].Before(today) {
		return spec, fmt.Errorf("%s has already gone by — Renfe sells from today (%s) onwards",
			outDates[0].Format("2006-01-02"), today.Format("2006-01-02"))
	}

	var retDates []time.Time
	var retDisp string
	var retIsRange bool
	hasReturn := false
	if sf.ret != "" {
		hasReturn = true
		if retDates, retDisp, retIsRange, err = parseDateSpec(sf.ret); err != nil {
			return spec, err
		}
		if len(retDates) != len(outDates) && len(outDates) > 1 && len(retDates) > 1 {
			return spec, fmt.Errorf("outbound date range and return date range must have the same number of days")
		}
		if retDates[0].Before(outDates[0]) {
			return spec, fmt.Errorf("the return date %s is before the outbound %s",
				retDates[0].Format("2006-01-02"), outDates[0].Format("2006-01-02"))
		}
	}

	for _, c := range []struct {
		flag string
		n    int
	}{{"--adults", sf.adults}, {"--children", sf.children}, {"--infants", sf.infants}} {
		if err := nonNegative(c.flag, c.n); err != nil {
			return spec, err
		}
	}
	adults := sf.adults
	if adults == 0 {
		adults = cfg.Defaults.Adults
	}
	if adults == 0 {
		adults = 1
	}

	return searchSpec{
		origin: origin, dest: dest,
		outDates: outDates, outDisplay: outDisp, outIsRange: outIsRange,
		retDates: retDates, retDisplay: retDisp, retIsRange: retIsRange,
		hasReturn:   hasReturn,
		adults:      adults,
		children:    sf.children,
		infants:     sf.infants,
		pet:         sf.pet,
		bike:        sf.bike,
		wheelchairH: sf.plazaH,
		direct:      sf.direct,
	}, nil
}

// buildQuery resolves the positional origin/destination (falling back to the
// configured defaults) and the flags into a client.Query.
func buildQuery(cmd string, cl *client.Client, cfg config.Config, sf *searchFlags, args []string) (client.Query, error) {
	spec, err := buildSearchSpec(cmd, cl, cfg, sf, args)
	if err != nil {
		return client.Query{}, err
	}
	var retDate time.Time
	if spec.hasReturn {
		retDate = spec.retDates[0]
	}
	return client.Query{
		Origin: spec.origin.Code, OriginName: spec.origin.Name,
		Destination: spec.dest.Code, DestName: spec.dest.Name,
		Date: spec.outDates[0], Return: retDate,
		Adults: spec.adults, Children: spec.children, Infants: spec.infants,
		Pet: spec.pet, Bike: spec.bike, WheelchairH: spec.wheelchairH, Direct: spec.direct,
	}, nil
}

// runSearch is the opening the journey commands share: read the config,
// build a client, resolve the positional route and the flags into a query, and
// run the search.
func runSearch(cmd string, sf *searchFlags, args []string) (*client.Client, client.Query, *client.Results, error) {
	cfg := loadConfig()
	cl := newClient(cfg)
	q, err := buildQuery(cmd, cl, cfg, sf, args)
	if err != nil {
		return nil, q, nil, err
	}
	res, err := cl.Search(q)
	if err != nil {
		return nil, q, nil, err
	}
	return cl, q, res, nil
}

func cmdSearch(args []string) error {
	fs, cf := newCommonFlags("search")
	sf := addSearchFlags(fs)
	train := fs.String("train", "", "only show journeys for this train number (e.g. 3063)")
	after := fs.String("after", "", "only outbound trains departing at or after this time (HH:MM)")
	before := fs.String("before", "", "only outbound trains departing at or before this time (HH:MM)")
	retAfter := fs.String("return-after", "", "same, for the return leg (default: the outbound window)")
	retBefore := fs.String("return-before", "", "same, for the return leg")
	cheapest := fs.Bool("cheapest", false, "sort by price instead of departure time")
	available := fs.Bool("available", false, "hide sold-out and unpriced journeys")
	fares := fs.Bool("fares", false, "list every fare bucket, not just the cheapest")
	limit := fs.Int("limit", 0, "cap the number of journeys shown")
	parseFlags(fs, args)

	if err := nonNegative("--limit", *limit); err != nil {
		return err
	}
	cfg := loadConfig()
	cl := newClient(cfg)
	spec, err := buildSearchSpec("search", cl, cfg, sf, fs.Args())
	if err != nil {
		return err
	}

	window, err := parseTimeWindow(*after, *before)
	if err != nil {
		return err
	}
	retWindow := window
	if *retAfter != "" || *retBefore != "" {
		if retWindow, err = parseTimeWindow(*retAfter, *retBefore); err != nil {
			return err
		}
	}

	var combinedOutJourneys []client.Journey
	var combinedRetJourneys []client.Journey
	var calendar []client.PriceDay
	fromName, toName := "", ""

	maxIter := len(spec.outDates)
	if len(spec.retDates) > maxIter {
		maxIter = len(spec.retDates)
	}

	for i := 0; i < maxIter; i++ {
		outIdx := i
		if outIdx >= len(spec.outDates) {
			outIdx = len(spec.outDates) - 1
		}
		d := spec.outDates[outIdx]

		var rDate time.Time
		if spec.hasReturn {
			retIdx := i
			if retIdx >= len(spec.retDates) {
				retIdx = len(spec.retDates) - 1
			}
			rDate = spec.retDates[retIdx]
		}

		q := client.Query{
			Origin: spec.origin.Code, OriginName: spec.origin.Name,
			Destination: spec.dest.Code, DestName: spec.dest.Name,
			Date: d, Return: rDate,
			Adults: spec.adults, Children: spec.children, Infants: spec.infants,
			Pet: spec.pet, Bike: spec.bike, WheelchairH: spec.wheelchairH, Direct: spec.direct,
		}

		res, err := cl.Search(q)
		if err != nil {
			if maxIter == 1 {
				return err
			}
			stderrLogf("search failed for date %s: %v", d.Format("2006-01-02"), err)
			if spec.outIsRange && !*available && i < len(spec.outDates) {
				combinedOutJourneys = append(combinedOutJourneys, client.Journey{
					Date:      d.Format("2006-01-02"),
					TrainType: "FAIL",
				})
			}
			if spec.retIsRange && !*available && spec.hasReturn && (i < len(spec.retDates) || len(spec.retDates) == 1) {
				combinedRetJourneys = append(combinedRetJourneys, client.Journey{
					Date:      rDate.Format("2006-01-02"),
					TrainType: "FAIL",
				})
			}
			continue
		}

		fromName = res.From
		toName = res.To
		if len(calendar) == 0 && len(res.Calendar) > 0 {
			calendar = res.Calendar
		}

		if i < len(spec.outDates) {
			filteredOut := filterJourneys(res.Journeys, *available, *cheapest, *limit, window, *train)
			if len(filteredOut) == 0 && spec.outIsRange && !*available {
				filteredOut = append(filteredOut, client.Journey{
					Date:      d.Format("2006-01-02"),
					TrainType: "NONE",
				})
			}
			combinedOutJourneys = append(combinedOutJourneys, filteredOut...)
		}

		if res.Return != nil && (spec.hasReturn && (i < len(spec.retDates) || len(spec.retDates) == 1)) {
			filteredRet := filterJourneys(res.Return.Journeys, *available, *cheapest, *limit, retWindow, *train)
			if len(filteredRet) == 0 && spec.retIsRange && !*available {
				filteredRet = append(filteredRet, client.Journey{
					Date:      rDate.Format("2006-01-02"),
					TrainType: "NONE",
				})
			}
			combinedRetJourneys = append(combinedRetJourneys, filteredRet...)
		}
	}

	if fromName == "" {
		fromName = spec.origin.Name
		toName = spec.dest.Name
	}

	finalRes := &client.Results{
		From:     fromName,
		To:       toName,
		Date:     spec.outDisplay,
		IsRange:  spec.outIsRange,
		Journeys: combinedOutJourneys,
		Calendar: calendar,
	}
	if spec.hasReturn {
		finalRes.Return = &client.Results{
			From:     toName,
			To:       fromName,
			Date:     spec.retDisplay,
			IsRange:  spec.retIsRange,
			Journeys: combinedRetJourneys,
		}
	}

	qForPrint := client.Query{
		Adults: spec.adults, Children: spec.children, Infants: spec.infants,
	}

	if emitted, err := emitStructured(cf, finalRes); emitted {
		return err
	}
	if err := printResults(os.Stdout, finalRes, qForPrint, *fares); err != nil {
		return err
	}
	if finalRes.Return == nil {
		return nil
	}
	fmt.Println()
	if err := printResults(os.Stdout, finalRes.Return, qForPrint, *fares); err != nil {
		return err
	}
	if note := sameDayNote(finalRes); note != "" {
		fmt.Print("\n" + note)
	}
	return nil
}

// sameDayNote warns when a round trip is booked out and back on one day. The
// two legs are listed independently, so a return that departs before the
// outbound can possibly arrive still appears as a normal option — on a long
// route that is most of them, and quoting one is quoting a trip nobody can
// make. Empty for an overnight trip, where the question does not arise.
func sameDayNote(res *client.Results) string {
	if res.Return == nil || res.Date != res.Return.Date {
		return ""
	}
	earliest, at := -1, ""
	for _, j := range res.Journeys {
		a := arrivalMinutes(j)
		if a < 0 || (earliest >= 0 && a >= earliest) {
			continue
		}
		earliest, at = a, j.Arrival
	}
	if earliest < 0 {
		return ""
	}
	if earliest >= dayMinutes {
		at += " the next day"
	}
	catchable, total := 0, 0
	for _, j := range res.Return.Journeys {
		d := clockMinutes(j.Departure)
		if d < 0 {
			continue
		}
		total++
		if d > earliest {
			catchable++
		}
	}
	switch {
	case total == 0:
		return ""
	case catchable == 0:
		return fmt.Sprintf("same-day return is NOT possible: the earliest arrival is %s and "+
			"every listed return leaves before that\n", at)
	case catchable == total:
		return ""
	default:
		return fmt.Sprintf("same-day return: earliest arrival %s, so only %d of the %d returns above "+
			"are catchable (the rest leave before you get there)\n", at, catchable, total)
	}
}

const dayMinutes = 24 * 60

// normaliseClock validates a user-supplied HH:MM and returns it zero-padded, so
// "8:27" matches a departure Renfe prints as "08:27". flag names the option in
// the error, since every option that takes a time validates through here.
func normaliseClock(raw, flag string) (string, error) {
	t, err := time.Parse("15:04", strings.TrimSpace(raw))
	if err != nil {
		return "", fmt.Errorf("bad %s time %q: use HH:MM, e.g. 15:30", flag, raw)
	}
	return t.Format("15:04"), nil
}

// clockMinutes turns "HH:MM" into minutes since midnight, or -1 when the value
// is not a time — a missing or malformed one must not read as 00:00.
func clockMinutes(hhmm string) int {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return -1
	}
	return t.Hour()*60 + t.Minute()
}

// arrivalMinutes is when a journey lands, counted from midnight on the day it
// left. A train departing 22:05 and arriving 01:30 lands at 1530 minutes, not
// 90: comparing the printed times makes the latest arrival of the day look
// like the earliest, which is exactly when a same-day return is impossible and
// the warning matters most. Without a departure time there is nothing to
// detect the wrap against, so the arrival is taken as given.
func arrivalMinutes(j client.Journey) int {
	arr := clockMinutes(j.Arrival)
	if arr < 0 {
		return -1
	}
	if dep := clockMinutes(j.Departure); dep >= 0 && arr < dep {
		arr += dayMinutes
	}
	return arr
}

// timeWindow bounds a departure time. Empty ends are open.
type timeWindow struct{ after, before string } // "HH:MM", lexically comparable

func (w timeWindow) contains(depart string) bool {
	if w.after != "" && depart < w.after {
		return false
	}
	return w.before == "" || depart <= w.before
}

func (w timeWindow) set() bool { return w.after != "" || w.before != "" }

// parseTimeWindow validates --after/--before. Renfe's own site filters by time
// in the browser over the results it already has, so this is done locally too:
// it costs no extra request, and the API takes no time parameter that works.
func parseTimeWindow(after, before string) (timeWindow, error) {
	w := timeWindow{}
	for _, f := range []struct {
		raw  string
		dst  *string
		name string
	}{{after, &w.after, "--after"}, {before, &w.before, "--before"}} {
		if f.raw == "" {
			continue
		}
		v, err := normaliseClock(f.raw, f.name)
		if err != nil {
			return w, err
		}
		*f.dst = v
	}
	if w.after != "" && w.before != "" && w.after > w.before {
		return w, fmt.Errorf("--after %s is later than --before %s — no train can match", w.after, w.before)
	}
	return w, nil
}

// filterJourneys applies the display options in a fixed order — drop, then
// sort, then cap — so --limit always counts what the user actually sees.
func filterJourneys(js []client.Journey, onlyAvailable, byPrice bool, limit int, window timeWindow, train string) []client.Journey {
	if window.set() {
		js = keepJourneys(js, func(j client.Journey) bool { return window.contains(j.Departure) })
	}
	if want := client.TrainNumber(train); want != "" {
		js = keepJourneys(js, func(j client.Journey) bool {
			for _, t := range j.Trains {
				if t == want {
					return true
				}
			}
			return false
		})
	}
	if onlyAvailable {
		js = keepJourneys(js, func(j client.Journey) bool { return j.Available })
	}
	if byPrice {
		sort.SliceStable(js, func(i, k int) bool {
			// Sold-out journeys quote no price; they sort last rather than first.
			pi, pk := js[i].Price, js[k].Price
			if (pi == 0) != (pk == 0) {
				return pk == 0
			}
			return pi < pk
		})
	}
	if limit > 0 && len(js) > limit {
		js = js[:limit]
	}
	return js
}

// keepJourneys returns the journeys matching pred. It copies rather than
// filtering in place: the caller's slice is the client's, and a round trip
// filters both directions off the same search.
func keepJourneys(js []client.Journey, pred func(client.Journey) bool) []client.Journey {
	kept := js[:0:0]
	for _, j := range js {
		if pred(j) {
			kept = append(kept, j)
		}
	}
	return kept
}
