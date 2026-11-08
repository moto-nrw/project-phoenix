package postgres

import (
	"context"
	"errors"
	"testing"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The cost bearer shares keep a Träger to its own shares, trail and issued
// shares below row level security (#4687): the Träger transaction drops its
// request role, so each assertion holds only because the adapter names the
// Träger in its statement. Removing one filter turns its assertion red. The
// writes go through the store, so the row model itself is covered too.

type costShareScope struct {
	careScope
	bearer int64
	share  int64
}

// seedCostShareScope writes a contract, a cost bearer, a share and an issued
// invoice share for the Träger in tx; with a store, the share goes through
// it (only the Träger of the transaction).
func seedCostShareScope(t *testing.T, ctx context.Context, tx bun.IDB, store *Store, scope careScope, name string) costShareScope {
	t.Helper()
	scope = seedCareScope(t, ctx, tx, scope, name)
	out := costShareScope{careScope: scope}
	require.NoError(t, tx.NewRaw(`INSERT INTO care_contract.contracts (organization_id, school_id, request_child_id)
		VALUES (?, ?, ?) RETURNING id`, scope.organization, scope.school, scope.contract).Scan(ctx, &out.contract))
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.cost_bearers (organization_id, name) VALUES (?, ?) RETURNING id`,
		scope.organization, "Jugendamt "+name).Scan(ctx, &out.bearer))
	if store != nil {
		var err error
		out.share, err = store.InsertCareCostShare(ctx, scope.organization, 1, costShareInput(out.contract, out.bearer))
		require.NoError(t, err, "InsertCareCostShare writes the row model")
	} else {
		require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.care_cost_shares (organization_id, care_contract_id, kind,
			cost_bearer_id, share_basis_points, valid_from, valid_until, reference, checked_on)
			VALUES (?, ?, 'youth_office', ?, 10000, DATE '2029-09-01', DATE '2030-07-31', 'Bescheid', DATE '2029-08-20') RETURNING id`,
			scope.organization, out.contract, out.bearer).Scan(ctx, &out.share))
	}
	var invoice int64
	require.NoError(t, tx.NewRaw(`SELECT id FROM parent_billing.care_invoices WHERE organization_id = ?`, scope.organization).
		Scan(ctx, &invoice))
	_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.care_invoice_cost_shares (organization_id, invoice_id, position, kind,
		cost_bearer_id, cost_bearer_name, share_basis_points, amount_cents) VALUES (?, ?, 1, 'youth_office', ?, 'Jugendamt', 10000, 3200)`,
		scope.organization, invoice, out.bearer)
	require.NoError(t, err)
	return out
}

func costShareInput(contract, bearer int64) parentbilling.CareCostShareInput {
	points := 10000
	return parentbilling.CareCostShareInput{ContractID: contract, Kind: parentbilling.CostShareYouthOffice, CostBearerID: bearer,
		ShareBasisPoints: &points, ValidFrom: calendar.NewDate(2029, 9, 1), ValidUntil: calendar.NewDate(2030, 7, 31),
		Reference: "Bescheid 4-12", CheckedOn: calendar.NewDate(2029, 8, 20)}
}

func TestCareCostShareFiltersKeepTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ownBase := newCareScope(t, db, "Träger A")
	otherBase := newCareScope(t, db, "Träger B")
	store := NewStore()
	rollback := errors.New("roll the rows back")

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, ownBase.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, ownBase.organization)
		require.NoError(t, err)
		var bypasses bool
		require.NoError(t, tx.NewRaw(`SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(ctx, &bypasses))
		require.True(t, bypasses, "the test runs below row level security")
		own := seedCostShareScope(t, ctx, tx, store, ownBase, "A")
		other := seedCostShareScope(t, ctx, tx, nil, otherBase, "B")

		all, err := store.CareCostShares(ctx, own.organization, nil)
		require.NoError(t, err)
		require.Len(t, all, 1, "CareCostShares of the Träger")
		assert.Equal(t, own.share, all[0].ID)
		assert.Equal(t, "Jugendamt A", all[0].CostBearerName, "the share carries its cost bearer's name")
		named, err := store.CareCostShares(ctx, own.organization, []int64{own.contract, other.contract})
		require.NoError(t, err)
		assert.Len(t, named, 1, "CareCostShares never answers another Träger's contract")

		_, err = store.UpdateCareCostShare(ctx, own.organization, 1, other.share, costShareInput(other.contract, other.bearer))
		assert.ErrorIs(t, err, parentbilling.ErrCareCostShareNotFound, "UpdateCareCostShare of another Träger")
		_, err = store.DeleteCareCostShare(ctx, own.organization, other.share)
		assert.ErrorIs(t, err, parentbilling.ErrCareCostShareNotFound, "DeleteCareCostShare of another Träger")

		var fields validation.Errors
		err = refused(t, ctx, tx, func() error {
			_, err := store.InsertCareCostShare(ctx, own.organization, 1, costShareInput(other.contract, own.bearer))
			return err
		})
		require.ErrorAs(t, err, &fields, "a share for another Träger's contract")
		assert.Contains(t, fields, "contract_id")
		err = refused(t, ctx, tx, func() error {
			foreign := costShareInput(own.contract, other.bearer)
			foreign.Kind = parentbilling.CostShareOtherSubsidy
			_, err := store.InsertCareCostShare(ctx, own.organization, 1, foreign)
			return err
		})
		require.ErrorAs(t, err, &fields, "a share naming another Träger's cost bearer")
		assert.Contains(t, fields, "cost_bearer_id")
		later := costShareInput(own.contract, own.bearer)
		later.ValidFrom = calendar.NewDate(2030, 1, 1)
		err = refused(t, ctx, tx, func() error {
			_, err := store.InsertCareCostShare(ctx, own.organization, 1, later)
			return err
		})
		assert.ErrorIs(t, err, parentbilling.ErrCareCostShareOverlap, "two shares of one kind on the same day")

		require.NoError(t, store.InsertCareCostShareEvents(ctx, own.organization, 1, "read", nil, []int64{own.contract}))

		var invoices []int64
		require.NoError(t, tx.NewRaw(`SELECT invoice_id FROM parent_billing.care_invoice_cost_shares ORDER BY organization_id = ? DESC`,
			own.organization).Scan(ctx, &invoices))
		issued, err := careCostSharesOf(ctx, tx, own.organization, invoices)
		require.NoError(t, err)
		assert.Len(t, issued, 1, "careCostSharesOf of the Träger")

		inUse, err := store.CostBearersInUse(ctx, own.organization)
		require.NoError(t, err)
		assert.Equal(t, map[int64]bool{own.bearer: true}, inUse, "CostBearersInUse names the shares of the Träger only")
		assert.ErrorIs(t, refused(t, ctx, tx, func() error { return store.DeleteCostBearer(ctx, own.organization, own.bearer) }),
			parentbilling.ErrCostBearerInUse,
			"a cost bearer a share names stays")

		deleted, err := store.DeleteCareCostShare(ctx, own.organization, own.share)
		require.NoError(t, err)
		assert.Equal(t, own.contract, deleted)
		var kept int
		require.NoError(t, tx.NewRaw(`SELECT count(*) FROM parent_billing.care_cost_shares WHERE organization_id = ?`, other.organization).
			Scan(ctx, &kept))
		assert.Equal(t, 1, kept, "the other Träger keeps its share")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// refused runs a write the database refuses under a savepoint, so the
// transaction goes on after it, and answers its error.
func refused(t *testing.T, ctx context.Context, tx bun.IDB, write func() error) error {
	t.Helper()
	_, err := tx.ExecContext(ctx, "SAVEPOINT refused")
	require.NoError(t, err)
	err = write()
	_, rollbackErr := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT refused")
	require.NoError(t, rollbackErr)
	return err
}
