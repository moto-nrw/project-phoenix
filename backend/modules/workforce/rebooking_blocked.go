package workforce

import "fmt"

// Wire codes of a blocked absence rebooking (#3258, #2514). The retained
// service may not import api/common, so each registered code is declared once
// here; the HTTP adapter reads it through the business rejection contract.
const (
	RebookingIntoSickReportCode        = "workforce.rebooking_into_sick_report"
	RebookingNothingSelectedCode       = "workforce.rebooking_nothing_selected"
	RebookingTooManyCode               = "workforce.rebooking_too_many"
	RebookingReasonRequiredCode        = "workforce.rebooking_reason_required"
	RebookingSickReportCode            = "workforce.rebooking_sick_report"
	RebookingRequestCode               = "workforce.rebooking_request"
	RebookingSameTypeCode              = "workforce.rebooking_same_type"
	RebookingNotOverCode               = "workforce.rebooking_not_over"
	RebookingHalfDayEdgeCode           = "workforce.rebooking_half_day_edge"
	RebookingOutsideBalanceCode        = "workforce.rebooking_outside_balance"
	RebookingNoWorkingDayCode          = "workforce.rebooking_no_working_day"
	RebookingBeforeVacationOpeningCode = "workforce.rebooking_before_vacation_opening"
	RebookingOverlapCode               = "workforce.rebooking_overlap"
	MonthClosedCode                    = "workforce.month_closed"
)

// RebookingBlockedError is a business rejection of an absence rebooking: its
// registered code and the values the reason names travel to the client as
// code and details, so no client reads the German reason (#2514). The reason
// stays the diagnostic text. It reports itself as ErrAbsenceRebookingBlocked.
type RebookingBlockedError struct {
	Reason string
	Code   string
	Values RebookingValues
}

// RebookingValues are the values a refusal names; unset ones stay off the
// wire.
type RebookingValues struct {
	Day   string `json:"day,omitempty"`
	Month string `json:"month,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

func (e RebookingBlockedError) Error() string { return e.Reason }

func (e RebookingBlockedError) Is(target error) bool { return target == ErrAbsenceRebookingBlocked }

// ErrorCode is the registered wire code of the refusal.
func (e RebookingBlockedError) ErrorCode() string { return e.Code }

// ErrorDetails are the values the refusal names; nil when it names none.
func (e RebookingBlockedError) ErrorDetails() any {
	if e.Values == (RebookingValues{}) {
		return nil
	}
	return e.Values
}

// RebookingBlocked refuses a rebooking with code; values carries what the
// reason names, which the client interpolates into its own text.
func RebookingBlocked(code string, values RebookingValues, format string, args ...any) error {
	return RebookingBlockedError{Reason: fmt.Sprintf(format, args...), Code: code, Values: values}
}
