package client

import (
	"fmt"
	"strings"
)

// accountPage is the signed-in landing page the account beans are called from.
const accountPage = "/vol/homeCustomers.do"

// Whoami returns the display name of the signed-in traveller, or "" when the
// session is anonymous or has expired. It is the cheapest way to check whether
// an imported browser session is still being accepted.
func (c *Client) Whoami() (string, error) {
	var name string
	if err := c.callDWR("sesionManager", "getIdentificacionUsuario", accountPage, nil, &name); err != nil {
		return "", err
	}
	return strings.TrimSpace(name), nil
}

// Card is the traveller's Renfe loyalty card (+Renfe).
type Card struct {
	Name   string `json:"name"`
	Number string `json:"number"`
	Level  string `json:"level"`
	Points string `json:"points"`
	Status string `json:"status,omitempty"`
}

// Card returns the signed-in traveller's loyalty card. It needs a session.
func (c *Client) Card() (*Card, error) {
	var raw struct {
		CardName   string `json:"cardName"`
		CardNumber string `json:"cardNumber"`
		Level      string `json:"level"`
		Points     string `json:"points"`
		Status     string `json:"status"`
	}
	if err := c.callDWR("homeCustomersManager", "getUserRenfeCard", accountPage, nil, &raw); err != nil {
		return nil, err
	}
	return &Card{
		Name:   raw.CardName,
		Number: raw.CardNumber,
		Level:  cardLevel(raw.Level),
		Points: raw.Points,
		Status: raw.Status,
	}, nil
}

// cardLevel names the +Renfe tiers the API only numbers. An unknown code is
// passed through rather than guessed at, so a new tier still prints something.
func cardLevel(code string) string {
	switch code {
	case "1":
		return "Básica"
	case "2":
		return "Clásica"
	case "3":
		return "Oro"
	case "4":
		return "Plata"
	default:
		return code
	}
}

// Trip is one upcoming journey on the traveller's account.
type Trip struct {
	Locator   string `json:"locator"`
	Date      string `json:"date"`
	Departure string `json:"departure"`
	Arrival   string `json:"arrival"`
	From      string `json:"from"`
	To        string `json:"to"`
	Train     string `json:"train,omitempty"`
	TrainType string `json:"train_type,omitempty"`
}

// Trips returns the traveller's upcoming journeys. An account with nothing
// booked yields an empty slice, not an error.
//
// An expired login also yields an empty array from the bean — silently, with no
// error — so "nothing booked" and "we could not look" are indistinguishable in
// the response. Since reporting the second as the first would tell a traveller
// they have no ticket when they may well have one, an empty result is confirmed
// against the session before it is believed.
func (c *Client) Trips() ([]Trip, error) {
	// The bean's field names are not documented and an empty account returns an
	// empty array, so decode the spellings the booking flow uses elsewhere and
	// let the rest fall away.
	var raw []struct {
		Localizador     string `json:"localizador"`
		Fecha           string `json:"fecha"`
		FechaIda        string `json:"fechaIda"`
		HoraSalida      string `json:"horaSalida"`
		HoraLlegada     string `json:"horaLlegada"`
		EstacionOrigen  string `json:"descripcionEstacionOrigen"`
		EstacionDestino string `json:"descripcionEstacionDestino"`
		Origen          string `json:"origen"`
		Destino         string `json:"destino"`
		CdgoTren        string `json:"cdgoTren"`
		TipoTren        string `json:"tipoTren"`
	}
	if err := c.callDWR("homeCustomersManager", "getMyJourneys", accountPage, nil, &raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		name, err := c.Whoami()
		if err != nil {
			return nil, fmt.Errorf("could not confirm the session before reporting an empty trip list: %w", err)
		}
		if name == "" {
			return nil, ErrSessionExpired
		}
	}
	out := make([]Trip, 0, len(raw))
	for _, r := range raw {
		out = append(out, Trip{
			Locator:   r.Localizador,
			Date:      firstNonEmpty(r.Fecha, r.FechaIda),
			Departure: r.HoraSalida,
			Arrival:   r.HoraLlegada,
			From:      firstNonEmpty(r.EstacionOrigen, r.Origen),
			To:        firstNonEmpty(r.EstacionDestino, r.Destino),
			Train:     TrainNumber(r.CdgoTren),
			TrainType: r.TipoTren,
		})
	}
	return out, nil
}
