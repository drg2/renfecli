# `renfe` CLI reference

Every command, its flags, and the `--json` shapes. Data → stdout, logs/errors → stderr. Exit codes:
`0` ok, `1` runtime error, `2` usage / unknown command. Common flags (anywhere after the
subcommand, including after the positionals): `--json`, `--toon` (TOON — same shapes as `--json`,
fewer tokens; prefer it when an agent parses the output). There is no `--lang`: Renfe's search bean
replies in Spanish whatever language the form asked for, so the flag was removed rather than left
lying about what it did.

## Read commands (anonymous, no login)

| Command | What it does |
|---|---|
| `renfe search <origin> <destination>` | Journeys with live fares for a date. See flags below. |
| `renfe calendar <origin> <destination>` | Cheapest fare per day around a date — the strip Renfe ships with every search, so it costs no extra request. `--cheapest` prints only the best day. **Main corridors only**: Renfe leaves the strip empty on secondary routes and the command then fails with `renfe returned no price calendar for …`. |
| `renfe stops <origin> <destination>` | A train's intermediate calls and onboard services. `--train N` or `--at HH:MM` picks the journey (default: the first); both are normalised, so `--at 8:27` matches the 08:27 train and `--train 03311` matches 3311. Takes the `search` date/passenger flags. Runs a search first — the bean matches against the session's journey list, so a bare train number is not enough. **The itinerary is the ticket's, not the train's**: on a train that carries on past your destination (Madrid–Barcelona on a service continuing to Figueres) the calling points stop at your station and the arrival is yours, not the train's terminus. |
| `renfe stations [query]` | Exits non-zero when a query matches nothing (as grep does). Station names and codes. `--limit N` (default 20, `0` = all), `--update` re-downloads the catalogue, ignoring the 7-day cache. No query lists everything. |

### `search` flags

| Flag | Effect |
|---|---|
| `--date <d>` | Departure date. `YYYY-MM-DD`, `DD/MM/YYYY`, `today`, `tomorrow`, a weekday name (the **next** one), or `+N` days. Default today. A date already gone by is **refused** before any request — Renfe answers one with the same empty list it uses for a date not yet on sale, so it used to come back as "wait for the sale window". |
| `--return <d>` | Round trip: prints the return journeys too **and** prices the outbound under round-trip rules. One request, not two. The two lists are independent, but when both dates match, `search` prints how many of the listed returns actually leave after the earliest arrival (or that none do) — counting a train that lands after midnight as arriving the next day. A return **before** the outbound is refused: Renfe accepts it and answers with two unrelated day-lists. A return day with no trains is reported as an empty leg, not dropped, so asking for two directions never silently returns one. |
| `--adults N` | Adult passengers (default 1, or `[defaults] adults`). All prices are quoted **per passenger**, never as a party total. |
| `--children N` | Children aged 4–13; they occupy a seat and count towards the per-person multiplication. `--infants N` for under-4s, who travel on a lap: Renfe carries them in their own form field, the quoted fare does not move when one is added, and `search` keeps them **out** of the "multiply by N" line while saying so. Negative counts are refused rather than searched as one passenger. |
| `--after HH:MM` / `--before HH:MM` | Bound the departure time, inclusive. Filtered locally over the results already fetched (Renfe's own site does this client-side too), so they add no request. `--after` later than `--before` is rejected rather than silently returning nothing. On a round trip they bound the **outbound**; `--return-after` / `--return-before` bound the return, and default to the outbound window when omitted. |
| `--direct` | Exclude journeys with a connection. Verified on Madrid→Huelva, where it drops the 2-leg AVE. |
| `--pet` / `--bike` | Travelling with a pet / a bicycle. **Strongly restrictive** — on Madrid–Barcelona `--pet` cuts 23 trains to 4 and `--bike` to none. An empty result is usually this, not the date. |
| `--wheelchair` | Request a wheelchair (H) space. A real search parameter, unlike ATENDO assistance, which the search ignores entirely. |
| `--cheapest` | Sort by price instead of departure time. Sold-out trains sort last, never first. |
| `--available` | Hide sold-out and unpriced journeys. |
| `--fares` | List every fare bucket per train, not just the cheapest. |
| `--limit N` | Cap the journeys shown. Applied **after** filtering and sorting, so it always counts what the user sees. `0` means no cap; a negative value is refused rather than read as "no cap". |

`--date` and `--return` are also accepted by `calendar`, along with the passenger flags — the strip
is priced for the party you describe.

## Account commands (bring-your-own browser session)

| Command | What it does |
|---|---|
| `renfe login [--from-browser <b>]` | Lift a signed-in session from a browser's cookie store. **Omit the flag** to search every installed browser (the usual form); `--from-browser` requires a value when given: `chrome`, `chromium`, `firefox`, `safari`, `edge`, `brave`. Every browser holding what looks like a session is tried against the server in turn, so a stale copy in one browser does not hide a live login in another. Reads the decrypted store, so HttpOnly cookies come along. Stored `0600` at `~/.renfe/session.json` and verified against the server before it reports success. |
| `renfe login --stdin` | Read a raw `Cookie` header from stdin instead, for browsers the cookie reader cannot open. |
| `renfe whoami` | Confirm the session and show the +Renfe card. Fails with an actionable message when the session is stale. |
| `renfe session [status]` | Who is signed in and how long the session lasts. Reports the servlet session and the login separately — the first can be healthy while the second has lapsed. |
| `renfe session keep` | Holds the session open (Renfe drops an idle one after ~30 min). Adaptive by default; `--every D` forces a fixed interval, `--for D` stops after a while, `--quiet` reports only problems. Stops with an error when the login drops, since no amount of refreshing brings it back. With `--json` it streams **one compact object per refresh, one per line** (JSON Lines — read it a line at a time), in the `session status` shape; no `stopped`/`done` chatter reaches that stream. A refresh whose login lookup fails is reported on stderr and retried, not emitted. |
| `renfe trips` | Upcoming journeys on the account. An account with nothing booked prints `no upcoming trips` and exits `0`; an expired login is reported as such, never as an empty list. |

There is **no** command that books, pays, changes or cancels anything.

## `--json` shapes

### `search`

```json
{
  "from": "MADRID-PUERTA DE ATOCHA-ALMUDENA GRANDES",
  "to": "BARCELONA-SANTS",
  "date": "22/09/2026",
  "journeys": [ /* Journey */ ],
  "calendar": [ /* PriceDay */ ],
  "return": { /* the same object again, for the return leg; absent on a one-way */ }
}
```

`Journey`:

```json
{
  "departure": "06:16",
  "arrival": "09:24",
  "date": "2026-09-22",
  "duration": "3 horas 8 minutos",   // Renfe's localised sentence
  "minutes": 188,                    // the same duration, for sorting/maths
  "train_type": "AVE",               // AVE | AVLO | AVANT | ALVIA | MD | …
  "trains": ["3301"],                // one entry per leg; >1 means a connection
  "direct": true,
  "from": "MADRID-PUERTA DE ATOCHA-ALMUDENA GRANDES",
  "to": "BARCELONA-SANTS",
  "from_code": "60000",
  "to_code": "71801",
  "price": 124.5,                    // cheapest available fare PER PASSENGER; 0 when nothing is on sale
  "available": true,                 // on sale TO THIS TRAVELLER: not full, quoting a price,
                                     //   and not down to wheelchair spaces only
  "wheelchair_only": false,          // present when the last seats are H spaces
  "sold_out": false,
  "fares": [ { "code": "N1010", "name": "Básico", "price": 124.5, "class": "T" } ],
                                     //   ALWAYS an array — `[]` on a sold-out train, never null.
                                     //   The same holds for "trains", "journeys", "legs", "stops".
  "transfer_time": "1 horas 56 minutos",  // connections only
  "station_change": "ATENCIÓN: Necesario cambio de estación…",  // when the legs use different stations
  "cheapest": false,                 // Renfe's own markers, not ours
  "fastest": true,
  "co2_saved": "…"                   // present only when Renfe sends it
}
```

Prices are **base fares**: the search carries no discount field, so Tarjeta Dorada / Carné Joven /
Familia Numerosa reductions are not reflected. `class` is `T` (turista) or `P` (preferente). **`price` is the cheapest bucket, which is not always
`fares[0]`** — on some trains a preferente bucket undercuts turista. Read `price` for "from X €";
read `fares` when the bucket matters.

### `calendar`

Absent on secondary routes (the command errors). An array of `PriceDay` (a single-element array with `--cheapest`):

```json
[ { "date": "2026-09-20", "min_price": 147, "available": true } ]
```

`min_price` is a number (not the comma-decimal string `fares` use) and arrives as whole euros. A day
with `"available": false` is not bookable whatever its price — always check the flag before quoting one.

### `stops`

```json
{
  "from": "MADRID-PUERTA DE ATOCHA-ALMUDENA GRANDES",
  "to": "HUELVA",
  "departure": "13:00",
  "arrival": "19:14",
  "transfer_time": "1 horas 40 minutos",
  "legs": [
    {
      "train": "2130",
      "train_type": "AVE",
      "from": "Madrid-puerta De Atocha-almudena Grandes",
      "to": "SEVILLA-SANTA JUSTA",
      "departure": "13:00",
      "arrival": "15:39",
      "stops": [ { "code": "50500", "name": "Córdoba-julio Anguita", "arrival": "14:44", "departure": "14:44" } ],
      "services": ["Cafetería/bar móvil"]
    }
  ]
}
```

`stops` holds only the intermediate calls — the journey's own endpoints are on the leg, not repeated
in the list. `transfer_time` is absent on a direct train.

### `stations`

```json
[ { "code": "BARCE", "name": "BARCELONA (TODAS)", "flat": "BARCELONA (TODAS)", "priority": 3 } ]
```

`code` is what `search` takes. `flat` is the unaccented form used for matching. `priority` is
Renfe's own ordering (city groups and main termini first) — it is why a bare city name resolves to
the group rather than a suburban halt. Codes that are letters (`MADRI`, `BARCE`, `VALEN`) are
city-wide groups; numeric codes are single stations.

### `whoami`

```json
{ "name": "…", "card": { "name": "…", "number": "…", "level": "Plata", "points": "203,00" } }
```

`points` is a comma-decimal string, as Renfe sends it. `card` is absent for an account without one.

### `session status`

```json
{ "name": "…", "signed_in": true, "seconds_remaining": 1740, "remaining": "29m 00s" }
```

`signed_in` is the one to branch on: `seconds_remaining` describes the servlet session, which
outlives the login.

### `login`

```json
{ "name": "MARC RIVERO" }
```

The name Renfe accepted the session as. Nothing is stored unless the server
confirms it, so a successful object means the session works.

### `trips`

An array of `{ "locator", "date", "departure", "arrival", "from", "to", "train", "train_type" }`;
`[]` when nothing is booked.

## `--toon`

Identical field names and shapes, tabular. A `calendar` result:

```
[6]{available,date,min_price}:
  true,2026-09-20,147
  true,2026-09-21,50
```

## Config

`~/.renfe/config.toml` (override the directory with `RENFE_CONFIG_DIR`):

```toml
[defaults]
origin = "Madrid"        # used when <origin> is omitted
destination = "Sevilla"  # used when <destination> is omitted
adults = 2
```

Positionals override the defaults; one positional is read as the **destination**. `RENFE_BASE_URL`
repoints the client at a proxy, mock or replay server for debugging.

The station catalogue is cached at `~/.renfe/stations.json` for 7 days; a failed refresh falls back
to the stale copy with a warning on stderr rather than failing the command.

## Errors worth handling by name

| Message | Means |
|---|---|
| `no station matches "X" — try: renfe stations X` | Bad name. Run the suggested command, don't guess a code. Foreign stations are listed under their local names (Marsella/Perpiñán/Aviñón/Oporto are aliased; others need the local spelling). |
| `origin and destination are the same station (…)` | A→A; Renfe would answer with the same empty list it uses for an unsellable date, so this is caught first. |
| `no trains for A → B on D with --bike …` | A filter excluded everything. Drop it and re-run before concluding anything about the route. |
| `no trains for A → B on D — the date may be outside Renfe's sale window…` | With no filter named: usually not-yet-on-sale. The horizon is shorter than people expect (~83 days on Madrid–Barcelona in Sept 2026) and moves in batches with the timetable. |
| `renfe has put this request in its virtual waiting room (queue-it)…` | Demand spike; everyone is queued. Do not retry in a loop: the wait only gets longer. |
| `renfe refused the request (HTTP 403) — its bot detection did not accept this client…` | The TLS disguise was not accepted. Wait a few minutes; it usually clears. Not a bad route, date or station. |
| `renfe's site is failing (HTTP 5xx)…` | Renfe's own trouble. A throttle is already retried several times before this surfaces, so retrying immediately will not help. |
| `not signed in — run: renfe login --from-browser chrome` | No stored session. |
| `<date> has already gone by — Renfe sells from today (<today>) onwards` | The date is in the past. Not a sale-window problem; re-ask with a real date. |
| `the return date <a> is before the outbound <b>` | The two dates are the wrong way round. Renfe would have answered with two unrelated day-lists. |
| `--adults cannot be negative (got -1)` / `--limit cannot be negative` | A typo in a count. Applies to `--adults`, `--children`, `--infants` and `--limit`. |
| `refusing to send the stored Renfe session to <host>` | `RENFE_BASE_URL` points somewhere that is neither `venta.renfe.com` nor this machine, so the credential is withheld. The anonymous commands still work; `RENFE_ALLOW_SESSION_ON_CUSTOM_HOST=1` opts the host in. |
| `the stored session is no longer signed in` | The user must log in again **in their browser**, then re-run `login`. Retrying the CLI will not help. |
| `the HTTP session is alive (… left) but the login could not be checked: …` | `session status` could not reach the login bean. This is **not** an expired login — do not tell the user to sign in again; retry. |
