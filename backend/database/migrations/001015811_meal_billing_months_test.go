package migrations

import (
	"context"
	"fmt"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestMealBillingErasureExtendsTheErasureOf809 keeps the erasure of 1.15.811
// the erasure of 1.15.809 with the statements for the Mensa months and the
// attendance retention marks.
func TestMealBillingErasureExtendsTheErasureOf809(t *testing.T) {
	t.Parallel()
	require.Equal(t, 1, strings.Count(demoVisitorErasureCareLettersUpSQL, mealBillingErasureAnchor))
	assert.Equal(t, strings.Replace(demoVisitorErasureCareLettersUpSQL, mealBillingErasureAnchor,
		mealBillingErasureAnchor+mealBillingErasureStatement, 1), demoVisitorErasureMealBillingSQL)
}

type mealBillingFixture struct {
	own, other             int64
	ownSchool, otherSchool int64
	ownMonth, otherMonth   int64
}

// newMealBillingFixture creates two Träger with a school each and one
// released March 2026 per school, inserted as the superuser.
func newMealBillingFixture(t *testing.T, db *bun.DB) mealBillingFixture {
	t.Helper()
	ctx := context.Background()
	suffix := testpkg.UniqueTestTenantID(t)
	f := mealBillingFixture{}
	f.own = testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Mensa", Slug: fmt.Sprintf("traeger-mensa-%d", suffix), Active: true}).ID
	f.other = testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Fremd", Slug: fmt.Sprintf("traeger-mensa-fremd-%d", suffix), Active: true}).ID
	f.ownSchool = testpkg.CreateTestSchoolInOrganization(t, db, f.own, "Mensa-Schule")
	f.otherSchool = testpkg.CreateTestSchoolInOrganization(t, db, f.other, "Fremde Mensa-Schule")
	for _, entry := range []struct {
		organization, school int64
		month                *int64
		student              string
	}{{f.own, f.ownSchool, &f.ownMonth, "Mia"}, {f.other, f.otherSchool, &f.otherMonth, "Ben"}} {
		require.NoError(t, db.NewRaw(`INSERT INTO schedule.meal_billing_months (organization_id, school_id, billing_month, child_count, meal_count)
			VALUES (?, ?, '2026-03-01', 1, 2) RETURNING id`, entry.organization, entry.school).Scan(ctx, entry.month))
		_, err := db.ExecContext(ctx, `INSERT INTO schedule.meal_billing_month_children
			(organization_id, month_id, student_id, first_name, last_name, school_class, registered_days, meal_days)
			VALUES (?, ?, 7, ?, 'Muster', '2a', 3, 2)`, entry.organization, *entry.month, entry.student)
		require.NoError(t, err)
	}
	return f
}

func countRows(t *testing.T, tx bun.IDB, query string, args ...any) int {
	t.Helper()
	var count int
	require.NoError(t, tx.NewRaw(query, args...).Scan(context.Background(), &count))
	return count
}

// TestMealBillingMonthsBelongToTheReleasingSchool pins the school policy of
// 1.15.811: a school reads and writes only its own releases, names only its
// own Träger, and changes or deletes none.
func TestMealBillingMonthsBelongToTheReleasingSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	f := newMealBillingFixture(t, db)

	asRole(t, db, "app.current_tenant_id", f.ownSchool, func(tx bun.Tx) {
		assert.Equal(t, 1, countRows(t, tx, `SELECT COUNT(*) FROM schedule.meal_billing_months`), "the school reads its own release")
		assert.Equal(t, 1, countRows(t, tx, `SELECT COUNT(*) FROM schedule.meal_billing_month_children`), "and only its children")
		var org int64
		require.NoError(t, tx.NewRaw(`SELECT schedule.meal_billing_school_organization(?)`, f.ownSchool).Scan(ctx, &org))
		assert.Equal(t, f.own, org)

		_, err := tx.ExecContext(ctx, `SAVEPOINT s`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO schedule.meal_billing_months (organization_id, school_id, billing_month, child_count, meal_count)
			VALUES (?, ?, '2026-04-01', 0, 0)`, f.other, f.ownSchool)
		assert.Equal(t, "42501", testpkg.PostgresSQLState(err), "a release names the school's own Träger")
		_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT s`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO schedule.meal_billing_months (organization_id, school_id, billing_month, child_count, meal_count)
			VALUES (?, ?, '2026-04-01', 0, 0)`, f.other, f.otherSchool)
		assert.Equal(t, "42501", testpkg.PostgresSQLState(err), "a school releases no month of another school")
		_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT s`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO schedule.meal_billing_month_children
			(organization_id, month_id, student_id, first_name, last_name, registered_days, meal_days) VALUES (?, ?, 8, 'X', 'Y', 1, 1)`, f.other, f.otherMonth)
		assert.Equal(t, "42501", testpkg.PostgresSQLState(err), "a school adds no child to another school's release")
		_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT s`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `UPDATE schedule.meal_billing_month_children SET meal_days = 3 WHERE month_id = ?`, f.ownMonth)
		assert.Equal(t, "42501", testpkg.PostgresSQLState(err), "a release never changes")
		_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT s`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `DELETE FROM schedule.meal_billing_months WHERE id = ?`, f.ownMonth)
		assert.Equal(t, "42501", testpkg.PostgresSQLState(err), "a school deletes no release")
		_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT s`)
		require.NoError(t, err)
		var none []int64
		require.NoError(t, tx.NewRaw(`SELECT student_id FROM schedule.traeger_meal_billing_children('2026-12-31')`).Scan(ctx, &none))
		assert.Empty(t, none, "the Träger read answers nothing in a school transaction")
	})
	asRole(t, db, "app.current_tenant_id", f.otherSchool, func(tx bun.Tx) {
		assert.Equal(t, 1, countRows(t, tx, `SELECT COUNT(*) FROM schedule.meal_billing_months WHERE school_id = ?`, f.otherSchool))
		assert.Zero(t, countRows(t, tx, `SELECT COUNT(*) FROM schedule.meal_billing_months WHERE school_id = ?`, f.ownSchool),
			"another school sees no release of this one")
	})
}

// TestTraegerReadsOnlyTheMealsOfItsOwnSchools runs the Träger's read below
// RLS (as the superuser, with the session settings alone) and as the request
// role: the function itself filters on the Träger of the session, so without
// that filter the other Träger's release would show.
func TestTraegerReadsOnlyTheMealsOfItsOwnSchools(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	f := newMealBillingFixture(t, db)

	type row struct {
		SchoolID  int64  `bun:"school_id"`
		FirstName string `bun:"first_name"`
		MealDays  int    `bun:"meal_days"`
	}
	read := func(tx bun.IDB, until string) []row {
		var rows []row
		require.NoError(t, tx.NewRaw(`SELECT school_id, first_name, meal_days FROM schedule.traeger_meal_billing_children(?)`, until).Scan(ctx, &rows))
		return rows
	}
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `SELECT set_config('app.current_org_id', ?, true)`, fmt.Sprint(f.own))
	require.NoError(t, err)
	assert.Equal(t, []row{{SchoolID: f.ownSchool, FirstName: "Mia", MealDays: 2}}, read(tx, "2026-03-31"),
		"below RLS the function still answers only the Träger of the session")
	assert.Empty(t, read(tx, "2026-02-28"), "nothing after the day asked")

	asRole(t, db, "app.current_org_id", f.other, func(tx bun.Tx) {
		assert.Equal(t, []row{{SchoolID: f.otherSchool, FirstName: "Ben", MealDays: 2}}, read(tx, "2026-03-31"))
		assert.Zero(t, countRows(t, tx, `SELECT COUNT(*) FROM schedule.meal_billing_months`), "the Träger reads no release table directly")
	})
}

// TestMealPositionsBillAChildMonthOnceAndNeverChange pins the Träger tables
// of 1.15.811: the Träger policy, one position per child and Mensa month and
// the refusal of any change.
func TestMealPositionsBillAChildMonthOnceAndNeverChange(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	f := newMealBillingFixture(t, db)
	var runID, invoiceID int64
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_runs (organization_id, billing_month) VALUES (?, '2026-04-01') RETURNING id`, f.own).Scan(ctx, &runID))
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_invoices (organization_id, run_id, number, number_year, number_sequence, issued_on,
		billing_month, care_contract_id, school_id, school_name, child_first_name, child_last_name, recipient_first_name,
		recipient_last_name, recipient_email, month_rule, total_cents)
		VALUES (?, ?, '2600001', 2026, 1, '2026-04-01', '2026-04-01', 1, ?, 'Mensa-Schule', 'Mia', 'Muster', 'Eva', 'Muster', '', 'version_on_first', 960)
		RETURNING id`, f.own, runID, f.ownSchool).Scan(ctx, &invoiceID))
	insert := `INSERT INTO parent_billing.care_meal_positions (organization_id, invoice_id, position, school_id, student_id, meal_month,
		mode, quantity, unit_cents, amount_cents, net_cents) VALUES (?, ?, ?, ?, 7, '2026-03-01', 'per_meal', 2, 480, 960, 960)`

	asRole(t, db, "app.current_org_id", f.own, func(tx bun.Tx) {
		_, err := tx.ExecContext(ctx, insert, f.own, invoiceID, 1, f.ownSchool)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `SAVEPOINT s`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, insert, f.own, invoiceID, 2, f.ownSchool)
		assert.ErrorContains(t, err, "uq_care_meal_positions_child_month", "a child's Mensa month is billed once")
		_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT s`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `UPDATE parent_billing.care_meal_positions SET quantity = 3 WHERE invoice_id = ?`, invoiceID)
		assert.Error(t, err, "an issued Mensa position never changes")
		_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT s`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.care_meal_tariffs (organization_id, school_id, valid_from, mode, amount_cents)
			VALUES (?, ?, '2026-01-01', 'per_meal', 480)`, f.other, f.otherSchool)
		assert.Equal(t, "42501", testpkg.PostgresSQLState(err), "the Träger sets no price of another Träger")
	})
	asRole(t, db, "app.current_org_id", f.other, func(tx bun.Tx) {
		assert.Zero(t, countRows(t, tx, `SELECT COUNT(*) FROM parent_billing.care_meal_positions`), "another Träger sees no Mensa position")
	})
	asRole(t, db, "app.current_tenant_id", f.ownSchool, func(tx bun.Tx) {
		assert.Zero(t, countRows(t, tx, `SELECT COUNT(*) FROM parent_billing.care_meal_tariffs`), "a school sees no Mensa price")
	})
}
