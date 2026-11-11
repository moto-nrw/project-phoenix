package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// Discarding an empty open holiday run (#6628), as the monthly and notice
// runs of #6626: only an open run with no invoice and no confirmed
// registration waiting for its contribution is discarded, under the locks
// of the release; it stays in the list with its trail, never changes again,
// and its period is free for a new run.

type discardedHolidayRunBody struct {
	billingRunBody
	DiscardedAt *string `json:"discarded_at"`
}

func discardHolidayRun(t *testing.T, f reviewFixture, token string, runID int64) (int, string) {
	t.Helper()
	rec := traegerJSON(t, f.traeger, token, http.MethodPost, billingRunPath(runID, "/discard"), nil)
	return rec.Code, rec.Body.String()
}

func TestEmptyHolidayRunIsDiscardedAndFreesItsPeriod(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger leerer Ferienlauf")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	run := startBillingRun(t, f, billing, f.period.ID)
	preview := billingRun(t, f, billing, run.ID)
	require.Empty(t, preview.Documents, "no registration is confirmed yet")
	require.Zero(t, preview.Unbilled)

	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/discard"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	discarded := decodeSchoolPortalData[discardedHolidayRunBody](t, rec.Body.Bytes())
	assert.Equal(t, "discarded", discarded.Status)
	require.NotNil(t, discarded.DiscardedAt, "the run names when it was discarded")
	assert.NotNil(t, discarded.Documents, "a discarded run shows an empty list, never null")
	assert.Empty(t, discarded.Documents)
	assert.Equal(t, "discarded", billingRun(t, f, billing, run.ID).Status, "the run stays, nothing is deleted")
	assert.Contains(t, runActions(t, f, run.ID), "discard", "the trail names who discarded the run")

	for _, change := range []struct {
		method, path string
		body         map[string]any
	}{
		{http.MethodPost, billingRunPath(run.ID, "/discard"), nil},
		{http.MethodPost, billingRunPath(run.ID, "/release"), map[string]any{"document_count": 0, "total_cents": 0, "fingerprint": "x"}},
		{http.MethodPost, billingRunPath(run.ID, "/corrections"), map[string]any{"registration_id": f.siblings, "child_id": 1, "total_cents": 100, "reason": "x"}},
		{http.MethodPut, fmt.Sprintf("%s/holiday/%d", runDueDatesPath, run.ID), map[string]any{"due_dates": []map[string]any{{"payment_method": "transfer", "due_on": "2027-01-15"}}}},
	} {
		rec := traegerJSON(t, f.traeger, billing, change.method, change.path, change.body)
		assert.Equal(t, http.StatusConflict, rec.Code, "%s %s: %s", change.method, change.path, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "parent_billing.billing_run_discarded", "%s %s", change.method, change.path)
	}
	ctx := context.Background()
	_, err := f.db.ExecContext(ctx, `UPDATE parent_billing.runs SET status = 'calculated', discarded_at = NULL WHERE id = ?`, run.ID)
	assert.Error(t, err, "the trigger keeps a discarded run as it is, for every role")
	_, err = f.db.ExecContext(ctx, `INSERT INTO parent_billing.run_due_dates (organization_id, holiday_run_id, payment_method, due_on)
		VALUES (?, ?, 'transfer', DATE '2027-01-15')`, f.office.organization, run.ID)
	assert.Error(t, err, "no due date is added to a discarded run, for every role")

	next := startBillingRun(t, f, billing, f.period.ID)
	assert.NotEqual(t, run.ID, next.ID, "the period is free for a new run")
}

func TestHolidayRunWithContentIsNeverDiscarded(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Ferienlauf mit Inhalt")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	run := startBillingRun(t, f, billing, f.period.ID)
	refused := func(why string) {
		t.Helper()
		code, body := discardHolidayRun(t, f, billing, run.ID)
		assert.Equal(t, http.StatusConflict, code, "%s: %s", why, body)
		assert.Contains(t, body, "parent_billing.billing_run_not_empty", why)
		assert.Equal(t, "calculated", billingRun(t, f, billing, run.ID).Status, why)
	}

	// A registration confirmed without its frozen contribution waits for it.
	_, err := f.db.ExecContext(context.Background(),
		`UPDATE enrollment.holiday_registrations SET status = 'confirmed' WHERE id = ?`, f.again)
	require.NoError(t, err)
	require.Equal(t, 1, billingRun(t, f, billing, run.ID).Unbilled)
	refused("a confirmed registration without its contribution")

	require.Equal(t, http.StatusOK, f.act(t, f.siblings, "confirm").Code)
	require.Len(t, billingRun(t, f, billing, run.ID).Documents, 1)
	refused("an invoice")
}

// holdPeriodBilling holds the invoices of the holiday period the way a
// release, a cancellation of an invoice or a sibling change does
// (LockPeriodBilling), so the test can change the period meanwhile.
func holdPeriodBilling(t *testing.T, f reviewFixture) bun.Tx {
	t.Helper()
	tx, err := f.db.BeginTx(context.Background(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.ExecContext(context.Background(), `SELECT pg_advisory_xact_lock(hashtextextended(
		'parent_billing.period_billing:' || ?::text || ':holiday:' || ?::text, 0))`, f.office.organization, f.period.ID)
	require.NoError(t, err)
	return tx
}

func waitForPeriodBilling(t *testing.T, f reviewFixture, done <-chan struct{}) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var waiting bool
		require.NoError(t, f.db.NewRaw(`SELECT EXISTS (SELECT 1 FROM pg_locks AS l
			WHERE l.locktype = 'advisory' AND NOT l.granted
				AND ((l.classid::bigint << 32) | l.objid::bigint) = hashtextextended(
					'parent_billing.period_billing:' || ?::text || ':holiday:' || ?::text, 0))`,
			f.office.organization, f.period.ID).Scan(context.Background(), &waiting))
		if waiting {
			return
		}
		select {
		case <-done:
			t.Fatal("the discard finished without waiting for the invoices of the period")
		case <-time.After(20 * time.Millisecond):
		}
	}
	t.Fatal("the discard never waited for the invoices of the period")
}

// A discard waits while a change holds the invoices of the period, then
// sees what the change added and refuses. Without the lock it would read
// the period before the change and discard a run that now bills a family.
func TestHolidayRunDiscardWaitsForTheInvoicesOfItsPeriod(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Ferienlauf Verwerfen wartet")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	run := startBillingRun(t, f, billing, f.period.ID)
	require.Empty(t, billingRun(t, f, billing, run.ID).Documents)

	change := holdPeriodBilling(t, f)
	_, err := change.ExecContext(context.Background(),
		`UPDATE enrollment.holiday_registrations SET status = 'confirmed' WHERE id = ?`, f.again)
	require.NoError(t, err)

	done := make(chan struct{})
	var code int
	var body string
	go func() {
		defer close(done)
		code, body = discardHolidayRun(t, f, billing, run.ID)
	}()
	waitForPeriodBilling(t, f, done)
	require.NoError(t, change.Commit())
	<-done
	assert.Equal(t, http.StatusConflict, code, body)
	assert.Contains(t, body, "parent_billing.billing_run_not_empty")
	assert.Equal(t, "calculated", billingRun(t, f, billing, run.ID).Status)
}

func TestHolidayRunDiscardedTwiceAtOnceIsDiscardedOnce(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Ferienlauf Verwerfen parallel")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	run := startBillingRun(t, f, billing, f.period.ID)
	codes := make([]int, 2)
	var wg sync.WaitGroup
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i], _ = discardHolidayRun(t, f, billing, run.ID)
		}()
	}
	wg.Wait()
	assert.ElementsMatch(t, []int{http.StatusOK, http.StatusConflict}, codes)
	var discards int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.run_events WHERE run_id = ? AND action = 'discard'`,
		run.ID).Scan(context.Background(), &discards))
	assert.Equal(t, 1, discards)
}
