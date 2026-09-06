package integration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type phaseRemovalFailure struct {
	*postgres.Store
	stage   string
	failure error
}

func (s *phaseRemovalFailure) DeletePhaseRequests(ctx context.Context, id int64) (int, error) {
	n, err := s.Store.DeletePhaseRequests(ctx, id)
	if err == nil && s.stage == "requests" {
		err = s.failure
	}
	return n, err
}

func (s *phaseRemovalFailure) DeletePhase(ctx context.Context, id int64) error {
	if err := s.Store.DeletePhase(ctx, id); err != nil {
		return err
	}
	if s.stage == "phase" {
		return s.failure
	}
	return nil
}

func TestRemovePhaseRollsBackEachWriteAndRetries(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for _, stage := range []string{"requests", "phase"} {
		t.Run(stage, func(t *testing.T) {
			testpkg.OwnTenant(t)
			ctx := testpkg.Ctx(t)
			store := postgres.NewStore(func(ctx context.Context) (bun.IDB, error) {
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
			failure := errors.New("injected removal failure")
			engine := &phaseRemovalFailure{Store: store, stage: stage, failure: failure}
			module := enrollment.NewModule(engine, tenant.NewTransactionRunner())
			phase := testpkg.CreateTestEnrollmentPhase(t, db)
			request := &enrollment.Request{PhaseID: phase.ID, GuardianFirstName: "First", GuardianLastName: "Last", GuardianEmail: "guardian@example.test", StatusToken: fmt.Sprintf("remove-phase-%d", phase.ID)}
			require.NoError(t, module.InsertRequest(ctx, request))
			child := &enrollment.RequestChild{RequestID: request.ID, FirstName: "Child", LastName: "Last", DateOfBirth: "2020-03-29"}
			require.NoError(t, module.InsertChild(ctx, child))
			guardian := &enrollment.RequestGuardian{RequestID: request.ID, FirstName: "Co", LastName: "Guardian"}
			require.NoError(t, module.CreateRequestGuardian(ctx, guardian))

			foreignTenant, _ := testpkg.CreateTestTenant(t, db)
			foreignCtx := testpkg.ContextForTenant(ctx, foreignTenant)
			_, err := module.RemovePhase(foreignCtx, phase.ID)
			require.ErrorIs(t, err, sql.ErrNoRows)

			count, err := module.RemovePhase(ctx, phase.ID)
			require.ErrorIs(t, err, failure)
			require.Zero(t, count, "a rolled-back delete must not report committed rows")
			_, err = module.Phase(ctx, phase.ID)
			require.NoError(t, err)
			_, err = module.RequestByID(ctx, request.ID, false)
			require.NoError(t, err)
			_, err = module.ChildByID(ctx, child.ID)
			require.NoError(t, err)
			guardians, err := module.RequestGuardians(ctx, []int64{request.ID})
			require.NoError(t, err)
			require.Len(t, guardians, 1)

			engine.failure = nil
			count, err = module.RemovePhase(ctx, phase.ID)
			require.NoError(t, err)
			require.Equal(t, 1, count)
			_, err = module.Phase(ctx, phase.ID)
			require.ErrorIs(t, err, sql.ErrNoRows)
			_, err = module.RequestByID(ctx, request.ID, false)
			require.ErrorIs(t, err, sql.ErrNoRows)
			_, err = module.ChildByID(ctx, child.ID)
			require.ErrorIs(t, err, sql.ErrNoRows)
			guardians, err = module.RequestGuardians(ctx, []int64{request.ID})
			require.NoError(t, err)
			require.Empty(t, guardians)
		})
	}
}
