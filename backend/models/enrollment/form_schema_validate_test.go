package enrollment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FormSchema.Validate is the last guard the schema service runs before
// it writes a new row. The duplicate-key + per-field validation here
// is what stops a broken schema from poisoning every parent submission.

func validSchema() *FormSchema {
	return &FormSchema{
		Name:      "Schuljahr",
		Version:   1,
		CreatedBy: 4321,
		Fields: []FormField{
			{Key: "allergies", Label: "Allergien", Type: FormFieldText, SortOrder: 0},
		},
	}
}

func TestFormSchema_Validate_HappyPath(t *testing.T) {
	assert.NoError(t, validSchema().Validate())
}

func TestFormSchema_Validate_RequiresName(t *testing.T) {
	s := validSchema()
	s.Name = ""
	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name")
}

func TestFormSchema_Validate_RequiresPositiveVersion(t *testing.T) {
	s := validSchema()
	s.Version = 0
	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version")
}

func TestFormSchema_Validate_RejectsNegativeVersion(t *testing.T) {
	s := validSchema()
	s.Version = -1
	err := s.Validate()
	require.Error(t, err)
}

func TestFormSchema_Validate_RequiresCreatedBy(t *testing.T) {
	s := validSchema()
	s.CreatedBy = 0
	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "created_by")
}

func TestFormSchema_Validate_RejectsNegativeCreatedBy(t *testing.T) {
	s := validSchema()
	s.CreatedBy = -1
	err := s.Validate()
	require.Error(t, err)
}

func TestFormSchema_Validate_EmptyFieldsOK(t *testing.T) {
	// A "no extra fields" schema is legal — admins might publish a
	// schema with only the standard fields enabled.
	s := validSchema()
	s.Fields = nil
	assert.NoError(t, s.Validate())
}

func TestFormSchema_Validate_RejectsUnknownCoreRequirement(t *testing.T) {
	s := validSchema()
	s.CoreRequirements = CoreRequirements{"guardian_fax": true}
	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown core requirement")
}

func TestFormSchema_Validate_RejectsDuplicateKey(t *testing.T) {
	s := validSchema()
	s.Fields = []FormField{
		{Key: "allergies", Label: "Allergien", Type: FormFieldText, SortOrder: 0},
		{Key: "allergies", Label: "Doppelt", Type: FormFieldText, SortOrder: 1},
	}
	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
	assert.Contains(t, err.Error(), "allergies")
}

func TestFormSchema_Validate_PropagatesFieldErrorWithIndex(t *testing.T) {
	// One bad field should fail validation with the offending index in
	// the error so the admin can find it in the editor list.
	s := validSchema()
	s.Fields = []FormField{
		{Key: "allergies", Label: "Allergien", Type: FormFieldText, SortOrder: 0},
		{Key: "", Label: "Missing key", Type: FormFieldText, SortOrder: 1},
	}
	err := s.Validate()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "field 1", "error must name the failing index")
}

func TestFormSchema_Validate_AcceptsMultipleDistinctFields(t *testing.T) {
	s := validSchema()
	s.Fields = []FormField{
		{Key: "allergies", Label: "Allergien", Type: FormFieldText, SortOrder: 0},
		{Key: "diet", Label: "Diät", Type: FormFieldText, SortOrder: 1},
		{Key: "notes", Label: "Notizen", Type: FormFieldTextarea, SortOrder: 2},
	}
	assert.NoError(t, s.Validate())
}

func TestFormSchema_TableName(t *testing.T) {
	s := &FormSchema{}
	assert.Equal(t, "enrollment.form_schemas", s.TableName())
}
