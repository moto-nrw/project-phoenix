package timetracking

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ---- month arithmetic ----------------------------------------------------

type monthKey struct {
	Year  int
	Month int
}

func monthOf(d calendar.Date) monthKey { return monthKey{Year: d.Year(), Month: int(d.Month())} }

func (k monthKey) firstDay() calendar.Date { return calendar.NewDate(k.Year, time.Month(k.Month), 1) }

// lastDay relies on NewDate's time.Date normalization for month 13.
func (k monthKey) lastDay() calendar.Date {
	return calendar.NewDate(k.Year, time.Month(k.Month)+1, 1).AddDays(-1)
}

// addMonths shifts a key by n whole months, normalizing via time.Date.
func (k monthKey) addMonths(n int) monthKey {
	return monthOf(calendar.NewDate(k.Year, time.Month(k.Month+n), 1))
}

func (k monthKey) next() monthKey {
	if k.Month == 12 {
		return monthKey{Year: k.Year + 1, Month: 1}
	}
	return monthKey{Year: k.Year, Month: k.Month + 1}
}

func (k monthKey) before(o monthKey) bool {
	return k.Year < o.Year || (k.Year == o.Year && k.Month < o.Month)
}

type monthAggregates struct {
	target           int
	targetToDate     int
	actual           int
	creditedSick     int
	creditedVacation int
	creditedTraining int
	creditedOther    int
	sickDays         float64
	vacationDays     float64
	trainingDays     float64
	hasShifts        bool
	plannedShift     int
	adjustment       int
	adjustments      []*StaffBalanceAdjustment
	// creditedByType splits the credit of the school's own absence types
	// (#4807): the DATEV export books a mapped type onto its wage type.
	creditedByType map[int64]AbsenceTypeCredit
}

// AbsenceTypeCredit is the credit of one school absence type in a month: the
// credited minutes, the absence days and the standard type whose category it
// is counted in.
type AbsenceTypeCredit struct {
	BaseType string
	Minutes  int
	Days     float64
}

func (a *monthAggregates) creditedTotal() int {
	return a.creditedSick + a.creditedVacation + a.creditedTraining + a.creditedOther
}

func (a *monthAggregates) balance() int {
	return a.actual + a.creditedTotal() + a.adjustment - a.targetToDate
}

// creditAbsenceDay books one credited absence day into its month's counters.
// The credit itself is already resolved by walkCreditedAbsenceDays; this only
// routes it to the Lohnart the month card reports.
func creditAbsenceDay(absence *StaffAbsence, credit int, fraction float64, agg *monthAggregates) {
	if agg == nil {
		return
	}
	if absence.AbsenceTypeID != nil && absence.AbsenceType != AbsenceTypeCompTime {
		if agg.creditedByType == nil {
			agg.creditedByType = make(map[int64]AbsenceTypeCredit)
		}
		entry := agg.creditedByType[*absence.AbsenceTypeID]
		entry.BaseType = absence.AbsenceType
		entry.Minutes += credit
		entry.Days += fraction
		agg.creditedByType[*absence.AbsenceTypeID] = entry
	}
	switch absence.AbsenceType {
	case AbsenceTypeSick:
		agg.creditedSick += credit
		agg.sickDays += fraction
	case AbsenceTypeVacation:
		agg.creditedVacation += credit
		agg.vacationDays += fraction
	case AbsenceTypeTraining:
		// Fortbildung split (#1417 2b writers): a Lohnart "Fortbildung"
		// needs hours OR days separately from "Sonstige" — same credit
		// math, own counters.
		agg.creditedTraining += credit
		agg.trainingDays += fraction
	case AbsenceTypeCompTime:
		// Nothing to credit (see walkCreditedAbsenceDays); the type still
		// needs its own case so it does not land in "Sonstige".
	default:
		agg.creditedOther += credit
	}
}

// effectiveAbsenceBoundaryHalves mirrors the absence service semantics: a
// bare half_day flag with no explicit boundary flags means both boundaries
// are half days.
func effectiveAbsenceBoundaryHalves(absence *StaffAbsence) (bool, bool) {
	if absence.HalfDay && !absence.StartHalfDay && !absence.EndHalfDay {
		return true, true
	}
	return absence.StartHalfDay, absence.EndHalfDay
}

func isHalfAbsenceDay(absence *StaffAbsence, d calendar.Date, startHalf, endHalf bool) bool {
	if absence.DateStart == absence.DateEnd {
		return d == absence.DateStart && (startHalf || endHalf)
	}
	return (d == absence.DateStart && startHalf) || (d == absence.DateEnd && endHalf)
}
