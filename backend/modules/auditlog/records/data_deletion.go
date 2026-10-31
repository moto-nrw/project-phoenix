package records

import (
	"errors"
	"time"
)

// DataDeletion represents a record of deleted data for GDPR compliance.
//
// A deletion has at most one subject: a student (visit cleanup, timetable
// cleanup, GDPR right-to-be-forgotten) OR a staff member (time-tracking
// retention). Only a retention run that logs a whole data kind leaves both
// empty (#3950); the chk_data_deletions_subject check constraint of migration
// 1.15.444 lists those types.
type DataDeletion struct {
	ID int64 `bun:"id,pk,autoincrement" json:"id"`
	TenantModel
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
	// DeletionTypeStudentRetention records the automatic deletion of one
	// child after the school's window past the last care day (#3962). It
	// names the child like a manual deletion and is written by the system.
	DeletionTypeStudentRetention = "student_retention"
	// DeletionTypeStaffRetention records the automatic deletion of one staff
	// member after the school's time-tracking window past the exit (#3963).
	// It names the staff member like the offboarding and is written by the
	// system.
	DeletionTypeStaffRetention = "staff_retention"
)

// Retention types that log one data kind of a nightly run without a subject
// (#3950): the row carries the count, never the deleted content.
const (
	DeletionTypeUnregisteredTagScanRetention = "unregistered_tag_scan_retention"
	DeletionTypeStatusDayRetention           = "status_day_retention"
	DeletionTypeParentMessageRetention       = "parent_message_retention"
	DeletionTypeAuthEventRetention           = "auth_event_retention"
	DeletionTypeDataAccessLogRetention       = "data_access_log_retention"
	DeletionTypeDeletionLogRetention         = "deletion_log_retention"
	DeletionTypeDeliveryRetention            = "delivery_retention"
	// The technical remains of #4046: finished cleanup and offboarding jobs,
	// finished care-withdrawal completions and enrollment rate-limit buckets.
	DeletionTypeFileCleanupRetention            = "file_cleanup_retention"
	DeletionTypeStaffCleanupRetention           = "staff_cleanup_retention"
	DeletionTypeStudentDocumentCleanupRetention = "student_document_cleanup_retention"
	DeletionTypeCareWithdrawalRetention         = "care_withdrawal_retention"
	DeletionTypeSubmissionRateLimitRetention    = "submission_rate_limit_retention"
	// Withdrawn and lapsed enrollments no child was created from, with
	// their invitations to enroll late (#3960).
	DeletionTypeSettledEnrollmentRetention = "settled_enrollment_retention"
	// Files parents attached to an enrollment: unclaimed uploads after a day,
	// those of a decided request after the school's window (#4783).
	DeletionTypeEnrollmentUploadRetention = "enrollment_upload_retention"
	// Staff sick notes (AU-Bescheinigungen) after the school's window (#3961).
	DeletionTypeStaffSickNoteRetention = "staff_sick_note_retention"
	// Daily attendance after the school's window (#3957).
	DeletionTypeAttendanceRetention = "attendance_retention"
	// Morning attendance a Lehrkraft marked, after the school's window (#4790).
	DeletionTypeMorningAttendanceRetention = "morning_attendance_retention"
	// Homework of the classes and the OGS feedback on it, after the school's
	// window (#4796).
	DeletionTypeHomeworkRetention = "homework_retention"
	// Parent contacts without a child after the school's window (#4043).
	DeletionTypeGuardianRetention = "guardian_retention"
	// The content a school creates itself, each after its window (#4045):
	// parent announcements and staff notices after the school's message
	// windows, ended appointments after its timetable window and cancelled or
	// deleted ones after the fixed feed window, grade transitions after its
	// student window and removed file-storage rows after the fixed window of
	// the cleanup intents.
	DeletionTypeParentAnnouncementRetention   = "parent_announcement_retention"
	DeletionTypeStaffNoticeRetention          = "staff_notice_retention"
	DeletionTypeAppointmentRetention          = "appointment_retention"
	DeletionTypeAppointmentTombstoneRetention = "appointment_tombstone_retention"
	DeletionTypeGradeTransitionRetention      = "grade_transition_retention"
	DeletionTypeRemovedFileRetention          = "removed_file_retention"
	// The weekly protocols of the groups (#4769), with their days, after the
	// school's attendance window counted from the week's Sunday.
	DeletionTypeWeekProtocolRetention = "week_protocol_retention"
	// The surveys (#4781) after the school's parent and staff message windows.
	DeletionTypeParentSurveyRetention = "parent_survey_retention"
	DeletionTypeStaffSurveyRetention  = "staff_survey_retention"
	// The protocols of #4044 and the setting protocol, each after its fixed
	// window.
	DeletionTypeStudentDeletionLogRetention       = "student_deletion_log_retention"
	DeletionTypeEnrollmentDeletionLogRetention    = "enrollment_deletion_log_retention"
	DeletionTypeEnrollmentRestorationLogRetention = "enrollment_restoration_log_retention"
	DeletionTypeTimeTrackingDeletionLogRetention  = "time_tracking_deletion_log_retention"
	DeletionTypeStaffMasterDataChangeRetention    = "staff_master_data_change_retention"
	DeletionTypePersonnelNumberChangeRetention    = "personnel_number_change_retention"
	DeletionTypeGuardianFinancialChangeRetention  = "guardian_financial_change_retention"
	DeletionTypeClassListChangeRetention          = "class_list_change_retention"
	DeletionTypeSubstitutionChangeRetention       = "substitution_change_retention"
	DeletionTypeFileEventRetention                = "file_event_retention"
	DeletionTypeExportTransferRetention           = "export_transfer_retention"
	DeletionTypeSettingAuditRetention             = "setting_audit_retention"
)

// ProtocolRetentionTypes are the protocols of #4044 the Audit Platform
// deletes itself, each after its fixed window and logged as its own data
// kind, in the order the nightly cleanup deletes them.
func ProtocolRetentionTypes() []string {
	return []string{
		DeletionTypeStudentDeletionLogRetention,
		DeletionTypeEnrollmentDeletionLogRetention,
		DeletionTypeEnrollmentRestorationLogRetention,
		DeletionTypeTimeTrackingDeletionLogRetention,
		DeletionTypeStaffMasterDataChangeRetention,
		DeletionTypePersonnelNumberChangeRetention,
		DeletionTypeGuardianFinancialChangeRetention,
		DeletionTypeClassListChangeRetention,
		DeletionTypeSubstitutionChangeRetention,
		DeletionTypeFileEventRetention,
	}
}

// IsDataKindRetention reports whether deletionType logs a whole data kind
// without a subject.
func IsDataKindRetention(deletionType string) bool {
	switch deletionType {
	case DeletionTypeUnregisteredTagScanRetention,
		DeletionTypeStatusDayRetention,
		DeletionTypeAttendanceRetention,
		DeletionTypeMorningAttendanceRetention,
		DeletionTypeHomeworkRetention,
		DeletionTypeGuardianRetention,
		DeletionTypeParentMessageRetention,
		DeletionTypeParentAnnouncementRetention,
		DeletionTypeStaffNoticeRetention,
		DeletionTypeAppointmentRetention,
		DeletionTypeAppointmentTombstoneRetention,
		DeletionTypeGradeTransitionRetention,
		DeletionTypeRemovedFileRetention,
		DeletionTypeParentSurveyRetention,
		DeletionTypeStaffSurveyRetention,
		DeletionTypeWeekProtocolRetention,
		DeletionTypeAuthEventRetention,
		DeletionTypeDataAccessLogRetention,
		DeletionTypeDeletionLogRetention,
		DeletionTypeDeliveryRetention,
		DeletionTypeFileCleanupRetention,
		DeletionTypeStaffCleanupRetention,
		DeletionTypeStudentDocumentCleanupRetention,
		DeletionTypeCareWithdrawalRetention,
		DeletionTypeSubmissionRateLimitRetention,
		DeletionTypeSettledEnrollmentRetention,
		DeletionTypeEnrollmentUploadRetention,
		DeletionTypeStaffSickNoteRetention,
		DeletionTypeStudentDeletionLogRetention,
		DeletionTypeEnrollmentDeletionLogRetention,
		DeletionTypeEnrollmentRestorationLogRetention,
		DeletionTypeTimeTrackingDeletionLogRetention,
		DeletionTypeStaffMasterDataChangeRetention,
		DeletionTypePersonnelNumberChangeRetention,
		DeletionTypeGuardianFinancialChangeRetention,
		DeletionTypeClassListChangeRetention,
		DeletionTypeSubstitutionChangeRetention,
		DeletionTypeFileEventRetention,
		DeletionTypeExportTransferRetention,
		DeletionTypeSettingAuditRetention:
		return true
	}
	return false
}

// Validate ensures the data-deletion record is internally consistent before
// it hits the DB. Mirrors the chk_data_deletions_subject constraint so
// callers get a clear Go-side error instead of a 23514 pg error.
func (dd *DataDeletion) Validate() error {
	hasStudent := dd.StudentID != nil && *dd.StudentID > 0
	hasStaff := dd.StaffID != nil && *dd.StaffID > 0

	if dd.DeletionType == "" {
		return errors.New("deletion type is required")
	}

	switch {
	case IsDataKindRetention(dd.DeletionType):
		if hasStudent || hasStaff {
			return errors.New("a data-kind retention row names no student or staff member")
		}
	case dd.DeletionType == DeletionTypeVisitRetention,
		dd.DeletionType == DeletionTypeManual,
		dd.DeletionType == DeletionTypeGDPRRequest,
		dd.DeletionType == DeletionTypeTimetableRetention,
		dd.DeletionType == DeletionTypeTimeTrackingRetention,
		dd.DeletionType == DeletionTypeStudentChangeLogRetention,
		dd.DeletionType == DeletionTypeStudentRetention,
		dd.DeletionType == DeletionTypeStaffRetention:
		if hasStudent == hasStaff {
			// Both set or both unset — the DB would reject this too.
			return errors.New("exactly one of student_id / staff_id must be set")
		}
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

// NewRetentionDataDeletion creates the row of one data kind a nightly
// retention run deleted (#3950). It names no subject and holds only the count.
func NewRetentionDataDeletion(deletionType string, recordsDeleted int) *DataDeletion {
	return &DataDeletion{
		DeletionType:   deletionType,
		RecordsDeleted: recordsDeleted,
		DeletionReason: "automated retention cleanup",
		DeletedBy:      "system",
		DeletedAt:      time.Now(),
		Metadata:       make(map[string]interface{}),
	}
}
