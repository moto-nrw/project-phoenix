package users

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsValidStaffDocumentCategory(t *testing.T) {
	t.Parallel()

	for _, category := range StaffDocumentCategories {
		assert.True(t, IsValidStaffDocumentCategory(category), "expected %q to be valid", category)
	}

	for _, category := range []string{"", "Arbeitsvertrag", "krankmeldung", "au-bescheinigung"} {
		assert.False(t, IsValidStaffDocumentCategory(category), "expected %q to be rejected", category)
	}
}

func TestStaffDocumentCategories_MatchConstants(t *testing.T) {
	t.Parallel()

	// The list is mirrored by the CHECK constraint on
	// users.staff_documents.category and by the frontend label map.
	assert.Equal(t, []string{
		StaffDocumentCategoryArbeitsvertrag,
		StaffDocumentCategoryZeugnis,
		StaffDocumentCategoryLohnabrechnung,
		StaffDocumentCategoryBewerbung,
		StaffDocumentCategoryAUBescheinigung,
		StaffDocumentCategorySonstiges,
	}, StaffDocumentCategories)
}

// validStaffDocument returns a row that passes Validate, so each case below
// can invalidate exactly one field.
func validStaffDocument() StaffDocument {
	return StaffDocument{
		StaffID:         9001,
		Category:        StaffDocumentCategoryArbeitsvertrag,
		FilenameDisplay: "Arbeitsvertrag.pdf",
		FilenameStored:  "0f1d2c3b-4a59-4c6d-8e7f-a0b1c2d3e4f5.pdf",
		SizeBytes:       2048,
		ContentType:     "application/pdf",
		UploadedBy:      4711,
	}
}

func TestStaffDocument_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*StaffDocument)
		wantErr string
	}{
		{name: "valid row", mutate: func(*StaffDocument) {}},
		{
			name:   "zero size is allowed",
			mutate: func(d *StaffDocument) { d.SizeBytes = 0 },
		},
		{
			name:    "missing staff id",
			mutate:  func(d *StaffDocument) { d.StaffID = 0 },
			wantErr: "staff_id is required",
		},
		{
			name:    "negative staff id",
			mutate:  func(d *StaffDocument) { d.StaffID = -1 },
			wantErr: "staff_id is required",
		},
		{
			name:    "unknown category",
			mutate:  func(d *StaffDocument) { d.Category = "krankmeldung" },
			wantErr: "unknown document category",
		},
		{
			name:    "empty category",
			mutate:  func(d *StaffDocument) { d.Category = "" },
			wantErr: "unknown document category",
		},
		{
			name:    "missing display filename",
			mutate:  func(d *StaffDocument) { d.FilenameDisplay = "" },
			wantErr: "filename_display is required",
		},
		{
			name:    "missing stored filename",
			mutate:  func(d *StaffDocument) { d.FilenameStored = "" },
			wantErr: "filename_stored is required",
		},
		{
			name:    "negative size",
			mutate:  func(d *StaffDocument) { d.SizeBytes = -1 },
			wantErr: "size_bytes must not be negative",
		},
		{
			name:    "missing content type",
			mutate:  func(d *StaffDocument) { d.ContentType = "" },
			wantErr: "content_type is required",
		},
		{
			name:    "missing uploader",
			mutate:  func(d *StaffDocument) { d.UploadedBy = 0 },
			wantErr: "uploaded_by is required",
		},
		{
			name:    "negative uploader",
			mutate:  func(d *StaffDocument) { d.UploadedBy = -5 },
			wantErr: "uploaded_by is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := validStaffDocument()
			tt.mutate(&doc)

			err := doc.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Equal(t, tt.wantErr, err.Error())
		})
	}
}

func TestStaffDocument_ValidateAcceptsEveryCategory(t *testing.T) {
	t.Parallel()

	for _, category := range StaffDocumentCategories {
		doc := validStaffDocument()
		doc.Category = category
		assert.NoError(t, doc.Validate(), "category %q should be storable", category)
	}
}
