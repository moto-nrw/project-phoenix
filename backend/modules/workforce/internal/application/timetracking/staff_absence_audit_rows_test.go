package timetracking_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application/timetracking"
	api "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The absence audit rows the composition serves write the recorded audit back
// into the caller's row, as the retained repository did (#3996).
func TestStaffAbsenceAuditRowsWriteTheRecordedAuditBack(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	workTime := api.MustNewWorkforce(testpkg.NewDatabase(t, db))
	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Audit", "Actor")
	ctx := testpkg.Ctx(t)
	day := calendar.NewDate(2026, time.March, 2).String()
	absence, err := workTime.CreateStaffAbsence(ctx, workforce.StaffAbsence{
		StaffID:     staff.ID,
		AbsenceType: workforce.AbsenceTypeVacation,
		DateStart:   day,
		DateEnd:     day,
		Status:      workforce.AbsenceStatusRequested,
		CreatedBy:   staff.ID,
	})
	require.NoError(t, err)

	fromStatus := workforce.AbsenceStatusRequested
	audit := &timetracking.StaffAbsenceAudit{
		AbsenceID:  absence.ID,
		FromStatus: &fromStatus,
		ToStatus:   workforce.AbsenceStatusApproved,
		ActorID:    account.ID,
		Note:       "approved",
	}
	require.NoError(t, workforceCompose.NewStaffAbsenceAuditRepository(workTime).Create(ctx, audit))

	assert.NotZero(t, audit.ID)
	assert.Equal(t, staff.TenantID, audit.TenantID)
	assert.False(t, audit.ChangedAt.IsZero())
}

func TestStaffAbsenceAuditRowsRejectAnAuditWithoutAbsence(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	workTime := api.MustNewWorkforce(testpkg.NewDatabase(t, db))

	err := workforceCompose.NewStaffAbsenceAuditRepository(workTime).Create(testpkg.Ctx(t), &timetracking.StaffAbsenceAudit{
		ToStatus: workforce.AbsenceStatusApproved,
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "absence_id is required")
}
