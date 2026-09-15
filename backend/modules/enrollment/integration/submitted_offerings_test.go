package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestSubmittedOfferingChoicesAreImmutableAndRetryable(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	phaseID, _, childID := testpkg.CreateAuditAdjustmentChain(t, db)
	offering := testpkg.CreateTestCareOffering(t, db, phaseID, "Submitted choice")
	module := enrollmentCompose.New()
	ctx := testpkg.Ctx(t)
	choices := []enrollment.SubmittedOfferingChoice{{CareOfferingID: offering.ID, SelectedDays: []string{"mon", "wed"}, Notes: new("original")}}
	require.NoError(t, module.RecordSubmittedOfferingChoices(ctx, childID, choices))
	first, err := module.SubmittedOfferingChoices(ctx, []int64{childID})
	require.NoError(t, err)
	require.Len(t, first, 1)
	require.Equal(t, []string{"mon", "wed"}, first[0].SelectedDays)
	require.Equal(t, "original", *first[0].Notes)
	require.NoError(t, module.RecordSubmittedOfferingChoices(ctx, childID, choices))
	retry, err := module.SubmittedOfferingChoices(ctx, []int64{childID})
	require.NoError(t, err)
	require.Equal(t, first, retry)
	choices[0].SelectedDays = []string{"fri"}
	require.ErrorIs(t, module.RecordSubmittedOfferingChoices(ctx, childID, choices), enrollment.ErrSubmittedOfferingChoiceConflict)
	after, err := module.SubmittedOfferingChoices(ctx, []int64{childID})
	require.NoError(t, err)
	require.Equal(t, first, after, "effective-care changes cannot rewrite the submitted choice")
}

func TestSubmittedOfferingChoiceBatchRollsBackAndRetries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	phaseID, _, childID := testpkg.CreateAuditAdjustmentChain(t, db)
	first := testpkg.CreateTestCareOffering(t, db, phaseID, "First choice")
	second := testpkg.CreateTestCareOffering(t, db, phaseID, "Second choice")
	module := enrollmentCompose.New()
	ctx := testpkg.Ctx(t)
	original := enrollment.SubmittedOfferingChoice{CareOfferingID: second.ID, Notes: new("submitted")}
	require.NoError(t, module.RecordSubmittedOfferingChoices(ctx, childID, []enrollment.SubmittedOfferingChoice{original}))
	batch := []enrollment.SubmittedOfferingChoice{{CareOfferingID: first.ID}, {CareOfferingID: second.ID, Notes: new("changed")}}
	require.ErrorIs(t, module.RecordSubmittedOfferingChoices(ctx, childID, batch), enrollment.ErrSubmittedOfferingChoiceConflict)
	rows, err := module.SubmittedOfferingChoices(ctx, []int64{childID})
	require.NoError(t, err)
	require.Len(t, rows, 1, "a conflict after the first insert must roll back that insert")
	require.Equal(t, second.ID, rows[0].CareOfferingID)
	batch[1] = original
	injected := errors.New("failure after Enrollment command")
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		if err := module.RecordSubmittedOfferingChoices(txCtx, childID, batch); err != nil {
			return err
		}
		return injected
	})
	require.ErrorIs(t, err, injected)
	rows, err = module.SubmittedOfferingChoices(ctx, []int64{childID})
	require.NoError(t, err)
	require.Len(t, rows, 1, "the owner command must join the workflow transaction")
	require.NoError(t, module.RecordSubmittedOfferingChoices(ctx, childID, batch))
	rows, err = module.SubmittedOfferingChoices(ctx, []int64{childID})
	require.NoError(t, err)
	require.Len(t, rows, 2)
}

func TestSubmittedOfferingChoicesEnforceTwoTenantIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := enrollmentCompose.New()
	type school struct {
		ctx                 context.Context
		childID, offeringID int64
	}
	var schools []school
	for _, name := range []string{"first", "second"} {
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			phaseID, _, childID := testpkg.CreateAuditAdjustmentChain(t, db)
			offering := testpkg.CreateTestCareOffering(t, db, phaseID, "Isolated choice")
			ctx := testpkg.Ctx(t)
			require.NoError(t, module.RecordSubmittedOfferingChoices(ctx, childID, []enrollment.SubmittedOfferingChoice{{CareOfferingID: offering.ID}}))
			schools = append(schools, school{ctx: testpkg.ContextForTenant(testpkg.Ctx(t), testpkg.Tenant(t)), childID: childID, offeringID: offering.ID})
		})
	}
	for i, own := range schools {
		foreign := schools[1-i]
		rows, err := module.SubmittedOfferingChoices(own.ctx, []int64{own.childID, foreign.childID})
		require.NoError(t, err)
		require.Len(t, rows, 1)
		require.Equal(t, own.childID, rows[0].RequestChildID)
		err = module.RecordSubmittedOfferingChoices(own.ctx, foreign.childID, []enrollment.SubmittedOfferingChoice{{CareOfferingID: own.offeringID}})
		require.Error(t, err, "tenant-safe child FK must reject foreign children")
		err = module.RecordSubmittedOfferingChoices(own.ctx, own.childID, []enrollment.SubmittedOfferingChoice{{CareOfferingID: foreign.offeringID}})
		require.Error(t, err, "tenant-safe offering FK must reject foreign offerings")
	}
}
