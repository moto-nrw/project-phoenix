package selection

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidSubmission              = errors.New("invalid submission")
	ErrCareOfferingClosed             = errors.New("one or more selected care offerings are not currently accepting applications")
	ErrCareOfferingUnavailable        = errors.New("one or more selected care offerings are not available for this child")
	ErrCareOfferingMissing            = errors.New("care offering selection is required for every child")
	ErrCareOfferingExactlyOneRequired = errors.New("exactly one care offering must be selected for every child")
	ErrRequiredCareOfferingMissing    = errors.New("a required care offering was not selected for every child")
	ErrCareOfferingRule               = fmt.Errorf("%w: care offering selection rule not satisfied", ErrInvalidSubmission)
	ErrSelectedDayNotAvailable        = fmt.Errorf("%w: selected day is not available for this offering", ErrInvalidSubmission)
	ErrDaySelectionRequired           = fmt.Errorf("%w: offering requires the parent to pick at least one day", ErrInvalidSubmission)
	ErrDaySelectionNotAllowed         = fmt.Errorf("%w: offering does not allow parent day selection (days_of_week_mode=fixed)", ErrInvalidSubmission)
)
