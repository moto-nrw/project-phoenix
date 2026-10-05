package enrollment

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The admin editors mark the rejected field from the code and the JSON key
// path the checks name (#2515). The text stays the diagnosis it was before.

func requireRejection(t *testing.T, err error, code, field string) {
	t.Helper()
	var rejection *InvalidInputError
	require.True(t, errors.As(err, &rejection), "want an InvalidInputError, got %v", err)
	assert.Equal(t, code, rejection.ErrorCode())
	assert.Equal(t, field, rejection.ErrorField())
}

func TestPhaseValidate_NamesCodeAndField(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		edit  func(*Phase)
		code  string
		field string
		text  string
	}{
		{"name", func(p *Phase) { p.Name = " " }, CodePhaseNameRequired, "name", "phase name is required"},
		{"start", func(p *Phase) { p.ServiceStartDate = "" }, CodePhaseServicePeriodInvalid, "service_start_date", "service_start_date is required"},
		{"end before start", func(p *Phase) { p.ServiceEndDate = Date("2026-08-01") }, CodePhaseServicePeriodInvalid, "service_end_date", "service_end_date must be on or after service_start_date"},
		{"class required", func(p *Phase) { p.RequireSchoolClass = true }, CodePhaseSchoolClassesInvalid, "available_school_classes", "require_school_class needs at least one available_school_class"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := validPhase()
			tc.edit(p)
			err := p.Validate()
			requireRejection(t, err, tc.code, tc.field)
			assert.Equal(t, tc.text, err.Error())
		})
	}
}

func TestFormSchemaValidate_NamesListEntryPath(t *testing.T) {
	t.Parallel()

	s := validSchema()
	s.Fields = append(s.Fields, FormField{Key: "Bad Key", Label: "X", Type: FormFieldText})
	err := s.Validate()
	requireRejection(t, err, CodeFormFieldKeyInvalid, "fields.1.key")
	assert.Contains(t, err.Error(), "field 1: form field key")

	s = validSchema()
	s.Fields = append(s.Fields, FormField{Key: "allergies", Label: "Nochmal", Type: FormFieldText})
	requireRejection(t, s.Validate(), CodeFormFieldKeyDuplicate, "fields.1.key")

	s = validSchema()
	s.Fields[0].Label = ""
	requireRejection(t, s.Validate(), CodeFormFieldLabelRequired, "fields.0.label")

	s = validSchema()
	s.LegalBlocks = []FormLegalBlock{{Key: "hinweis", Kind: LegalBlockKindNotice, Title: "T", Label: "L", Enabled: true, Required: true}}
	requireRejection(t, s.Validate(), CodeLegalBlockRequiredNotAllowed, "legal_blocks.0.required")
}

func TestNestInput_KeepsPlainErrorsPlain(t *testing.T) {
	t.Parallel()

	err := NestInput(errors.New("boom"), "fields.2", "field %d", 2)
	assert.EqualError(t, err, "field 2: boom")
	var rejection *InvalidInputError
	assert.False(t, errors.As(err, &rejection))
}
