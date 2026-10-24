package students

import (
	errors "errors"
	http "net/http"
	strconv "strconv"
	time "time"

	common "github.com/moto-nrw/project-phoenix/api/common"
	sessiontoken "github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// changeHistoryEntry is the per-child change-history row returned to the
// frontend (issue #1455). Values are already display-ready German strings, so
// the UI renders a plain "Vorher → Nachher" table without field-specific logic.
type changeHistoryEntry struct {
	ID        string    `json:"id"`
	Field     string    `json:"field"`
	OldValue  string    `json:"old_value"`
	NewValue  string    `json:"new_value"`
	EditedBy  string    `json:"edited_by"`
	ChangedAt time.Time `json:"changed_at"`
}

// getStudentChangeHistory returns the per-child change history. Gated on full
// access (admin / group supervisor) so it reads as a support tool for the OGS
// office, not general staff surveillance.
func (rs *Resource) getStudentChangeHistory(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	if !rs.checkStudentFullAccess(r, student) {
		renderError(w, r, common.ErrorForbidden(errors.New("full access required to view change history")))
		return
	}

	if rs.StudentAuditService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("student audit service not configured")))
		return
	}

	edits, err := rs.StudentAuditService.ListStudentChangeHistory(r.Context(), student.ID)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	// Document rows name a category and a filename ("Ärztliches Attest:
	// Attest_Epilepsie.pdf"). Full access is not enough to read those: the
	// Dokumente tab gates health and custody paperwork behind their own
	// permissions, and a history that showed them anyway would simply be the
	// way around that gate.
	userPermissions := sessiontoken.PermissionsFromCtx(r.Context())

	entries := make([]changeHistoryEntry, 0, len(edits))
	for _, e := range edits {
		if !rs.canSeeChangeHistoryEntry(e.FieldName, userPermissions) {
			continue
		}
		entries = append(entries, changeHistoryEntry{
			ID:        strconv.FormatInt(e.ID, 10),
			Field:     e.FieldName,
			OldValue:  derefOrEmpty(e.OldValue),
			NewValue:  derefOrEmpty(e.NewValue),
			EditedBy:  e.EditedByName,
			ChangedAt: e.CreatedAt,
		})
	}

	common.Respond(w, r, http.StatusOK, entries, "Change history retrieved successfully")
}

// canSeeChangeHistoryEntry filters document rows by the same per-category
// permissions the document endpoints enforce. Everything else passes: those
// fields are covered by the full-access gate above.
//
// A row whose category cannot be determined (the categoryless form written
// before the category moved into the field name) is shown only to a caller who
// may see every category — an unreadable label must not default to visible.
func (rs *Resource) canSeeChangeHistoryEntry(fieldName string, userPermissions []string) bool {
	category, isDocument := documentHistoryCategory(fieldName)
	if !isDocument {
		return true
	}
	if rs.StudentDocumentService == nil {
		return false
	}
	if category == "" {
		return rs.StudentDocumentService.CanSeeEveryStudentDocumentCategory(userPermissions)
	}
	return rs.StudentDocumentService.CanSeeStudentDocumentCategory(category, userPermissions)
}

func derefOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
