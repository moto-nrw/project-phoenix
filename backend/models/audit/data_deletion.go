package audit

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// DataDeletion represents a record of deleted data for GDPR compliance.
//
// A deletion has exactly one subject: either a student (visit cleanup,
// timetable cleanup, GDPR right-to-be-forgotten) OR a staff member
// (time-tracking retention). The DB enforces this via the
// chk_data_deletions_subject_xor check constraint added in migration
// 1.15.58 — student_id XOR staff_id must be non-null.
type DataDeletion struct {
	ID int64 `bun:"id,pk,autoincrement" json:"id"`
	base.TenantModel
	// StudentID and StaffID are mutually exclusive. Use NewDataDeletion
	// (student) or NewStaffDataDeletion (staff) to construct rows — they
	// guarantee the invariant the DB check expects.
	StudentID      *int64                 `bun:"student_id" json:"student_id,omitempty"`
	StaffID        *int64                 `bun:"staff_id" json:"staff_id,omitempty"`
	DeletionType   string                 `bun:"deletion_type,notnull" json:"deletion_type"`
	RecordsDeleted int                    `bun:"records_deleted,notnull" json:"records_deleted"`
	DeletionReason string                 `bun:"deletion_reason" json:"deletion_reason,omitempty"`
	DeletedBy      string                 `bun:"deleted_by,notnull" json:"deleted_by"` // 'system' or account username
	DeletedAt      time.Time              `bun:"deleted_at,notnull,default:now()" json:"deleted_at"`
	Metadata       map[string]interface{} `bun:"metadata,type:jsonb" json:"metadata,omitempty"`
}

// DeletionType constants
const (
	DeletionTypeVisitRetention        = "visit_retention"
	DeletionTypeManual                = "manual"
	DeletionTypeGDPRRequest           = "gdpr_request"
	DeletionTypeTimetableRetention    = "timetable_retention"
	DeletionTypeTimeTrackingRetention = "time_tracking_retention"
	// DeletionTypeStudentChangeLogRetention records automated cleanup of the
	// per-child change history (audit.student_field_edits, issue #1455).
	DeletionTypeStudentChangeLogRetention = "student_change_log_retention"
)

// Validate ensures the data-deletion record is internally consistent before
// it hits the DB. Mirrors the chk_data_deletions_subject_xor constraint so
// callers get a clear Go-side error instead of a 23514 pg error.
func (dd *DataDeletion) Validate() error {
	hasStudent := dd.StudentID != nil && *dd.StudentID > 0
	hasStaff := dd.StaffID != nil && *dd.StaffID > 0
	if hasStudent == hasStaff {
		// Both set or both unset — the DB would reject this too.
		return errors.New("exactly one of student_id / staff_id must be set")
	}

	if dd.DeletionType == "" {
		return errors.New("deletion type is required")
	}

	switch dd.DeletionType {
	case DeletionTypeVisitRetention,
		DeletionTypeManual,
		DeletionTypeGDPRRequest,
		DeletionTypeTimetableRetention,
		DeletionTypeTimeTrackingRetention,
		DeletionTypeStudentChangeLogRetention:
		// Valid types
	default:
		return errors.New("invalid deletion type")
	}

	if dd.RecordsDeleted < 0 {
		return errors.New("records deleted cannot be negative")
	}

	if dd.DeletedBy == "" {
		return errors.New("deleted by is required")
	}

	if dd.DeletedAt.IsZero() {
		dd.DeletedAt = time.Now()
	}

	return nil
}

// GetMetadata returns the metadata map
func (dd *DataDeletion) GetMetadata() map[string]interface{} {
	if dd.Metadata == nil {
		dd.Metadata = make(map[string]interface{})
	}
	return dd.Metadata
}

// SetMetadata sets metadata information
func (dd *DataDeletion) SetMetadata(key string, value interface{}) {
	if dd.Metadata == nil {
		dd.Metadata = make(map[string]interface{})
	}
	dd.Metadata[key] = value
}

// NewDataDeletion creates a student-scoped deletion record. Use this for
// visit retention, timetable retention, and student-initiated GDPR requests.
func NewDataDeletion(studentID int64, deletionType string, recordsDeleted int, deletedBy string) *DataDeletion {
	return &DataDeletion{
		StudentID:      &studentID,
		DeletionType:   deletionType,
		RecordsDeleted: recordsDeleted,
		DeletedBy:      deletedBy,
		DeletedAt:      time.Now(),
		Metadata:       make(map[string]interface{}),
	}
}

// NewStaffDataDeletion creates a staff-scoped deletion record. Use this for
// time-tracking retention and staff-initiated GDPR requests.
func NewStaffDataDeletion(staffID int64, deletionType string, recordsDeleted int, deletedBy string) *DataDeletion {
	return &DataDeletion{
		StaffID:        &staffID,
		DeletionType:   deletionType,
		RecordsDeleted: recordsDeleted,
		DeletedBy:      deletedBy,
		DeletedAt:      time.Now(),
		Metadata:       make(map[string]interface{}),
	}
}
