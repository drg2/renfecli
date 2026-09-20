package client

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// madridCadiz is the journey the browser capture in testdata was taken from.
func madridCadiz() Journey {
	return Journey{
		Departure: "07:05", Arrival: "11:58", Date: "2026-09-22",
		TrainType: "ALVIA", Direct: true, Trains: []string{"2074"},
		From: "MADRID-PUERTA DE ATOCHA-ALMUDENA GRANDES", To: "CÁDIZ",
		FromCode: "60000", ToCode: "51405",
		legs: []journeyLeg{{
			CdgoTren: "02074", FechaCI: "2026-09-16", CodGrupo: "1", TipoTren: "ALVIA",
			OrigenCode: "60000", DestinoCode: "51405",
			OrigenName: "MADRID-PUERTA DE ATOCHA-ALMUDENA GRANDES", DestinoName: "CÁDIZ",
			// The search reports leg times with seconds; the bean wants HH:MM.
			HoraSalida: "07:05:00", HoraLlegada: "11:58:00",
		}},
	}
}

// TestRecorridoBodyMatchesBrowser pins the encoded request against the body
// Chrome actually sent. getRecorrido answers a mismatched object with an empty
// array rather than an error, so a drift here would look like "no itinerary"
// instead of failing loudly — hence the byte-level comparison.
func TestRecorridoBodyMatchesBrowser(t *testing.T) {
	params, err := recorridoParams(madridCadiz())
	if err != nil {
		t.Fatal(err)
	}
	body := dwrCall("trainEnlacesManager", "getRecorrido", "/vol/buscarTrenEnlaces.do", "ss/renfecli", params)

	want := []string{
		// The nested leg object's members are declared before the array that
		// references them, exactly as DWR's own client orders them.
		"c0-e1=boolean:true\n",
		"c0-e2=string:2026-09-22\n",
		"c0-e3=string:\n",
		"c0-e6=string:02074\n",
		"c0-e7=string:2026-09-16\n",
		"c0-e9=string:ALVIA\n",
		"c0-e12=string:MADRID-PUERTA%20DE%20ATOCHA-ALMUDENA%20GRANDES\n",
		"c0-e13=string:C%C3%81DIZ\n",
		"c0-e14=string:07%3A05\n",
		"c0-e15=string:11%3A58\n",
		"c0-e5=Object_Object:{codTren:reference:c0-e6, fechaCI:reference:c0-e7, codGrupo:reference:c0-e8, tipoTren:reference:c0-e9, cdgoEstacionOrigen:reference:c0-e10, cdgoEstacionDestino:reference:c0-e11, descEstacionOrigen:reference:c0-e12, descEstacionDestino:reference:c0-e13, horaSalida:reference:c0-e14, horaLlegada:reference:c0-e15}\n",
		"c0-e4=array:[reference:c0-e5]\n",
		"c0-param0=Object_Object:{directo:reference:c0-e1, fecha:reference:c0-e2, duracionTrasbordo:reference:c0-e3, trenes:reference:c0-e4, descripcionEstacionOrigen:reference:c0-e16, descripcionEstacionDestino:reference:c0-e17, codigoEstacionOrigen:reference:c0-e18, codigoEstacionDestino:reference:c0-e19, horaSalida:reference:c0-e20, horaLlegada:reference:c0-e21}\n",
	}
	for _, w := range want {
		if !strings.Contains(body, w) {
			t.Errorf("body missing:\n%s\ngot:\n%s", w, body)
		}
	}
}

// The leg times must lose their seconds: Renfe answers "07:05:00" with an empty
// array, which is indistinguishable from a genuinely unknown route.
func TestRecorridoTrimsSecondsFromLegTimes(t *testing.T) {
	params, _ := recorridoParams(madridCadiz())
	body := dwrCall("b", "m", "/p", "s", params)
	if strings.Contains(body, "07%3A05%3A00") {
		t.Errorf("leg time kept its seconds:\n%s", body)
	}
}

// The separator that cost an afternoon: DWR joins array elements with a bare
// comma but object members with a comma AND a space. With one leg there is no
// separator, so a direct train worked and every connection came back as a
// ConversionException.
func TestRecorridoArraySeparatorHasNoSpace(t *testing.T) {
	j := madridCadiz()
	second := j.legs[0]
	second.CdgoTren = "13039"
	second.TipoTren = "MD"
	j.legs = append(j.legs, second)
	j.Direct = false
	j.TransferTime = "1 horas 40 minutos"

	params, err := recorridoParams(j)
	if err != nil {
		t.Fatal(err)
	}
	body := dwrCall("b", "m", "/p", "s", params)
	if !strings.Contains(body, "array:[reference:c0-e5,reference:c0-e16]") {
		t.Errorf("array must join with a bare comma, and number the second leg from e16:\n%s", body)
	}
	// The object literal keeps its space, so the two must not share a joiner.
	if !strings.Contains(body, "codTren:reference:c0-e17, fechaCI:") {
		t.Errorf("object members must keep the comma+space:\n%s", body)
	}
	// directo is a real boolean and duracionTrasbordo carries the search's own
	// wording; an empty string here is rejected for a connection.
	if !strings.Contains(body, "c0-e1=boolean:false\n") {
		t.Errorf("directo should be boolean:false:\n%s", body)
	}
	if !strings.Contains(body, "c0-e3=string:1%20horas%2040%20minutos\n") {
		t.Errorf("duracionTrasbordo missing:\n%s", body)
	}
}

func TestStopsNeedsLegData(t *testing.T) {
	// A Journey that did not come from a Search on this client carries no legs.
	if _, err := New().Stops(Journey{Trains: []string{"2074"}}); err == nil {
		t.Fatal("expected an error")
	} else if !strings.Contains(err.Error(), "Search") {
		t.Errorf("the error should say where the journey must come from, got %q", err)
	}
}

func TestDecodeRecorridoFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/recorrido.dwr")
	if err != nil {
		t.Fatal(err)
	}
	var legs []recorridoLeg
	if err := decodeDWR(string(raw), &legs); err != nil {
		t.Fatalf("decodeDWR: %v", err)
	}
	if len(legs) != 1 {
		t.Fatalf("got %d legs, want 1", len(legs))
	}
	// The fixture is a train that terminates where the ticket does, so the
	// segment's destination is not among the calling points and nothing is
	// clipped — the ordinary case.
	it := legs[0].leg(journeyLeg{
		TipoTren: "ALVIA", OrigenCode: "60000", DestinoCode: "51405",
		OrigenName: "MADRID", DestinoName: "CÁDIZ",
		HoraSalida: "07:05:00", HoraLlegada: "11:58:00",
	})
	if it.From != "MADRID" || it.To != "CÁDIZ" {
		t.Errorf("endpoints = %s → %s, want the segment's own", it.From, it.To)
	}
	if it.Departure != "07:05" || it.Arrival != "11:58" {
		t.Errorf("times = %s → %s, want the seconds trimmed", it.Departure, it.Arrival)
	}
	if it.Train != "2074" {
		t.Errorf("train = %q, want 2074 (leading zeros trimmed)", it.Train)
	}
	if len(it.Stops) != 7 {
		t.Fatalf("got %d stops, want 7", len(it.Stops))
	}
	first, last := it.Stops[0], it.Stops[6]
	if first.Name != "Ciudad Real" || first.Arrival != "08:01" || first.Code != "37200" {
		t.Errorf("first stop = %+v", first)
	}
	if last.Name != "San Fernando-bahía Sur" {
		t.Errorf("last stop = %+v", last)
	}
	// The endpoints are not repeated in the stop list.
	for _, s := range it.Stops {
		if s.Code == "60000" || s.Code == "51405" {
			t.Errorf("stop list should hold only intermediate calls, found %s", s.Name)
		}
	}
	if len(it.Services) != 5 || !strings.Contains(strings.Join(it.Services, "|"), "Sala Club") {
		t.Errorf("services = %v", it.Services)
	}
}

func TestHHMM(t *testing.T) {
	for in, want := range map[string]string{
		"07:05:00": "07:05", "07:05": "07:05", "": "", "7:5": "7:5",
	} {
		if got := hhmm(in); got != want {
			t.Errorf("hhmm(%q) = %q, want %q", in, got, want)
		}
	}
}

// An expired login makes getMyJourneys answer with an empty array and no error,
// which is indistinguishable from an account with nothing booked. Reporting the
// first as the second tells a traveller they have no ticket when they may have
// one, so an empty list is confirmed against the session first.
func TestTripsEmptyIsCheckedAgainstTheSession(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if strings.Contains(r.URL.Path, "getMyJourneys") {
			_, _ = fmt.Fprint(w, `r.handleCallback("0","0",[]);`)
			return
		}
		// getIdentificacionUsuario: nobody is signed in.
		_, _ = fmt.Fprint(w, `r.handleCallback("0","0","");`)
	}))
	defer srv.Close()

	c := New()
	c.BaseURL = srv.URL
	if _, err := c.Trips(); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("err = %v, want ErrSessionExpired rather than an empty trip list", err)
	}
	if calls < 2 {
		t.Errorf("the empty list should have been confirmed with a second call, got %d", calls)
	}
}

// A signed-in account with nothing booked must still report an empty list, not
// an error.
func TestTripsEmptyForASignedInAccount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "getMyJourneys") {
			_, _ = fmt.Fprint(w, `r.handleCallback("0","0",[]);`)
			return
		}
		_, _ = fmt.Fprint(w, `r.handleCallback("0","0","MARC RIVERO");`)
	}))
	defer srv.Close()

	c := New()
	c.BaseURL = srv.URL
	trips, err := c.Trips()
	if err != nil {
		t.Fatalf("Trips: %v", err)
	}
	if len(trips) != 0 {
		t.Errorf("got %d trips, want none", len(trips))
	}
}

// The bean answers with the train's whole run. A Madrid–Barcelona seat on a
// train that carries on to Figueres came back with Girona and Figueres among
// the stops and Figueres as the arrival — a journey ending in another city.
func TestItineraryIsClippedToTheTicketsSegment(t *testing.T) {
	run := []recorridoStop{
		{Code: "04007", Name: "Guadalajara"},
		{Code: "04040", Name: "Zaragoza"},
		{Code: "71801", Name: "Barcelona-sants"},
		{Code: "79300", Name: "Girona"},
	}
	leg := recorridoLeg{
		Origen: "Madrid", Destino: "Figueres-vilafant",
		HoraSalida: "07:27", HoraLlegada: "12:24", NumTrem: "03073",
		Stops: run,
	}
	got := leg.leg(journeyLeg{
		TipoTren: "AVE", OrigenCode: "60000", DestinoCode: "71801",
		OrigenName: "MADRID", DestinoName: "BARCELONA-SANTS",
		HoraSalida: "07:27", HoraLlegada: "11:11",
	})
	if got.To != "BARCELONA-SANTS" || got.Arrival != "11:11" {
		t.Errorf("the leg ends at %s %s, want BARCELONA-SANTS 11:11", got.To, got.Arrival)
	}
	var names []string
	for _, s := range got.Stops {
		names = append(names, s.Name)
	}
	if len(names) != 2 || names[0] != "Guadalajara" || names[1] != "Zaragoza" {
		t.Errorf("calling points = %v, want only those before Barcelona", names)
	}

	// Boarding mid-route clips the other end too.
	got = leg.leg(journeyLeg{OrigenCode: "04040", DestinoCode: "71801"})
	if len(got.Stops) != 0 {
		t.Errorf("Zaragoza→Barcelona has no intermediate calls here, got %d", len(got.Stops))
	}

	// With no segment to clip against, the train's own run is reported rather
	// than an empty list.
	got = leg.leg(journeyLeg{})
	if len(got.Stops) != 4 || got.To != "Figueres-vilafant" {
		t.Errorf("unclipped leg = %s with %d stops", got.To, len(got.Stops))
	}
}
