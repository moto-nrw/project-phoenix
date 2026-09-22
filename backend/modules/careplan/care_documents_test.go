package careplan

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCareDocumentForValidation(category string) CareDocument {
	return CareDocument{
		StudentID:       91,
		Category:        category,
		FilenameDisplay: "Attest.pdf",
		FilenameStored:  "0f5b9d4e-1d3a-4a2f-9a3c-1b2c3d4e5f60.pdf",
		SizeBytes:       2048,
		ContentType:     "application/pdf",
		UploadedBy:      7,
	}
}

// The category split is what decides which permission a document needs, so it
// is worth pinning rather than trusting a switch statement: health paperwork
// needs student_documents:health, custody rulings student_documents:legal, and
// everything else stays with the office.
func TestStudentDocumentCategoryTiers(t *testing.T) {
	t.Parallel()

	health := []string{
		StudentDocumentCategoryAttest,
		StudentDocumentCategoryImpfnachweis,
		StudentDocumentCategoryMedikamentenplan,
	}
	for _, category := range StudentDocumentCategories {
		assert.True(t, IsValidStudentDocumentCategory(category), category)
		assert.NotEmpty(t, StudentDocumentCategoryLabels[category],
			"every category needs a German label — the office never sees the storage key")

		assert.Equal(t, slices.Contains(health, category), IsHealthStudentDocumentCategory(category), category)
		assert.Equal(t, category == StudentDocumentCategorySorgerecht,
			IsLegalStudentDocumentCategory(category), category)
	}

	assert.False(t, IsValidStudentDocumentCategory("erfundene_kategorie"))
	assert.False(t, IsHealthStudentDocumentCategory("erfundene_kategorie"))
	assert.False(t, IsLegalStudentDocumentCategory("erfundene_kategorie"))
}

func TestValidateCareDocument(t *testing.T) {
	t.Parallel()

	require.NoError(t, ValidateCareDocument(newCareDocumentForValidation(StudentDocumentCategoryAttest)))

	orphan := newCareDocumentForValidation(StudentDocumentCategorySonstiges)
	orphan.StudentID = 0
	require.Error(t, ValidateCareDocument(orphan))

	nameless := newCareDocumentForValidation(StudentDocumentCategorySonstiges)
	nameless.FilenameDisplay = ""
	require.Error(t, ValidateCareDocument(nameless))
}
