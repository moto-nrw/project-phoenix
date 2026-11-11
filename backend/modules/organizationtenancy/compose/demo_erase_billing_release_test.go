package compose

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// withoutIssuedInvoice removes the invoice leaveVisitorHolidayRegistration
// issued, with its mailed letter and its push, so the registration waits for
// its first invoice.
func withoutIssuedInvoice(t *testing.T, order demoOrder) {
	t.Helper()
	for _, statement := range []string{
		`DELETE FROM platform.push_outbox AS push USING parent_billing.documents AS document
			WHERE document.number = 'F-' || ?0 AND push.organization_id = document.organization_id
				AND push.kind = 'parent_billing_document' AND push.related_entity_id = document.id`,
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

	_, erased := retireNow(t, order.db, order.queue, nil)
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
		SELECT organization_id, 'parent_billing_letter', jsonb_build_object('address', 'besuch@example.org', 'name', ?),
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

// waitsOn reports whether a session whose query holds marker waits for a
// lock of one of the kinds (pg_locks.locktype): the state the test steps
// on, never a guess of how long a step takes.
func waitsOn(t *testing.T, order demoOrder, marker string, locktypes ...string) bool {
	t.Helper()
	var waiting int
	err := order.db.NewRaw(`SELECT count(*) FROM pg_locks AS lock
		JOIN pg_stat_activity AS activity ON activity.pid = lock.pid
		WHERE activity.datname = current_database() AND activity.pid <> pg_backend_pid()
			AND NOT lock.granted AND lock.locktype IN (?) AND strpos(activity.query, ?) > 0`,
		bun.List(locktypes), marker).Scan(t.Context(), &waiting)
	return err == nil && waiting > 0
}

// periodLock holds the period lock of parent_billing (LockPeriodBilling) of
// the run in a transaction of its own.
func periodLock(t *testing.T, order demoOrder, runID int64) bun.Tx {
	t.Helper()
	tx, err := order.db.BeginTx(t.Context(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.NewRaw(`SELECT pg_advisory_xact_lock(hashtextextended('parent_billing.period_billing:' || organization_id::text || ':' || ? || ':' || holiday_period_id::text, 0))
		FROM parent_billing.runs WHERE id = ?`, "holiday", runID).Exec(t.Context())
	require.NoError(t, err)
	return tx
}

// A release commits while the erasure waits for the period lock, and a send
// of the letter it issued takes the released run and writes its mail, push
// and mailing before the erasure gets the period. The erasure has to lock
// that run after the period lock and wait for the send, or it cleans up the
// copies before the send commits and the mail and push stay (#6015).
//
// In production the release's commit frees the period, and the send and the
// erasure then race for the run: PostgreSQL does not queue a lock request
// behind a waiter once the holder committed. Here a transaction of its own
// holds the period, so the send deterministically takes the run first, the
// case that leaves copies behind without the second pass. Every step waits
// for the lock state in pg_locks, and the erasure's waits stay a few
// statements long, far below its lock timeout of 100 ms.
func TestDemoErasureIncludesASendOfAConcurrentRelease(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	withoutIssuedInvoice(t, order)
	ctx := t.Context()
	var runID int64
	require.NoError(t, order.db.NewRaw(`SELECT run.id FROM parent_billing.runs AS run
		JOIN enrollment.holiday_registration_children AS child
			ON child.organization_id = run.organization_id AND child.holiday_period_id = run.holiday_period_id
		WHERE child.home_school_id = ?`, order.schoolID).Scan(ctx, &runID))
	period := periodLock(t, order, runID)
	release, err := order.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = release.Rollback() })
	var documentID int64
	require.NoError(t, release.NewRaw(`WITH run AS (
			SELECT run.organization_id, run.id, run.holiday_period_id FROM parent_billing.runs AS run WHERE run.id = ?3 FOR UPDATE),
		document AS (
			INSERT INTO parent_billing.documents (organization_id, run_id, service, number, number_year,
				number_sequence, issued_on, holiday_period_id, holiday_registration_id, recipient_first_name, recipient_last_name,
				recipient_email, service_from, service_until, total_cents)
			SELECT run.organization_id, run.id, 'holiday', 'S-' || ?0, 2026, ?0 + 2000000, DATE '2026-06-20', run.holiday_period_id,
				child.holiday_registration_id, ?1, ?2, 'besuch@example.org', DATE '2026-07-01', DATE '2026-07-31', 12000
			FROM run JOIN enrollment.holiday_registration_children AS child
				ON child.organization_id = run.organization_id AND child.holiday_period_id = run.holiday_period_id
			WHERE child.home_school_id = ?0
			RETURNING organization_id, run_id, id),
		letter AS (
			INSERT INTO parent_billing.document_letters (organization_id, document_id, run_id, filename, pdf)
			SELECT organization_id, id, run_id, 'brief.pdf', '\x255044'::bytea FROM document)
		SELECT id FROM document`, order.schoolID, visitorFirst, visitorLast, runID).Scan(ctx, &documentID))

	order.waitForLocks(t)
	type outcome struct {
		erased int
		err    error
	}
	erasure := make(chan outcome, 1)
	go func() {
		_, erased, err := order.queue.RetireDemoSchools(t.Context(), nil)
		erasure <- outcome{erased: erased, err: err}
	}()
	// The first pass found no invoice; the erasure waits for the period.
	require.Eventually(t, func() bool {
		return waitsOn(t, order, "retire_expired_demo_schools", "advisory")
	}, 5*time.Second, time.Millisecond, "the erasure waits for the period lock")

	require.NoError(t, release.Commit())
	send, err := order.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = send.Rollback() })
	var mailID, pushID int64
	require.NoError(t, send.NewRaw(`WITH run AS (
			SELECT id FROM parent_billing.runs WHERE id = ?3 FOR UPDATE),
		document AS (
			SELECT document.organization_id, document.run_id, document.id
			FROM parent_billing.documents AS document JOIN run ON run.id = document.run_id WHERE document.id = ?2),
		outbox AS (
			INSERT INTO platform.email_outbox (organization_id, kind, recipient, payload, status)
			SELECT organization_id, 'parent_billing_letter', jsonb_build_object('address', 'besuch@example.org', 'name', ?0::text),
				jsonb_build_object('guardian_name', ?0::text), 'pending' FROM document
			RETURNING id),
		push AS (
			INSERT INTO platform.push_outbox (organization_id, kind, related_entity_type, related_entity_id, recipient, payload)
			SELECT organization_id, 'parent_billing_document', 'parent_billing_document', id,
				jsonb_build_object('email', 'besuch@example.org'), jsonb_build_object('title', ?1::text) FROM document
			RETURNING id),
		mailing AS (
			INSERT INTO parent_billing.document_mailings (organization_id, run_id, document_id, delivery_id, recipient_email)
			SELECT document.organization_id, document.run_id, document.id, outbox.id, 'besuch@example.org' FROM document, outbox)
		SELECT outbox.id, push.id FROM outbox, push`, visitorFull, "Brief für "+visitorFull, documentID, runID).Scan(ctx, &mailID, &pushID),
		"the send takes the released run and writes its copies while the erasure waits for the period")
	require.NoError(t, period.Commit())

	// Without the run lock after the period lock the erasure instead cleans
	// up the copies it sees, none yet, and waits at the letter the mailing
	// names; the mail and the push of the send then stay.
	require.Eventually(t, func() bool {
		return waitsOn(t, order, "retire_expired_demo_schools", "transactionid", "tuple")
	}, 5*time.Second, time.Millisecond, "the erasure waits for the send")
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

// A correction of the visitor's invoice (correctable: the released run, then
// the period) while the erasure runs (#6015). The erasure has to take the run
// before the period too: with the period first, the correction holding the
// run and waiting for the period and the erasure holding the period and
// waiting for the run close a cycle, which the erasure's lock timeout ends by
// deferring the order.
//
// The correction takes run, period and cancellation in its order before the
// erasure starts, so the test does not depend on how long a step takes under
// load: the erasure's first wait (pg_locks) has to be for the run, not for
// the period, and it lasts only until the correction's commit, far below its
// lock timeout of 100 ms.
func TestDemoErasureWaitsForAConcurrentCorrection(t *testing.T) {
	t.Parallel()
	order := newHiddenDemoOrder(t)
	ctx := t.Context()
	correction, err := order.db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = correction.Rollback() })
	var runID int64
	require.NoError(t, correction.NewRaw(`SELECT run.id FROM parent_billing.runs AS run
		JOIN parent_billing.documents AS document ON document.organization_id = run.organization_id AND document.run_id = run.id
		WHERE document.number = 'F-' || ? FOR UPDATE OF run`, order.schoolID).Scan(ctx, &runID))
	var cancellationID int64
	require.NoError(t, correction.NewRaw(`WITH period AS (
			SELECT pg_advisory_xact_lock(hashtextextended('parent_billing.period_billing:' || organization_id::text || ':' || ?1 || ':' || holiday_period_id::text, 0)) AS locked
			FROM parent_billing.runs WHERE id = ?2)
		INSERT INTO parent_billing.documents (organization_id, run_id, service, number, number_year,
			number_sequence, issued_on, holiday_period_id, holiday_registration_id, recipient_first_name, recipient_last_name,
			recipient_email, service_from, service_until, total_cents, kind, reference_document_id, reason)
		SELECT organization_id, run_id, service, 'ST-' || ?0, number_year, number_sequence + 3000000, issued_on, holiday_period_id,
			holiday_registration_id, recipient_first_name, recipient_last_name, recipient_email, service_from, service_until,
			-total_cents, 'cancellation', id, 'Storno'
		FROM parent_billing.documents, period WHERE number = 'F-' || ?0 RETURNING id`, order.schoolID, "holiday", runID).Scan(ctx, &cancellationID))

	order.waitForLocks(t)
	type outcome struct {
		erased int
		err    error
	}
	erasure := make(chan outcome, 1)
	go func() {
		_, erased, err := order.queue.RetireDemoSchools(t.Context(), nil)
		erasure <- outcome{erased: erased, err: err}
	}()
	// With the period before the run the erasure would wait for the
	// advisory lock here, never for the run.
	require.Eventually(t, func() bool {
		return waitsOn(t, order, "retire_expired_demo_schools", "transactionid", "tuple")
	}, 5*time.Second, time.Millisecond, "the erasure waits for the run the correction holds, before it asks for the period")
	require.False(t, waitsOn(t, order, "retire_expired_demo_schools", "advisory"), "the erasure does not wait for the period")
	require.NoError(t, correction.Commit())
	select {
	case result := <-erasure:
		require.NoError(t, result.err)
		require.Equal(t, 1, result.erased, "the erasure waits for the correction instead of deferring the order")
	case <-time.After(5 * time.Second):
		t.Fatal("erasure did not finish after the correction committed")
	}
	assert.False(t, invoiceRemains(t, order, cancellationID), "the cancellation of the visitor's invoice goes")
	order.requireFullyErased(t)
}
