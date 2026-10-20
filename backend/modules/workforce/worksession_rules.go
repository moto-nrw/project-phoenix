package workforce

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

var validBalanceAdjustmentTypes = []string{
	BalanceAdjustmentTypePayout, BalanceAdjustmentTypeCompTime,
	BalanceAdjustmentTypeReset, BalanceAdjustmentTypeOpening,
}

const (
	minQuotaYear    = 2000
	maxQuotaYear    = 2100
	minQuotaDays    = 0
	maxQuotaDays    = 366
	openingDayScale = 10
)

func (ws WorkSession) Validate() error {
	if ws.StaffID <= 0 {
		return invalidWorkSession("staff ID is required")
	}
	if ws.CheckInTime.IsZero() {
		return invalidWorkSession("check-in time is required")
	}
	if ws.Status != WorkSessionStatusPresent && ws.Status != WorkSessionStatusHomeOffice {
		return invalidWorkSession("status must be 'present' or 'home_office'")
	}
	if ws.CheckOutTime != nil && ws.CheckInTime.After(*ws.CheckOutTime) {
		return invalidWorkSession("check-in time must be before check-out time")
	}
	if ws.BreakMinutes < 0 {
		return invalidWorkSession("break minutes cannot be negative")
	}
	if ws.CreatedBy <= 0 {
		return invalidWorkSession("created_by is required")
	}
	if err := validateWorkSessionCalendarDate(ws.Date, "date"); err != nil {
		return invalidWorkSession(err.Error())
	}
	return nil
}

var validWorkSessionOrderFields = []WorkSessionOrderField{
	WorkSessionOrderID, WorkSessionOrderStaffID, WorkSessionOrderDate, WorkSessionOrderCheckInTime,
}

func (f WorkSessionFilter) Validate() error {
	for _, pair := range []struct{ value, field string }{
		{f.Date, "date"}, {f.DateFrom, "date_from"}, {f.DateTo, "date_to"}, {f.DateBefore, "date_before"},
	} {
		if pair.value == "" {
			continue
		}
		if err := validateWorkSessionCalendarDate(pair.value, pair.field); err != nil {
			return invalidWorkSession(err.Error())
		}
	}
	for _, order := range f.Order {
		if !slices.Contains(validWorkSessionOrderFields, order.Field) {
			return invalidWorkSession("unsupported order field " + string(order.Field))
		}
	}
	if f.Limit < 0 || f.Offset < 0 {
		return invalidWorkSession("limit and offset must not be negative")
	}
	return nil
}

func (b WorkSessionBreak) Validate() error {
	if b.SessionID <= 0 {
		return invalidWorkSession("session ID is required")
	}
	if b.StartedAt.IsZero() {
		return invalidWorkSession("started_at is required")
	}
	if b.EndedAt != nil && b.StartedAt.After(*b.EndedAt) {
		return invalidWorkSession("started_at must be before ended_at")
	}
	if b.DurationMinutes < 0 {
		return invalidWorkSession("duration_minutes cannot be negative")
	}
	return nil
}

func (f WorkSessionBreakFilter) Validate() error {
	if f.Limit < 0 || f.Offset < 0 {
		return invalidWorkSession("limit and offset must not be negative")
	}
	return nil
}

func (a StaffBalanceAdjustment) Validate() error {
	if a.StaffID <= 0 {
		return invalidWorkSession("staff ID is required")
	}
	if !slices.Contains(validBalanceAdjustmentTypes, a.Type) {
		return invalidWorkSession("invalid balance adjustment type")
	}
	if a.EffectiveDate == "" {
		return invalidWorkSession("effective_date is required")
	}
	if err := validateWorkSessionCalendarDate(a.EffectiveDate, "effective_date"); err != nil {
		return invalidWorkSession(err.Error())
	}
	if a.DecidedBy <= 0 {
		return invalidWorkSession("decided_by is required")
	}
	return nil
}

func (f StaffBalanceAdjustmentFilter) Validate() error {
	for _, pair := range []struct{ value, field string }{
		{f.EffectiveFrom, "effective_from"}, {f.EffectiveTo, "effective_to"},
	} {
		if pair.value == "" {
			continue
		}
		if err := validateWorkSessionCalendarDate(pair.value, pair.field); err != nil {
			return invalidWorkSession(err.Error())
		}
	}
	if f.Limit < 0 || f.Offset < 0 {
		return invalidWorkSession("limit and offset must not be negative")
	}
	return nil
}

func hasOneDecimalPlace(value float64) bool {
	return math.Abs(value*openingDayScale-math.Round(value*openingDayScale)) < 1e-9
}

func (q StaffVacationQuota) Validate() error {
	if q.ChangedBy > 0 && strings.TrimSpace(q.ChangeReason) == "" {
		return invalidWorkSession("reason is required")
	}
	if q.StaffID <= 0 {
		return invalidWorkSession("staff_id is required")
	}
	if q.Year < minQuotaYear || q.Year > maxQuotaYear {
		return invalidWorkSession("year out of range")
	}
	if q.EntitledDays < minQuotaDays || q.EntitledDays > maxQuotaDays {
		return invalidWorkSession("entitled_days out of range")
	}
	if q.CarryoverDays < minQuotaDays || q.CarryoverDays > maxQuotaDays {
		return invalidWorkSession("carryover_days out of range")
	}
	if !hasOneDecimalPlace(q.EntitledDays) || !hasOneDecimalPlace(q.CarryoverDays) {
		return invalidWorkSession("vacation quota days must have at most one decimal place")
	}
	return nil
}

var validStaffVacationOrderFields = []StaffVacationOrderField{
	StaffVacationOrderID, StaffVacationOrderStaffID, StaffVacationOrderYear, StaffVacationOrderEntitledDays,
}

func (f StaffVacationFilter) Validate() error {
	for _, order := range f.Order {
		if !slices.Contains(validStaffVacationOrderFields, order.Field) {
			return invalidWorkSession("unsupported order field " + string(order.Field))
		}
	}
	if f.Limit < 0 || f.Offset < 0 {
		return invalidWorkSession("limit and offset must not be negative")
	}
	return nil
}

func validateWorkSessionCalendarDate(value, field string) error {
	if value == "" {
		return nil
	}
	parsed, err := calendar.ParseDate(value)
	if err != nil || parsed.String() != value {
		return &InvalidWorkTimeError{Reason: fmt.Sprintf("%s must be a %s date", field, DateLayout)}
	}
	return nil
}
