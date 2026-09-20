package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"unicode"
)

// StationsURL is the catalogue the renfe.com search box itself loads: every
// station the booking engine accepts, with the codes the API expects. It is a
// JavaScript file (`var estacionesEstatico=[…];`) served as ISO-8859-1.
const StationsURL = "https://www.renfe.com/content/dam/renfe/es/General/buscadores/javascript/estacionesEstaticas.js"

// Station is one entry of that catalogue.
type Station struct {
	Code string `json:"code"` // cdgoEstacion — what the search API takes
	Name string `json:"name"` // display name, accented
	Flat string `json:"flat"` // unaccented name, what the site matches against
	UIC  string `json:"uic,omitempty"`
	// Priority is Renfe's own ordering of the catalogue (nmroPrioridad): the
	// city-wide groups and main termini first, request stops last. It is what
	// makes "madrid" resolve to MADRID (TODAS) rather than a suburban halt.
	Priority int `json:"priority"`
}

// Stations fetches the station catalogue. Callers should cache it (see the
// stations command); it changes a few times a year, not per request.
func (c *Client) Stations() ([]Station, error) {
	data, err := c.doRetry("estacionesEstaticas.js", func() ([]byte, error) {
		req, err := http.NewRequest("GET", StationsURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("user-agent", c.UserAgent)
		req.Header.Set("accept", "*/*")
		return c.do(req)
	})
	if err != nil {
		return nil, err
	}
	return parseStations(data)
}

// parseStations pulls the first array literal out of the catalogue script. The
// file is ISO-8859-1 (its second array, estacionesDestacada, is a subset we
// ignore), so bytes are widened to runes — which *is* latin-1 decoding — before
// the JSON decoder sees them.
func parseStations(data []byte) ([]Station, error) {
	s := latin1(data)
	open := strings.IndexByte(s, '[')
	if open < 0 {
		return nil, fmt.Errorf("station catalogue: no array literal")
	}
	body, err := sliceCallArg(s, open)
	if err != nil {
		return nil, fmt.Errorf("station catalogue: %w", err)
	}
	var raw []struct {
		Code     string `json:"cdgoEstacion"`
		Name     string `json:"desgEstacion"`
		Flat     string `json:"desgEstacionPlano"`
		UIC      string `json:"cdgoUic"`
		Priority int    `json:"nmroPrioridad"`
	}
	if err := json.Unmarshal([]byte(body), &raw); err != nil {
		return nil, fmt.Errorf("station catalogue: %w", err)
	}
	out := make([]Station, 0, len(raw))
	for _, r := range raw {
		if r.Code == "" || r.Name == "" {
			continue
		}
		out = append(out, Station{
			Code: r.Code, Name: r.Name, Flat: firstNonEmpty(r.Flat, r.Name),
			UIC: r.UIC, Priority: r.Priority,
		})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("station catalogue: empty")
	}
	return out, nil
}

// latin1 decodes ISO-8859-1 bytes, where every byte is its own code point.
func latin1(b []byte) string {
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = rune(c)
	}
	return string(r)
}

// FindStations ranks the catalogue against a user's query. Matching is on the
// unaccented, case-folded name so "cordoba" finds "CÓRDOBA", and results are
// ordered exact code → exact name → prefix → substring, then by Renfe's own
// station priority — so "madrid" resolves to the MADRID (TODAS) group, exactly
// as the website's own autocomplete would rank it.
func FindStations(all []Station, query string) []Station {
	q := applyExonym(fold(query))
	if q == "" {
		return nil
	}
	type scored struct {
		s    Station
		rank int
	}
	var hits []scored
	for _, s := range all {
		name := fold(s.Flat)
		switch {
		case strings.EqualFold(s.Code, query):
			hits = append(hits, scored{s, 0})
		case name == q:
			hits = append(hits, scored{s, 1})
		case strings.HasPrefix(name, q):
			hits = append(hits, scored{s, 2})
		case strings.Contains(name, q):
			hits = append(hits, scored{s, 3})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].rank != hits[j].rank {
			return hits[i].rank < hits[j].rank
		}
		return hits[i].s.Priority < hits[j].s.Priority
	})
	out := make([]Station, len(hits))
	for i, h := range hits {
		out[i] = h.s
	}
	return out
}

// ResolveStation picks the single best match for a query, or reports the
// ambiguity. A query that is already a station code resolves to itself.
func ResolveStation(all []Station, query string) (Station, error) {
	hits := FindStations(all, query)
	if len(hits) == 0 {
		return Station{}, fmt.Errorf("no station matches %q — try: renfe stations %s", query, query)
	}
	return hits[0], nil
}

// exonyms maps the Spanish name of a foreign station onto the spelling the
// catalogue uses. Renfe serves a handful of destinations in France and Portugal
// and lists them under their local names, so a Spanish speaker asking for
// "Marsella" or "Oporto" would otherwise be told no such station exists. Only
// places actually served are listed; there is no Lisboa in the catalogue.
var exonyms = map[string]string{
	"marsella":  "marseille",
	"perpinan":  "perpignan",
	"avinon":    "avignon",
	"oporto":    "porto campanha",
	"aquisgran": "", // deliberately absent: not served, so it must not match
}

// applyExonym rewrites a folded query through the exonym table; unknown names
// and names mapped to "" pass through unchanged.
func applyExonym(folded string) string {
	if to, ok := exonyms[folded]; ok && to != "" {
		return to
	}
	return folded
}

// fold normalises a name for matching: lower case, accents stripped, runs of
// punctuation and whitespace collapsed to a single space.
func fold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	lastSpace := true
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if d, ok := deaccent[r]; ok {
			r = d
		}
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastSpace = false
		case !lastSpace:
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

// deaccent covers the accented letters that occur in Spanish, Catalan, Galician
// and Basque station names — the full set present in the catalogue.
var deaccent = map[rune]rune{
	'á': 'a', 'à': 'a', 'ä': 'a', 'â': 'a',
	'é': 'e', 'è': 'e', 'ë': 'e', 'ê': 'e',
	'í': 'i', 'ì': 'i', 'ï': 'i', 'î': 'i',
	'ó': 'o', 'ò': 'o', 'ö': 'o', 'ô': 'o',
	'ú': 'u', 'ù': 'u', 'ü': 'u', 'û': 'u',
	'ñ': 'n', 'ç': 'c',
}
