package users_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newStudentDocument(category string) *users.StudentDocument {
	doc := &users.StudentDocument{StudentID: 91}
	doc.Category = category
	doc.FilenameDisplay = "Attest.pdf"
	doc.FilenameStored = "0f5b9d4e-1d3a-4a2f-9a3c-1b2c3d4e5f60.pdf"
	doc.SizeBytes = 2048
	doc.ContentType = "application/pdf"
	doc.UploadedBy = 7
	return doc
}

// The category split is what decides which permission a document needs, so it
// is worth pinning rather than trusting a switch statement: health paperwork
// needs student_documents:health, custody rulings student_documents:legal, and
// everything else stays with the office.
func TestStudentDocumentCategoryTiers(t *testing.T) {
	health := []string{
		users.StudentDocumentCategoryAttest,
		users.StudentDocumentCategoryImpfnachweis,
		users.StudentDocumentCategoryMedikamentenplan,
	}
	for _, category := range users.StudentDocumentCategories {
		assert.True(t, users.IsValidStudentDocumentCategory(category), category)
		assert.NotEmpty(t, users.StudentDocumentCategoryLabels[category],
			"every category needs a German label — the office never sees the storage key")

		wantHealth := false
		for _, h := range health {
			if h == category {
				wantHealth = true
			}
		}
		assert.Equal(t, wantHealth, users.IsHealthStudentDocumentCategory(category), category)
		assert.Equal(t, category == users.StudentDocumentCategorySorgerecht,
			users.IsLegalStudentDocumentCategory(category), category)
	}

	assert.False(t, users.IsValidStudentDocumentCategory("erfundene_kategorie"))
	assert.False(t, users.IsHealthStudentDocumentCategory("erfundene_kategorie"))
	assert.False(t, users.IsLegalStudentDocumentCategory("erfundene_kategorie"))
}

func TestStudentDocumentValidate(t *testing.T) {
	doc := newStudentDocument(users.StudentDocumentCategoryAttest)
	require.NoError(t, doc.Validate())
	assert.Equal(t, int64(91), doc.GetOwnerID())

	// A free-text category would slip past the CHECK constraint's intent and,
	// worse, past the permission mapping — an unknown category maps to the
	// office permission, which is the weakest gate of the three.
	unknown := newStudentDocument("erfundene_kategorie")
	require.Error(t, unknown.Validate())

	orphan := newStudentDocument(users.StudentDocumentCategorySonstiges)
	orphan.StudentID = 0
	require.Error(t, orphan.Validate())

	// The shared file validation still applies through the embed.
	nameless := newStudentDocument(users.StudentDocumentCategorySonstiges)
	nameless.FilenameDisplay = ""
	require.Error(t, nameless.Validate())
}
