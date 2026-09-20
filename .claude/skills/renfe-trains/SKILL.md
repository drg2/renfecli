---
name: renfe-trains
description: >-
  Search Spanish train times and live fares on Renfe (renfe.com / venta.renfe.com) by driving the
  local `renfe` CLI: resolve stations, price a route on a date, find the cheapest day to travel,
  compare fare buckets, and read the user's own +Renfe account and upcoming trips. Use whenever the
  user asks about trains in Spain — times, prices, "when is it cheapest", AVE/AVLO/Avant options, or
  a specific journey. Spanish phrasings: "¿cuánto cuesta el AVE a Barcelona?", "trenes Madrid
  Sevilla el viernes", "¿qué día sale más barato?", "mira precios de tren", "billetes de Renfe",
  "horarios de tren", "¿hay tren directo a Valencia?". Catalan: "trens de Barcelona a Girona",
  "quant costa el tren". English: "find me a train from Madrid to Seville", "cheapest day to take
  the AVE", "Renfe prices". Read-only: it NEVER books, reserves or pays — the user finishes any
  purchase themselves at renfe.com. Do NOT use for other operators (Ouigo, Iryo, Alsa, FlixBus) or
  for non-Spanish rail.
---

# Renfe trains

Turn a travel question into real Renfe times and prices by driving the `renfe` CLI. It replays the
same HTTP endpoints `venta.renfe.com` uses, so the fares are the live ones the website would quote —
this skill is the playbook for using it accurately.

## Three things that are non-negotiable (read first)

1. **This CLI cannot book anything.** There is no `book`, no `buy`, no `pay`. It searches, prices and
   reads the user's own account. Never say you "reserved", "booked" or "bought" a ticket — you
   can't. Give the user the journey and let them finish at `renfe.com` themselves.
2. **Prices are live and move.** Renfe is yield-priced: the same seat costs different amounts hour to
   hour, and a fare quoted now may be gone in minutes. Always say *when* you looked ("as of today's
   search"), and never present a price as a guarantee or re-use one from earlier in the conversation
   without re-running the search.
3. **Never print, store or commit cookie values.** `login` handles the session; `whoami` confirms it.
   If you need to show the user their session state, run `renfe whoami`, never `cat ~/.renfe/session.json`.

## Locate the binary

Run `renfe` from `PATH`. If it isn't there, build it from the repo (Go 1.26.6+):

```bash
go install github.com/seifreed/renfecli/cmd/renfe@latest   # anywhere, needs Go 1.26.6+
make build                                                 # or, in the checkout: ./renfe
renfe version
```

A `v*` tag also publishes prebuilt binaries for Linux, macOS and Windows on the
repo's Releases page, each with a `SHA256SUMS` beside it — prefer those when the
user has no Go toolchain.

Search, prices and station lookup need **no login**.

## Resolve the stations first

Origin and destination are matched against Renfe's own catalogue, so plain names work:

```bash
renfe search madrid barcelona --date friday
```

A bare city name resolves to the **city-wide group** (`MADRID (TODAS)`, code `MADRI`), which searches
every station in that city — usually what the user means, and what renfe.com does. Be aware of what
that implies: Madrid→Barcelona may come back from Atocha *or* Chamartín depending on the train, and
the printed `from`/`to` tell you which. When the user names a specific station, pass it:

```bash
renfe stations valencia          # see the options and their codes
renfe search "valencia-joaquin sorolla" madrid --date +3
```

Reach for `renfe stations <name>` **before** guessing whenever a name is ambiguous (Valencia,
Santiago, Córdoba all have several entries) or when a search returns a route that looks wrong.

## The core workflow

### 1. Price the route on the date the user asked for

```bash
renfe search madrid sevilla --date +30 --adults 2
```

Use the date the user actually asked for. `+N` is used here only because an
example with a fixed date rots twice over: once the day passes the CLI refuses
it, and a date far enough ahead to stay in the future is beyond Renfe's sale
window and returns nothing.

Output is one line per train: type, times, duration, train number, cheapest fare, and Renfe's own
`[cheapest]` / `[fastest]` markers. Add `--available` to drop sold-out and unpriced trains — do this
whenever you are going to recommend something, so you never propose a train that cannot be bought.

A train down to its last wheelchair (H) spaces still quotes a fare, so it would otherwise look
ordinary. It prints `49,80 € — wheelchair spaces only`, counts as unavailable, and becomes available
again under `--wheelchair`. Never quote that price to a traveller who did not ask for an H space.

Useful narrowing, in the order you usually want it:

- `--direct` — no connections. Use when the user says "directo" or the route has slow multi-leg options.
- `--cheapest` — sort by price instead of departure time. Sold-out trains sort last.
- `--after HH:MM` / `--before HH:MM` — bound the outbound departure time (`--return-after` /
  `--return-before` do the return). Use these for "por la tarde", "que salga después de comer",
  "el primero de la mañana". They filter locally over results already fetched, so they cost
  nothing — reach for them instead of dumping 23 trains and eyeballing.
- `--limit N` — cap the list. A full day on a busy corridor is 20+ trains; show the user 5–8, not all of them.
- `--fares` — every fare bucket, not just the cheapest (see below).

### 2. Answer "when is it cheapest?" with `calendar`, not repeated searches

Renfe ships a cheapest-fare-per-day strip with every search, so this is **one request**, not one per day:

```bash
renfe calendar madrid sevilla --date +3
```

Never loop `renfe search` over a range of dates to build this yourself — it is slower, hammers the
site, and the strip is already authoritative. If the user wants a window wider than the strip
returns, say what the strip covers rather than faking the rest.

**The strip only exists on main corridors.** Renfe populates it for the busy routes and leaves it
empty elsewhere, so on a secondary route the command fails outright:

```
error: renfe returned no price calendar for BARCELONA-SANTS → ALBACETE-LOS LLANOS
```

That is a route limitation, not a bug and not a sold-out day. Say so and fall back to pricing the
specific dates the user cares about with `search` — but still do not sweep a whole range.

### 3. Explain the fare buckets when price matters

`--fares` lists every bucket on each train. They are not interchangeable, and the difference is
usually refundability and seat class, not comfort alone:

| Bucket | Class | What the user is buying |
|---|---|---|
| Básico / Básica | turista (`T`) | cheapest, no changes, no refund |
| Elige | turista (`T`) | changeable for a fee, partial refund |
| Elige Confort | preferente (`P`) | Elige terms in the preferente cabin |
| Prémium | preferente (`P`) | most flexible, includes catering on most AVE |

AVLO (Renfe's low-cost brand) uses its own codes and usually offers a single cheap bucket. When you
quote "from X €", say which bucket it is — a user comparing a Básico to a competitor's flexible fare
is comparing the wrong things.

The table above is the shape of the ladder, not a contract: Renfe changes fare conditions, and the
exact change/refund terms for a given ticket are the ones shown at `renfe.com` at purchase. Point the
user there for the fine print rather than asserting penalties from memory.

### 4. Round trips are a single search

```bash
renfe search madrid barcelona --date friday --return sunday
```

This prints **both** directions and prices the outbound under round-trip rules. Do not run two
one-way searches to fake it: the round-trip prices are genuinely different, and Renfe answers both
legs in one request anyway.

**A connection may change station, and that changes what the transfer time means.** Renfe warns
about it and both `search` and `stops` print the warning:

```
⚠ ATENCIÓN: Necesario cambio de estación de enlace. Transbordo necesario entre las estaciones de
  MADRID-PUERTA DE ATOCHA-ALMUDENA GRANDES y MADRID-CHAMARTÍN-CLARA CAMPOAMOR.
```

Barcelona→Santander is the common case: 1h56 of "enlace" that is really a taxi or metro across
Madrid with luggage. Never present such a connection as a comfortable wait — surface the warning, and
say the margin is smaller than it looks. In `--json` it is `station_change`.

**The two lists are independent, and pairing them is your job.** Nothing in Renfe's answer — or in
the CLI — checks that a return actually departs after the outbound arrives. On a short hop that
hardly matters; on a long route it is the whole answer. Before quoting a same-day round trip, take
the outbound's `arrival` and keep only returns whose `departure` is later, then report the real
choice. `--json` plus a couple of lines of script is the reliable way; see the day-trip section.

**Time windows are per leg.** `--after`/`--before` bound the **outbound**; `--return-after` /
`--return-before` bound the return. Give neither of the latter and the outbound window carries over,
which is what you want when narrowing "afternoon trains" on both legs — but say which leg you meant
when you report the result.

## Day trips: check it is possible before you quote a price

When the user wants to go and come back the same day, the first question is not the fare — it is
whether the trip exists at all. Spanish rail has plenty of city pairs that look routine and take six
hours each way with a change.

`search` now does the counting for you when both dates are the same, and prints it under the tables:

```
same-day return: earliest arrival 12:35, so only 2 of the 10 returns above are catchable
```

Read that line before quoting anything, and lead your answer with it. A real example — Barcelona↔Albacete, one Wednesday: 5½–8 h per leg,
earliest arrival 12:35, and only **two** of a dozen returns departed later. The honest answer opened
with "this leaves you 1h15 in Albacete", not with "from 95,85 €".

If the day trip is unattractive, say so plainly and offer the alternative (a later date, or an
overnight). Quoting a cheap fare for a combination nobody would actually travel is a worse answer
than saying the trip does not work.

## Dates

`--date` takes `YYYY-MM-DD`, `DD/MM/YYYY`, `today`, `tomorrow`, a weekday name, or `+N` days. A
weekday means the **next** one — asking for "friday" on a Friday gets the week ahead, not a date
whose trains have already left. When the user says "el viernes" and today is Friday, confirm which
one they mean rather than assuming.

"La semana que viene" is ambiguous near a week boundary — said on a Sunday it usually means the week
starting tomorrow, but not always. When two readings are both plausible and a search is cheap, price
**both** and label them, rather than asking and stalling: the second date often turns out cheaper,
which is useful information either way.

Renfe sells a limited window ahead, and it is **shorter than people expect**: measured at 83 days on
Madrid–Barcelona in September 2026, with a hard cliff on one date (sales open in batches tied to the
timetable, so the horizon jumps rather than sliding daily). Do not quote a number of months to the
user as if it were a rule — if they need the exact horizon, probe it, and say which route and day you
measured.

A date beyond it returns `no trains for … — the date may be outside Renfe's sale window`. Treat
that as "not on sale yet", not "no such route". A date **already gone by** is a different answer:
the CLI refuses it outright (`… has already gone by — Renfe sells from today (…) onwards`) rather
than spending two requests to come back empty. If you see that, you mis-parsed the year — re-ask.

## Prices are PER PASSENGER

Renfe quotes each fare per traveller, not per booking. `renfe search --adults 3` still prints
`from 25,30 €`, and the party pays 75,90 €. Whenever you searched for more than one person,
multiply before you report a trip cost — and say which number you are giving.

Multiply by the travellers who **occupy a seat**: adults and children aged 4–13. An under-4 given
with `--infants` rides on a lap, Renfe carries them in a separate field, and the quoted fare does
not move when one is added — so `--adults 2 --infants 1` is two fares, not three. `search` says as
much in its own second line; do not add the infant back in. What Renfe charges for a lap infant at
the counter, if anything, is not something this API reveals: if the user needs that figure, say so
rather than inventing one.

## The prices are base prices — discounts come later

This is the easiest way to mislead a user. The search form carries **no discount field at all**:
Tarjeta Dorada, Carné Joven and Familia Numerosa are applied further into the booking flow, per
traveller, not at search time. So the figures the CLI reports are the undiscounted fare.

If the user mentions being a pensioner, under 26, or a large family — or their +Renfe card level
suggests it — quote the price **and** say it is before their card's discount, which is substantial
(Tarjeta Dorada is worth roughly a quarter to a half of the fare on many services). Never tell such a
user "the ticket costs X" flat; they will pay less, and a wrong number in that direction still
damages the decision they are making.

Likewise, promo codes (`codPromocional` on the website) are not exposed by the CLI.

## An empty result is ambiguous — read the error

Renfe returns the same empty list whatever the cause, so the CLI does the diagnosis and **names an
active filter first**:

```
error: no trains for MADRID (TODAS) → BARCELONA (TODAS) on 2026-09-22 with --bike
       — try dropping that filter, or the date may be outside Renfe's sale window
```

That is the common trap: `--bike` and `--pet` genuinely exclude most trains (on a Madrid–Barcelona
day, `--pet` cuts 23 trains to 4, and `--bike` to none), so an empty result usually means the filter,
not the date. Drop the filter and re-run before telling the user the route does not exist.

## Intermediate stops

`renfe stops` answers "does this train stop at Córdoba?", "what time does it reach Sevilla?" and
"what's on board?".

```bash
renfe stops madrid cadiz --date +7 --train 2074
renfe stops madrid huelva --date +2 --at 13:00     # or pick it by departure time
```

It runs a search first — Renfe has no endpoint that takes a bare train number, so the itinerary is
matched against the journey list held in the session. That means the train must be one that appears
in *that* search: to detail a train you saw earlier, re-run `stops` with the same route and date
rather than expecting a number alone to work.

The itinerary is **the ticket's, not the train's**. Many services run beyond the destination you
searched — the 07:27 Madrid–Barcelona carries on to Figueres — and the calling points stop at your
station, with your arrival time, not the train's terminus. So "the last stop shown" is where the
user gets off, and you can read it as such.

A journey with a connection prints both legs, each with its own calling points, plus the transfer
time. The onboard services it lists ("Cafetería/bar móvil", wheelchair-accessible toilet, Sala Club)
are Renfe's own for that train, and are worth surfacing when the user's question is about
accessibility or a long trip.

## International: France and Portugal are in scope

Renfe sells a handful of cross-border journeys and the CLI reaches them, but the catalogue lists
foreign stations under their **local** names. The CLI aliases the common Spanish ones (Marsella,
Perpiñán, Aviñón, Oporto); for anything else, pass the local spelling.

```console
$ renfe search barcelona "marseille st charles" --date +3
AVE INT   16:31 → 21:32  5h 01m   tren 9725   from 109.00 €

$ renfe search vigo oporto --date +3
TRENCELT  08:58 → 10:24  2h 26m   tren 420    from 16.75 €
```

Served: **France** — Marseille, Avignon, Aix-en-Provence, Narbonne, Perpignan, Montpellier, Nîmes,
Lyon, Valence. **Portugal** — Porto Campanhã, Nine, Viana do Castelo. There is **no Lisboa** in the
catalogue, so do not offer Madrid–Lisboa; say it is not sold here.

Train types beyond the usual: `AVE INT` (cross-border AVE) and `TRENCELT` (the Vigo–Porto Celta).

## What this CLI does not cover

- **Cercanías (the C-lines)** are not sold through this flow. A short hop may still come back as a
  Media Distancia (`MD`) train, which *is* sellable, but do not expect commuter-line frequencies.
  A pair served *only* by Cercanías (Atocha→Alcalá de Henares, verified) returns the ordinary empty
  result, whose message talks about the sale window — do not repeat that to the user; say it is a
  Cercanías route. For those timetables send them to `renfe.com/es/es/cercanias`.
- **Other operators.** Ouigo, Iryo and Alsa run the same corridors and are often cheaper; this CLI
  sees only Renfe. When price is the user's whole question, say that the comparison is Renfe-only.
- **Seat maps, coach numbers and live delays.** Not exposed by the endpoints the CLI uses.
  Intermediate stops *are* — see `renfe stops`.
- **Assistance (ATENDO)** cannot be filtered here — the search ignores the flag entirely (verified:
  byte-identical responses with it on and off). It is arranged when booking, or on 912 320 320.
  `--wheelchair` is different: it asks for an H space and is a real search parameter.

## When Renfe puts you in a queue

Renfe fronts the booking site with Queue-it during demand spikes (a timetable opening, a promotion).
The CLI detects it and says so:

```
error: renfe has put this request in its virtual waiting room (queue-it) — the site is under heavy load…
```

Do **not** retry in a loop: everyone is queued, retrying does not jump the line, and hammering the
site is exactly what the waiting room exists to stop. Tell the user to wait, or to hold their place
at `venta.renfe.com` in a browser.

## The user's own account (optional, read-only)

Search needs no login. The account commands read a session the user is already holding in their
browser; the CLI never asks for or stores a password.

```bash
renfe login --from-browser chrome   # chrome|chromium|firefox|safari|edge|brave; empty = any
renfe whoami                        # confirms the session, shows the +Renfe card
renfe trips                         # upcoming journeys on the account
```

`trips` never passes off a lapsed login as an empty account: Renfe answers both with an empty array,
so the CLI confirms the session before reporting "no upcoming trips". Take that message at face
value — it means the account really has nothing booked.

If `whoami` reports the session is no longer signed in, the fix is for the **user** to log in again
in their browser and re-run `login` — there is nothing the CLI can do about it, so say that plainly
instead of retrying.

### Sessions expire fast — and there are two of them

Renfe drops an idle session after about 30 minutes, so a session lifted at the start of a long task
is often dead by the end of it. Before a sequence of account commands, check with `renfe session`;
for a long stretch of work, start `renfe session keep` (it sleeps until just before each lapse).

The distinction that matters: Renfe runs a **servlet session** and a **login** on top of it, with
separate clocks. The first can report a healthy 29 minutes while the second is already gone — which
is exactly when `renfe session` prints `NOT signed in` despite time remaining. Never read "session
expires in 29m" as "you are signed in"; `whoami` is what answers that.

**A browser that looks signed in often is not.** Its cookie file keeps the last session it wrote
even after that session has died server-side — `JSESSIONID` is a session cookie the browser holds in
memory, so the copy on disk routinely outlives it. `login` reads the disk copy, so it can find a
complete-looking set of cookies that Renfe then refuses. It verifies against the server before
storing anything and says exactly that:

```
error: those cookies are stale: the browser still has them on disk, but Renfe no longer accepts
       them, so nothing was stored.
```

When that happens, do not retry — ask the user to open venta.renfe.com and **reload** it. A tab left
open for a while shows a page rendered when they were signed in; the session behind it may be long
gone, which is why "but I have it open in Chrome" and "the CLI says I am not signed in" are both
true at once. Trust `login`'s verdict over the browser's appearance.

## Agent output

Every command takes `--json` and `--toon`. Prefer `--toon` when you are parsing the result yourself:
same shapes, markedly fewer tokens. Data goes to stdout, diagnostics to stderr, so `2>/dev/null` is
safe when you only want the data. Exit codes: `0` ok, `1` runtime error, `2` usage.

## Reporting results to the user

- Lead with the answer (the train, or the cheapest day), not the table.
- Quote the fare bucket alongside the price, and the train type (AVE/AVLO/Avant/MD) — they imply very
  different journeys.
- Say when you looked. Prices move.
- If nothing is available, say which constraint caused it (sold out, outside the sale window, no
  direct service) rather than an unqualified "no trains".

## Reference

`references/cli-reference.md` — every command, its flags and the `--json` shapes.
