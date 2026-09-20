package client

import (
	"fmt"
	"strings"
)

// Stop is one station a train calls at between the journey's own endpoints.
type Stop struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Arrival   string `json:"arrival"`
	Departure string `json:"departure"`
}

// Itinerary is a journey's route. A journey with a connection has one Leg per
// train, each with its own calling points and onboard services.
type Itinerary struct {
	From         string `json:"from"`
	To           string `json:"to"`
	Departure    string `json:"departure"`
	Arrival      string `json:"arrival"`
	TransferTime string `json:"transfer_time,omitempty"`
	// StationChange carries Renfe's warning when the connection means moving
	// between two different stations.
	StationChange string         `json:"station_change,omitempty"`
	Legs          []ItineraryLeg `json:"legs"`
}

// ItineraryLeg is one train of an itinerary.
type ItineraryLeg struct {
	Train     string   `json:"train"`
	TrainType string   `json:"train_type,omitempty"`
	From      string   `json:"from"`
	To        string   `json:"to"`
	Departure string   `json:"departure"`
	Arrival   string   `json:"arrival"`
	Stops     []Stop   `json:"stops"`
	Services  []string `json:"services,omitempty"`
}

// Stops fetches a journey's intermediate stops.
//
// It MUST follow a Search on the same Client: the bean matches the train
// against the journey list cached in the HTTP session, and answers an empty
// array — not an error — when that list is missing or does not contain it.
// Calling it with a Journey from a different Client (or after another search
// on the same one) is what "no itinerary" almost always means.
func (c *Client) Stops(j Journey) (*Itinerary, error) {
	if len(j.legs) == 0 {
		return nil, fmt.Errorf("journey carries no leg data — it did not come from a Search on this client")
	}
	params, err := recorridoParams(j)
	if err != nil {
		return nil, err
	}
	var raw []recorridoLeg
	if err := c.callDWR("trainEnlacesManager", "getRecorrido", searchPage, params, &raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return nil, fmt.Errorf("renfe returned no itinerary for train %s — "+
			"the search session may have expired; re-run the search",
			strings.Join(j.Trains, "+"))
	}
	it := &Itinerary{
		From: j.From, To: j.To,
		Departure: j.Departure, Arrival: j.Arrival,
		TransferTime:  j.TransferTime,
		StationChange: j.StationChange,
		Legs:          make([]ItineraryLeg, 0, len(raw)),
	}
	for i := range raw {
		var seg journeyLeg
		if i < len(j.legs) {
			seg = j.legs[i]
		}
		it.Legs = append(it.Legs, raw[i].leg(seg))
	}
	return it, nil
}

// recorridoLeg mirrors the slice of the getRecorrido payload we consume.
type recorridoLeg struct {
	Origen       string `json:"origen"`
	Destino      string `json:"destino"`
	HoraSalida   string `json:"horaSalida"`
	HoraLlegada  string `json:"horaLlegada"`
	NumTrem      string `json:"numTrem"`
	Prestaciones []struct {
		Descripcion string `json:"descripcion"`
	} `json:"prestaciones"`
	// The stop list is the same bean name the search uses for its journey list;
	// here each entry is a calling point, not a journey.
	Stops []recorridoStop `json:"trainTravelEnlaceViewBean"`
}

// recorridoStop is one calling point of a train's run.
type recorridoStop struct {
	Code    string `json:"cdgoEstacion"`
	Name    string `json:"descEstacion"`
	Llegada string `json:"llegada"`
	Salida  string `json:"salida"`
}

// leg projects one train's reply onto the part of it the ticket covers.
//
// The bean answers with the train's WHOLE run, which is not what the traveller
// bought: a Madrid–Barcelona seat on a train continuing to Figueres came back
// with Girona and Figueres among the stops and Figueres marked as the arrival,
// which reads as a journey ending at 12:24 in another city. So the endpoints
// come from the segment the search reported, and the calling points are clipped
// to what lies between them — falling back to the train's own values when there
// is no segment to clip against.
func (r *recorridoLeg) leg(seg journeyLeg) ItineraryLeg {
	it := ItineraryLeg{
		Train:     TrainNumber(r.NumTrem),
		TrainType: seg.TipoTren,
		From:      firstNonEmpty(seg.OrigenName, r.Origen),
		To:        firstNonEmpty(seg.DestinoName, r.Destino),
		Departure: hhmm(firstNonEmpty(seg.HoraSalida, r.HoraSalida)),
		Arrival:   hhmm(firstNonEmpty(seg.HoraLlegada, r.HoraLlegada)),
	}
	run := clipRun(r.Stops, seg)
	it.Stops = make([]Stop, 0, len(run))
	for _, s := range run {
		it.Stops = append(it.Stops, Stop{
			Code: s.Code, Name: s.Name,
			Arrival: hhmm(s.Llegada), Departure: hhmm(s.Salida),
		})
	}
	for _, p := range r.Prestaciones {
		if p.Descripcion != "" {
			it.Services = append(it.Services, p.Descripcion)
		}
	}
	return it
}

// clipRun cuts a train's calling points down to those between the segment's
// own endpoints. A code that is not in the list leaves that end alone: on a
// train that starts or terminates where the ticket does — the ordinary case —
// the endpoint is not a calling point at all, and cutting on a failed match
// would empty the list.
func clipRun(stops []recorridoStop, seg journeyLeg) []recorridoStop {
	if i := indexOfStop(stops, seg.OrigenCode); i >= 0 {
		stops = stops[i+1:]
	}
	if i := indexOfStop(stops, seg.DestinoCode); i >= 0 {
		stops = stops[:i]
	}
	return stops
}

func indexOfStop(stops []recorridoStop, code string) int {
	if code == "" {
		return -1
	}
	for i, s := range stops {
		if s.Code == code {
			return i
		}
	}
	return -1
}

// recorridoParams renders the journey as the object getRecorrido expects. The
// bean does not take a train number: it takes the journey back, leg by leg,
// exactly as the search handed it over — which is why Journey keeps the raw
// leg fields (codTren with its leading zeros, fechaCI, codGrupo) that nothing
// else uses.
func recorridoParams(j Journey) ([]DWRParam, error) {
	legs := make([][]DWRParam, 0, len(j.legs))
	for _, l := range j.legs {
		if l.CdgoTren == "" {
			return nil, fmt.Errorf("leg is missing its train code")
		}
		legs = append(legs, []DWRParam{
			dwrStr("codTren", l.CdgoTren),
			dwrStr("fechaCI", l.FechaCI),
			dwrStr("codGrupo", l.CodGrupo),
			dwrStr("tipoTren", l.TipoTren),
			dwrStr("cdgoEstacionOrigen", l.OrigenCode),
			dwrStr("cdgoEstacionDestino", l.DestinoCode),
			dwrStr("descEstacionOrigen", l.OrigenName),
			dwrStr("descEstacionDestino", l.DestinoName),
			dwrStr("horaSalida", hhmm(l.HoraSalida)),
			dwrStr("horaLlegada", hhmm(l.HoraLlegada)),
		})
	}
	return []DWRParam{
		dwrBool("directo", j.Direct),
		dwrStr("fecha", j.Date),
		// Non-empty on a connection; the bean echoes it back and rejects a
		// mismatch, so it must be the string the search reported.
		dwrStr("duracionTrasbordo", j.TransferTime),
		dwrObjects("trenes", legs),
		dwrStr("descripcionEstacionOrigen", j.From),
		dwrStr("descripcionEstacionDestino", j.To),
		dwrStr("codigoEstacionOrigen", j.FromCode),
		dwrStr("codigoEstacionDestino", j.ToCode),
		dwrStr("horaSalida", j.Departure),
		dwrStr("horaLlegada", j.Arrival),
	}, nil
}

// hhmm trims the seconds Renfe puts on a leg's times ("07:05:00" → "07:05").
// The request is rejected — silently, with an empty result — if they are left on.
func hhmm(t string) string {
	if len(t) == 8 && t[2] == ':' && t[5] == ':' {
		return t[:5]
	}
	return t
}
