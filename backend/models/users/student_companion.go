package users

import (
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
)

// Sentinel errors for companion links ("läuft mit", Laufgemeinschaft). People
// Directory's departure contract owns them; these names are the same values,
// so every errors.Is call site keeps matching whichever package it names.
var (
	ErrCompanionSelfLink           = departure.ErrCompanionSelfLink
	ErrCompanionInvalidWeekday     = departure.ErrCompanionInvalidWeekday
	ErrCompanionStudentIDRequired  = departure.ErrCompanionStudentIDRequired
	ErrCompanionWouldLoseDeparture = departure.ErrCompanionWouldLoseDeparture
	ErrCompanionLockBusy           = departure.ErrCompanionLockBusy
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

// StudentCompanion is one undirected "walks home with" edge between two
// children on one weekday (users.student_companions).
//
// There is deliberately no group entity: a Laufgemeinschaft is the connected
// component of these edges for a given weekday, resolved by the reader. See the
// 1.15.209 migration for why.
//
// StudentLowID always holds the smaller of the two student ids (DB CHECK), so a
// pair is stored exactly once per weekday. Care Plan's domain builds the edges;
// this row is the People Directory read shape.
type StudentCompanion struct {
	base.Model `bun:"schema:users,table:student_companions"`
	base.TenantModel
	StudentLowID  int64 `bun:"student_low_id,notnull" json:"student_low_id"`
	StudentHighID int64 `bun:"student_high_id,notnull" json:"student_high_id"`
	Weekday       int   `bun:"weekday,notnull" json:"weekday"`
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
	return DeparturePlan{AllowedDepartureModes: allowed, DepartureDays: days}.AccompaniedDays()
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
// weekday they walk together. People Directory's departure contract owns it.
type CompanionLink = departure.CompanionLink

// CompanionWeekdayShortLabels are the German two-letter weekday labels of the
// offline lists; People Directory's departure contract owns them.
var CompanionWeekdayShortLabels = departure.CompanionWeekdayShortLabels
