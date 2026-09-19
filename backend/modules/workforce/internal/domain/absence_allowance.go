package domain

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
)

type InvalidAbsenceAllowanceError struct{ Reason string }

func (e *InvalidAbsenceAllowanceError) Error() string { return e.Reason }
func (e *InvalidAbsenceAllowanceError) Unwrap() error { return ErrAbsenceTypeAllowanceInvalid }

type SetAbsenceTypeAllowance struct {
	StaffID, AbsenceTypeID int64
	Year                   int
	EntitledDays           float64
	Reason                 string
	ChangedBy              int64
}

func (input SetAbsenceTypeAllowance) Validate() error {
	var reason string
	switch {
	case input.StaffID <= 0:
		reason = "staff_id is required"
	case input.AbsenceTypeID <= 0:
		reason = "absence_type_id is required"
	case input.Year < 2000 || input.Year > 2100:
		reason = "year out of range"
	case input.EntitledDays < 0 || input.EntitledDays > 366:
		reason = "entitled_days out of range"
	case math.Abs(input.EntitledDays*2-math.Round(input.EntitledDays*2)) > 0.000001:
		reason = "entitled_days must use whole or half days"
	case strings.TrimSpace(input.Reason) == "" || input.ChangedBy <= 0:
		reason = "Begründung und ändernde Person sind erforderlich"
	default:
		return nil
	}
	return &InvalidAbsenceAllowanceError{Reason: reason}
}

type AbsenceTypeAllowanceSummary struct {
	StaffID, AbsenceTypeID                               int64
	Year                                                 int
	EntitledDays, TakenDays, ReservedDays, RemainingDays float64
	// ExpiresOn is the last calendar day the rest of this year can be used:
	// 31.12. of the year, or the type's carryover day in the following year.
	ExpiresOn string
	// ExpiredDays is the rest that was left when ExpiresOn passed. It is
	// shown, not dropped, and RemainingDays is zero from then on (#3257).
	ExpiredDays float64
	// BookingDays is what a previewed booking takes from this year.
	BookingDays float64
	// CarriedIn is the previous year's rest that is still usable in this
	// year; nil when the type does not carry a rest over.
	CarriedIn *AbsenceTypeAllowanceCarry
}

// AbsenceTypeAllowanceCarry is the previous year's account as seen from the
// year it is carried into.
type AbsenceTypeAllowanceCarry struct {
	Year                       int
	RemainingDays, ExpiredDays float64
	ExpiresOn                  string
}

var daysInMonth = [...]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}

// ValidateCarryoverUntil accepts "" (the rest expires on 31.12.) or a
// "MM-DD" day that exists in every year, so 29 February is rejected.
func ValidateCarryoverUntil(value string) error {
	if value == "" {
		return nil
	}
	invalid := invalidAbsenceType("ungültiges Verfallsdatum, erwartet wird Monat und Tag (MM-TT)")
	if len(value) != 5 || value[2] != '-' {
		return invalid
	}
	month, monthErr := strconv.Atoi(value[:2])
	day, dayErr := strconv.Atoi(value[3:])
	if monthErr != nil || dayErr != nil || month < 1 || month > 12 || day < 1 || day > daysInMonth[month-1] {
		return invalid
	}
	return nil
}

// AllowanceExpiresOn is the last day the rest of year can be booked.
func AllowanceExpiresOn(year int, carryoverUntil string) string {
	if carryoverUntil == "" {
		return fmt.Sprintf("%04d-12-31", year)
	}
	return fmt.Sprintf("%04d-%s", year+1, carryoverUntil)
}

// AllowanceUse is what one booking spends on one weekday. Days before the
// previous year's expiry may draw on that year's rest, but only when the
// booking was entered by then: an expired rest cannot be booked any more.
type AllowanceUse struct {
	AbsenceID int64
	Day       string
	Days      float64
	Pending   bool
	EnteredOn string
}

// AllowanceLedger distributes the bookings of one person and one type over
// the yearly accounts. Days of a year go to that year's account; days inside
// the previous year's carryover window first use up that older rest, which
// would otherwise expire (#3257). Bookings are applied in calendar order, so
// the result does not depend on the order the rows were entered in.
type AllowanceLedger struct {
	carryoverUntil string
	today          string
	entitled       map[int]float64
	taken          map[int]float64
	reserved       map[int]float64
	booked         map[int64]map[int]float64
}

func BuildAllowanceLedger(entitlements map[int]float64, carryoverUntil, today string, uses []AllowanceUse) AllowanceLedger {
	ledger := AllowanceLedger{
		carryoverUntil: carryoverUntil, today: today, entitled: entitlements,
		taken: map[int]float64{}, reserved: map[int]float64{}, booked: map[int64]map[int]float64{},
	}
	ordered := slices.Clone(uses)
	slices.SortStableFunc(ordered, func(a, b AllowanceUse) int {
		if c := strings.Compare(a.Day, b.Day); c != 0 {
			return c
		}
		if a.Pending != b.Pending {
			if a.Pending {
				return 1
			}
			return -1
		}
		return compareInt64(a.AbsenceID, b.AbsenceID)
	})
	for _, use := range ordered {
		year, err := strconv.Atoi(use.Day[:4])
		if err != nil || use.Days <= 0 {
			continue
		}
		open := use.Days
		if previous := year - 1; carryoverUntil != "" {
			expiresOn := AllowanceExpiresOn(previous, carryoverUntil)
			if use.Day <= expiresOn && use.EnteredOn <= expiresOn {
				if drawn := min(open, max(0, ledger.rawRemaining(previous))); drawn > 0 {
					ledger.book(use, previous, drawn)
					open -= drawn
				}
			}
		}
		if open > 0 {
			ledger.book(use, year, open)
		}
	}
	return ledger
}

func compareInt64(a, b int64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

func (l AllowanceLedger) book(use AllowanceUse, year int, days float64) {
	if use.Pending {
		l.reserved[year] += days
	} else {
		l.taken[year] += days
	}
	if l.booked[use.AbsenceID] == nil {
		l.booked[use.AbsenceID] = map[int]float64{}
	}
	l.booked[use.AbsenceID][year] += days
}

func (l AllowanceLedger) rawRemaining(year int) float64 {
	return l.entitled[year] - l.taken[year] - l.reserved[year]
}

// Booked is what the booking with absenceID takes from year.
func (l AllowanceLedger) Booked(absenceID int64, year int) float64 {
	return l.booked[absenceID][year]
}

// BookedYears lists the accounts the booking with absenceID draws on.
func (l AllowanceLedger) BookedYears(absenceID int64) []int {
	years := make([]int, 0, len(l.booked[absenceID]))
	for year, days := range l.booked[absenceID] {
		if days > 0 {
			years = append(years, year)
		}
	}
	slices.Sort(years)
	return years
}

// Overdrawn reports every year whose account ends below zero and lower than
// in before, i.e. the years a change pushes into the negative.
func (l AllowanceLedger) Overdrawn(before AllowanceLedger) []int {
	years := []int{}
	seen := map[int]bool{}
	for _, source := range []map[int]float64{l.entitled, l.taken, l.reserved} {
		for year := range source {
			if seen[year] {
				continue
			}
			seen[year] = true
			if after := l.rawRemaining(year); after < 0 && after < before.rawRemaining(year) {
				years = append(years, year)
			}
		}
	}
	slices.Sort(years)
	return years
}

// Summary is the account of year as the Leitung reads it. A positive rest
// past its expiry is reported as expired instead of remaining.
func (l AllowanceLedger) Summary(staffID, absenceTypeID int64, year int) AbsenceTypeAllowanceSummary {
	summary := l.account(staffID, absenceTypeID, year)
	if l.carryoverUntil != "" {
		previous := l.account(staffID, absenceTypeID, year-1)
		if previous.EntitledDays > 0 {
			summary.CarriedIn = &AbsenceTypeAllowanceCarry{
				Year: previous.Year, RemainingDays: previous.RemainingDays,
				ExpiredDays: previous.ExpiredDays, ExpiresOn: previous.ExpiresOn,
			}
		}
	}
	return summary
}

func (l AllowanceLedger) account(staffID, absenceTypeID int64, year int) AbsenceTypeAllowanceSummary {
	summary := AbsenceTypeAllowanceSummary{
		StaffID: staffID, AbsenceTypeID: absenceTypeID, Year: year,
		EntitledDays: l.entitled[year], TakenDays: l.taken[year], ReservedDays: l.reserved[year],
		RemainingDays: l.rawRemaining(year), ExpiresOn: AllowanceExpiresOn(year, l.carryoverUntil),
	}
	if l.today > summary.ExpiresOn && summary.RemainingDays > 0 {
		summary.ExpiredDays, summary.RemainingDays = summary.RemainingDays, 0
	}
	return summary
}
