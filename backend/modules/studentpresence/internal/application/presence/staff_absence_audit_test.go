package presence_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	api "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecordStaffAbsenceAuditReturnsIDAndTenant(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	workTime := api.MustNewWorkforce(testpkg.NewDatabase(t, db))
	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Audit", "Actor")
	ctx := testpkg.TenantContext(staff.TenantID)
	today := calendar.TodayDate().String()
	absence, err := workTime.CreateStaffAbsence(ctx, &workforce.StaffAbsence{
		StaffID:     staff.ID,
		AbsenceType: workforce.AbsenceTypeVacation,
		DateStart:   today,
		DateEnd:     today,
		Status:      workforce.AbsenceStatusRequested,
		CreatedBy:   staff.ID,
	})
	require.NoError(t, err)

	fromStatus := workforce.AbsenceStatusRequested
	audit, err := workTime.RecordStaffAbsenceAudit(ctx, workforce.StaffAbsenceAudit{
		AbsenceID:  absence.ID,
		FromStatus: &fromStatus,
		ToStatus:   workforce.AbsenceStatusApproved,
		ActorID:    account.ID,
		Note:       "approved",
	})

	require.NoError(t, err)
	assert.NotZero(t, audit.ID)
	assert.Equal(t, staff.TenantID, audit.TenantID)
}
