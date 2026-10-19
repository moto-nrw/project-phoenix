package studentdeletion

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewDeletionAuditAttributesAConfirmedDeletionToTheAccount(t *testing.T) {
	t.Parallel()
	tenantID, accountID, studentID := int64(17), int64(29), int64(41)
	counts := Counts{AttendanceRecords: 10, GuardianLinks: 11}

	audit := NewDeletionAudit(Actor{TenantID: tenantID, AccountID: accountID},
		Result{StudentID: studentID, Reason: ReasonDuplicate, Counts: counts, PrimaryRowsDeleted: 12})

	assert.Equal(t, DeletionAudit{
		TenantID: tenantID, StudentID: studentID, DeletionType: DeletionTypeManual, RecordsDeleted: 10 + 11 + 12,
		DeletedBy: "account:29", ActorAccountID: &accountID, Reason: ReasonDuplicate, Counts: counts,
	}, audit)
}

func TestNewDeletionAuditWritesTheRetentionRunBySystem(t *testing.T) {
	t.Parallel()
	tenantID, studentID := int64(17), int64(41)

	audit := NewDeletionAudit(Actor{TenantID: tenantID}, Result{StudentID: studentID, Reason: ReasonRetentionExpired, PrimaryRowsDeleted: 12})

	assert.Equal(t, DeletionTypeRetention, audit.DeletionType)
	assert.Equal(t, "system", audit.DeletedBy)
	assert.Nil(t, audit.ActorAccountID)
	assert.Equal(t, 12, audit.RecordsDeleted)
}

func TestDeletionAuditMetadataMarksTheStudentDeletionWithItsCounts(t *testing.T) {
	t.Parallel()
	counts := Counts{Consents: 10}
	metadata := map[string]any{}

	DeletionAudit{Counts: counts}.EachMetadata(func(key string, value any) { metadata[key] = value })

	assert.Equal(t, map[string]any{"student_deletion": true, "counts": counts}, metadata)
}
