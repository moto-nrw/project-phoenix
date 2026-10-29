package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The form of a holiday period (#5198). Invariants:
//
//  1. Every period has a version: creating a period writes version 1 with
//     the standard template in the same Träger transaction; the migration
//     wrote version 1 without questions for every older period.
//  2. A version never changes. Saving writes the next version under the
//     lock of the period's row, so two saves of one period get two
//     versions, one after the other.
//  3. A registration is checked against the version it is stored with and
//     keeps it; a later version changes no submitted registration.
//  4. An answer to a field hidden by its condition is dropped and a hidden
//     required field blocks nothing.
//  5. An answer to a field marked as health information needs the consent
//     to health information and reaches only the detail of a registration.

// Limits of an answer. They bound the request like the fixed fields.
const (
	holidayAnswerTextMaxLength     = 500
	holidayAnswerTextareaMaxLength = 2000
)

// HolidayFormStore persists the versions of the forms of a Träger's
// periods in the Träger transaction.
type HolidayFormStore interface {
	InsertHolidayFormDocument(context.Context, int64, int64, int64, string, []byte) (enrollment.HolidayFormDocument, error)
	ReadHolidayFormDocument(context.Context, int64, int64, string) (enrollment.HolidayFormDocument, bool, error)
	DeleteHolidayFormDocument(context.Context, int64, int64, string) (bool, error)
	CurrentHolidayPeriodForm(ctx context.Context, organizationID, periodID int64) (enrollment.HolidayPeriodForm, bool, error)
	// HolidayPeriodForms reads selected versions, or all versions when versions is empty.
	HolidayPeriodForms(ctx context.Context, organizationID, periodID int64, versions []int) (map[int]enrollment.HolidayPeriodForm, error)
	// InsertHolidayPeriodForm writes the next version; a zero createdBy
	// names no author.
	InsertHolidayPeriodForm(ctx context.Context, organizationID, periodID, createdBy int64, fields []enrollment.HolidayFormField, blocks []enrollment.FormLegalBlock) (enrollment.HolidayPeriodForm, error)
}

// HolidayPeriodForm returns the current version of a period's form with the
// standard template.
func (h *HolidayPeriods) HolidayPeriodForm(ctx context.Context, organizationID, periodID int64) (enrollment.HolidayFormEditor, error) {
	if err := holidayPeriodKey(organizationID, periodID); err != nil {
		return enrollment.HolidayFormEditor{}, err
	}
	if _, err := foundHolidayPeriod(h.store.FindHolidayPeriod(ctx, organizationID, periodID, false)); err != nil {
		return enrollment.HolidayFormEditor{}, err
	}
	form, err := foundHolidayForm(periodID)(h.forms.CurrentHolidayPeriodForm(ctx, organizationID, periodID))
	if err != nil {
		return enrollment.HolidayFormEditor{}, err
	}
	return enrollment.HolidayFormEditor{HolidayPeriodForm: form, Template: enrollment.HolidayFormTemplate()}, nil
}

// SaveHolidayPeriodForm checks the fields and writes them as the next
// version under the lock of the period's row.
func (h *HolidayPeriods) SaveHolidayPeriodForm(ctx context.Context, organizationID, periodID, accountID int64, fields []enrollment.HolidayFormField, blocks []enrollment.FormLegalBlock) (enrollment.HolidayFormEditor, error) {
	if err := holidayPeriodKey(organizationID, periodID); err != nil {
		return enrollment.HolidayFormEditor{}, err
	}
	if _, err := foundHolidayPeriod(h.store.FindHolidayPeriod(ctx, organizationID, periodID, true)); err != nil {
		return enrollment.HolidayFormEditor{}, err
	}
	if fields == nil {
		fields = []enrollment.HolidayFormField{}
	}
	if err := enrollment.ValidateHolidayFormFields(fields); err != nil {
		return enrollment.HolidayFormEditor{}, err
	}
	if err := h.validateHolidayLegalBlocks(ctx, organizationID, periodID, blocks); err != nil {
		return enrollment.HolidayFormEditor{}, err
	}
	for i := range fields {
		fields[i].SortOrder = i + 1
	}
	form, err := h.forms.InsertHolidayPeriodForm(ctx, organizationID, periodID, accountID, fields, blocks)
	if err != nil {
		return enrollment.HolidayFormEditor{}, err
	}
	return enrollment.HolidayFormEditor{HolidayPeriodForm: form, Template: enrollment.HolidayFormTemplate()}, nil
}

// foundHolidayForm turns the store's answer for the current version of a
// period's form into the form; a period without one breaks invariant 1.
func foundHolidayForm(periodID int64) func(enrollment.HolidayPeriodForm, bool, error) (enrollment.HolidayPeriodForm, error) {
	return func(form enrollment.HolidayPeriodForm, found bool, err error) (enrollment.HolidayPeriodForm, error) {
		if err != nil {
			return enrollment.HolidayPeriodForm{}, fmt.Errorf("enrollment: read holiday form: %w", err)
		}
		if !found {
			return enrollment.HolidayPeriodForm{}, fmt.Errorf("enrollment: holiday period %d has no form version", periodID)
		}
		return form, nil
	}
}

// holidayFormSchema views the questions of a version as a form schema, so
// the visibility and requirement checks of the online registration apply
// unchanged.
func holidayFormSchema(form enrollment.HolidayPeriodForm) *enrollment.FormSchema {
	fields := make([]enrollment.FormField, len(form.Fields))
	for i, field := range form.Fields {
		fields[i] = field.FormField
	}
	return &enrollment.FormSchema{Fields: fields}
}

// holidayAnswerSet holds the decoded answers of a registration: per
// family and per child in the order of the children. The visibility and
// requirement checks of the online registration read decoded values.
type holidayAnswerSet struct {
	family   map[string]any
	children []map[string]any
}

// decodeHolidayAnswers decodes the answers of a registration. An answer
// that is no JSON value counts as missing; the request decoder admits only
// valid JSON.
func decodeHolidayAnswers(input enrollment.HolidayRegistrationInput) holidayAnswerSet {
	set := holidayAnswerSet{family: decodeAnswers(input.CustomData), children: make([]map[string]any, len(input.Children))}
	for i, child := range input.Children {
		set.children[i] = decodeAnswers(child.CustomData)
	}
	return set
}

func decodeAnswers(answers enrollment.HolidayAnswers) map[string]any {
	out := make(map[string]any, len(answers))
	for key, raw := range answers {
		var value any
		if err := json.Unmarshal(raw, &value); err == nil {
			out[key] = value
		}
	}
	return out
}

// encodeAnswers turns checked answers back into the stored shape; nil
// becomes an empty set.
func encodeAnswers(answers map[string]any) (enrollment.HolidayAnswers, error) {
	out := make(enrollment.HolidayAnswers, len(answers))
	for key, value := range answers {
		raw, err := json.Marshal(value)
		if err != nil {
			return nil, fmt.Errorf("enrollment: encode holiday answer %q: %w", key, err)
		}
		out[key] = raw
	}
	return out, nil
}

// withHolidayAnswers stores the checked answers on the registration.
func withHolidayAnswers(input enrollment.HolidayRegistrationInput, set holidayAnswerSet) (enrollment.HolidayRegistrationInput, error) {
	family, err := encodeAnswers(set.family)
	if err != nil {
		return input, err
	}
	input.CustomData = family
	children := make([]enrollment.HolidayChildInput, len(input.Children))
	for i, child := range input.Children {
		if child.CustomData, err = encodeAnswers(set.children[i]); err != nil {
			return input, err
		}
		children[i] = child
	}
	input.Children = children
	return input, nil
}

// sanitizeHolidayAnswers trims text answers, drops empty ones, and keeps
// only the answers of visible questions of the form (invariant 4).
func sanitizeHolidayAnswers(form enrollment.HolidayPeriodForm, set holidayAnswerSet) holidayAnswerSet {
	schema := holidayFormSchema(form)
	byKey := buildFieldsByKey(schema)
	family := trimHolidayAnswers(set.family)
	out := holidayAnswerSet{
		family:   sanitizeVisibleAnswers(schema, false, family, fieldVisibilityContext{guardianAnswers: family, fieldsByKey: byKey}),
		children: make([]map[string]any, len(set.children)),
	}
	for i, child := range set.children {
		answers := trimHolidayAnswers(child)
		out.children[i] = sanitizeVisibleAnswers(schema, true, answers,
			fieldVisibilityContext{guardianAnswers: out.family, childAnswers: answers, fieldsByKey: byKey})
	}
	return out
}

func trimHolidayAnswers(answers map[string]any) map[string]any {
	out := make(map[string]any, len(answers))
	for key, value := range answers {
		if text, ok := value.(string); ok {
			value = strings.TrimSpace(text)
			if value == "" {
				continue
			}
		}
		if value == nil {
			continue
		}
		out[key] = value
	}
	return out
}

// validateHolidayAnswers checks the sanitized answers of one scope against
// the form: type, choice, length and the required questions a parent sees.
// The errors are keyed by field key.
func validateHolidayAnswers(form enrollment.HolidayPeriodForm, child bool, answers map[string]any, ctx fieldVisibilityContext) validation.Errors {
	errs := validation.Errors{}
	for i := range form.Fields {
		field := &form.Fields[i].FormField
		if field.AppliesToCh != child || field.Type == enrollment.FormFieldInfo || !fieldVisible(field, ctx) {
			continue
		}
		value, present := answers[field.Key]
		if present {
			if err := holidayAnswerValid(*field, value); err != nil {
				errs[field.Key] = err
				continue
			}
		}
		if field.Required && !customAnswerSatisfiesRequired(*field, answers) {
			errs[field.Key] = errors.New("is required")
		}
	}
	return errs
}

// holidayAnswerValid checks one answer against the type of its question.
func holidayAnswerValid(field enrollment.FormField, value any) error {
	switch field.Type {
	case enrollment.FormFieldBoolean:
		if _, ok := value.(bool); !ok {
			return errors.New("must be yes or no")
		}
		return nil
	case enrollment.FormFieldSelect:
		choice, ok := value.(string)
		if !ok || !slices.ContainsFunc(field.Options, func(option enrollment.FormFieldOption) bool { return option.Value == choice }) {
			return errors.New("is not an option of the field")
		}
		return nil
	case enrollment.FormFieldDate:
		text, ok := value.(string)
		if !ok {
			return errors.New("must be a date")
		}
		if _, err := calendar.ParseDate(text); err != nil {
			return errors.New("must be a date")
		}
		return nil
	case enrollment.FormFieldText, enrollment.FormFieldTextarea:
		return holidayTextAnswerValid(field, value)
	default:
		return errors.New("the field takes no answer")
	}
}

// holidayTextAnswerValid checks a text answer against the length of its
// field type.
func holidayTextAnswerValid(field enrollment.FormField, value any) error {
	text, ok := value.(string)
	if !ok {
		return errors.New("must be text")
	}
	limit := holidayAnswerTextMaxLength
	if field.Type == enrollment.FormFieldTextarea {
		limit = holidayAnswerTextareaMaxLength
	}
	if utf8.RuneCountInString(text) > limit {
		return fmt.Errorf("is longer than %d characters", limit)
	}
	return nil
}

// holidayAnswersHaveHealth reports whether an answer of the registration
// belongs to a question marked as health information (invariant 5).
func holidayAnswersHaveHealth(form enrollment.HolidayPeriodForm, set holidayAnswerSet) bool {
	for _, field := range form.Fields {
		if !field.Health {
			continue
		}
		if !field.AppliesToCh {
			if _, ok := set.family[field.Key]; ok {
				return true
			}
			continue
		}
		for _, child := range set.children {
			if _, ok := child[field.Key]; ok {
				return true
			}
		}
	}
	return false
}

// holidayExportColumns are the questions without health information of the
// exported versions, newest version first: a key keeps the label of the
// newest exported version that asks it. Health classifications are checked
// across every version of the period, independently of the exported rows.
func holidayExportColumns(forms map[int]enrollment.HolidayPeriodForm, exportedVersions []int) []enrollment.HolidayFormField {
	versions := make([]int, 0, len(forms))
	health := map[string]bool{}
	for version, form := range forms {
		if slices.Contains(exportedVersions, version) {
			versions = append(versions, version)
		}
		for _, field := range form.Fields {
			if field.Health {
				health[field.Key] = true
			}
		}
	}
	slices.Sort(versions)
	slices.Reverse(versions)
	columns := []enrollment.HolidayFormField{}
	seen := map[string]bool{}
	for _, version := range versions {
		for _, field := range forms[version].Fields {
			if field.Type == enrollment.FormFieldInfo || health[field.Key] || seen[field.Key] {
				continue
			}
			seen[field.Key] = true
			columns = append(columns, field)
		}
	}
	return columns
}
