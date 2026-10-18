package records

import "strconv"

// NewStudentNoteDeletion is the deletion log entry of a student note a staff
// member deleted: one manual deletion attributed to the account.
func NewStudentNoteDeletion(studentID, noteID, actorAccountID int64) *DataDeletion {
	record := NewDataDeletion(studentID, DeletionTypeManual, 1, "account:"+strconv.FormatInt(actorAccountID, 10))
	record.DeletionReason = "student note deleted"
	record.SetMetadata("student_note_id", noteID)
	record.SetMetadata("deleted_by_account_id", actorAccountID)
	return record
}

// NewLegacyStudentNoteDeletion is the deletion log entry of a student note
// the system removed when it cleared the legacy supervisor notes.
func NewLegacyStudentNoteDeletion(studentID, noteID int64) *DataDeletion {
	record := NewDataDeletion(studentID, DeletionTypeManual, 1, "system")
	record.DeletionReason = "legacy supervisor notes cleared"
	record.SetMetadata("student_note_id", noteID)
	record.SetMetadata("source", "supervisor_notes")
	return record
}

// NewTimetableRetentionDeletion is the deletion log entry of one child the
// timetable retention touched: the deleted rows, the window, the cutoff date
// and, when there is one, a sample of the instance ids.
func NewTimetableRetentionDeletion(tenantID, studentID int64, recordsDeleted, retentionDays int, cutoffDate string, instanceIDsSample []int64) *DataDeletion {
	record := NewDataDeletion(studentID, DeletionTypeTimetableRetention, recordsDeleted, "system")
	record.SetTenantID(tenantID)
	record.DeletionReason = "automated timetable retention cleanup"
	record.SetMetadata("retention_days", retentionDays)
	record.SetMetadata("cutoff_date", cutoffDate)
	if len(instanceIDsSample) > 0 {
		record.SetMetadata("instance_ids_sample", instanceIDsSample)
	}
	return record
}

// NewStudentChangeLogRetentionDeletion is the deletion log entry of one
// child's change history the retention deleted, with the window and the
// run's cutoff instant (RFC 3339).
func NewStudentChangeLogRetentionDeletion(tenantID, studentID int64, editsDeleted, retentionDays int, cutoff string) *DataDeletion {
	record := NewDataDeletion(studentID, DeletionTypeStudentChangeLogRetention, editsDeleted, "system")
	record.SetTenantID(tenantID)
	record.DeletionReason = "automated student change-log retention cleanup"
	record.SetMetadata("retention_days", retentionDays)
	record.SetMetadata("cutoff", cutoff)
	return record
}

// NewStaffRetentionDeletion is the deletion log entry of one erased staff
// member: the data kind staff_retention, the counts per record kind and the
// reason retention_expired, never the deleted content, written by the
// system like the automatic deletion of a child.
func NewStaffRetentionDeletion(tenantID, staffID int64, recordsDeleted int, personAnonymized bool, counts map[string]int64) *DataDeletion {
	record := NewStaffDataDeletion(staffID, DeletionTypeStaffRetention, recordsDeleted, "system")
	record.DeletionReason = "retention_expired"
	record.SetMetadata("person_anonymized", personAnonymized)
	for kind, count := range counts {
		record.SetMetadata(kind, count)
	}
	record.SetTenantID(tenantID)
	return record
}
