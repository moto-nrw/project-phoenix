package enrollment

import (
	"errors"
	"fmt"
)

// InvalidInputError is a rejected input value (#2515): Code is its
// registered wire code, Field the JSON key path of the value ("" when the
// request as a whole is wrong). The HTTP adapter answers 400 with both
// through common.InputRejection. Err keeps the diagnostic text, so Error()
// stays byte-identical to the plain error the check returned before.
type InvalidInputError struct {
	Code  string
	Field string
	Err   error
}

func (e *InvalidInputError) Error() string { return e.Err.Error() }

func (e *InvalidInputError) Unwrap() error { return e.Err }

// ErrorCode is the registered wire code of the rejection.
func (e *InvalidInputError) ErrorCode() string { return e.Code }

// ErrorField is the JSON key path of the rejected value.
func (e *InvalidInputError) ErrorField() string { return e.Field }

// invalidInput marks err as the rejected value at field with code.
func invalidInput(code, field string, err error) error {
	return &InvalidInputError{Code: code, Field: field, Err: err}
}

// invalidInputf is invalidInput with a formatted diagnostic text.
func invalidInputf(code, field, format string, args ...any) error {
	return invalidInput(code, field, fmt.Errorf(format, args...))
}

// InvalidInput marks err as the rejected value at field with code, for
// checks outside this package (application services, HTTP adapters).
func InvalidInput(code, field string, err error) error {
	return invalidInput(code, field, err)
}

// NestInput adds context to err like fmt.Errorf("<context>: %w") and, when
// err carries an InvalidInputError, prefixes its field with prefix, so a
// check of one list entry names its place in the request
// ("fields.3" + "key" = "fields.3.key").
func NestInput(err error, prefix, format string, args ...any) error {
	wrapped := fmt.Errorf(format+": %w", append(args, err)...)
	var rejection *InvalidInputError
	if !errors.As(err, &rejection) {
		return wrapped
	}
	field := prefix
	if rejection.Field != "" {
		field = prefix + "." + rejection.Field
	}
	return &InvalidInputError{Code: rejection.Code, Field: field, Err: wrapped}
}

// Wire codes of rejected input values (#2515). The package may not import
// api/common, so each registered code is declared once here.
const (
	CodePhaseNameRequired         = "enrollment.phase_name_required"
	CodePhaseServicePeriodInvalid = "enrollment.phase_service_period_invalid"
	CodePhaseWindowInvalid        = "enrollment.phase_window_invalid"
	CodePhaseSchoolClassesInvalid = "enrollment.phase_school_classes_invalid"
	// CodePhaseEligibilitySettingRequired: a class or grade restriction
	// needs the matching Klassen-/Klassenstufen-Abfrage switched on.
	CodePhaseEligibilitySettingRequired = "enrollment.phase_eligibility_setting_required"

	CodeSchemaNameRequired           = "enrollment.schema_name_required"
	CodeFormGradeCollectionRequired  = "enrollment.form_grade_collection_required"
	CodeFormFieldKeyInvalid          = "enrollment.form_field_key_invalid"
	CodeFormFieldKeyDuplicate        = "enrollment.form_field_key_duplicate"
	CodeFormFieldLabelRequired       = "enrollment.form_field_label_required"
	CodeFormFieldContentRequired     = "enrollment.form_field_content_required"
	CodeFormFieldOptionsInvalid      = "enrollment.form_field_options_invalid"
	CodeFormFieldAllowedTimesInvalid = "enrollment.form_field_allowed_times_invalid"
	CodeFormFieldTargetInvalid       = "enrollment.form_field_target_invalid"
	CodeFormFieldTargetDuplicate     = "enrollment.form_field_target_duplicate"
	CodeFormFieldVisibilityInvalid   = "enrollment.form_field_visibility_invalid"
	CodeLegalBlockKeyInvalid         = "enrollment.legal_block_key_invalid"
	CodeLegalBlockKeyDuplicate       = "enrollment.legal_block_key_duplicate"
	CodeLegalBlockIncomplete         = "enrollment.legal_block_incomplete"
	CodeLegalBlockRequiredNotAllowed = "enrollment.legal_block_required_not_allowed"

	CodeConsentRequired       = "enrollment.consent_required"
	CodeChildRequired         = "enrollment.child_required"
	CodeChildNameRequired     = "enrollment.child_name_required"
	CodeChildBirthDateInvalid = "enrollment.child_birth_date_invalid"
	CodeChildGradeRequired    = "enrollment.child_grade_required"
	CodeGuardianNameRequired  = "enrollment.guardian_name_required"
	CodeGuardianEmailRequired = "enrollment.guardian_email_required"
	CodeGuardianPhoneRequired = "enrollment.guardian_phone_required"
	CodeInvalidEmail          = "enrollment.invalid_email"
	CodeInvalidPhone          = "enrollment.invalid_phone"
	CodeFieldRequired         = "enrollment.field_required"
	CodeCompanionNoteRequired = "enrollment.companion_note_required"
	CodePickupTimeNotAllowed  = "enrollment.pickup_time_not_allowed"
	CodeDepartureModeLimit    = "enrollment.departure_mode_limit"

	CodeMessageRequired               = "enrollment.message_required"
	CodeChangeRequestNoteRequired     = "enrollment.change_request_note_required"
	CodeChangeRequestGuardianLinked   = "enrollment.change_request_guardian_linked"
	CodeCorrectionReasonRequired      = "enrollment.correction_reason_required"
	CodeCorrectionSchoolClassMismatch = "enrollment.correction_school_class_mismatch"
	CodeRolloverDeadlineRequired      = "rollover.deadline_required"
)
