package migrations

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The lunch of the holiday care (#5631): the schema accepts a meal line and
// position, and the migration runs down and up again. The down keeps the
// lunch of weeks and bookings, which the repeatable home school function
// reads, so installing the repeatable definitions after it still works.
func TestHolidayLunchMigrationRunsDownAndUpAgain(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()

	constraint := func(table, name string) string {
		var definition string
		require.NoError(t, db.NewRaw(`SELECT pg_get_constraintdef(oid) FROM pg_constraint
			WHERE conrelid = ?::regclass AND conname = ?`, table, name).Scan(ctx, &definition))
		return definition
	}
	column := func(schema, table, name string) bool {
		var exists bool
		require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM information_schema.columns
			WHERE table_schema = ? AND table_name = ? AND column_name = ?)`, schema, table, name).Scan(ctx, &exists))
		return exists
	}
	homeSchoolColumns := func() string {
		var result string
		require.NoError(t, db.NewRaw(`SELECT pg_get_function_result('enrollment.holiday_home_school_bookings()'::regprocedure)`).Scan(ctx, &result))
		return result
	}

	assert.Contains(t, constraint("parent_billing.holiday_contribution_lines", "chk_holiday_contribution_lines_kind"), "'meal'")
	assert.Contains(t, constraint("parent_billing.document_positions", "chk_document_positions_kind"), "'meal'")
	assert.Contains(t, constraint("parent_billing.revenue_accounts", "chk_revenue_accounts_scope"), "'holiday_meals'")
	assert.True(t, column("parent_billing", "document_positions", "booked_days"))
	assert.Contains(t, homeSchoolColumns(), "lunch boolean", "the home school reads whether its child eats lunch")

	require.NoError(t, holidayLunchDown(ctx, db))
	assert.NotContains(t, constraint("parent_billing.holiday_contribution_lines", "chk_holiday_contribution_lines_kind"), "'meal'")
	assert.NotContains(t, constraint("parent_billing.revenue_accounts", "chk_revenue_accounts_scope"), "'holiday_meals'")
	assert.False(t, column("parent_billing", "document_positions", "booked_days"))
	assert.False(t, column("enrollment", "holiday_guests", "lunch"))
	assert.True(t, column("enrollment", "holiday_registration_bookings", "lunch"), "the repeatable home school function reads it")
	assert.True(t, column("enrollment", "holiday_period_weeks", "lunch_price_cents"), "the repeatable home school function reads it")
	require.NoError(t, applyRepeatableDefinitions(ctx, db), "the repeatable definitions install after the down migration")

	require.NoError(t, holidayLunchUp(ctx, db))
	require.NoError(t, applyRepeatableDefinitions(ctx, db))
	assert.Contains(t, constraint("parent_billing.holiday_contribution_lines", "chk_holiday_contribution_lines_kind"), "'meal'")
	assert.True(t, column("enrollment", "holiday_guests", "lunch"))
	assert.Contains(t, homeSchoolColumns(), "lunch boolean")
}

// The down migration refuses while a lunch is billed: dropping the kind would
// lose what a frozen contribution or an issued invoice holds.
func TestHolidayLunchDownRefusesWhileALunchIsBilled(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()

	// The check alone, in a transaction that rolls back: a contribution line
	// needs its whole chain of Träger, period, registration and child, and
	// the refusal reads nothing but the kind.
	err := db.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		if _, err := tx.ExecContext(ctx, `ALTER TABLE parent_billing.holiday_contribution_lines
			DROP CONSTRAINT fk_holiday_contribution_lines_contribution`); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.holiday_contribution_lines
				(organization_id, holiday_contribution_id, position, kind, amount_cents, week_starts_on, week_ends_on, booked_days)
			VALUES (0, 0, 1, 'meal', 2150, '2027-07-12', '2027-07-16', 5)`); err != nil {
			return err
		}
		_, err := tx.ExecContext(ctx, holidayLunchDownSQL)
		return err
	})
	require.ErrorContains(t, err, "holiday lunch is billed")
	var kept bool
	require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_constraint
		WHERE conname = 'chk_holiday_contribution_lines_meal')`).Scan(ctx, &kept))
	assert.True(t, kept, "the refused down migration changed nothing")
}
