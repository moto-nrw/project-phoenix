package domain

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// RegistrationModel is how parents register a child for lunch (#3873).
type RegistrationModel string

const (
	// ModelSingle: register and cancel until one deadline per meal day.
	ModelSingle RegistrationModel = "ongoing_single"
	// ModelSplit: registering and cancelling have deadlines of their own.
	ModelSplit RegistrationModel = "ongoing_split"
	// ModelAnnual: parents register once until a fixed date; afterwards only
	// the school registers, and parents cancel when the school allows it.
	ModelAnnual RegistrationModel = "annual"
)

// DeadlineKind is how a deadline is anchored to the meal day.
type DeadlineKind string

const (
	DeadlineDaysBefore   DeadlineKind = "days_before"
	DeadlinePreviousWeek DeadlineKind = "previous_week"
)

// AnnualChanges says who changes a registration after the annual deadline.
type AnnualChanges string

const (
	AnnualParentsCancel AnnualChanges = "parents_cancel"
	AnnualSchoolOnly    AnnualChanges = "school_only"
)

// SickMode says what an absence reported after the cancellation deadline does.
type SickMode string

const (
	SickStaysOrdered SickMode = "stays_ordered"
	SickCancelled    SickMode = "cancelled"
)

// Rhythm is the period one order covers.
type Rhythm string

const (
	RhythmDaily  Rhythm = "daily"
	RhythmWeekly Rhythm = "weekly"
)

// Detail is how far an order breaks its numbers down.
type Detail string

const (
	DetailTotal   Detail = "total"
	DetailByClass Detail = "by_class"
)

// maxLeadDays bounds the days-before deadline; the settings registry
// validates the same range.
const maxLeadDays = 30

// Deadline is one deadline of a meal day: either LeadDays before it or on
// Weekday of the previous week, at Clock (HH:MM, Berlin).
type Deadline struct {
	Kind     DeadlineKind
	LeadDays int
	Weekday  time.Weekday
	Clock    string
}

// Valid reports whether the deadline can be evaluated.
func (d Deadline) Valid() bool {
	if _, err := time.Parse("15:04", d.Clock); err != nil {
		return false
	}
	switch d.Kind {
	case DeadlineDaysBefore:
		return d.LeadDays >= 0 && d.LeadDays <= maxLeadDays
	case DeadlinePreviousWeek:
		return d.Weekday >= time.Monday && d.Weekday <= time.Friday
	default:
		return false
	}
}

// At is the instant the deadline of day ends.
func (d Deadline) At(day Date) time.Time {
	anchor := day.AddDays(-d.LeadDays)
	if d.Kind == DeadlinePreviousWeek {
		anchor = day.StartOfISOWeek().AddDays(-7 + int(d.Weekday) - int(time.Monday))
	}
	return CutoffAt(anchor, d.Clock)
}

// OrderingRules are a school's rules for registering, cancelling and
// ordering lunch. The zero AnnualDeadline means the school has not set one:
// the annual registration then stays open.
type OrderingRules struct {
	Model          RegistrationModel
	Registration   Deadline
	Cancellation   Deadline
	AnnualDeadline Date
	AnnualChanges  AnnualChanges
	Sick           SickMode
	Rhythm         Rhythm
	Detail         Detail
}

// DefaultOrderingRules are the rules of a school that changed nothing: one
// deadline on the meal day at the given clock time, as before #3873.
func DefaultOrderingRules(clock string) OrderingRules {
	deadline := Deadline{Kind: DeadlineDaysBefore, Clock: clock}
	return OrderingRules{
		Model: ModelSingle, Registration: deadline, Cancellation: deadline,
		AnnualChanges: AnnualParentsCancel, Sick: SickStaysOrdered, Rhythm: RhythmDaily, Detail: DetailByClass,
	}
}

// Valid reports whether every rule the model uses can be evaluated.
func (r OrderingRules) Valid() bool {
	switch r.Model {
	case ModelSingle:
		if !r.Registration.Valid() {
			return false
		}
	case ModelSplit:
		if !r.Registration.Valid() || !r.Cancellation.Valid() {
			return false
		}
	case ModelAnnual:
		if !r.Cancellation.Valid() || (r.AnnualChanges != AnnualParentsCancel && r.AnnualChanges != AnnualSchoolOnly) {
			return false
		}
	default:
		return false
	}
	return (r.Sick == SickStaysOrdered || r.Sick == SickCancelled) &&
		(r.Rhythm == RhythmDaily || r.Rhythm == RhythmWeekly) &&
		(r.Detail == DetailTotal || r.Detail == DetailByClass)
}

// RegistrationAt is when registering for day ends. Under the annual model a
// day follows the cancellation deadline while the annual registration is
// open; the annual deadline itself is checked separately.
func (r OrderingRules) RegistrationAt(day Date) time.Time {
	switch r.Model {
	case ModelAnnual:
		return r.Cancellation.At(day)
	default:
		return r.Registration.At(day)
	}
}

// CancellationAt is when cancelling day ends.
func (r OrderingRules) CancellationAt(day Date) time.Time {
	switch r.Model {
	case ModelSingle:
		return r.Registration.At(day)
	default:
		return r.Cancellation.At(day)
	}
}

// FreezeAt is when the order of day is fixed: the earlier of both
// deadlines. Every change after it reaches the caterer as a change to the
// order.
func (r OrderingRules) FreezeAt(day Date) time.Time {
	registration, cancellation := r.RegistrationAt(day), r.CancellationAt(day)
	if cancellation.Before(registration) {
		return cancellation
	}
	return registration
}

// AbsenceCutoff is the last instant an absence report still cancels the
// meal of day. With SickCancelled it is the end of the meal day.
func (r OrderingRules) AbsenceCutoff(day Date) time.Time {
	if r.Sick == SickCancelled {
		return day.EndOfDay()
	}
	return r.CancellationAt(day)
}

// annualClosed reports whether the annual registration has ended at now.
func (r OrderingRules) annualClosed(now time.Time) bool {
	return r.Model == ModelAnnual && r.AnnualDeadline != "" && now.After(r.AnnualDeadline.EndOfDay())
}

// ParentCanRegister reports whether a parent may still register for day.
func (r OrderingRules) ParentCanRegister(now time.Time, day Date) bool {
	return open(now, day, r.RegistrationAt(day)) && !r.annualClosed(now)
}

// ParentCanCancel reports whether a parent may still cancel day.
func (r OrderingRules) ParentCanCancel(now time.Time, day Date) bool {
	if r.annualClosed(now) && r.AnnualChanges == AnnualSchoolOnly {
		return false
	}
	return open(now, day, r.CancellationAt(day))
}

// ParentCanChangeSchedule reports whether a parent may still change the
// weekly pattern. Under the annual model that ends with the annual deadline.
func (r OrderingRules) ParentCanChangeSchedule(now time.Time) bool {
	return !r.annualClosed(now)
}

// NextEffectiveDate is the first weekday from today whose order is not fixed
// yet, the earliest day a new weekly pattern may change.
func (r OrderingRules) NextEffectiveDate(now time.Time) Date {
	date := calendar.DateFromTime(now)
	for range 2 * (maxLeadDays + 14) {
		if date.Weekday() != time.Saturday && date.Weekday() != time.Sunday && !now.After(r.FreezeAt(date)) {
			return date
		}
		date = date.AddDays(1)
	}
	return date
}

// open reports whether now is on or before deadline and day is not past.
func open(now time.Time, day Date, deadline time.Time) bool {
	return !day.Before(calendar.DateFromTime(now)) && !now.After(deadline)
}
