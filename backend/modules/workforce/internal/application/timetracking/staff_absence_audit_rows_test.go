package timetracking_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	api "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The native audit command returns its persisted identity, tenant and timestamp.
func TestStaffAbsenceAuditRowsWriteTheRecordedAuditBack(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	workTime := api.MustNewWorkforce(testpkg.NewDatabase(t, db))
	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Audit", "Actor")
	ctx := testpkg.Ctx(t)
	day := calendar.NewDate(2026, time.March, 2).String()
	absence, err := workTime.CreateStaffAbsence(ctx, &workforce.StaffAbsence{
		StaffID:     staff.ID,
		AbsenceType: workforce.AbsenceTypeVacation,
		DateStart:   day,
		DateEnd:     day,
		Status:      workforce.AbsenceStatusRequested,
		CreatedBy:   staff.ID,
	})
	require.NoError(t, err)

	fromStatus := workforce.AbsenceStatusRequested
	audit := workforce.StaffAbsenceAudit{
		AbsenceID:  absence.ID,
		FromStatus: &fromStatus,
		ToStatus:   workforce.AbsenceStatusApproved,
		ActorID:    account.ID,
		Note:       "approved",
	}
	audit, err = workTime.RecordStaffAbsenceAudit(ctx, audit)
	require.NoError(t, err)

	assert.NotZero(t, audit.ID)
	assert.Equal(t, staff.TenantID, audit.TenantID)
	assert.False(t, audit.ChangedAt.IsZero())
}

func TestStaffAbsenceAuditRowsRejectAnAuditWithoutAbsence(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	workTime := api.MustNewWorkforce(testpkg.NewDatabase(t, db))

	_, err := workTime.RecordStaffAbsenceAudit(testpkg.Ctx(t), workforce.StaffAbsenceAudit{
		ToStatus: workforce.AbsenceStatusApproved,
	})

	require.Error(t, err)
	assert.ErrorIs(t, err, workforce.ErrInvalidStaffAbsence)
	assert.Contains(t, err.Error(), "absence_id is required")
}
