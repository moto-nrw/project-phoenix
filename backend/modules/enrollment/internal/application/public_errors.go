package application

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/selection"
)

// The HTTP adapters classify refusals by Enrollment's public values, but some
// originate in the selection contract or in Care Plan, which the adapters do
// not reach. publicError marks such an error with the Enrollment value as
// well: errors.Is then matches both, and the text stays the originating one.

// publicErrorAliases pairs an originating value with its Enrollment value.
// The more specific selection values come first, so their Enrollment values
// carry the broad ErrInvalidSubmission themselves.
var publicErrorAliases = []struct {
	origin error
	public error
}{
	{selection.ErrCareOfferingRule, enrollment.ErrCareOfferingRule},
	{selection.ErrSelectedDayNotAvailable, enrollment.ErrSelectedDayNotAvailable},
	{selection.ErrDaySelectionRequired, enrollment.ErrDaySelectionRequired},
	{selection.ErrDaySelectionNotAllowed, enrollment.ErrDaySelectionNotAllowed},
	{selection.ErrInvalidSubmission, enrollment.ErrInvalidSubmission},
	{selection.ErrCareOfferingClosed, enrollment.ErrCareOfferingClosed},
	{selection.ErrCareOfferingUnavailable, enrollment.ErrCareOfferingUnavailable},
	{selection.ErrCareOfferingMissing, enrollment.ErrCareOfferingMissing},
	{selection.ErrCareOfferingExactlyOneRequired, enrollment.ErrCareOfferingExactlyOneRequired},
	{selection.ErrRequiredCareOfferingMissing, enrollment.ErrRequiredCareOfferingMissing},
	{careplan.ErrCareOfferingsDisabled, enrollment.ErrCareOfferingsDisabled},
	{careplan.ErrOfferingAdjustmentInvalid, enrollment.ErrOfferingAdjustmentInvalid},
	{careplan.ErrCompleteWithdrawalConfirmationRequired, enrollment.ErrCompleteWithdrawalConfirmationRequired},
	{careplan.ErrBookingRequestNotFound, enrollment.ErrDecisionRequestNotFound},
	{careplan.ErrBookingChildNotFound, enrollment.ErrDecisionChildNotFound},
	{careplan.ErrCareOfferingNotFound, enrollment.ErrCareOfferingNotFound},
	{careplan.ErrCareOfferingConfigInvalid, enrollment.ErrCareOfferingInvalid},
	{careplan.ErrCareOfferingTemplatePeriodMismatch, enrollment.ErrCareOfferingTemplatePeriodMismatch},
	{careplan.ErrCareOfferingGroupRuleConflict, enrollment.ErrCareOfferingGroupRuleConflict},
	{careplan.ErrCareOfferingDaysRequired, enrollment.ErrCareOfferingDaysRequired},
	{careplan.ErrCareOfferingPickupTimesRequired, enrollment.ErrCareOfferingPickupTimesRequired},
}

// publicError returns err marked with the Enrollment value of every
// originating value it carries. nil stays nil; an error without one is
// returned unchanged.
func publicError(err error) error {
	if err == nil {
		return nil
	}
	var marks []error
	for _, alias := range publicErrorAliases {
		if errors.Is(err, alias.origin) && !errors.Is(err, alias.public) && !markedBy(marks, alias.public) {
			marks = append(marks, alias.public)
		}
	}
	if len(marks) == 0 {
		return err
	}
	return &publicMarkedError{error: err, marks: marks}
}

func markedBy(marks []error, target error) bool {
	for _, mark := range marks {
		if errors.Is(mark, target) {
			return true
		}
	}
	return false
}

// publicMarkedError keeps the originating error's text and chain and adds the
// Enrollment values to errors.Is.
type publicMarkedError struct {
	error
	marks []error
}

func (e *publicMarkedError) Unwrap() []error {
	return append([]error{e.error}, e.marks...)
}

// PublicError marks err with Enrollment's public values for a caller outside
// the application, such as the composition's decision contract.
func PublicError(err error) error { return publicError(err) }
