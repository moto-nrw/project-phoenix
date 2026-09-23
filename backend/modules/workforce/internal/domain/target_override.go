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

// TargetOverrideError carries a caller-facing reason and unwraps to its kind.
type TargetOverrideError struct {
	Kind   error
	Reason string
}

func (e *TargetOverrideError) Error() string { return e.Reason }
func (e *TargetOverrideError) Unwrap() error { return e.Kind }

func invalidTargetOverride(reason string) error {
	return &TargetOverrideError{Kind: ErrInvalidStaffTargetOverride, Reason: reason}
}

func rejectedTargetOverride(format string, args ...any) error {
	return &TargetOverrideError{Kind: ErrStaffTargetOverrideRejected, Reason: fmt.Sprintf(format, args...)}
}

// StaffTargetOverride is a Sonderarbeitszeit: for every Monday to Friday in
// [StartDate, EndDate] the staff member's daily target is DailyMinutes,
// whatever the schedule or a closure day would say. Statutory holidays stay
// at zero. Dates are calendar days in DateLayout, EndDate is inclusive.
type StaffTargetOverride struct {
	ID           int64
	TenantID     int64
	StaffID      int64
	StartDate    string
	EndDate      string
	DailyMinutes int
	CreatedBy    *int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// StaffTargetOverrideFields is the writable part of a Sonderarbeitszeit.
type StaffTargetOverrideFields struct {
	StartDate    string
	EndDate      string
	DailyMinutes int
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
		return invalidTargetOverride("Das Ende liegt vor dem Anfang. Bitte prüfen Sie den Zeitraum.")
	}
	if int(end.Sub(start).Hours()/24)+1 > MaxTargetOverrideDays {
		return invalidTargetOverride(fmt.Sprintf("Der Zeitraum ist zu lang. Erlaubt sind höchstens %d Tage.", MaxTargetOverrideDays))
	}
	if fields.DailyMinutes < 0 || fields.DailyMinutes > MaxDailyMinutes {
		return invalidTargetOverride(fmt.Sprintf("Die Stunden pro Tag müssen zwischen 0 und %d liegen.", MaxDailyMinutes/60))
	}
	return nil
}

// RejectTargetOverrideOverlap refuses a range that shares a day with another
// Sonderarbeitszeit of the same staff member.
func RejectTargetOverrideOverlap(existing []StaffTargetOverride, fields StaffTargetOverrideFields) error {
	for _, other := range existing {
		if other.StartDate <= fields.EndDate && fields.StartDate <= other.EndDate {
			return rejectedTargetOverride(
				"Für diesen Zeitraum gibt es schon eine Sonderarbeitszeit (%s bis %s). Löschen Sie diese zuerst oder wählen Sie andere Tage.",
				germanDate(other.StartDate), germanDate(other.EndDate),
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
			return rejectedTargetOverride(
				"Der %s %d ist abgeschlossen. Öffnen Sie den Monat zuerst wieder. Das geht im Reiter Zeiterfassung.",
				germanMonthNames[snapshot.Month-1], snapshot.Year,
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
				result[override.StaffID][key] = override.DailyMinutes
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
