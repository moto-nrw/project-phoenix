package records

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeletionRecordConstructorsKeepTheirLogShape(t *testing.T) {
	t.Parallel()
	const tenantID, studentID, staffID, noteID, accountID, guests int64 = 41, 42, 43, 44, 45, 2

	note := NewStudentNoteDeletion(studentID, noteID, accountID)
	assert.Equal(t, DeletionTypeManual, note.DeletionType)
	assert.Equal(t, "account:45", note.DeletedBy)
	assert.Equal(t, "student note deleted", note.DeletionReason)
	assert.Equal(t, map[string]interface{}{"student_note_id": noteID, "deleted_by_account_id": accountID}, note.Metadata)
	require.NoError(t, note.Validate())

	legacy := NewLegacyStudentNoteDeletion(studentID, noteID)
	assert.Equal(t, "system", legacy.DeletedBy)
	assert.Equal(t, "legacy supervisor notes cleared", legacy.DeletionReason)
	assert.Equal(t, map[string]interface{}{"student_note_id": noteID, "source": "supervisor_notes"}, legacy.Metadata)

	timetable := NewTimetableRetentionDeletion(tenantID, studentID, 3, 365, "2026-01-01", nil)
	assert.Equal(t, tenantID, timetable.TenantID)
	assert.Equal(t, DeletionTypeTimetableRetention, timetable.DeletionType)
	assert.Equal(t, "automated timetable retention cleanup", timetable.DeletionReason)
	assert.Equal(t, map[string]interface{}{"retention_days": 365, "cutoff_date": "2026-01-01"}, timetable.Metadata)
	sampled := NewTimetableRetentionDeletion(tenantID, studentID, 3, 365, "2026-01-01", []int64{noteID})
	assert.Equal(t, []int64{noteID}, sampled.Metadata["instance_ids_sample"])

	changeLog := NewStudentChangeLogRetentionDeletion(tenantID, studentID, 2, 730, "2026-01-01T00:00:00Z")
	assert.Equal(t, DeletionTypeStudentChangeLogRetention, changeLog.DeletionType)
	assert.Equal(t, "automated student change-log retention cleanup", changeLog.DeletionReason)
	assert.Equal(t, map[string]interface{}{"retention_days": 730, "cutoff": "2026-01-01T00:00:00Z"}, changeLog.Metadata)

	staff := NewStaffRetentionDeletion(tenantID, staffID, 5, true, map[string]int64{"guests": guests})
	assert.Equal(t, tenantID, staff.TenantID)
	require.NotNil(t, staff.StaffID)
	assert.Equal(t, staffID, *staff.StaffID)
	assert.Nil(t, staff.StudentID)
	assert.Equal(t, DeletionTypeStaffRetention, staff.DeletionType)
	assert.Equal(t, 5, staff.RecordsDeleted)
	assert.Equal(t, "retention_expired", staff.DeletionReason)
	assert.Equal(t, "system", staff.DeletedBy)
	assert.False(t, staff.DeletedAt.IsZero())
	assert.Equal(t, map[string]interface{}{"person_anonymized": true, "guests": guests}, staff.Metadata)
}
