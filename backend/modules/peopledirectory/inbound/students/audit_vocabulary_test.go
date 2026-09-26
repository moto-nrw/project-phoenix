package students

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDocumentHistoryCategory pins the stored change-history field names of a
// document event: the categorised form, the categoryless legacy form, and a
// field that is no document event at all.
func TestDocumentHistoryCategory(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		field      string
		category   string
		isDocument bool
	}{
		{field: "document_medical", category: "medical", isDocument: true},
		{field: "document_", category: "", isDocument: true},
		{field: "document", category: "", isDocument: true},
		{field: "documents", category: "", isDocument: false},
		{field: "first_name", category: "", isDocument: false},
	} {
		category, isDocument := documentHistoryCategory(tt.field)
		assert.Equal(t, tt.category, category, tt.field)
		assert.Equal(t, tt.isDocument, isDocument, tt.field)
	}
}
