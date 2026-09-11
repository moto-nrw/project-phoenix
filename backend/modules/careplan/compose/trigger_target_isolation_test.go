package compose

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestReplaceAutoAddTriggersRejectsForeignTarget(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	trigger := createOffering(t, ctx, module, offeringFields(phase.ID, "Local trigger"))

	otherTenantID := testpkg.UniqueTestTenantID(t)
	otherCtx := tenantContext(t, db, otherTenantID)
	foreignPhaseID := insertPhase(t, db, otherCtx, otherTenantID, "Foreign phase")
	foreignTarget := createOffering(t, otherCtx, module, offeringFields(foreignPhaseID, "Foreign target"))

	err := module.ReplaceAutoAddTriggers(ctx, foreignTarget.ID, []int64{trigger.ID})
	require.ErrorIs(t, err, careplan.ErrCareOfferingNotFound)
}
