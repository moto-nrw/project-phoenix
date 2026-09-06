package integration

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type schemaRepointFailure struct {
	*postgres.Store
	before  bool
	failure error
}

func (r *schemaRepointFailure) RepointPhaseSchemas(ctx context.Context, ids []int64, target int64) (int64, error) {
	if r.before && r.failure != nil {
		return 0, r.failure
	}
	changed, err := r.Store.RepointPhaseSchemas(ctx, ids, target)
	if err != nil {
		return 0, err
	}
	return changed, r.failure
}

func TestSchemaPublishStandaloneRollsBackEveryWrite(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	for _, stage := range []string{"after schema insert", "after phase repoint"} {
		t.Run(stage, func(t *testing.T) {
			testpkg.OwnTenant(t)
			ctx := testpkg.Ctx(t)
			actor := testpkg.CreateTestAccount(t, db, "schema-rollback")
			phase := testpkg.CreateTestEnrollmentPhase(t, db)
			failure := errors.New("injected schema publish failure")
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
			engine := &schemaRepointFailure{Store: store, before: stage == "after schema insert", failure: failure}
			module := enrollment.NewModule(engine, tenant.NewTransactionRunner())
			input := enrollment.FormSchema{Name: "Application", CreatedBy: actor.ID}
			first, err := module.PublishSchema(ctx, input)
			require.NoError(t, err)
			ownerPhase, err := module.Phase(ctx, phase.ID)
			require.NoError(t, err)
			ownerPhase.FormSchemaID = &first.ID
			require.NoError(t, module.UpdatePhase(ctx, ownerPhase))
			_, err = module.PublishSchema(ctx, input)
			require.ErrorIs(t, err, failure)
			versions, err := module.SchemaVersions(ctx)
			require.NoError(t, err)
			require.Len(t, versions, 1, "the inserted version must roll back")
			require.Equal(t, first.ID, versions[0].ID)
			current, err := module.Phase(ctx, phase.ID)
			require.NoError(t, err)
			require.Equal(t, &first.ID, current.FormSchemaID, "the phase update must roll back")
			engine.failure = nil
			retried, err := module.PublishSchema(ctx, input)
			require.NoError(t, err)
			require.Equal(t, 2, retried.Version, "rollback must not consume a lineage version")
			versions, err = module.SchemaVersions(ctx)
			require.NoError(t, err)
			require.Len(t, versions, 2)
			current, err = module.Phase(ctx, phase.ID)
			require.NoError(t, err)
			require.Equal(t, &retried.ID, current.FormSchemaID)
		})
	}
}
