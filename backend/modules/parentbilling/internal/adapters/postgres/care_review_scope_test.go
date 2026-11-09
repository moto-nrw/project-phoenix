package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The review's own filters keep a Träger to its rows below row level
// security (#5916): the Träger transaction drops its request role, so each
// assertion holds only because the adapter names the Träger in its own
// statement. Removing one filter turns its assertion red.

// seedCareReviewScope gives the open run of the scope its school and an
// exclusion of its contract.
func seedCareReviewScope(t *testing.T, ctx context.Context, tx bun.IDB, scope careScope) {
	t.Helper()
	_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.care_run_schools (organization_id, run_id, school_id, billing_month)
		VALUES (?, ?, ?, DATE '2029-10-01')`, scope.organization, scope.open, scope.school)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.care_run_events (organization_id, run_id, action, details)
		VALUES (?, ?, 'exclude', jsonb_build_object('contract_ids', jsonb_build_array(?::bigint),
			'services', jsonb_build_array('care'), 'reason', 'Prüfung'))`,
		scope.organization, scope.open, scope.contract)
	require.NoError(t, err)
}

func TestCareReviewFiltersKeepTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newCareScope(t, db, "Träger A")
	other := newCareScope(t, db, "Träger B")
	store := NewStore()
	october := calendar.NewDate(2029, 10, 1)
	rollback := errors.New("roll the review rows back")

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own.organization)
		require.NoError(t, err)
		own = seedCareScope(t, ctx, tx, own, "Träger A")
		other = seedCareScope(t, ctx, tx, other, "Träger B")
		seedCareReviewScope(t, ctx, tx, own)
		seedCareReviewScope(t, ctx, tx, other)

		open, err := store.OpenCareRunSchools(ctx, own.organization, october)
		require.NoError(t, err)
		assert.Equal(t, map[int64]int64{own.school: own.open}, open, "OpenCareRunSchools")

		run, err := store.CareRun(ctx, own.organization, own.open, false)
		require.NoError(t, err)
		assert.Equal(t, []int64{own.school}, run.SchoolIDs, "the schools of a run are its own")

		exclusions, err := store.CareRunExclusions(ctx, own.organization, other.open)
		require.NoError(t, err)
		assert.Empty(t, exclusions, "CareRunExclusions of another Träger's run")
		exclusions, err = store.CareRunExclusions(ctx, own.organization, own.open)
		require.NoError(t, err)
		assert.Equal(t, "Prüfung", exclusions[parentbilling.CareInvoiceKey{ContractID: own.contract, Service: parentbilling.ServiceCare}].Reason)

		date := calendar.NewDate(2029, 10, 30)
		assert.ErrorIs(t, store.SetCareRunInvoiceDate(ctx, own.organization, other.open, &date), parentbilling.ErrBillingRunReleased,
			"SetCareRunInvoiceDate of another Träger's run")
		schools, err := closeCareRunSchools(ctx, tx, own.organization, other.open)
		require.NoError(t, err)
		assert.Empty(t, schools, "closeCareRunSchools of another Träger's run")

		var otherOpen bool
		require.NoError(t, tx.NewRaw(`SELECT open FROM parent_billing.care_run_schools WHERE run_id = ?`, other.open).Scan(ctx, &otherOpen))
		assert.True(t, otherOpen, "the other Träger's school stays open")
		var otherDate *calendar.Date
		require.NoError(t, tx.NewRaw(`SELECT invoice_date FROM parent_billing.care_runs WHERE id = ?`, other.open).Scan(ctx, &otherDate))
		assert.Nil(t, otherDate, "the other Träger's run keeps its date")

		// The open index is per Träger: the other Träger's open run of the
		// same school ID and month does not block this one.
		require.NoError(t, insertCareRunSchools(ctx, tx, own.organization, own.released, october, []int64{other.school}))
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// Transaction timestamps can run backwards relative to the run lock. The
// last appended action, not the latest transaction start, decides inclusion.
func TestCareReviewReplaysExclusionsInWriteOrder(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := newCareScope(t, db, "Träger Ausschluss Reihenfolge")
	store := NewStore()
	rollback := errors.New("roll the review rows back")
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, scope.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, scope.organization)
		require.NoError(t, err)
		scope = seedCareScope(t, ctx, tx, scope, "Träger Ausschluss Reihenfolge")
		_, err = store.CareRun(ctx, scope.organization, scope.open, true)
		require.NoError(t, err)
		for _, event := range []struct{ action, timestamp, reason string }{
			{"exclude", "2029-10-01 08:00:00+00", "Erste Prüfung"},
			{"include", "2029-10-01 10:00:00+00", ""},
			{"exclude", "2029-10-01 09:00:00+00", "Erneute Prüfung"},
		} {
			_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.care_run_events
				(organization_id, run_id, action, occurred_at, details)
				VALUES (?, ?, ?, ?::timestamptz,
				jsonb_build_object('contract_ids', jsonb_build_array(?::bigint), 'services', jsonb_build_array('care'), 'reason', ?::text))`,
				scope.organization, scope.open, event.action, event.timestamp, scope.contract, event.reason)
			require.NoError(t, err)
		}
		exclusions, err := store.CareRunExclusions(ctx, scope.organization, scope.open)
		require.NoError(t, err)
		assert.Equal(t, "Erneute Prüfung", exclusions[parentbilling.CareInvoiceKey{ContractID: scope.contract, Service: parentbilling.ServiceCare}].Reason,
			"a successful exclusion after an inclusion stays in force even if its transaction started earlier")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// An exclusion names one invoice of its contract (#6346): the Mensa invoice
// of its own stays out while the care invoice is taken back, and an entry of
// before (migration 1.15.865: both services) covers both invoices until each
// is returned on its own. An entry without a known service names none.
func TestCareRunExclusionsAreKeptPerInvoice(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := newCareScope(t, db, "Träger Ausschluss je Rechnung")
	store := NewStore()
	rollback := errors.New("roll the review rows back")
	care := parentbilling.CareInvoiceKey{ContractID: 0, Service: parentbilling.ServiceCare}
	meal := parentbilling.CareInvoiceKey{ContractID: 0, Service: parentbilling.ServiceMeal}
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, scope.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, scope.organization)
		require.NoError(t, err)
		scope = seedCareScope(t, ctx, tx, scope, "Träger Ausschluss je Rechnung")
		care.ContractID, meal.ContractID = scope.contract, scope.contract
		event := func(action, services, reason string) {
			t.Helper()
			_, err := tx.ExecContext(ctx, `INSERT INTO parent_billing.care_run_events (organization_id, run_id, action, details)
				VALUES (?, ?, ?, jsonb_build_object('contract_ids', jsonb_build_array(?::bigint), 'services', ?::jsonb, 'reason', ?::text))`,
				scope.organization, scope.open, action, scope.contract, services, reason)
			require.NoError(t, err)
		}
		read := func() map[parentbilling.CareInvoiceKey]parentbilling.CareExclusion {
			t.Helper()
			exclusions, err := store.CareRunExclusions(ctx, scope.organization, scope.open)
			require.NoError(t, err)
			return exclusions
		}

		event("exclude", `["meal"]`, "Mensa wird geprüft")
		exclusions := read()
		assert.Equal(t, "Mensa wird geprüft", exclusions[meal].Reason)
		assert.NotContains(t, exclusions, care, "excluding the Mensa invoice keeps the care invoice")

		event("exclude", `["care", "meal"]`, "Bestand")
		event("include", `["care"]`, "")
		exclusions = read()
		assert.NotContains(t, exclusions, care, "the return of the care invoice")
		assert.Equal(t, "Bestand", exclusions[meal].Reason, "the Mensa invoice stays out after the care returned")

		event("include", `["meal"]`, "")
		event("exclude", `["holiday"]`, "keine Rechnung des Laufs")
		assert.Empty(t, read(), "every invoice returned, and an unknown service names none")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
