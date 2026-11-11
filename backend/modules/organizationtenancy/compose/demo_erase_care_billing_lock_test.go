package compose

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// pendingCareRelease reproduces the release of a monthly run (#4684): it
// holds the monthly billing of the Träger, has read the contracts and
// inserts the invoice of the school's contract with the visitor's name.
func pendingCareRelease(t *testing.T, order demoOrder) (bun.Tx, int64) {
	t.Helper()
	ctx := t.Context()
	tx, err := order.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.NewRaw(`SELECT pg_advisory_xact_lock(hashtextextended('parent_billing.care_billing:' || organization_id::text, 0))
		FROM platform.schools WHERE id = ?`, order.schoolID).Exec(ctx)
	require.NoError(t, err)
	var invoiceID int64
	require.NoError(t, tx.NewRaw(`WITH contract AS (
			SELECT id, organization_id FROM care_contract.contracts WHERE school_id = ?0),
		run AS (
			INSERT INTO parent_billing.care_runs (organization_id, billing_month, status, released_at, issued_on,
				document_count, total_cents, month_rule)
			SELECT organization_id, DATE '2026-10-01', 'released', NOW(), DATE '2026-10-01', 1, 3200, 'version_on_first'
			FROM contract RETURNING id, organization_id)
		INSERT INTO parent_billing.care_invoices (organization_id, run_id, number, number_year, number_sequence, issued_on,
			billing_month, care_contract_id, school_id, school_name, child_first_name, child_last_name,
			recipient_first_name, recipient_last_name, recipient_email, month_rule, total_cents)
		SELECT run.organization_id, run.id, 'R-' || ?0, 2026, ?0 + 1, DATE '2026-10-01', DATE '2026-10-01', contract.id, ?0,
			'OGS', 'Mia', 'Schneider', ?1, ?2, 'eltern@example.org', 'version_on_first', 3200
		FROM run JOIN contract ON contract.organization_id = run.organization_id
		RETURNING id`, order.schoolID, visitorFirst, visitorLast).Scan(ctx, &invoiceID))
	return tx, invoiceID
}

func careInvoiceExists(t *testing.T, order demoOrder, invoiceID int64) bool {
	t.Helper()
	var count int
	require.NoError(t, order.db.NewRaw(`SELECT count(*) FROM parent_billing.care_invoices WHERE id = ?`, invoiceID).Scan(t.Context(), &count))
	return count > 0
}

// A release that read the contracts before the erasure must not leave its
// invoice behind: the erasure waits for the monthly billing of the Träger
// and deletes the invoice the release committed.
func TestDemoErasureIncludesAConcurrentMonthlyRelease(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	release, invoiceID := pendingCareRelease(t, order)
	order.waitForLocks(t)
	type outcome struct {
		erased int
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		_, erased, err := order.queue.RetireDemoSchools(t.Context(), nil)
		done <- outcome{erased: erased, err: err}
	}()
	require.Eventually(t, func() bool {
		return lockWaits(t, order.db, "retire_expired_demo_schools")
	}, 5*time.Second, time.Millisecond, "the erasure waits for the monthly billing")
	require.NoError(t, release.Commit())
	select {
	case result := <-done:
		require.NoError(t, result.err)
		require.Equal(t, 1, result.erased, "the release commits within the lock timeout")
	case <-time.After(5 * time.Second):
		t.Fatal("erasure did not finish after the release committed")
	}
	assert.False(t, careInvoiceExists(t, order, invoiceID), "the concurrent release leaves no visitor name")
	order.requireFullyErased(t)
}

// A release that keeps the monthly billing defers the order unchanged; the
// next run erases it with the invoice.
func TestDemoErasureDefersAMonthlyReleaseThatKeepsItsLock(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	before := order.snapshot(t)
	release, invoiceID := pendingCareRelease(t, order)

	_, erased := retireNow(t, order.db, order.queue, nil)
	assert.Zero(t, erased)
	assert.Equal(t, before, order.snapshot(t), "the deferred order is unchanged")
	require.NoError(t, release.Commit(), "erasure does not abort the release")
	require.True(t, careInvoiceExists(t, order, invoiceID))

	order.requireErasedNext(t)
	assert.False(t, careInvoiceExists(t, order, invoiceID))
}
