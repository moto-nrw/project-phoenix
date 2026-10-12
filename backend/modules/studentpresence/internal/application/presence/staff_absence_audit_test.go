package presence_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/adapters/timerecords"
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaffAbsenceAuditRepository_Create(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	owners := repositories.NewUnobservedTimetableDependencies(db)
	repos := repositories.NewFactory(db, owners)
	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Audit", "Actor")
	ctx := testpkg.TenantContext(staff.TenantID)
	today := timezone.TodayDate()
	absence := &timerecords.StaffAbsence{
		StaffID:     staff.ID,
		AbsenceType: workforce.AbsenceTypeVacation,
		DateStart:   today,
		DateEnd:     today,
		Status:      workforce.AbsenceStatusRequested,
		CreatedBy:   staff.ID,
	}
	require.NoError(t, repos.StaffAbsence.Create(ctx, absence))

	fromStatus := workforce.AbsenceStatusRequested
	audit := &timerecords.StaffAbsenceAudit{
		AbsenceID:  absence.ID,
		FromStatus: &fromStatus,
		ToStatus:   workforce.AbsenceStatusApproved,
		ActorID:    account.ID,
		Note:       "approved",
	}

	err := workforceLegacy.NewStaffAbsenceAuditRepository(owners.Workforce).Create(ctx, audit)

	require.NoError(t, err)
	assert.NotZero(t, audit.ID)
	assert.Equal(t, staff.TenantID, audit.TenantID)
}
