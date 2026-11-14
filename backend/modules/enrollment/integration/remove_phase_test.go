package integration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	"github.com/moto-ogs/project-phoenix/modules/enrollment/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type phaseRemovalFailure struct {
	*postgres.Store
	failure error
}

func (s *phaseRemovalFailure) DeletePhase(ctx context.Context, id int64) error {
	if err := s.Store.DeletePhase(ctx, id); err != nil {
		return err
	}
	return s.failure
}

func newRemovalStore() *postgres.Store {
	return postgres.NewStore(func(ctx context.Context) (bun.IDB, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, errors.New("transaction required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, errors.New("unexpected transaction type")
		}
		return tx, nil
	}, func(ctx context.Context) (int64, error) {
		id, err := tenant.TenantFromContext(ctx)
		if err != nil {
			return 0, err
		}
		return id.Int64(), nil
	})
}

// RemovePhase refuses a phase that holds enrollment requests and keeps the
// request, its child and its guardians (#6836): the requests are evidence
// and never leave with their phase.
func TestRemovePhaseKeepsPhaseWithRequests(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.OwnTenant(t)
	ctx := testpkg.Ctx(t)
	module := compose.NewWithStore(newRemovalStore(), nil)
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	request := &enrollment.Request{PhaseID: phase.ID, GuardianFirstName: "First", GuardianLastName: "Last", GuardianEmail: "guardian@example.test", StatusToken: fmt.Sprintf("remove-phase-%d", phase.ID)}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, module.InsertRequest, request))
	child := &enrollment.RequestChild{RequestID: request.ID, FirstName: "Child", LastName: "Last", DateOfBirth: "2020-03-29"}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, module.InsertChild, child))
	guardian := &enrollment.RequestGuardian{RequestID: request.ID, FirstName: "Co", LastName: "Guardian"}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, module.CreateRequestGuardian, guardian))

	foreignTenant, _ := testpkg.CreateTestTenant(t, db)
	require.ErrorIs(t, module.RemovePhase(testpkg.ContextForTenant(ctx, foreignTenant), phase.ID), sql.ErrNoRows)

	require.ErrorIs(t, module.RemovePhase(ctx, phase.ID), enrollment.ErrPhaseHasRequests)
	_, err := testpkg.TenantQuery1(t, ctx, db, module.Phase, phase.ID)
	require.NoError(t, err)
	_, err = testpkg.TenantQuery2(t, ctx, db, module.RequestByID, request.ID, false)
	require.NoError(t, err)
	_, err = testpkg.TenantQuery1(t, ctx, db, module.ChildByID, child.ID)
	require.NoError(t, err)
	guardians, err := testpkg.TenantQuery1(t, ctx, db, module.RequestGuardians, []int64{request.ID})
	require.NoError(t, err)
	require.Len(t, guardians, 1)
}

// A failed removal of an empty phase rolls back, and the retry removes it.
func TestRemovePhaseRollsBackAndRetries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	testpkg.OwnTenant(t)
	ctx := testpkg.Ctx(t)
	failure := errors.New("injected removal failure")
	engine := &phaseRemovalFailure{Store: newRemovalStore(), failure: failure}
	module := compose.NewWithStore(engine, nil)
	phase := testpkg.CreateTestEnrollmentPhase(t, db)

	require.ErrorIs(t, module.RemovePhase(ctx, phase.ID), failure)
	_, err := testpkg.TenantQuery1(t, ctx, db, module.Phase, phase.ID)
	require.NoError(t, err)

	engine.failure = nil
	require.NoError(t, module.RemovePhase(ctx, phase.ID))
	_, err = testpkg.TenantQuery1(t, ctx, db, module.Phase, phase.ID)
	require.ErrorIs(t, err, sql.ErrNoRows)
}
