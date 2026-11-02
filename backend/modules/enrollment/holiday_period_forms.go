package enrollment

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// The registration form of a holiday period (#5198, spec #4611 B7/B8): the
// Träger-Büro adds its own questions to the fixed fields of the public
// holiday registration with the building blocks of the schools' online
// registration. Each save writes a new version, like enrollment.form_schemas;
// a registration keeps the version it was submitted with, so a later change
// of the form changes no submitted registration. The form belongs to the
// period and lives in the Träger transaction (ADR 0105).
//
// Legal blocks and their document references belong to the same immutable version.

// HolidayFormField is one question of a holiday period's form: a FormField
// of the online registration that may be marked as health information.
// Health answers need the consent to health information, are sealed at rest
// and appear only in the health read of a registration (right
// traeger_health:read) and, when shared, at the hosting school, never in a
// list, an export or a mail (ADR 0110 point 7, ADR 0114 point 7, #4722).
// Health is the one marker for that: every stored version and every answer
// on the wire carries it, false included, so a later reader such as the
// hosting school's copy without health information (#5202) can tell the two
// kinds of question apart by the field alone, never by its key or label.
//
// ShareWithHost lets the school that hosts a week see the answer in its
// participant list and on the sheet of a holiday guest (#5202, ADR 0116 and
// ADR 0120 addenda). A health question carries it too since #4722: its answer
// reaches the hosting school with the fixed health fields, sealed, and only
// when the parents agreed under consent texts of
// HolidayConsentVersionHostHealth. The hosting school's functions check both
// markers again in the database and treat a missing marker as health
// information. It is stored on every version, false included, and acts from
// the version that sets it on.
type HolidayFormField struct {
	FormField
	Health        bool `json:"health"`
	ShareWithHost bool `json:"share_with_host"`
}

// HolidayAnswers are the answers to the questions of a form by field key.
// Each value is the JSON of one answer: true or false for a yes/no
// question, a string for every other question ("YYYY-MM-DD" for a date, the
// option value for a choice).
type HolidayAnswers map[string]json.RawMessage

// HolidayPeriodForm is one version of the form of a period.
type HolidayPeriodForm struct {
	LegalBlocks []FormLegalBlock   `json:"legal_blocks"`
	Version     int                `json:"version"`
	Fields      []HolidayFormField `json:"fields"`
	CreatedAt   time.Time          `json:"created_at"`
}

// HolidayFormMaxFields bounds one version: the paper forms of a Träger
// carry a dozen questions.
const HolidayFormMaxFields = 60

// holidayFormFieldTypes are the field types a holiday form offers. The
// structured types, file uploads and targets write into a student's master
// data, and a holiday registration has no student to write to (ADR 0110
// point 4).
var holidayFormFieldTypes = map[FormFieldType]bool{
	FormFieldBoolean:  true,
	FormFieldText:     true,
	FormFieldTextarea: true,
	FormFieldDate:     true,
	FormFieldSelect:   true,
	FormFieldInfo:     true,
}

var (
	// ErrHolidayFormFieldNotAllowed reports a field type, target, condition
	// source or attribute the holiday form does not offer.
	ErrHolidayFormFieldNotAllowed = errors.New("the holiday registration form does not offer this field")
	// ErrHolidayRegistrationFormChanged reports a registration filled in on
	// an older version of the form than the current one.
	ErrHolidayRegistrationFormChanged = errors.New("the holiday registration form changed")
)

// CodeHolidayFormFieldNotAllowed is the wire code of
// ErrHolidayFormFieldNotAllowed.
const CodeHolidayFormFieldNotAllowed = "enrollment.holiday_form_field_not_allowed"

// ValidateHolidayFormFields checks and normalizes the fields of a new
// version: every check of a FormField of the online registration, unique
// keys and conditions on an existing yes/no or selection field, and the
// limits of the holiday form. A rejection names the field path, such as
// "fields.2.type".
func ValidateHolidayFormFields(fields []HolidayFormField) error {
	if len(fields) > HolidayFormMaxFields {
		return invalidInputf(CodeHolidayFormFieldNotAllowed, "fields", "a holiday form has at most %d fields", HolidayFormMaxFields)
	}
	byKey := make(map[string]*FormField, len(fields))
	for i := range fields {
		prefix := fmt.Sprintf("fields.%d", i)
		if err := validateHolidayFormField(&fields[i]); err != nil {
			return NestInput(err, prefix, "field %d", i)
		}
		if _, dup := byKey[fields[i].Key]; dup {
			return invalidInputf(CodeFormFieldKeyDuplicate, prefix+".key", "duplicate form field key %q", fields[i].Key)
		}
		byKey[fields[i].Key] = &fields[i].FormField
	}
	for i := range fields {
		if err := validateFieldVisibility(&fields[i].FormField, byKey); err != nil {
			return NestInput(err, fmt.Sprintf("fields.%d", i), "field %d", i)
		}
	}
	return nil
}

// validateHolidayFormField refuses what the holiday form does not offer
// before the checks of the online registration run.
func validateHolidayFormField(f *HolidayFormField) error {
	notAllowed := func(field, format string, args ...any) error {
		return invalidInput(CodeHolidayFormFieldNotAllowed, field,
			fmt.Errorf("%w: %s", ErrHolidayFormFieldNotAllowed, fmt.Sprintf(format, args...)))
	}
	switch {
	case !holidayFormFieldTypes[f.Type]:
		return notAllowed("type", "field type %q", f.Type)
	case f.Target != "":
		return notAllowed("target", "target %q", f.Target)
	case len(f.Attachments) > 0:
		return notAllowed("attachments", "attachments")
	case f.FileOptions != nil:
		return notAllowed("file_options", "file options")
	case f.Validation != nil:
		return notAllowed("validation", "validation rules")
	case len(f.AllowedTimes) > 0:
		return notAllowed("allowed_times", "allowed times")
	case len(f.SingleModeGrades) > 0:
		return notAllowed("single_mode_grades", "single mode grades")
	case len(f.Translations) > 0:
		return notAllowed("translations", "translations")
	case f.VisibleWhen != nil && f.VisibleWhen.Source != ConditionSourceField:
		return notAllowed("visible_when", "condition source %q", f.VisibleWhen.Source)
	case f.Health && f.Type == FormFieldInfo:
		return notAllowed("health", "an information block collects no health information")
	case f.ShareWithHost && f.Type == FormFieldInfo:
		return notAllowed("share_with_host", "an information block has no answer to share")
	}
	// A check of the online registration without a wire code of its own
	// refuses an attribute the field cannot carry, such as a required
	// information block.
	if err := f.Validate(); err != nil {
		var rejection *InvalidInputError
		if !errors.As(err, &rejection) {
			return invalidInput(CodeHolidayFormFieldNotAllowed, "", err)
		}
		return err
	}
	return nil
}

// HolidayFormTemplate is the standard template of a new period (B7 of the
// requirements memo of 14.10.2026): per child, optional, the health
// questions marked. The way home and the trips go to the hosting school
// (#5202), and so do the health questions, like the fixed health fields
// (#4722). "Standardvorlage einfügen" in the editor adds the fields whose
// key is missing.
func HolidayFormTemplate() []HolidayFormField {
	child := func(order int, key, label string, typ FormFieldType, health bool, options ...FormFieldOption) HolidayFormField {
		return HolidayFormField{FormField: FormField{
			Key: key, Label: label, Type: typ, SortOrder: order, AppliesToCh: true, Options: options,
		}, Health: health}
	}
	shared := func(field HolidayFormField) HolidayFormField {
		field.ShareWithHost = true
		return field
	}
	return []HolidayFormField{
		shared(child(1, "lebensmittelunvertraeglichkeiten", "Lebensmittelunverträglichkeiten", FormFieldTextarea, true)),
		shared(child(2, "tetanus_impfschutz", "Tetanus-Impfschutz", FormFieldSelect, true,
			FormFieldOption{Label: "Ja", Value: "ja"}, FormFieldOption{Label: "Nein", Value: "nein"},
			FormFieldOption{Label: "Weiß nicht", Value: "weiss_nicht"})),
		shared(child(3, "foerderbedarf", "Förderbedarf", FormFieldTextarea, true)),
		shared(child(4, "koerperliche_einschraenkungen", "Körperliche Einschränkungen", FormFieldTextarea, true)),
		shared(child(5, "darf_allein_nach_hause", "Darf allein nach Hause gehen", FormFieldBoolean, false)),
		shared(child(6, "darf_an_ausfluegen_teilnehmen", "Darf an Ausflügen teilnehmen", FormFieldBoolean, false)),
	}
}

// HolidayFormEditor is the form of a period with the template the editor
// offers.
type HolidayFormEditor struct {
	HolidayPeriodForm
	Template []HolidayFormField `json:"template"`
}
