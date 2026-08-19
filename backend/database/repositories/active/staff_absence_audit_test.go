package active_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStaffAbsenceAuditRepository_Create(t *testing.T) {
	db := testpkg.SetupTestDB(t)

	repos := repositories.NewFactory(db)
	staff, account := testpkg.CreateTestStaffWithAccount(t, db, "Audit", "Actor")
	ctx := testpkg.TenantContext(staff.TenantID)
	today := timezone.TodayDate()
	absence := &active.StaffAbsence{
		StaffID:     staff.ID,
		AbsenceType: active.AbsenceTypeVacation,
		DateStart:   today,
		DateEnd:     today,
		Status:      active.AbsenceStatusRequested,
		CreatedBy:   staff.ID,
	}
	require.NoError(t, repos.StaffAbsence.Create(ctx, absence))

	fromStatus := active.AbsenceStatusRequested
	audit := &active.StaffAbsenceAudit{
		AbsenceID:  absence.ID,
		FromStatus: &fromStatus,
		ToStatus:   active.AbsenceStatusApproved,
		ActorID:    account.ID,
		Note:       "approved",
	}

	err := repos.StaffAbsenceAudit.Create(ctx, audit)

	require.NoError(t, err)
	assert.NotZero(t, audit.ID)
	assert.Equal(t, staff.TenantID, audit.TenantID)
}
