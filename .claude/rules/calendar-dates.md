# Calendar Dates: timezone.Date, Not time.Time

**RULE: A calendar day (no clock, no timezone) is represented as `timezone.Date` everywhere in the backend** — model fields for `DATE` columns, repository signatures, query parameters compared against `DATE` columns, and API payloads (`YYYY-MM-DD`). `time.Time` is for instants only. On the frontend, calendar dates travel as `"YYYY-MM-DD"` strings and are never derived via `.toISOString()`.

## Why

bun converts EVERY `time.Time` parameter to UTC before binding
(`BaseDialect.AppendTime`: `tm.UTC().AppendFormat(...)`). A Berlin-midnight
value (`2026-02-02 00:00 CET`) is sent as `2026-02-01 23:00 UTC`; PostgreSQL
casts to DATE and stores `2026-02-01` — one day behind, whenever the wall
clock is between 00:00 and 02:00 Berlin time. Reads were wrong symmetrically:
DATE columns scan back as UTC midnight. This single mechanism caused dozens
of production and test-flake fixes before the 2026-06 migration eliminated
the bug class.

`timezone.Date` (backend/internal/timezone/date.go) is a string-backed
`YYYY-MM-DD` value with no instant and no location. bun binds that string
directly — there is nothing for the driver to shift. The old
compensation helpers `DateOfUTC`/`TodayUTC` are deleted; the remaining
`Today()`/`DateOf()` return instants and exist only for TIMESTAMPTZ boundary
math.

## Need X → use Y (backend)

| Need | Use | Never |
|---|---|---|
| Today as a calendar day | `timezone.TodayDate()` | `timezone.Today()`, `time.Now().Truncate(24h)` |
| Calendar day of an instant | `timezone.DateFromTime(t)` | manual UTC/Berlin truncation |
| Model field for a `DATE` column | `timezone.Date` / `*timezone.Date` (nullable) | `time.Time`, zero-Date sentinels |
| Parse an API date | `timezone.ParseDate("2026-06-10")` | `time.Parse("2006-01-02", s)` |
| Serialize a date | `d.String()` (ISO) / `d.Format("02.01.2006")` (German) | `.Format` on a scanned instant |
| Compare / sort dates | `==`, `d.Before(o)`, `d.After(o)`, `d.Compare(o)`; map keys work | comparing `time.Time` against DATE values |
| Date arithmetic | `d.AddDays(n)`, `a.DaysUntil(b)` (DST-exact), `d.Weekday()` | `AddDate`, `Sub().Hours()/24` |
| Day boundaries as instants (TIMESTAMPTZ scans) | `d.BerlinMidnight()`, `d.EndOfDay()` (23:59:59 Berlin) | `timezone.EndOfDay(t)` on raw scanned values |
| Actual instant (created_at, check_in_time) | `time.Time` (TIMESTAMPTZ) | — |
| Clock time without date (TIME column) | `timezone.NormalizeWallClock()` normalization | TIMESTAMPTZ |

## Need X → use Y (frontend)

| Need | Use | Never |
|---|---|---|
| `Date` → `"YYYY-MM-DD"` | `toISODate(d)` from `~/lib/date-helpers` | `d.toISOString().split("T")[0]` / `.slice(0, 10)` |
| Today as `"YYYY-MM-DD"` | `todayISO()` | `new Date().toISOString()...` |
| `"YYYY-MM-DD"` → `Date` (local midnight) | `parseISODate(s)` | `new Date("YYYY-MM-DD")` (UTC midnight) |
| Render a date string in German | `formatDate(s)` from `~/lib/date-helpers` (handles date-only and timestamps) | new file-local `formatDate` duplicates |

`.toISOString()` returns the UTC date — one day behind Berlin between 00:00
and 02:00. The oxlint rule `date-safety/no-utc-date-extraction`
(`frontend/scripts/oxlint-plugin-date-safety.mjs`) rejects deriving calendar
dates from it.

## Forbidden / Required (backend)

```go
// FORBIDDEN — time.Time field on a DATE column (shifts a day around midnight)
Date time.Time `bun:"date,notnull" json:"date"`

// FORBIDDEN — UTC-day math on instants (forbidigo-banned)
today := time.Now().Truncate(24 * time.Hour)

// FORBIDDEN — passing time.Time where a DATE column is compared; it takes
// bun's UTC path silently
query.Where("date = ?", someTime)

// CORRECT — DATE column field
Date timezone.Date `bun:"date,notnull,type:date" json:"date"`

// CORRECT — today's calendar day, comparison, boundary instant
today := timezone.TodayDate()
if entry.Date == today { ... }
q.Where("check_in_time < ?", entry.Date.EndOfDay())
```

**NULL semantics**: the zero value is `timezone.Date("")` and means unset; do
not bind it as a date or use it as a sentinel. An optional date is
`*timezone.Date`, where `nil` binds NULL.

## Enforcement

1. **`TestDateColumnTypes`** (`backend/test/calendar_date_verification_test.go`)
   discovers every DATE column by scanning the SQL in
   `database/migrations/*.go`, maps it to its model field via `go/parser`,
   and fails when the field is `time.Time`. Its maps (`unmappedDateColumns`,
   `renamedDateColumns`, `droppedDateColumns`) classify columns the scan
   cannot join; stale entries fail the test. **A new DATE column compiles
   only with a `timezone.Date` field.**
2. The same test bans `Truncate(24 * time.Hour)` date math everywhere
   (`no_truncate_24h_date_math`, empty allowlist).
3. **forbidigo** (`.golangci.yml`) bans `time.Time.Truncate` in production
   code; sub-day alignment needs `//nolint:forbidigo // <why>`.
4. **oxlint** `date-safety/no-utc-date-extraction` bans
   `.toISOString().{split,slice,substring,substr}` in the frontend.
5. **`TestActivityInstanceWallClockRatchet`**
   (`backend/test/wall_clock_verification_test.go`) rejects
   `ActivityInstance.StartTime`/`EndTime` assignments derived from
   `time.Now()` or `timezone.Now()` unless they pass through
   `timezone.NormalizeWallClock()`. These fields map to PostgreSQL `TIME`; binding a
   Berlin instant converts it to UTC and can reverse the clock interval around
   UTC midnight.

## Detection

```bash
rg --type go 'time\.Time' backend/models/ | rg -i '`bun:"[^"]*"' | rg -i 'date|day|birthday|valid_'  # suspect fields
rg --type go 'Truncate\(24 \* time\.Hour\)' backend/
cd backend && go test ./test/ -run 'Test(DateColumnTypes|ActivityInstanceWallClockRatchet)' # backend date/time gates
rg -n '\.toISOString\(\)\s*\.\s*(split|slice)' frontend/src/                  # frontend UTC extraction
```

## Code Review Checklist

- [ ] New `DATE` column in a migration → model field is `timezone.Date`; the registry test stays green without new allowlist entries
- [ ] No `time.Time` field whose name suggests a calendar day (`date`, `*_date`, `day`, `birthday`, `valid_from`, …)
- [ ] No raw `time.Time` argument compared against a DATE column in Where clauses or raw SQL
- [ ] Ad-hoc repo result structs (ModelTableExpr pattern) scanning DATE columns use `timezone.Date` too — the registry test does NOT see these
- [ ] API contract checked: a model-serialized date field emits `"YYYY-MM-DD"` — verify consumers when flipping a previously RFC3339 field
- [ ] Tests construct dates via `timezone.NewDate(...)`/`TodayDate()` — never `time.Now()`-derived instants for calendar days
- [ ] Frontend serializes via `toISODate`/`todayISO`/`parseISODate` — no `.toISOString()` date extraction

## Scope boundary

TIME WITHOUT TIME ZONE columns (wall-clock times like `11:30`) are a separate
concern: normalize via `timezone.NormalizeWallClock()` (see rule 11 in CLAUDE.md).
`ActivityInstance` writes have a source ratchet; other TIME-backed models still
rely on their repository/service normalization and code review. Instants
(TIMESTAMPTZ) stay `time.Time` everywhere.
