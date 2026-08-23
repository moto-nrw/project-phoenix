package audit

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// StudentFieldEdit records a single field change on a student profile for the
// per-child audit trail (issue #1455).
//
// EditedByName is a snapshot of the editor's display name at the moment of the
// change. Storing it denormalized is deliberate: an audit log must reflect who
// performed the change even after that account is renamed or deleted, and it
// keeps the change-history read a single-table query with no join.
type StudentFieldEdit struct {
	ID int64 `bun:"id,pk,autoincrement" json:"id"`
	base.TenantModel
	StudentID    int64     `bun:"student_id,notnull" json:"student_id"`
	EditedBy     int64     `bun:"edited_by,notnull" json:"edited_by"`
	EditedByName string    `bun:"edited_by_name,notnull" json:"edited_by_name"`
	FieldName    string    `bun:"field_name,notnull" json:"field_name"`
	OldValue     *string   `bun:"old_value" json:"old_value"`
	NewValue     *string   `bun:"new_value" json:"new_value"`
	CreatedAt    time.Time `bun:"created_at,notnull,default:now()" json:"created_at"`
}

// Valid field names for student audit entries. These mirror the human-editable
// student profile fields the OGS office asks about (status, notes, pickup).
// Attendance/check-in changes are intentionally out of scope for the first
// stage (they originate from devices/parents and have their own history).
const (
	StudentFieldStatus          = "status"
	StudentFieldSupervisorNotes = "supervisor_notes"
	StudentFieldExtraInfo       = "extra_info"
	StudentFieldHealthInfo      = "health_info"
	StudentFieldPickupStatus    = "pickup_status"
	// StudentFieldCareEnd records the last care day — the inclusive upper
	// bound of the enrollment interval (#2487). Values are German dates, or
	// empty when no end of care is recorded. The categorised exit REASON is
	// deliberately not part of this trail: the change history is readable with
	// users:read, the reason only with users:delete.
	StudentFieldCareEnd                = "care_end"
	StudentFieldDepartureDays          = "departure_days"
	StudentFieldDepartureCompanionNote = "departure_companion_note"

	// StudentFieldDocumentPrefix marks an upload or deletion in the child's
	// document file (#777). The full field name carries the category
	// ("document_attest"), because the reader of a change history must be
	// filtered by the same per-category permissions the Dokumente tab
	// enforces — otherwise the history becomes a side channel around them.
	//
	// The value carries the category label and the display filename, never
	// the document's content: the trail must survive the file it describes,
	// and must not become a second copy of what a deletion removed.
	StudentFieldDocumentPrefix = "document_"

	// StudentFieldDocument is the categoryless form. Nothing writes it any
	// more; readers treat it as "category unknown" and show it only to a
	// caller who may see every category.
	StudentFieldDocument = "document"

	// StudentFieldEditSystemActor identifies automated changes. edited_by has
	// no foreign key so zero can safely represent the scheduler without
	// attributing its work to a real account.
	StudentFieldEditSystemActorID   int64 = 0
	StudentFieldEditSystemActorName       = "System"
)

// StudentDocumentField builds the field name for a document event.
func StudentDocumentField(category string) string {
	return StudentFieldDocumentPrefix + category
}

// StudentDocumentCategoryFromField returns the category encoded in a document
// field name, or "" when the field is not a categorised document field (a
// non-document field, or the legacy categoryless form).
func StudentDocumentCategoryFromField(field string) string {
	if !strings.HasPrefix(field, StudentFieldDocumentPrefix) {
		return ""
	}
	return strings.TrimPrefix(field, StudentFieldDocumentPrefix)
}

// IsStudentDocumentField reports whether the field records a document event,
// in either the categorised or the legacy form.
func IsStudentDocumentField(field string) bool {
	return field == StudentFieldDocument || strings.HasPrefix(field, StudentFieldDocumentPrefix)
}

// Validate ensures the edit record is well-formed before persistence.
func (e *StudentFieldEdit) Validate() error {
	if e.StudentID <= 0 {
		return errors.New("student ID is required")
	}
	if e.EditedBy < 0 ||
		(e.EditedBy == StudentFieldEditSystemActorID && e.EditedByName != StudentFieldEditSystemActorName) {
		return errors.New("edited by is required")
	}
	if e.EditedByName == "" {
		return errors.New("edited by name is required")
	}
	if e.FieldName == "" {
		return errors.New("field name is required")
	}

	switch e.FieldName {
	case StudentFieldStatus, StudentFieldSupervisorNotes, StudentFieldExtraInfo,
		StudentFieldHealthInfo, StudentFieldPickupStatus, StudentFieldDepartureDays,
		StudentFieldDepartureCompanionNote, StudentFieldCareEnd, StudentFieldDocument:
		// Valid field names
	default:
		// Document fields carry their category in the name. The vocabulary of
		// categories belongs to the users domain, so only the shape is checked
		// here.
		if StudentDocumentCategoryFromField(e.FieldName) == "" {
			return errors.New("invalid field name")
		}
	}

	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}

	return nil
}

// StudentFieldEditRepository defines operations for managing student field
// edit audit records.
type StudentFieldEditRepository interface {
	CreateBatch(ctx context.Context, edits []*StudentFieldEdit) error
	GetByStudentID(ctx context.Context, studentID int64) ([]*StudentFieldEdit, error)

	// CountOlderThanByStudent returns, per student, the number of edit rows
	// created strictly before cutoff. Used by the retention cleanup to write
	// one audit.data_deletions row per affected student before deleting.
	CountOlderThanByStudent(ctx context.Context, cutoff time.Time) (map[int64]int, error)

	// DeleteOlderThan removes all edit rows created strictly before cutoff and
	// returns the number deleted. Tenant-scoped via RLS on the caller's tx.
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}
