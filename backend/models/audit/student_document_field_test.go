package audit_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The document field name carries its category (#777). Without it the change
// history cannot be filtered by the same per-category permissions the Dokumente
// tab enforces, and reading "Ärztliches Attest: Attest_Epilepsie.pdf" in the
// Historie tab becomes a way around student_documents:health.
func TestStudentDocumentFieldRoundTrip(t *testing.T) {
	field := audit.StudentDocumentField("attest")
	assert.Equal(t, "document_attest", field)
	assert.Equal(t, "attest", audit.StudentDocumentCategoryFromField(field))
	assert.True(t, audit.IsStudentDocumentField(field))
}

func TestStudentDocumentCategoryFromFieldIgnoresOtherFields(t *testing.T) {
	// An ordinary field carries no category...
	assert.Empty(t, audit.StudentDocumentCategoryFromField(audit.StudentFieldHealthInfo))
	assert.False(t, audit.IsStudentDocumentField(audit.StudentFieldHealthInfo))

	// ...and neither does the legacy categoryless form. A reader that cannot
	// name the category must fall back to "show only to a caller who may see
	// every category", so an unreadable label never passes as visible.
	assert.Empty(t, audit.StudentDocumentCategoryFromField(audit.StudentFieldDocument))
	assert.True(t, audit.IsStudentDocumentField(audit.StudentFieldDocument))
}

func TestStudentFieldEditValidateAcceptsDocumentFields(t *testing.T) {
	newValue := "Ärztliches Attest: Attest.pdf"
	edit := &audit.StudentFieldEdit{
		StudentID:    91,
		EditedBy:     7,
		EditedByName: "Test Person",
		FieldName:    audit.StudentDocumentField("attest"),
		NewValue:     &newValue,
		CreatedAt:    time.Now(),
	}
	require.NoError(t, edit.Validate())

	// The legacy categoryless form stays storable so old rows keep validating.
	edit.FieldName = audit.StudentFieldDocument
	require.NoError(t, edit.Validate())

	// A prefix-less unknown field is still rejected: the shape check must not
	// have opened the field vocabulary up in general.
	edit.FieldName = "erfundenes_feld"
	require.Error(t, edit.Validate())
}
