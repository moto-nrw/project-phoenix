package migrations

import (
	"context"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestSchoolErasureRetainsBillingOnlyForRestrictedBillingRole(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	schoolA := testpkg.Tenant(t)
	schoolB, _ := testpkg.CreateTestTenant(t, db)
	seedErasureSchool(t, db, schoolA)
	seedErasureSchool(t, db, schoolB)
	var before, after string
	require.NoError(t, db.NewRaw(`SELECT jsonb_agg(to_jsonb(r) ORDER BY school_id)::text FROM platform.billing_key_date_counts r WHERE school_id IN (?,?)`, schoolA, schoolB).Scan(t.Context(), &before))
	result := eraseSchoolAsCLI(t, db, schoolA, beginErasure(t, db, schoolA))
	require.Equal(t, "db_done", result.Status)
	require.NotContains(t, result.RowCounts, "platform.billing_key_date_counts")
	require.NoError(t, db.NewRaw(`SELECT jsonb_agg(to_jsonb(r) ORDER BY school_id)::text FROM platform.billing_key_date_counts r WHERE school_id IN (?,?)`, schoolA, schoolB).Scan(t.Context(), &after))
	require.Equal(t, before, after, "immutable invoice reference survives erasure")
	require.NoError(t, db.RunInTx(t.Context(), nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_admin`)
		require.NoError(t, err)
		var count int
		require.NoError(t, tx.NewRaw(`SELECT count(*) FROM platform.live_billing_key_date_counts WHERE school_id IN (?,?)`, schoolA, schoolB).Scan(ctx, &count))
		require.Equal(t, 1, count, "regular operator billing cannot read erased schools")
		return nil
	}))
	require.Error(t, db.RunInTx(t.Context(), nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_admin`); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, `SELECT * FROM platform.billing_key_date_counts`)
		return err
	}), "BYPASSRLS must not circumvent the table privilege boundary")
	require.NoError(t, db.RunInTx(t.Context(), nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_billing_retention`)
		require.NoError(t, err)
		var count int
		require.NoError(t, tx.NewRaw(`SELECT count(*) FROM platform.billing_key_date_counts WHERE school_id=?`, schoolA).Scan(ctx, &count))
		require.Equal(t, 1, count)
		return nil
	}))
}

func TestSchoolErasureBillingExpiryUsesEndOfCalendarYear(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	schoolID := testpkg.Tenant(t)
	var year int
	require.NoError(t, db.NewRaw(`SELECT extract(year FROM clock_timestamp() AT TIME ZONE 'Europe/Berlin')::int`).Scan(t.Context(), &year))
	for _, y := range []int{year - 11, year - 10, year - 9} {
		day, err := calendar.ParseDate(fmt.Sprintf("%d-12-01", y))
		require.NoError(t, err)
		erasureExec(t, db, `INSERT INTO platform.billing_key_date_counts(school_id,period,key_date,school_name,organization_name,active_students,active_terminals)
 VALUES (?,?::date,?::date,'Fixture school','Fixture organization',1,1)`, schoolID, day, day)
	}
	orderID := beginErasure(t, db, schoolID)
	require.Equal(t, "db_done", eraseSchoolAsCLI(t, db, schoolID, orderID).Status)
	require.NoError(t, db.RunInTx(t.Context(), nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_billing_retention`)
		require.NoError(t, err)
		var removed int
		require.NoError(t, tx.NewRaw(`SELECT platform.delete_expired_billing_key_date_counts()`).Scan(ctx, &removed))
		require.Equal(t, 1, removed, "only the earlier calendar year's ten-year window has expired")
		require.NoError(t, tx.NewRaw(`SELECT platform.delete_expired_billing_key_date_counts()`).Scan(ctx, &removed))
		require.Zero(t, removed, "expiry is repeatable")
		var remaining int
		require.NoError(t, tx.NewRaw(`SELECT count(*) FROM platform.billing_key_date_counts WHERE school_id=?`, schoolID).Scan(ctx, &remaining))
		require.Equal(t, 2, remaining)
		return nil
	}))
}
