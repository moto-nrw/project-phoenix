package contracttest_test

import (
	"context"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-ogs/project-phoenix/modules/careplan/compose"
	enrollmentTest "github.com/moto-ogs/project-phoenix/modules/enrollment/enrollmenttest"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPhaseDeleteProtectsMovedTraegerOffering(t *testing.T) {
	t.Parallel()
	env := setupDecisionTest(t)
	phases := boundsPhases(t, env, nil)
	catalog := testCareOfferingCatalog(t, env.db)
	copies, err := careplanCompose.NewTraegerOfferingCopies(catalog, func(context.Context) error { return nil })
	require.NoError(t, err)
	var id int64
	require.NoError(t, fixtureEnrollmentCommand(testpkg.Ctx(t), func(ctx context.Context) error {
		copy, err := copies.ApplyTraegerOffering(ctx, careplan.TraegerOfferingCopy{SourceID: testpkg.UniqueTestTenantID(t), Version: 1, PhaseID: env.sourcePhase.ID, Fields: traegerOfferingDraft("Geschützte Kopie", 20).Fields})
		if err != nil {
			return err
		}
		id = copy.CopyID
		ordinary := *env.sourcePhase
		ordinary.ID = 0
		ordinary.Name = "Eigene Phase"
		created, err := phases.CreatePhase(ctx, &ordinary)
		if err != nil {
			return err
		}
		offering, err := catalog.FindOffering(ctx, id)
		if err != nil {
			return err
		}
		offering.PhaseID = created.ID
		if _, err = catalog.UpdateOffering(ctx, offering); err != nil {
			return err
		}
		err = phases.DeletePhase(ctx, created.ID)
		assert.ErrorIs(t, err, enrollmentTest.ErrTraegerCopyDelete)
		kept, readErr := catalog.FindOffering(ctx, id)
		require.NoError(t, readErr)
		assert.NotNil(t, kept.Traeger)
		return nil
	}))
}
