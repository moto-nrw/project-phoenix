package timetable

import "errors"

// Wire codes of timetable refusals raised below the HTTP adapter (#2516). The
// owner may not import api/common, so each registered code it raises is
// declared once here; the adapter reads it through CodedError, and a test in
// the adapter pins every constant to the generated registry.
const (
	CodeOfferingSourceInvalid       = "timetable.offering_source_invalid"
	CodeOfferingSourceTooMany       = "timetable.offering_source_too_many"
	CodeOfferingSourceNotFound      = "timetable.offering_source_not_found"
	CodeOfferingSourceInactive      = "timetable.offering_source_inactive"
	CodeOfferingSourceMixedPhases   = "timetable.offering_source_mixed_phases"
	CodeOfferingSourceOutsidePeriod = "timetable.offering_source_outside_period"
	CodeTemplateGradeAboveMax       = "timetable.template_grade_above_max"
	CodeSeriesEndBeforeStart        = "timetable.series_end_before_start"
	CodeSeriesEndOutsidePeriod      = "timetable.series_end_outside_period"
	CodeTemplateSplitInPast         = "timetable.template_split_in_past"
	CodeOperationStale              = "timetable.operation_stale"
	CodeInstanceNotActive           = "timetable.instance_not_active"
	CodeAttendanceFrozen            = "timetable.attendance_frozen"
	CodeReopenStudentActive         = "timetable.reopen_student_active"
	CodeReopenAttendanceChanged     = "timetable.reopen_attendance_changed"
	CodeReopenSupervisionChanged    = "timetable.reopen_supervision_changed"
	CodeReopenStaffBusy             = "timetable.reopen_staff_busy"
	CodeGuardianNoticeTextInvalid   = "timetable.guardian_notice_text_invalid"
	CodeGuardianNoticePast          = "timetable.guardian_notice_past"
	CodeStudentNotCheckedIn         = "timetable.student_not_checked_in"
	CodeWindowEndBeforeStart        = "timetable.window_end_before_start"
	CodeWindowTooLarge              = "timetable.window_too_large"
	CodeInstanceWeekend             = "timetable.instance_weekend"
)

// CodedError is a refused timetable request that names its registered wire
// code and the values its reason names (#2516). Err keeps the diagnostic text
// and the sentinel chain callers classify with errors.Is; Values travel as the
// response details, so no client reads the sentence. It satisfies the shared
// business-rejection contract of api/common: an adapter that does not
// classify it yet still answers with its code instead of a server error.
type CodedError struct {
	Err    error
	Code   string
	Values RefusalValues
}

// RefusalValues are the values a refusal names (#2516). Dates travel as
// YYYY-MM-DD; unset values stay off the wire.
// RefusalDateLayout formats the days a refusal names. The catalog text shows
// them as they are, so they travel in the German form (like workforce
// rebooking refusals), not as YYYY-MM-DD.
const RefusalDateLayout = "02.01.2006"

type RefusalValues struct {
	Date     string `json:"date,omitempty"`
	Start    string `json:"start,omitempty"`
	End      string `json:"end,omitempty"`
	Max      int    `json:"max,omitempty"`
	Given    int    `json:"given,omitempty"`
	MaxDays  int    `json:"max_days,omitempty"`
	MaxWeeks int    `json:"max_weeks,omitempty"`
	Grade    int    `json:"grade,omitempty"`
	Offering string `json:"offering,omitempty"`
}

// IsZero reports whether the refusal names no value.
func (v RefusalValues) IsZero() bool { return v == RefusalValues{} }

func (e *CodedError) Error() string { return e.Err.Error() }

func (e *CodedError) Unwrap() error { return e.Err }

// ErrorCode is the registered wire code of the refusal.
func (e *CodedError) ErrorCode() string { return e.Code }

// ErrorDetails are the values the refusal names; nil when it names none.
func (e *CodedError) ErrorDetails() any {
	if e.Values.IsZero() {
		return nil
	}
	return e.Values
}

// WithCode attaches a registered wire code and the values the refusal names
// to err without changing its text or its sentinel chain.
func WithCode(err error, code string, values ...RefusalValues) error {
	coded := &CodedError{Err: err, Code: code}
	if len(values) > 0 {
		coded.Values = values[0]
	}
	return coded
}

// AsCoded returns the outermost CodedError in err's chain.
func AsCoded(err error) (*CodedError, bool) {
	var coded *CodedError
	if errors.As(err, &coded) {
		return coded, true
	}
	return nil, false
}
