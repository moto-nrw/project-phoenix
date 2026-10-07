package domain

import (
	"errors"
	"fmt"
	"time"
)

// MaxTargetOverrideDays bounds one Sonderarbeitszeit range. Holiday care or a
// project phase spans weeks, not years; the bound also caps the day expansion
// every Soll read performs.
const MaxTargetOverrideDays = 366

var (
	ErrStaffTargetOverrideNotFound = errors.New("staff target override not found")
	ErrInvalidStaffTargetOverride  = errors.New("invalid staff target override")
	// ErrStaffTargetOverrideRejected marks a well-formed change the staff
	// member's time account cannot take: an overlapping range or a closed
	// month. The reason is caller-facing German text.
	ErrStaffTargetOverrideRejected = errors.New("staff target override rejected")
)

// Wire codes of a refused Sonderarbeitszeit (#2514). The domain may not
// import api/common, so each registered code is declared once here.
// workforce.month_closed is also declared as workforce.MonthClosedCode for
// the retained rebooking service: the domain package and the module root do
// not import each other, so neither can borrow the other's constant.
const (
	TargetOverrideRangeInvalidCode = "workforce.target_override_range_invalid"
	TargetOverrideTooLongCode      = "workforce.target_override_too_long"
	TargetOverrideHoursInvalidCode = "workforce.target_override_hours_invalid"
	TargetOverrideOverlapCode      = "workforce.target_override_overlap"
	TargetOverrideMonthClosedCode  = "workforce.month_closed"
)

// TargetOverrideValues are the values a coded refusal names; only the ones
// its code uses are set.
type TargetOverrideValues struct {
	MaxDays   int
	MaxHours  int
	StartDate string
	EndDate   string
	Month     string
}

// TargetOverrideError carries a caller-facing reason and unwraps to its kind.
// Code and Values name the refusal for the client when it has its own
// registered code; Reason stays the diagnostic text.
type TargetOverrideError struct {
	Kind   error
	Reason string
	Code   string
	Values TargetOverrideValues
}

func (e *TargetOverrideError) Error() string { return e.Reason }
func (e *TargetOverrideError) Unwrap() error { return e.Kind }

func invalidTargetOverride(reason string) error {
	return &TargetOverrideError{Kind: ErrInvalidStaffTargetOverride, Reason: reason}
}

// invalidTargetOverrideWithCode refuses input with a registered code and the
// values the reason names.
func invalidTargetOverrideWithCode(code string, values TargetOverrideValues, reason string) error {
	return &TargetOverrideError{Kind: ErrInvalidStaffTargetOverride, Reason: reason, Code: code, Values: values}
}

func rejectedTargetOverride(code string, values TargetOverrideValues, format string, args ...any) error {
	return &TargetOverrideError{
		Kind: ErrStaffTargetOverrideRejected, Reason: fmt.Sprintf(format, args...), Code: code, Values: values,
	}
}

// TargetOverrideWeekdays is the number of per-weekday targets a
// Sonderarbeitszeit carries: Monday to Friday, index 0 is Monday.
const TargetOverrideWeekdays = 5

// StaffTargetOverride is a Sonderarbeitszeit: for every Monday to Friday in
// [StartDate, EndDate] the staff member's daily target is DailyMinutes,
// whatever the schedule or a closure day would say. With WeekdayMinutes set
// (#3745) each weekday has its own target instead and DailyMinutes is zero.
// Statutory holidays stay at zero. Dates are calendar days in DateLayout,
// EndDate is inclusive.
type StaffTargetOverride struct {
	ID             int64
	TenantID       int64
	StaffID        int64
	StartDate      string
	EndDate        string
	DailyMinutes   int
	WeekdayMinutes []int
	CreatedBy      *int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// MinutesOn returns the target the range sets on a Monday to Friday.
func (o StaffTargetOverride) MinutesOn(weekday time.Weekday) int {
	if o.WeekdayMinutes == nil {
		return o.DailyMinutes
	}
	index := int(weekday) - int(time.Monday)
	if index < 0 || index >= len(o.WeekdayMinutes) {
		return 0
	}
	return o.WeekdayMinutes[index]
}

// StaffTargetOverrideFields is the writable part of a Sonderarbeitszeit.
// WeekdayMinutes, when set, holds Monday to Friday and replaces DailyMinutes.
type StaffTargetOverrideFields struct {
	StartDate      string
	EndDate        string
	DailyMinutes   int
	WeekdayMinutes []int
}

// TargetOverrideQuery selects the Sonderarbeitszeiten of the given staff
// members. A non-empty From/To narrows to ranges touching [From, To]; Days
// asks for the expansion into effective days as well.
type TargetOverrideQuery struct {
	StaffIDs []int64
	From     string
	To       string
	Days     bool
}

// TargetOverrideDays maps staff member and calendar day to the daily target a
// Sonderarbeitszeit sets on that day.
type TargetOverrideDays map[int64]map[string]int

// ValidateStaffTargetOverrideFields checks the shape of one range.
func ValidateStaffTargetOverrideFields(fields StaffTargetOverrideFields) error {
	start, err := time.Parse(DateLayout, fields.StartDate)
	if err != nil || start.Format(DateLayout) != fields.StartDate {
		return invalidTargetOverride("start_date must be a " + DateLayout + " date")
	}
	end, err := time.Parse(DateLayout, fields.EndDate)
	if err != nil || end.Format(DateLayout) != fields.EndDate {
		return invalidTargetOverride("end_date must be a " + DateLayout + " date")
	}
	if end.Before(start) {
		return invalidTargetOverrideWithCode(TargetOverrideRangeInvalidCode, TargetOverrideValues{}, "Das Ende liegt vor dem Anfang. Bitte prüfen Sie den Zeitraum.")
	}
	if int(end.Sub(start).Hours()/24)+1 > MaxTargetOverrideDays {
		return invalidTargetOverrideWithCode(TargetOverrideTooLongCode, TargetOverrideValues{MaxDays: MaxTargetOverrideDays},
			fmt.Sprintf("Der Zeitraum ist zu lang. Erlaubt sind höchstens %d Tage.", MaxTargetOverrideDays))
	}
	return validateTargetOverrideMinutes(fields)
}

func validateTargetOverrideMinutes(fields StaffTargetOverrideFields) error {
	outOfRange := invalidTargetOverrideWithCode(TargetOverrideHoursInvalidCode, TargetOverrideValues{MaxHours: MaxDailyMinutes / 60},
		fmt.Sprintf("Die Stunden pro Tag müssen zwischen 0 und %d liegen.", MaxDailyMinutes/60))
	if fields.WeekdayMinutes == nil {
		if fields.DailyMinutes < 0 || fields.DailyMinutes > MaxDailyMinutes {
			return outOfRange
		}
		return nil
	}
	if len(fields.WeekdayMinutes) != TargetOverrideWeekdays {
		return invalidTargetOverride("Bitte geben Sie für Montag bis Freitag jeweils Stunden ein.")
	}
	if fields.DailyMinutes != 0 {
		return invalidTargetOverride("Bitte geben Sie entweder Stunden pro Tag oder Stunden für die Wochentage ein.")
	}
	for _, minutes := range fields.WeekdayMinutes {
		if minutes < 0 || minutes > MaxDailyMinutes {
			return outOfRange
		}
	}
	return nil
}

// RejectTargetOverrideOverlap refuses a range that shares a day with another
// Sonderarbeitszeit of the same staff member.
func RejectTargetOverrideOverlap(existing []StaffTargetOverride, fields StaffTargetOverrideFields) error {
	for _, other := range existing {
		if other.StartDate <= fields.EndDate && fields.StartDate <= other.EndDate {
			start, end := germanDate(other.StartDate), germanDate(other.EndDate)
			return rejectedTargetOverride(
				TargetOverrideOverlapCode, TargetOverrideValues{StartDate: start, EndDate: end},
				"Für diesen Zeitraum gibt es schon eine Sonderarbeitszeit (%s bis %s). Löschen Sie diese zuerst oder wählen Sie andere Tage.",
				start, end,
			)
		}
	}
	return nil
}

// RejectClosedTargetOverrideMonths refuses a range touching a closed month:
// its closing balance is frozen, so the Stundenkonto could not follow the new
// target.
func RejectClosedTargetOverrideMonths(closed []*StaffMonthBalanceSnapshot, fields StaffTargetOverrideFields) error {
	for _, snapshot := range closed {
		if snapshot == nil || snapshot.ReopenedAt != nil {
			continue
		}
		month := fmt.Sprintf("%04d-%02d", snapshot.Year, snapshot.Month)
		if fields.StartDate[:7] <= month && month <= fields.EndDate[:7] {
			name := fmt.Sprintf("%s %d", germanMonthNames[snapshot.Month-1], snapshot.Year)
			return rejectedTargetOverride(
				TargetOverrideMonthClosedCode, TargetOverrideValues{Month: name},
				"Der %s ist abgeschlossen. Öffnen Sie den Monat zuerst wieder. Das geht im Reiter Zeiterfassung.",
				name,
			)
		}
	}
	return nil
}

// ExpandTargetOverrides turns the ranges into the days they set a target on,
// clamped to [from, to]: Monday to Friday only, never a statutory holiday.
// Ranges of one staff member never overlap, so each day has one value.
func ExpandTargetOverrides(overrides []StaffTargetOverride, from, to string, holidays map[string]bool) TargetOverrideDays {
	result := make(TargetOverrideDays)
	for _, override := range overrides {
		start := max(override.StartDate, from)
		end := min(override.EndDate, to)
		day, err := time.Parse(DateLayout, start)
		if err != nil {
			continue
		}
		for key := start; key <= end; key = day.Format(DateLayout) {
			if weekday := day.Weekday(); weekday != time.Saturday && weekday != time.Sunday && !holidays[key] {
				if result[override.StaffID] == nil {
					result[override.StaffID] = make(map[string]int)
				}
				result[override.StaffID][key] = override.MinutesOn(weekday)
			}
			day = day.AddDate(0, 0, 1)
		}
	}
	return result
}

var germanMonthNames = [...]string{
	"Januar", "Februar", "März", "April", "Mai", "Juni",
	"Juli", "August", "September", "Oktober", "November", "Dezember",
}

// germanDate renders a validated DateLayout day as DD.MM.YYYY.
func germanDate(value string) string {
	parsed, err := time.Parse(DateLayout, value)
	if err != nil {
		return value
	}
	return parsed.Format("02.01.2006")
}
