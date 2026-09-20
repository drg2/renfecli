package client

import (
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
)

// searchPage is the results page DWR calls claim to originate from. The server
// keys the search context by HTTP session, but DWR echoes the page back and
// rejects a call whose page does not look like one of its own.
const searchPage = "/vol/buscarTrenEnlaces.do"

// Query is one journey search: a route, a date, and who is travelling.
type Query struct {
	Origin      string // station code, e.g. "60000"
	Destination string // station code, e.g. "71801"
	OriginName  string // display name; the form echoes it back in the UI
	DestName    string
	Date        time.Time // departure date (day precision)
	Return      time.Time // optional return date; zero for one-way
	Adults      int
	Children    int  // 4-13
	Infants     int  // under 4, no seat
	Pet         bool // travelling with a pet
	Bike        bool // travelling with a bicycle
	WheelchairH bool // request an H (wheelchair) space
	Direct      bool // exclude journeys with a connection
}

// Journey is one bookable train (or connection) on the searched route.
type Journey struct {
	Departure string   `json:"departure"` // "06:27"
	Arrival   string   `json:"arrival"`   // "09:50"
	Date      string   `json:"date"`      // "2026-09-22"
	Duration  string   `json:"duration"`  // "3 horas 23 minutos"
	Minutes   int      `json:"minutes"`   // duration in minutes, for sorting
	TrainType string   `json:"train_type"`
	Trains    []string `json:"trains"` // train numbers, one per leg
	Direct    bool     `json:"direct"`
	From      string   `json:"from"`
	To        string   `json:"to"`
	FromCode  string   `json:"from_code"`
	ToCode    string   `json:"to_code"`
	// Price is the cheapest available fare PER PASSENGER (Renfe quotes per
	// traveller, not per booking), 0 when nothing is on sale.
	Price     float64 `json:"price"`
	Available bool    `json:"available"` // on sale to the traveller who searched
	SoldOut   bool    `json:"sold_out"`
	// WheelchairOnly means the only seats left are wheelchair (H) spaces. The
	// train still quotes a fare, so without this it reads as ordinary
	// availability to someone who cannot buy it.
	WheelchairOnly bool   `json:"wheelchair_only,omitempty"`
	Fares          []Fare `json:"fares"`
	Cheapest       bool   `json:"cheapest"` // Renfe's own "best price" marker
	Fastest        bool   `json:"fastest"`
	CO2Saved       string `json:"co2_saved,omitempty"`
	// TransferTime is how long the connection sits between legs ("1 horas 40
	// minutos"); empty on a direct train.
	TransferTime string `json:"transfer_time,omitempty"`
	// StationChange is Renfe's warning when the connection means moving between
	// two different stations — crossing Madrid from Atocha to Chamartín, say.
	// An hour of "transfer time" means something very different then, so it is
	// surfaced rather than left in the payload.
	StationChange string `json:"station_change,omitempty"`

	// legs carries the raw per-leg fields Stops has to hand back to Renfe. They
	// are meaningless outside that call (fechaCI is the train's timetable epoch,
	// not a date anyone travels on), so they stay unexported and out of --json.
	legs []journeyLeg
}

// TrainNumber renders Renfe's zero-padded train code ("03311") as the number
// printed on the departure board ("3311"). Journey.Trains and Trip.Train are
// normalised with it, so anything matched against them — a number typed on a
// command line, say — has to go through it too.
func TrainNumber(code string) string {
	return strings.TrimLeft(strings.TrimSpace(code), "0")
}

// journeyLeg is one leg of a journey as the search reported it, kept verbatim —
// getRecorrido matches on these exact values, down to the leading zeros on the
// train code.
type journeyLeg struct {
	CdgoTren    string
	FechaCI     string
	CodGrupo    string
	TipoTren    string
	OrigenCode  string
	DestinoCode string
	OrigenName  string
	DestinoName string
	HoraSalida  string
	HoraLlegada string
}

// Fare is one purchasable fare bucket on a journey (Básico, Elige, Prémium…).
// Price is per passenger.
type Fare struct {
	Code  string  `json:"code"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
	Class string  `json:"class"` // "T" tourist, "P" preferente
}

// PriceDay is one entry of the cheapest-price-per-day strip Renfe returns
// alongside the results — the data behind the site's price calendar.
type PriceDay struct {
	Date      string  `json:"date"`
	MinPrice  float64 `json:"min_price"`
	Available bool    `json:"available"`
}

// Results is one direction of a search: its journeys plus, when Renfe sent one,
// the surrounding price calendar. A round trip carries the second direction in
// Return.
type Results struct {
	From     string     `json:"from"`
	To       string     `json:"to"`
	Date     string     `json:"date"`
	Journeys []Journey  `json:"journeys"`
	Calendar []PriceDay `json:"calendar,omitempty"`
	Return   *Results   `json:"return,omitempty"`
}

// Seated is how many of the party a fare is quoted for. Renfe prices per
// passenger, so a caller showing a party total multiplies by this — and by
// this rather than by the headcount, because an infant under 4 travels on a
// lap: the form carries them in their own field (ninosMenores, apart from
// ninos) precisely because they take no seat, and the quote does not move when
// one is added. Whether Renfe asks anything at all for them is not visible in
// the reply, so they are named separately instead of being multiplied in.
func (q Query) Seated() int {
	n := q.Adults + q.Children
	if n <= 0 {
		return 1
	}
	return n
}

// dwrDateFormat is the dd/MM/yyyy the booking form and the DWR beans both use.
const dwrDateFormat = "02/01/2006"

// Search runs a journey search: it posts the booking form (which stores the
// route in the HTTP session) and then asks the train-list bean for the results.
// The two requests share one session, so they cannot be reordered or run
// concurrently for different routes on the same Client.
//
// A round trip needs no second call: when the form carried a return date, the
// one reply carries both directions, and the return leg lands in Results.Return.
func (c *Client) Search(q Query) (*Results, error) {
	if q.Origin == "" || q.Destination == "" {
		return nil, fmt.Errorf("origin and destination station codes are required")
	}
	// Renfe answers an A→A search with the same empty list it uses for an
	// unsellable date, so without this the user is told to check the calendar
	// for a journey that cannot exist.
	if q.Origin == q.Destination {
		return nil, fmt.Errorf("origin and destination are the same station (%s)",
			firstNonEmpty(q.OriginName, q.Origin))
	}
	if q.Adults <= 0 {
		q.Adults = 1
	}
	if err := c.submitSearchForm(q); err != nil {
		return nil, err
	}
	var reply trainsListReply
	if err := c.callDWR("trainEnlacesManager", "getTrainsList", searchPage, q.dwrParams(), &reply); err != nil {
		return nil, err
	}
	return reply.results(q)
}

// submitSearchForm posts buscarTren.do. Its HTML response is discarded: the
// train list on that page is rendered client-side from the DWR call that
// follows. What matters is the server-side search context it leaves behind.
func (c *Client) submitSearchForm(q Query) error {
	date := q.Date.Format(dwrDateFormat)
	ret := ""
	if !q.Return.IsZero() {
		ret = q.Return.Format(dwrDateFormat)
	}
	form := url.Values{
		"tipoBusqueda":       {"autocomplete"},
		"currenLocation":     {"menuBusqueda"},
		"vengoderenfecom":    {"SI"},
		"desOrigen":          {q.OriginName},
		"desDestino":         {q.DestName},
		"cdgoOrigen":         {q.Origin},
		"cdgoDestino":        {q.Destination},
		"idiomaBusqueda":     {"ES"},
		"FechaIdaSel":        {date},
		"FechaVueltaSel":     {ret},
		"_fechaIdaVisual":    {date},
		"_fechaVueltaVisual": {ret},
		"adultos_":           {strconv.Itoa(q.Adults)},
		"ninos_":             {strconv.Itoa(q.Children)},
		"ninosMenores":       {strconv.Itoa(q.Infants)},
		"codPromocional":     {""},
		"plazaH":             {boolStr(q.WheelchairH)},
		"sinEnlace":          {boolStr(q.Direct)},
		"conMascota":         {boolStr(q.Pet)},
		"conBicicleta":       {boolStr(q.Bike)},
		"asistencia":         {"false"},
		"franjaHoraI":        {""},
		"franjaHoraV":        {""},
		"Idioma":             {"es"},
		"Pais":               {"ES"},
	}
	_, err := c.postForm("/vol/buscarTren.do", form)
	return err
}

// dwrParams renders the query as the ordered key/value pairs the getTrainsList
// bean expects. The order mirrors the web app's own call so the request body is
// indistinguishable from the browser's.
func (q Query) dwrParams() []DWRParam {
	ret := ""
	if !q.Return.IsZero() {
		ret = q.Return.Format(dwrDateFormat)
	}
	idaVuelta := ""
	if ret != "" {
		idaVuelta = "true"
	}
	return []DWRParam{
		dwrStr("atendo", "false"),
		dwrStr("sinEnlace", boolStr(q.Direct)),
		dwrStr("plazaH", boolStr(q.WheelchairH)),
		dwrStr("tipoFranjaI", ""),
		dwrStr("tipoFranjaV", ""),
		dwrStr("horaFranjaIda", ""),
		dwrStr("horaFranjaVuelta", ""),
		dwrStr("fechaSalida", q.Date.Format(dwrDateFormat)),
		dwrStr("fechaVuelta", ret),
		dwrStr("adultos", strconv.Itoa(q.Adults)),
		dwrStr("ninos", strconv.Itoa(q.Children)),
		dwrStr("ninosMenores", strconv.Itoa(q.Infants)),
		dwrStr("trayecto", "I"),
		dwrStr("idaVuelta", idaVuelta),
		dwrStr("conMascota", boolStr(q.Pet)),
		dwrStr("conBicicleta", boolStr(q.Bike)),
	}
}

// trainsListReply mirrors the slice of the getTrainsList payload we consume;
// the bean returns far more (ATENDO assistance slots, loyalty banners, UI
// copy) and unlisted fields are dropped by the JSON decoder.
type trainsListReply struct {
	ListadoTrenes []struct {
		DescripcionEstacionOrigen string `json:"descripcionEstacionOrigen"`
		DescripcionEstacionDest   string `json:"descripcionEstacionDestino"`
		FechaOrigen               string `json:"fechaOrigen"`
		HayError                  bool   `json:"hayError"`
		CdgoError                 string `json:"cdgoError"`
		MensajeListaTrenVacia     string `json:"mensajeListaTrenVacia"`
		ViajeIda                  bool   `json:"viajeIda"`
		Trains                    []struct {
			HoraSalida       string `json:"horaSalida"`
			HoraLlegada      string `json:"horaLlegada"`
			Fecha            string `json:"fecha"`
			DuracionViaje    string `json:"duracionViaje"`
			DuracionTrasbord string `json:"duracionTrasbordo"`
			DuracionMinutos  int    `json:"duracionViajeTotalEnMinutos"`
			TipoTrenUno      string `json:"tipoTrenUno"`
			TipoTrenDos      string `json:"tipoTrenDos"`
			Directo          bool   `json:"directo"`
			Completo         bool   `json:"completo"`
			SoloPlazaH       bool   `json:"soloPlazaH"`
			Cheapest         bool   `json:"cheapest"`
			Fastest          bool   `json:"fastest"`
			AhorroCo2        string `json:"ahorroCo2"`
			TarifaMinima     string `json:"tarifaMinima"`
			EstacionOrigen   string `json:"descripcionEstacionOrigen"`
			EstacionDestino  string `json:"descripcionEstacionDestino"`
			CdgoOrigen       string `json:"codigoEstacionOrigen"`
			CdgoDestino      string `json:"codigoEstacionDestino"`
			MsgCambioEstac   string `json:"msgCambioEstacionTransbordo"`
			Trayectos        []struct {
				CdgoTren    string `json:"cdgoTren"`
				TipoTren    string `json:"tipoTren"`
				FechaCI     string `json:"fechaCI"`
				CodGrupo    string `json:"codGrupo"`
				OrigenCode  string `json:"cdgoEstacionOrigenTrayecto"`
				DestinoCode string `json:"cdgoEstacionDestinoTrayecto"`
				OrigenName  string `json:"descripcionEstacionOrigen"`
				DestinoName string `json:"descripcionEstacionDestino"`
				HoraSalida  string `json:"horaSalida"`
				HoraLlegada string `json:"horaLlegada"`
			} `json:"trayectos"`
			Tarifas []struct {
				CodigoTarifa string `json:"codigoTarifa"`
				Titulo       string `json:"titulo"`
				PrecioTarifa string `json:"precioTarifa"`
				CdgoClase    string `json:"cdgoClase"`
			} `json:"tarifasDisponibles"`
		} `json:"listviajeViewEnlaceBean"`
		PriceCalendar struct {
			Days []struct {
				Date      string  `json:"date"`
				MinPrice  float64 `json:"minPrice"`
				Available bool    `json:"minPriceAvailable"`
			} `json:"journeysPriceCalendar"`
		} `json:"resultPriceCalendar"`
	} `json:"listadoTrenes"`
}

// results projects the reply onto the public types. Renfe wraps the outbound
// and (for a round trip) return legs in one list keyed by viajeIda, so the
// first non-outbound leg becomes Results.Return.
func (r *trainsListReply) results(q Query) (*Results, error) {
	var out, ret *Results
	for i := range r.ListadoTrenes {
		leg, err := r.leg(i, q)
		if err != nil {
			return nil, err
		}
		switch {
		case out == nil && r.ListadoTrenes[i].ViajeIda:
			out = leg
		case ret == nil && !r.ListadoTrenes[i].ViajeIda:
			ret = leg
		}
	}
	if out == nil {
		// An empty listadoTrenes is all Renfe ever says, whatever the cause, so
		// the message has to do the diagnosis. A restrictive filter is the one
		// cause the caller can act on immediately, and it is easy to forget that
		// --bike or --pet is still set, so name it first.
		return nil, fmt.Errorf("no trains for %s → %s on %s%s",
			firstNonEmpty(q.OriginName, q.Origin), firstNonEmpty(q.DestName, q.Destination),
			q.Date.Format("2006-01-02"), q.emptyResultHint())
	}
	// Renfe returns an empty return leg even for a one-way search, so only a
	// caller who asked for one gets it — but then they get it whether or not it
	// has trains, since "your return day is empty" is the answer to the question
	// they asked, and dropping the leg silently answers a different one.
	if ret != nil && (!q.Return.IsZero() || len(ret.Journeys) > 0) {
		out.Return = ret
	}
	return out, nil
}

// emptyResultHint explains a result Renfe returned empty. Active filters are
// listed first because they are the one cause the caller can lift; otherwise
// the date is the usual suspect — Renfe sells a limited window ahead (measured
// at ~3 months on the Madrid–Barcelona corridor in September 2026, but it moves
// with the timetable, so no figure is quoted as if it were a rule).
func (q Query) emptyResultHint() string {
	var filters []string
	for _, f := range []struct {
		on   bool
		name string
	}{
		{q.Pet, "--pet"},
		{q.Bike, "--bike"},
		{q.WheelchairH, "--wheelchair"},
		{q.Direct, "--direct"},
	} {
		if f.on {
			filters = append(filters, f.name)
		}
	}
	if len(filters) > 0 {
		those := "that filter"
		if len(filters) > 1 {
			those = "those filters"
		}
		return fmt.Sprintf(" with %s — try dropping %s, or the date may be outside Renfe's sale window",
			strings.Join(filters, " "), those)
	}
	return " — the date may be outside Renfe's sale window (it sells a limited number of months ahead), or the route may not be served"
}

// cheapestFare is the lowest price actually on sale in a fare list; 0 when the
// list is empty or nothing in it is priced.
func cheapestFare(fares []Fare) float64 {
	best := 0.0
	for _, f := range fares {
		if f.Price > 0 && (best == 0 || f.Price < best) {
			best = f.Price
		}
	}
	return best
}

// leg projects one direction of the reply onto the public types.
func (r *trainsListReply) leg(i int, q Query) (*Results, error) {
	leg := r.ListadoTrenes[i]
	if leg.HayError {
		msg := leg.MensajeListaTrenVacia
		if msg == "" {
			msg = "error " + leg.CdgoError
		}
		return nil, fmt.Errorf("renfe: %s", msg)
	}
	out := &Results{
		From: leg.DescripcionEstacionOrigen,
		To:   leg.DescripcionEstacionDest,
		Date: leg.FechaOrigen,
		// Allocated empty, never left nil: these go out as --json, where a nil
		// slice marshals to null and breaks a caller that iterates it. A day
		// with no trains is [], not null.
		Journeys: []Journey{},
	}
	for _, t := range leg.Trains {
		j := Journey{
			Departure: t.HoraSalida,
			Arrival:   t.HoraLlegada,
			Date:      t.Fecha,
			Duration:  t.DuracionViaje,
			Minutes:   t.DuracionMinutos,
			TrainType: firstNonEmpty(t.TipoTrenUno, t.TipoTrenDos),
			Direct:    t.Directo,
			From:      t.EstacionOrigen,
			To:        t.EstacionDestino,
			FromCode:  t.CdgoOrigen,
			ToCode:    t.CdgoDestino,
			Cheapest:  t.Cheapest,
			Fastest:   t.Fastest,
			SoldOut:   t.Completo,
		}
		if !t.Directo {
			j.TransferTime = t.DuracionTrasbord
			j.StationChange = plainText(t.MsgCambioEstac)
		}
		if t.AhorroCo2 != "" && t.AhorroCo2 != "0" {
			j.CO2Saved = t.AhorroCo2
		}
		j.Trains = make([]string, 0, len(t.Trayectos))
		for _, tramo := range t.Trayectos {
			j.Trains = append(j.Trains, TrainNumber(tramo.CdgoTren))
			j.legs = append(j.legs, journeyLeg{
				CdgoTren: tramo.CdgoTren, FechaCI: tramo.FechaCI, CodGrupo: tramo.CodGrupo,
				TipoTren:   tramo.TipoTren,
				OrigenCode: tramo.OrigenCode, DestinoCode: tramo.DestinoCode,
				OrigenName: tramo.OrigenName, DestinoName: tramo.DestinoName,
				HoraSalida: tramo.HoraSalida, HoraLlegada: tramo.HoraLlegada,
			})
		}
		// A sold-out train quotes no fare buckets at all — the common way this
		// slice would otherwise reach --json as null.
		j.Fares = make([]Fare, 0, len(t.Tarifas))
		for _, f := range t.Tarifas {
			j.Fares = append(j.Fares, Fare{
				Code:  f.CodigoTarifa,
				Name:  f.Titulo,
				Price: dwrPrice(f.PrecioTarifa),
				Class: f.CdgoClase,
			})
		}
		// tarifaMinima is Renfe's own cheapest-fare figure. When it is missing,
		// the fare list still has the answer — but in Renfe's order, which is
		// not price order, so the first entry is whichever bucket it felt like
		// listing first (Prémium, on a train with no Básico left).
		j.Price = dwrPrice(t.TarifaMinima)
		if j.Price == 0 {
			j.Price = cheapestFare(j.Fares)
		}
		// A journey is on sale when it quotes a fare and is not flagged full.
		// The bean also sends "operativo" and "razonNoDisponible"; neither is
		// decoded, because the first is a seat-map flag that is false on plenty
		// of bookable trains and the second is an opaque code ("3", "8") that
		// rides along on available trains too.
		j.WheelchairOnly = t.SoloPlazaH
		j.Available = !t.Completo && j.Price > 0
		// A train down to its last wheelchair spaces still quotes a fare, but
		// only someone who asked for an H space can actually buy it.
		if j.WheelchairOnly && !q.WheelchairH {
			j.Available = false
		}
		out.Journeys = append(out.Journeys, j)
	}
	for _, d := range leg.PriceCalendar.Days {
		out.Calendar = append(out.Calendar, PriceDay{Date: d.Date, MinPrice: d.MinPrice, Available: d.Available})
	}
	sort.SliceStable(out.Journeys, func(a, b int) bool {
		return out.Journeys[a].Departure < out.Journeys[b].Departure
	})
	return out, nil
}

// plainText flattens the small HTML Renfe puts in its notices into one line.
func plainText(s string) string {
	s = strings.ReplaceAll(s, "<br>", " ")
	s = strings.ReplaceAll(s, "<br/>", " ")
	s = strings.ReplaceAll(s, "<br />", " ")
	return strings.Join(strings.Fields(s), " ")
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
