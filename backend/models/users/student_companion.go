package users

import (
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

// CompanionLink is the per-child view of the edges: one companion plus every
// weekday they walk together. People Directory's departure contract owns it.
type CompanionLink = departure.CompanionLink

// CompanionWeekdayShortLabels are the German two-letter weekday labels of the
// offline lists; People Directory's departure contract owns them.
var CompanionWeekdayShortLabels = departure.CompanionWeekdayShortLabels
