package users

import (
	"errors"
	"fmt"
	"sort"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Sentinel errors for companion links ("läuft mit", Laufgemeinschaft).
var (
	// ErrCompanionSelfLink is returned when a child is linked to itself.
	ErrCompanionSelfLink = errors.New("a child cannot be its own departure companion")

	// ErrCompanionInvalidWeekday is returned for a weekday outside Mon..Fri.
	ErrCompanionInvalidWeekday = errors.New("companion weekday must be one of mon/tue/wed/thu/fri")
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
// 1.15.208 migration for why.
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
		return nil, errors.New("both student IDs are required for a companion link")
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
