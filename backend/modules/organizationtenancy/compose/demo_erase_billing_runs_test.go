package compose

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// The demo erasure takes the billing runs it leaves without an invoice, a
// Jahresbescheid or a school (#6627, decision of 11.11.): such a run goes
// whole, a released one too, as its invoices already go with the school. A
// run that keeps content of another school stays; an open one of other
// schools only loses the retired school. A real Träger is never touched.

// careRunOf writes a monthly run of the Träger of the first school in
// status with its schools and one invoice per school of invoices, billed
// to a contract of that school. Each open run takes its own month, as a
// school is in one open run per month.
func careRunOf(t *testing.T, db *bun.DB, status string, month int, schools, invoices []int64) int64 {
	t.Helper()
	ctx := t.Context()
	var id int64
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_runs (organization_id, billing_month, status, released_at, issued_on,
			document_count, total_cents, month_rule, discarded_at)
		SELECT organization_id, make_date(2027, ?1, 1), ?2,
			CASE WHEN ?2 = 'released' THEN NOW() END, CASE WHEN ?2 = 'released' THEN make_date(2027, ?1, 1) END,
			CASE WHEN ?2 = 'released' THEN ?3::int END, CASE WHEN ?2 = 'released' THEN 3200 * ?3::bigint END,
			CASE WHEN ?2 = 'released' THEN 'version_on_first' END, CASE WHEN ?2 = 'discarded' THEN NOW() END
		FROM platform.schools WHERE id = ?0 RETURNING id`, schools[0], month, status, len(invoices)).Scan(ctx, &id))
	for _, school := range schools {
		_, err := db.NewRaw(`INSERT INTO parent_billing.care_run_schools (organization_id, run_id, school_id, billing_month, open)
			SELECT organization_id, ?1, id, make_date(2027, ?2, 1), ?3 FROM platform.schools WHERE id = ?0`,
			school, id, month, status == "calculated").Exec(ctx)
		require.NoError(t, err)
	}
	for i, school := range invoices {
		_, err := db.NewRaw(`INSERT INTO parent_billing.care_invoices (organization_id, run_id, number, number_year, number_sequence,
				issued_on, billing_month, care_contract_id, school_id, school_name, child_first_name, child_last_name,
				recipient_first_name, recipient_last_name, recipient_email, month_rule, total_cents)
			SELECT contract.organization_id, ?1, 'L-' || ?1 || '-' || ?2, 2027, ?1 * 10 + ?2, make_date(2027, ?3, 1),
				make_date(2027, ?3, 1), contract.id, ?0, 'OGS', 'Mia', 'Schneider', 'Eva', 'Muster', 'eva@example.org',
				'version_on_first', 3200
			FROM care_contract.contracts AS contract WHERE contract.school_id = ?0 ORDER BY contract.id LIMIT 1`,
			school, id, i, month).Exec(ctx)
		require.NoError(t, err)
	}
	return id
}

// noticeRunOf writes a Jahreslauf of the Träger of the first school in
// status for year and schools, without a Jahresbescheid.
func noticeRunOf(t *testing.T, db *bun.DB, status string, year int, schools []int64) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.care_notice_runs (organization_id, billing_year, period_from, period_until,
			school_ids, status, released_at, issued_on, notice_count, total_cents, discarded_at)
		SELECT organization_id, ?1, make_date(?1, 1, 1), make_date(?1, 12, 31), ?2, ?3,
			CASE WHEN ?3 = 'released' THEN NOW() END, CASE WHEN ?3 = 'released' THEN make_date(?1, 1, 1) END,
			CASE WHEN ?3 = 'released' THEN 0 END, CASE WHEN ?3 = 'released' THEN 0 END,
			CASE WHEN ?3 = 'discarded' THEN NOW() END
		FROM platform.schools WHERE id = ?0 RETURNING id`, schools[0], year, pgdialect.Array(schools), status).Scan(t.Context(), &id))
	return id
}

// runOfDocument is the holiday or monthly run of the invoice the visitor
// fixture issued for the school.
func runOfDocument(t *testing.T, db *bun.DB, table, number string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`SELECT run_id FROM `+table+` WHERE number = ?`, number).Scan(t.Context(), &id))
	return id
}

func runSchools(t *testing.T, db *bun.DB, runID int64) []int64 {
	t.Helper()
	var schools []int64
	require.NoError(t, db.NewRaw(`SELECT school_id FROM parent_billing.care_run_schools WHERE run_id = ? ORDER BY school_id`, runID).
		Scan(t.Context(), &schools))
	return schools
}

func noticeRunSchools(t *testing.T, db *bun.DB, runID int64) []int64 {
	t.Helper()
	var schools []int64
	require.NoError(t, db.NewRaw(`SELECT unnest(school_ids) FROM parent_billing.care_notice_runs WHERE id = ?`, runID).
		Scan(t.Context(), &schools))
	return schools
}

func TestDemoErasureTakesTheRunsItLeavesEmpty(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	db, hidden := order.db, order.schoolID
	open := schoolOfOrganization(t, db, hidden, false)
	leaveVisitorTraces(t, db, open)

	// The visitor fixture issued a monthly invoice, two Jahresbescheide and
	// a holiday invoice of the retired school, each in a released run of
	// its own; the open school has the same.
	releasedCare := runOfDocument(t, db, "parent_billing.care_invoices", fmt.Sprintf("M-%d", hidden))
	releasedNotice := runOfDocument(t, db, "parent_billing.care_notices", fmt.Sprintf("JB-%d-2026", hidden))
	releasedHoliday := runOfDocument(t, db, "parent_billing.documents", fmt.Sprintf("F-%d", hidden))
	kept := []int64{
		runOfDocument(t, db, "parent_billing.care_invoices", fmt.Sprintf("M-%d", open)),
		runOfDocument(t, db, "parent_billing.care_notices", fmt.Sprintf("JB-%d-2026", open)),
	}
	keptHoliday := runOfDocument(t, db, "parent_billing.documents", fmt.Sprintf("F-%d", open))

	releasedWithoutInvoice := careRunOf(t, db, "released", 1, []int64{hidden}, nil)
	openOfTheSchool := careRunOf(t, db, "calculated", 2, []int64{hidden}, nil)
	discardedOfTheSchool := careRunOf(t, db, "discarded", 3, []int64{hidden}, nil)
	releasedMixed := careRunOf(t, db, "released", 4, []int64{hidden, open}, []int64{hidden, open})
	releasedEmptied := careRunOf(t, db, "released", 5, []int64{hidden, open}, []int64{hidden})
	openMixed := careRunOf(t, db, "calculated", 6, []int64{hidden, open}, nil)
	openNotice := noticeRunOf(t, db, "calculated", 2030, []int64{hidden})
	openMixedNotice := noticeRunOf(t, db, "calculated", 2031, []int64{open, hidden})
	releasedOpenNotice := noticeRunOf(t, db, "released", 2032, []int64{open, hidden})

	_, erased := retireNow(t, order.queue, nil)
	require.Equal(t, 1, erased)

	assert.Zero(t, remainingRows(t, db, "parent_billing.care_runs", []int64{
		releasedCare, releasedWithoutInvoice, openOfTheSchool, discardedOfTheSchool, releasedEmptied}),
		"a monthly run left without an invoice or a school goes, released, open or discarded")
	assert.Zero(t, remainingRows(t, db, "parent_billing.care_notice_runs", []int64{releasedNotice, openNotice}),
		"a Jahreslauf left without a Jahresbescheid or a school goes")
	assert.Zero(t, remainingRows(t, db, "parent_billing.runs", []int64{releasedHoliday}),
		"a holiday run left without an invoice goes")
	var trail int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM parent_billing.care_run_events WHERE run_id = ?`, releasedCare).Scan(t.Context(), &trail))
	assert.Zero(t, trail, "with its trail")

	assert.Equal(t, 2, remainingRows(t, db, "parent_billing.care_runs", []int64{kept[0], releasedMixed}),
		"a monthly run that keeps an invoice of another school stays")
	assert.Equal(t, []int64{hidden, open}, runSchools(t, db, releasedMixed), "a released run never changes, its schools neither")
	var invoices, total int
	require.NoError(t, db.NewRaw(`SELECT document_count, total_cents FROM parent_billing.care_runs WHERE id = ?`, releasedMixed).
		Scan(t.Context(), &invoices, &total))
	assert.Equal(t, []int{2, 6400}, []int{invoices, total}, "its sums stay as released")
	assert.Equal(t, []int64{open}, runSchools(t, db, openMixed), "an open run of other schools only loses the retired school")
	assert.Equal(t, 2, remainingRows(t, db, "parent_billing.care_notice_runs", []int64{kept[1], releasedOpenNotice}))
	assert.Equal(t, []int64{open}, noticeRunSchools(t, db, openMixedNotice), "an open Jahreslauf only loses the retired school")
	assert.Equal(t, []int64{open, hidden}, noticeRunSchools(t, db, releasedOpenNotice), "a released Jahreslauf never changes")
	assert.Equal(t, 1, remainingRows(t, db, "parent_billing.runs", []int64{keptHoliday}))
}

// runsOfOrganization holds every run of the Träger with its schools, as
// JSON, so a test sees any change.
func runsOfOrganization(t *testing.T, db *bun.DB, organizationID int64) []string {
	t.Helper()
	var rows []string
	require.NoError(t, db.NewRaw(`
		SELECT 'care ' || to_jsonb(run)::text || ' ' || COALESCE((SELECT jsonb_agg(to_jsonb(school) ORDER BY school.school_id)::text
			FROM parent_billing.care_run_schools AS school WHERE school.run_id = run.id), '')
		FROM parent_billing.care_runs AS run WHERE run.organization_id = ?0
		UNION ALL SELECT 'notice ' || to_jsonb(run)::text FROM parent_billing.care_notice_runs AS run WHERE run.organization_id = ?0
		UNION ALL SELECT 'holiday ' || to_jsonb(run)::text FROM parent_billing.runs AS run WHERE run.organization_id = ?0
		ORDER BY 1`, organizationID).Scan(t.Context(), &rows))
	return rows
}

// The runs of a real Träger stay as they are whatever state they are in,
// even a released run whose invoices are gone: the erasure takes only the
// runs of the retired demo school.
func TestDemoErasureNeverTouchesTheRunsOfARealTraeger(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	db, real := order.db, order.realID
	leaveVisitorTraces(t, db, real)
	careRunOf(t, db, "released", 1, []int64{real}, nil)
	careRunOf(t, db, "calculated", 2, []int64{real}, nil)
	careRunOf(t, db, "discarded", 3, []int64{real}, nil)
	noticeRunOf(t, db, "released", 2030, []int64{real})
	noticeRunOf(t, db, "calculated", 2031, []int64{real})
	_, err := db.NewRaw(`INSERT INTO parent_billing.runs (organization_id, service, holiday_period_id, period_name, service_from,
			service_until, status, released_at, issued_on, document_count, total_cents)
		SELECT organization_id, 'holiday', ?0, 'Herbstferien', DATE '2027-10-11', DATE '2027-10-22', 'released', NOW(),
			DATE '2027-09-20', 1, 12000
		FROM platform.schools WHERE id = ?0`, real).Exec(t.Context())
	require.NoError(t, err, "a released holiday run whose invoice is gone")
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, real).Scan(t.Context(), &organization))
	before := runsOfOrganization(t, db, organization)
	require.Len(t, before, 10, "six runs and the four of the visitor fixture")

	_, erased := retireNow(t, order.queue, nil)
	require.Equal(t, 1, erased)
	assert.Equal(t, before, runsOfOrganization(t, db, organization), "no run of a real Träger changes")
}

// A run another transaction holds defers the whole order unchanged, as
// every lock of the erasure does (#5540): it waits at most the lock timeout
// and never erases a part. The next run erases the order with the run.
func TestDemoErasureDefersAnOrderWhoseRunIsHeld(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	run := careRunOf(t, order.db, "calculated", 2, []int64{order.schoolID}, nil)
	before := order.snapshot(t)
	hold, err := order.db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = hold.Rollback() })
	_, err = hold.NewRaw(`SELECT id FROM parent_billing.care_runs WHERE id = ? FOR UPDATE`, run).Exec(t.Context())
	require.NoError(t, err)

	_, erased := retireNow(t, order.queue, nil)
	assert.Zero(t, erased)
	assert.Equal(t, before, order.snapshot(t), "the deferred order is unchanged")
	assert.Equal(t, 1, remainingRows(t, order.db, "parent_billing.care_runs", []int64{run}))
	require.NoError(t, hold.Rollback())

	order.requireErasedNext(t)
	assert.Zero(t, remainingRows(t, order.db, "parent_billing.care_runs", []int64{run}))
}
