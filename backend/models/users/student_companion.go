package users

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Sentinel errors for companion links ("läuft mit", Laufgemeinschaft).
var (
	// ErrCompanionSelfLink is returned when a child is linked to itself.
	ErrCompanionSelfLink = errors.New("a child cannot be its own departure companion")

	// ErrCompanionInvalidWeekday is returned for a weekday outside Mon..Fri.
	ErrCompanionInvalidWeekday = errors.New("companion weekday must be one of mon/tue/wed/thu/fri")

	// ErrCompanionStudentIDRequired is returned when an edge is built without a
	// usable child id on one end — which is what a missing or non-positive
	// companion_student_id in the request body decodes to. A sentinel, not a
	// fresh error: the handler's error table maps it to a 400, whereas an
	// untyped error would leak malformed client input as a 500.
	ErrCompanionStudentIDRequired = errors.New("Bitte ein Kind für die Laufgemeinschaft auswählen.") //nolint:staticcheck // ST1005: user-facing German message

	// ErrCompanionWouldLoseDeparture indicates that removing a link would leave
	// the OTHER child with an accompanied ("Anderes Kind") departure plan and no
	// remaining detail — neither another link nor a free-text note. Refused
	// rather than silently narrowing that child's plan: one child's edit must
	// never change another child's departure permissions, and leaving the
	// contradiction in place would block every later edit of that child.
	//
	// Lives in the model layer (not services/users) because the repository's
	// shared departure-plan write path enforces the same invariant for callers
	// that bypass the student service (enrollment approval, imports); the
	// services package re-exports it so existing errors.Is call sites keep
	// matching the same instance.
	ErrCompanionWouldLoseDeparture = errors.New("Ein verknüpftes Kind hätte danach keine Angabe mehr dazu, mit wem es nach Hause geht. Bitte zuerst den Heimweg dieses Kindes anpassen.") //nolint:staticcheck // ST1005: user-facing German message

	// ErrCompanionLockBusy indicates that a linked child's row is currently
	// locked by another transaction that this one may NOT wait for without
	// risking a deadlock — the companion graph is discovered while locks are
	// already held, so an id below the ones we hold has to be taken without
	// waiting (see the lock protocol on StudentRepository.lockCompanionFarEnds
	// and api/students lockStudentCompanionGraph).
	//
	// It is a transient, retriable conflict, not a data problem: the same
	// request succeeds once the other edit commits. Mapped to 409 so the client
	// can say "please try again" instead of showing a 500 for a legitimate edit.
	ErrCompanionLockBusy = errors.New("Ein verknüpftes Kind wird gerade an anderer Stelle bearbeitet. Bitte in einem Moment erneut speichern.") //nolint:staticcheck // ST1005: user-facing German message
)

// CompanionWeekdayNumbers maps the weekday keys used across the departure model
// (departure_days, allowed_departure_modes, bus_days) to the SMALLINT stored in
// users.student_companions. The API speaks keys — the same ones the departure
// UI already uses — while the column stays a compact 1..5 matching
// schedule.student_pickup_schedules.
var CompanionWeekdayNumbers = map[string]int{
	PickupDayMonday:    1,
	PickupDayTuesday:   2,
	PickupDayWednesday: 3,
	PickupDayThursday:  4,
	PickupDayFriday:    5,
}

// CompanionWeekdayKeys is the reverse of CompanionWeekdayNumbers.
var CompanionWeekdayKeys = map[int]string{
	1: PickupDayMonday,
	2: PickupDayTuesday,
	3: PickupDayWednesday,
	4: PickupDayThursday,
	5: PickupDayFriday,
}

// CompanionWeekdayShortLabels are the German two-letter weekday labels the
// offline lists (Tagesliste, Wochenliste, Klassenliste) already use for the
// departure plan. Kept next to the weekday maps so a companion detail never
// renders a different abbreviation than the plan it belongs to.
var CompanionWeekdayShortLabels = map[string]string{
	PickupDayMonday:    "Mo",
	PickupDayTuesday:   "Di",
	PickupDayWednesday: "Mi",
	PickupDayThursday:  "Do",
	PickupDayFriday:    "Fr",
}

// CompanionWeekdayNumber translates a weekday key into its stored number.
func CompanionWeekdayNumber(day string) (int, error) {
	number, ok := CompanionWeekdayNumbers[day]
	if !ok {
		return 0, fmt.Errorf("%w: got %q", ErrCompanionInvalidWeekday, day)
	}
	return number, nil
}

// StudentCompanion is one undirected "walks home with" edge between two
// children on one weekday (users.student_companions).
//
// There is deliberately no group entity: a Laufgemeinschaft is the connected
// component of these edges for a given weekday, resolved by the reader. See the
// 1.15.209 migration for why.
//
// StudentLowID always holds the smaller of the two student ids (DB CHECK), so a
// pair is stored exactly once per weekday. Construct via NewStudentCompanion
// rather than setting the fields directly.
type StudentCompanion struct {
	base.Model `bun:"schema:users,table:student_companions"`
	base.TenantModel
	StudentLowID  int64 `bun:"student_low_id,notnull" json:"student_low_id"`
	StudentHighID int64 `bun:"student_high_id,notnull" json:"student_high_id"`
	Weekday       int   `bun:"weekday,notnull" json:"weekday"`
}

// NewStudentCompanion builds an edge between two students, normalizing the pair
// into the stored low/high order.
func NewStudentCompanion(studentID, companionID int64, weekday int) (*StudentCompanion, error) {
	if studentID <= 0 || companionID <= 0 {
		return nil, ErrCompanionStudentIDRequired
	}
	if studentID == companionID {
		return nil, ErrCompanionSelfLink
	}
	if _, ok := CompanionWeekdayKeys[weekday]; !ok {
		return nil, fmt.Errorf("%w: got %d", ErrCompanionInvalidWeekday, weekday)
	}

	low, high := studentID, companionID
	if low > high {
		low, high = high, low
	}
	return &StudentCompanion{
		StudentLowID:  low,
		StudentHighID: high,
		Weekday:       weekday,
	}, nil
}

// Other returns the id at the far end of the edge as seen from studentID, and
// whether studentID is part of the edge at all.
func (c *StudentCompanion) Other(studentID int64) (int64, bool) {
	switch studentID {
	case c.StudentLowID:
		return c.StudentHighID, true
	case c.StudentHighID:
		return c.StudentLowID, true
	default:
		return 0, false
	}
}

// AccompaniedWeekdays returns the weekday keys on which a child may leave with
// another child, reading the unified allowed-modes map first and falling back to
// the exclusive departure_days for records that predate it.
//
// This is what couples a Laufgemeinschaft to the departure plan: a link may only
// exist on a day the plan actually allows. Without that, a child's Stammdaten
// could read "geht immer alleine" and "läuft mit Sophie" at the same time.
func AccompaniedWeekdays(allowed AllowedDepartureModes, days DepartureDays) map[string]bool {
	out := make(map[string]bool, len(PickupDayOrder))
	for _, day := range PickupDayOrder {
		if modes, ok := allowed[day]; ok {
			for _, mode := range modes {
				if mode == DepartureAccompanied {
					out[day] = true
				}
			}
		}
		if days[day] == DepartureAccompanied {
			out[day] = true
		}
	}
	return out
}

// WithAccompaniedDays returns a copy of the allowed-modes map that additionally
// permits the accompanied mode on the given weekdays.
//
// Widening only: the existing modes of every day are kept, so permitting "läuft
// mit" never takes away the bus or a pickup. Removing a mode is not this
// function's job and must not become it — that would let one child's edit
// silently narrow another child's departure permissions.
func WithAccompaniedDays(allowed AllowedDepartureModes, days []string) AllowedDepartureModes {
	out := make(AllowedDepartureModes, len(allowed)+len(days))
	for day, modes := range allowed {
		out[day] = append([]DepartureMode(nil), modes...)
	}
	for _, day := range days {
		if _, ok := CompanionWeekdayNumbers[day]; !ok {
			continue
		}
		already := false
		for _, mode := range out[day] {
			if mode == DepartureAccompanied {
				already = true
			}
		}
		if !already {
			out[day] = append(out[day], DepartureAccompanied)
		}
	}
	return out.Normalize()
}

// CompanionLink is the per-child view of the edges: one companion plus every
// weekday they walk together. This is the shape the child detail view edits and
// the API speaks; it is not persisted.
type CompanionLink struct {
	CompanionStudentID int64    `json:"companion_student_id"`
	FirstName          string   `json:"first_name,omitempty"`
	LastName           string   `json:"last_name,omitempty"`
	Weekdays           []string `json:"weekdays"`
}

// CompanionDaysFromLinks folds a link list into the per-weekday cover set
// Student.DepartureCompanionDays expects: the days on which the child has a
// structured "mit wem" answer.
func CompanionDaysFromLinks(links []CompanionLink) map[string]bool {
	days := make(map[string]bool, len(PickupDayOrder))
	for _, link := range links {
		for _, day := range link.Weekdays {
			days[day] = true
		}
	}
	return days
}

// FilterCompanionLinksToDays keeps only the weekdays the given set allows and
// drops a link that has none left. Pure derivation: it writes nothing and never
// widens.
//
// It exists for readers that render a companion list NEXT TO a departure plan
// the links were not reconciled against — the class roster prints a plan taken
// from the approved enrollment phase, while the links belong to the live child
// and follow every later Stammdaten edit. Printing them unfiltered puts "Di:
// Bus" and "läuft dienstags mit Mia" on the same line of the sheet staff carry
// to the door. The update path has no use for this: there the links are trimmed
// against the plan being written (TrimCompanionsToDays), which also refuses to
// strand the child at the far end.
func FilterCompanionLinksToDays(links []CompanionLink, allowedDays map[string]bool) []CompanionLink {
	filtered := make([]CompanionLink, 0, len(links))
	for _, link := range links {
		kept := make([]string, 0, len(link.Weekdays))
		for _, day := range link.Weekdays {
			if allowedDays[day] {
				kept = append(kept, day)
			}
		}
		if len(kept) == 0 {
			continue
		}
		copied := link
		copied.Weekdays = kept
		filtered = append(filtered, copied)
	}
	return filtered
}

// CompanionLinksFingerprint is an order-independent fingerprint of a companion
// list: the state a client read, in one comparable string.
//
// It exists because the submitted list REPLACES the stored one. Two staff
// members editing the same child from the same snapshot both send a complete
// list, and the row locks only decide who writes first — the second write would
// otherwise delete the links the first one just committed, with nothing in the
// data to notice it. The client echoes the fingerprint of the list it loaded
// and the write path compares it against the stored links while holding the
// subject's row lock (see validateCompanionUpdate).
//
// The format is MIRRORED by companionsFingerprint() in
// frontend/src/lib/student-companion-api.ts, which the forms already use for
// their dirty check — "<id>:<mon,tue,…>" per link, links sorted as strings and
// joined with "|". Both sides build it from the same wire data, so the strings
// match byte for byte; change one and you must change the other.
func CompanionLinksFingerprint(links []CompanionLink) string {
	entries := make([]string, 0, len(links))
	for _, link := range links {
		requested := make(map[string]bool, len(link.Weekdays))
		for _, day := range link.Weekdays {
			requested[day] = true
		}
		days := make([]string, 0, len(PickupDayOrder))
		for _, day := range PickupDayOrder {
			if requested[day] {
				days = append(days, day)
			}
		}
		entries = append(entries, strconv.FormatInt(link.CompanionStudentID, 10)+":"+strings.Join(days, ","))
	}
	sort.Strings(entries)
	return strings.Join(entries, "|")
}

// CompanionDisplayName is the companion's full name, falling back to the id for
// a link whose names were not joined in.
func CompanionDisplayName(link CompanionLink) string {
	name := strings.TrimSpace(link.FirstName + " " + link.LastName)
	if name == "" {
		return "Kind #" + strconv.FormatInt(link.CompanionStudentID, 10)
	}
	return name
}

// FormatCompanionLinks renders the structured "läuft mit" links for the offline
// lists: "Mia Schulz (Mo, Di), Tom Meier".
//
// Every export that prints an accompanied departure needs it. A child whose
// "mit wem" is answered by links may legitimately have NO free-text note (the
// note is only required for a day no link covers), so a list built from the
// note alone would tell staff "Mit anderem Kind" and nothing else — on the one
// sheet they use when the app is not at hand. The weekdays are named unless the
// link covers all five, so a Monday-only Laufgemeinschaft cannot be read as a
// standing arrangement.
func FormatCompanionLinks(links []CompanionLink) string {
	parts := make([]string, 0, len(links))
	for _, link := range links {
		requested := make(map[string]bool, len(link.Weekdays))
		for _, day := range link.Weekdays {
			requested[day] = true
		}
		days := make([]string, 0, len(PickupDayOrder))
		for _, day := range PickupDayOrder {
			if requested[day] {
				days = append(days, CompanionWeekdayShortLabels[day])
			}
		}
		if len(days) == 0 {
			continue
		}
		part := CompanionDisplayName(link)
		if len(days) < len(PickupDayOrder) {
			part += " (" + strings.Join(days, ", ") + ")"
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, ", ")
}

// CompanionLinksFromEdges folds the edges touching studentID into one entry per
// companion with their weekdays in Mon..Fri order. Edges not touching studentID
// are ignored.
func CompanionLinksFromEdges(studentID int64, edges []*StudentCompanion) []CompanionLink {
	byCompanion := make(map[int64]map[int]bool)
	for _, edge := range edges {
		if edge == nil {
			continue
		}
		other, ok := edge.Other(studentID)
		if !ok {
			continue
		}
		if byCompanion[other] == nil {
			byCompanion[other] = make(map[int]bool)
		}
		byCompanion[other][edge.Weekday] = true
	}

	links := make([]CompanionLink, 0, len(byCompanion))
	for companionID, weekdays := range byCompanion {
		numbers := make([]int, 0, len(weekdays))
		for number := range weekdays {
			numbers = append(numbers, number)
		}
		sort.Ints(numbers)

		keys := make([]string, 0, len(numbers))
		for _, number := range numbers {
			if key, ok := CompanionWeekdayKeys[number]; ok {
				keys = append(keys, key)
			}
		}
		links = append(links, CompanionLink{CompanionStudentID: companionID, Weekdays: keys})
	}

	sort.Slice(links, func(i, j int) bool {
		return links[i].CompanionStudentID < links[j].CompanionStudentID
	})
	return links
}
