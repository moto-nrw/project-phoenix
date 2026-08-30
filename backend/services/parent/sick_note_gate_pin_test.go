package parent_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The Krankmeldung keeps the behaviour it has today (#2267 changes nothing
// here): with operations.parent_sick_requires_approval OFF the report takes
// effect at once, with it ON it becomes a pending request the OGS decides.
// These two pins exist so a change to the request pipeline cannot quietly move
// the sick note onto the approval path or off it.

func TestSickNoteStaysImmediateWhenApprovalDisabled(t *testing.T) {
	t.Parallel()

	svc, _, _, _ := buildAbsenceApprovalServices(t, false, true)
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	day := timezone.TodayDate()
	res, err := svc.SubmitSickNote(ctx, chain.AccountID, chain.StudentID,
		[]timezone.Date{day}, "Fieber", activeModels.StudentStatusDaySick, nil)
	require.NoError(t, err)

	assert.Nil(t, res.PendingRequest, "no approval gate means no request row")
	require.Len(t, res.StatusDays, 1, "the sick day is written immediately")
	assert.Equal(t, day, res.StatusDays[0].Date)
	assert.Equal(t, activeModels.StudentStatusDaySick, res.StatusDays[0].Status)
	assert.Equal(t, activeModels.StudentStatusSourceParent, res.StatusDays[0].Source)

	var pending int
	require.NoError(t, db.NewSelect().
		Table("active.excused_absence_requests").
		ColumnExpr("COUNT(*)").
		Where("student_id = ?", chain.StudentID).
		Scan(ctx, &pending))
	assert.Zero(t, pending, "an ungated sick note must not leave a request behind")
}

func TestSickNoteGatedCreatesRequest(t *testing.T) {
	t.Parallel()

	svc, _, _, _ := buildAbsenceApprovalServices(t, true, true)
	db := testpkg.SetupTestDB(t)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	day := timezone.TodayDate()
	res, err := svc.SubmitSickNote(ctx, chain.AccountID, chain.StudentID,
		[]timezone.Date{day}, "Fieber", activeModels.StudentStatusDaySick, nil)
	require.NoError(t, err)

	require.NotNil(t, res.PendingRequest, "the gate turns the sick note into a request")
	assert.Empty(t, res.StatusDays, "nothing is written before the OGS decides")
	assert.Equal(t, activeModels.StudentStatusDaySick, res.PendingRequest.AbsenceStatus)
	assert.Equal(t, activeModels.ExcusedRequestStatusPending, res.PendingRequest.Status)
	assert.Equal(t, chain.AccountID, res.PendingRequest.SubmittedBy)

	var days int
	require.NoError(t, db.NewSelect().
		Table("active.student_status_days").
		ColumnExpr("COUNT(*)").
		Where("student_id = ?", chain.StudentID).
		Scan(ctx, &days))
	assert.Zero(t, days, "a gated sick note writes no status day")
}
