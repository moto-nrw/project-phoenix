package students

import (
	strings "strings"
)

// The change history (audit.student_field_edits) records a document upload
// or deletion under "document_<category>"; the bare "document" is the
// categoryless form written before the category moved into the field name.
const (
	documentHistoryField       = "document"
	documentHistoryFieldPrefix = "document_"
)

// documentHistoryCategory reports whether a change-history field records a
// document event and, for the categorised form, its category.
func documentHistoryCategory(field string) (category string, isDocument bool) {
	if category, ok := strings.CutPrefix(field, documentHistoryFieldPrefix); ok {
		return category, true
	}
	return "", field == documentHistoryField
}
