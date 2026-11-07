package compose

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// withoutIssuedInvoice removes the invoice leaveVisitorHolidayRegistration
// issued, with its mailed letter, so the registration waits for its first
// invoice.
func withoutIssuedInvoice(t *testing.T, order demoOrder) {
	t.Helper()
	for _, statement := range []string{
		`DELETE FROM platform.email_outbox AS outbox USING parent_billing.document_mailings AS mailing, parent_billing.documents AS document
			WHERE document.number = 'F-' || ?0 AND mailing.organization_id = document.organization_id AND mailing.document_id = document.id
				AND outbox.organization_id = mailing.organization_id AND outbox.id = mailing.delivery_id`,
		`DELETE FROM parent_billing.documents WHERE number = 'F-' || ?0`,
	} {
		_, err := order.db.NewRaw(statement, order.schoolID).Exec(t.Context())
		require.NoError(t, err, statement)
	}
}

// pendingBillingRelease reproduces the release of a billing run
// (ReleaseBillingRun): it takes the period lock of parent_billing
// (LockPeriodBilling) and issues an invoice for the visitor's holiday
// registration, which names the registration only by ID.
func pendingBillingRelease(t *testing.T, order demoOrder) (bun.Tx, int64) {
	t.Helper()
	ctx := t.Context()
	tx, err := order.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.NewRaw(`SELECT pg_advisory_xact_lock(hashtextextended('parent_billing.period_billing:' || child.organization_id::text || ':' || ? || ':' || child.holiday_period_id::text, 0))
		FROM enrollment.holiday_registration_children AS child WHERE child.home_school_id = ?`, "holiday", order.schoolID).Exec(ctx)
	require.NoError(t, err)
	var documentID int64
	require.NoError(t, tx.NewRaw(`INSERT INTO parent_billing.documents (organization_id, run_id, service, number, number_year,
			number_sequence, issued_on, holiday_period_id, holiday_registration_id, recipient_first_name, recipient_last_name,
			recipient_email, service_from, service_until, total_cents)
		SELECT run.organization_id, run.id, 'holiday', 'R-' || child.home_school_id, 2026, child.home_school_id + 1000000, DATE '2026-06-20',
			run.holiday_period_id, child.holiday_registration_id, ?, ?, 'besuch@example.org', DATE '2026-07-01', DATE '2026-07-31', 12000
		FROM enrollment.holiday_registration_children AS child
		JOIN parent_billing.runs AS run ON run.organization_id = child.organization_id AND run.holiday_period_id = child.holiday_period_id
		WHERE child.home_school_id = ? RETURNING id`, visitorFirst, visitorLast, order.schoolID).Scan(ctx, &documentID))
	return tx, documentID
}

func invoiceRemains(t *testing.T, order demoOrder, documentID int64) bool {
	t.Helper()
	var exists bool
	require.NoError(t, order.db.NewRaw(`SELECT EXISTS (SELECT 1 FROM parent_billing.documents WHERE id = ?)`, documentID).Scan(t.Context(), &exists))
	return exists
}

// A billing run released while the erasure runs cannot leave an invoice of
// the visitor's registration behind (#6015): the erasure waits for the period
// lock and then sees the invoice.
func TestDemoErasureIncludesAConcurrentBillingRelease(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	withoutIssuedInvoice(t, order)
	release, documentID := pendingBillingRelease(t, order)
	type outcome struct {
		erased int
		err    error
	}
	done := make(chan outcome, 1)
	go func() {
		_, erased, err := order.queue.RetireDemoSchools(t.Context(), nil)
		done <- outcome{erased: erased, err: err}
	}()
	// Without the period lock the erasure finishes before the release
	// commits, and the invoice stays without its registration.
	require.Eventually(t, func() bool {
		return lockWaits(t, order.db, "retire_expired_demo_schools")
	}, 5*time.Second, time.Millisecond, "the erasure waits for the period lock of the release")
	require.NoError(t, release.Commit())
	select {
	case result := <-done:
		require.NoError(t, result.err)
		require.Equal(t, 1, result.erased, "the release commits within the lock timeout")
	case <-time.After(5 * time.Second):
		t.Fatal("erasure did not finish after the release committed")
	}
	assert.False(t, invoiceRemains(t, order, documentID), "the concurrent invoice of the visitor's registration goes")
	order.requireFullyErased(t)
}

// A release that keeps its lock defers the whole order; the next run
// erases it with the invoice.
func TestDemoErasureDefersABillingReleaseThatKeepsItsLock(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	withoutIssuedInvoice(t, order)
	before := order.snapshot(t)
	release, documentID := pendingBillingRelease(t, order)

	_, erased := retireNow(t, order.queue, nil)
	assert.Zero(t, erased)
	assert.Equal(t, before, order.snapshot(t), "the deferred order is unchanged")
	assert.Equal(t, 1, visitorTraces(t, order.db, order.schoolID)["enrollment.holiday_registrations"], "the registration stays for the next run")
	require.NoError(t, release.Commit(), "erasure does not abort the release")
	require.True(t, invoiceRemains(t, order, documentID))

	order.requireErasedNext(t)
	assert.False(t, invoiceRemains(t, order, documentID))
}

// pendingLetterSend reproduces the mailing of a confirmation letter
// (MailBillingLetter): it locks the billing run of the visitor's invoice and
// enqueues the letter's mail and push, which name the send only by ID.
func pendingLetterSend(t *testing.T, order demoOrder) (bun.Tx, int64) {
	t.Helper()
	ctx := t.Context()
	tx, err := order.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.NewRaw(`SELECT run.id FROM parent_billing.runs AS run
		JOIN parent_billing.documents AS document ON document.organization_id = run.organization_id AND document.run_id = run.id
		WHERE document.number = 'F-' || ? FOR UPDATE OF run`, order.schoolID).Exec(ctx)
	require.NoError(t, err)
	var mailID int64
	require.NoError(t, tx.NewRaw(`INSERT INTO platform.email_outbox (organization_id, kind, recipient, payload, status)
		SELECT organization_id, 'billing_letter', jsonb_build_object('address', 'besuch@example.org', 'name', ?),
			jsonb_build_object('guardian_name', ?), 'pending'
		FROM parent_billing.documents WHERE number = 'F-' || ? RETURNING id`, visitorFull, visitorFull, order.schoolID).Scan(ctx, &mailID))
	_, err = tx.NewRaw(`INSERT INTO parent_billing.document_mailings (organization_id, run_id, document_id, delivery_id, recipient_email)
		SELECT organization_id, run_id, id, ?, 'besuch@example.org' FROM parent_billing.documents WHERE number = 'F-' || ?`,
		mailID, order.schoolID).Exec(ctx)
	require.NoError(t, err)
	return tx, mailID
}

// A letter mailed while the erasure runs leaves no copy of the visitor on
// the outbox (#6015): the erasure waits for the run lock of the send before
// it cleans up the copies.
func TestDemoErasureIncludesAConcurrentLetterSend(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	send, mailID := pendingLetterSend(t, order)
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
	}, 5*time.Second, time.Millisecond, "the erasure waits for the send")
	require.NoError(t, send.Commit())
	select {
	case result := <-done:
		require.NoError(t, result.err)
		require.Equal(t, 1, result.erased, "the send commits within the lock timeout")
	case <-time.After(5 * time.Second):
		t.Fatal("erasure did not finish after the send committed")
	}
	// Without the run locks the erasure cleans up the copies before it
	// waits for the send at the letter, and this mail stays.
	assert.Empty(t, remainingMails(t, order.db, mailID), "the concurrent mail of the letter goes")
	order.requireFullyErased(t)
}

// A release in progress when the erasure starts, then a send of the letter
// it issued: the send queues on the run the release holds, so it holds the
// run once the release commits. Each wait of the erasure ends within its
// lock timeout of 100 ms. The erasure has to lock that run after the
// period lock and wait for the send, or the send's copies stay (#6015).
func TestDemoErasureIncludesASendOfAConcurrentRelease(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	withoutIssuedInvoice(t, order)
	ctx := t.Context()
	release, err := order.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = release.Rollback() })
	var runID, documentID int64
	require.NoError(t, release.NewRaw(`SELECT run.id FROM parent_billing.runs AS run
		JOIN enrollment.holiday_registration_children AS child
			ON child.organization_id = run.organization_id AND child.holiday_period_id = run.holiday_period_id
		WHERE child.home_school_id = ? FOR UPDATE OF run`, order.schoolID).Scan(ctx, &runID))
	_, err = release.NewRaw(`SELECT pg_advisory_xact_lock(hashtextextended('parent_billing.period_billing:' || organization_id::text || ':' || ? || ':' || holiday_period_id::text, 0))
		FROM parent_billing.runs WHERE id = ?`, "holiday", runID).Exec(ctx)
	require.NoError(t, err)
	require.NoError(t, release.NewRaw(`INSERT INTO parent_billing.documents (organization_id, run_id, service, number, number_year,
			number_sequence, issued_on, holiday_period_id, holiday_registration_id, recipient_first_name, recipient_last_name,
			recipient_email, service_from, service_until, total_cents)
		SELECT run.organization_id, run.id, 'holiday', 'S-' || ?0, 2026, ?0 + 2000000, DATE '2026-06-20', run.holiday_period_id,
			child.holiday_registration_id, ?1, ?2, 'besuch@example.org', DATE '2026-07-01', DATE '2026-07-31', 12000
		FROM parent_billing.runs AS run
		JOIN enrollment.holiday_registration_children AS child
			ON child.organization_id = run.organization_id AND child.holiday_period_id = run.holiday_period_id
		WHERE run.id = ?3 AND child.home_school_id = ?0 RETURNING id`, order.schoolID, visitorFirst, visitorLast, runID).Scan(ctx, &documentID))
	_, err = release.NewRaw(`INSERT INTO parent_billing.document_letters (organization_id, document_id, run_id, filename, pdf)
		SELECT organization_id, id, run_id, 'brief.pdf', '\x255044'::bytea FROM parent_billing.documents WHERE id = ?`, documentID).Exec(ctx)
	require.NoError(t, err)

	send, err := order.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = send.Rollback() })
	sendLocked := make(chan error, 1)
	go func() {
		_, err := send.NewRaw(`SELECT id /* letter send */ FROM parent_billing.runs WHERE id = ? FOR UPDATE`, runID).Exec(ctx)
		sendLocked <- err
	}()
	require.Eventually(t, func() bool {
		return lockWaits(t, order.db, "letter send")
	}, 5*time.Second, time.Millisecond, "the send queues on the run the release holds")
	type outcome struct {
		erased int
		err    error
	}
	erasure := make(chan outcome, 1)
	go func() {
		_, erased, err := order.queue.RetireDemoSchools(t.Context(), nil)
		erasure <- outcome{erased: erased, err: err}
	}()
	require.Eventually(t, func() bool {
		return lockWaits(t, order.db, "retire_expired_demo_schools")
	}, 5*time.Second, time.Millisecond, "the erasure waits for the period lock of the release")

	require.NoError(t, release.Commit())
	require.NoError(t, <-sendLocked)

	var mailID, pushID int64
	require.NoError(t, send.NewRaw(`INSERT INTO platform.email_outbox (organization_id, kind, recipient, payload, status)
		SELECT organization_id, 'billing_letter', jsonb_build_object('address', 'besuch@example.org', 'name', ?),
			jsonb_build_object('guardian_name', ?), 'pending'
		FROM parent_billing.documents WHERE id = ? RETURNING id`, visitorFull, visitorFull, documentID).Scan(ctx, &mailID))
	require.NoError(t, send.NewRaw(`INSERT INTO platform.push_outbox (organization_id, kind, related_entity_type, related_entity_id, recipient, payload)
		SELECT organization_id, 'parent_billing_document', 'parent_billing_document', id,
			jsonb_build_object('email', 'besuch@example.org'), jsonb_build_object('title', ?)
		FROM parent_billing.documents WHERE id = ? RETURNING id`, "Brief für "+visitorFull, documentID).Scan(ctx, &pushID))
	_, err = send.NewRaw(`INSERT INTO parent_billing.document_mailings (organization_id, run_id, document_id, delivery_id, recipient_email)
		SELECT organization_id, run_id, id, ?, 'besuch@example.org' FROM parent_billing.documents WHERE id = ?`, mailID, documentID).Exec(ctx)
	require.NoError(t, err, "the letter of the released invoice is still there for the send")
	// Without the run lock after the period lock the erasure no longer waits
	// for the send here: it cleaned up the copies before the send wrote them.
	require.Eventually(t, func() bool {
		return lockWaits(t, order.db, "retire_expired_demo_schools")
	}, 5*time.Second, time.Millisecond, "the erasure waits for the send of the released run")
	require.NoError(t, send.Commit())
	select {
	case result := <-erasure:
		require.NoError(t, result.err)
		require.Equal(t, 1, result.erased, "release and send commit within the lock timeouts")
	case <-time.After(5 * time.Second):
		t.Fatal("erasure did not finish after the send committed")
	}
	assert.False(t, invoiceRemains(t, order, documentID), "the released invoice goes")
	assert.Empty(t, remainingMails(t, order.db, mailID), "the mail of the send goes")
	var pushes int
	require.NoError(t, order.db.NewRaw(`SELECT count(*) FROM platform.push_outbox WHERE id = ?`, pushID).Scan(ctx, &pushes))
	assert.Zero(t, pushes, "the push of the send goes")
	order.requireFullyErased(t)
}
