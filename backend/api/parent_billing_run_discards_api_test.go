package api

import (
	"context"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Discarding an empty open run (#6626): only an open run that has nothing
// to issue or to settle is discarded, under the lock of the release; it
// stays in the list with its trail, never changes again, and its month or
// year is free for a new run.

type discardedRunBody struct {
	careRunBody
	DiscardedAt *string `json:"discarded_at"`
}

func (f careBillingFixture) discardRun(t *testing.T, path string) (int, string) {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, path, nil)
	return rec.Code, rec.Body.String()
}

func TestEmptyCareRunIsDiscardedAndFreesItsMonth(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger leerer Monatslauf")
	run := f.startRun(t, careMonth)
	require.Empty(t, f.run(t, run.ID).Invoices)

	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/discard"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	discarded := decodeSchoolPortalData[discardedRunBody](t, rec.Body.Bytes())
	assert.Equal(t, "discarded", discarded.Status)
	require.NotNil(t, discarded.DiscardedAt, "the run names when it was discarded")
	assert.NotNil(t, discarded.Invoices, "a discarded run shows an empty list, never null")
	assert.Empty(t, discarded.Invoices)
	assert.Equal(t, "discarded", f.run(t, run.ID).Status, "the run stays, nothing is deleted")

	var actor *int64
	require.NoError(t, f.db.NewRaw(`SELECT actor_account_id FROM parent_billing.care_run_events
		WHERE run_id = ? AND action = 'discard'`, run.ID).Scan(context.Background(), &actor))
	assert.NotNil(t, actor, "the trail names who discarded the run")

	for _, change := range []struct {
		method, path string
		body         map[string]any
	}{
		{http.MethodPost, careRunPath(run.ID, "/discard"), nil},
		{http.MethodPost, careRunPath(run.ID, "/release"), map[string]any{"document_count": 0, "total_cents": 0, "fingerprint": "x"}},
		{http.MethodPut, careRunPath(run.ID, "/invoice-date"), map[string]any{"invoice_date": "2030-01-30"}},
	} {
		rec := traegerJSON(t, f.traeger, f.office.token, change.method, change.path, change.body)
		assert.Equal(t, http.StatusConflict, rec.Code, "%s %s: %s", change.method, change.path, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "parent_billing.billing_run_discarded", "%s %s", change.method, change.path)
	}
	_, err := f.db.ExecContext(context.Background(),
		`UPDATE parent_billing.care_runs SET status = 'calculated', discarded_at = NULL WHERE id = ?`, run.ID)
	assert.Error(t, err, "the trigger keeps a discarded run as it is, for every role")

	next := f.startRun(t, careMonth)
	assert.NotEqual(t, run.ID, next.ID, "the month is free for a new run")
}

func TestCareRunWithContentIsNeverDiscarded(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Monatslauf mit Inhalt")
	school := f.office.schools[0]
	mia := careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	run := f.startRun(t, careMonth)
	refused := func(why string) {
		t.Helper()
		code, body := f.discardRun(t, careRunPath(run.ID, "/discard"))
		assert.Equal(t, http.StatusConflict, code, "%s: %s", why, body)
		assert.Contains(t, body, "parent_billing.billing_run_not_empty", why)
		assert.Equal(t, "calculated", f.run(t, run.ID).Status, why)
	}
	require.NotEmpty(t, f.run(t, run.ID).Unpriced)
	refused("a booking without a price waits for its price")

	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, f.run(t, run.ID).Invoices, 1)
	refused("an invoice")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/exclusions"),
		map[string]any{"contract_id": mia, "service": "care", "reason": "Rückfrage"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	refused("an excluded invoice is content the release closes")
}

// A discard waits while a change holds the monthly billing, then sees what
// the change added and refuses. Without the lock it would read the month
// before the change and discard a run that now bills a child.
func TestCareRunDiscardWaitsForAChangeOfTheMonthlyBilling(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Verwerfen wartet")
	school := f.office.schools[0]
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	run := f.startRun(t, careMonth)
	require.Empty(t, f.run(t, run.ID).Invoices)

	change := holdCareBilling(t, f.db, f.office.organization)
	careContract(t, change, f.office.organization, school, "Mia", careMonth, sjrServices()...)

	done := make(chan struct{})
	var code int
	var body string
	go func() {
		defer close(done)
		code, body = f.discardRun(t, careRunPath(run.ID, "/discard"))
	}()
	waitForCareBilling(t, f.db, f.office.organization, done)
	require.NoError(t, change.Commit())
	<-done
	assert.Equal(t, http.StatusConflict, code, body)
	assert.Contains(t, body, "parent_billing.billing_run_not_empty")
	assert.Equal(t, "calculated", f.run(t, run.ID).Status)
}

func TestCareRunDiscardedTwiceAtOnceIsDiscardedOnce(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Verwerfen parallel")
	run := f.startRun(t, careMonth)
	codes := make([]int, 2)
	var wg sync.WaitGroup
	for i := range codes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes[i], _ = f.discardRun(t, careRunPath(run.ID, "/discard"))
		}()
	}
	wg.Wait()
	assert.ElementsMatch(t, []int{http.StatusOK, http.StatusConflict}, codes)
	var discards int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.care_run_events WHERE run_id = ? AND action = 'discard'`,
		run.ID).Scan(context.Background(), &discards))
	assert.Equal(t, 1, discards)
}

func TestEmptyNoticeRunIsDiscardedAndFreesTheYear(t *testing.T) {
	t.Parallel()
	f, _ := newNoticeFixture(t, "Träger leerer Jahreslauf verwerfen")
	writeLegalRemedy(t, f.db, f.office.organization)
	// The second school has no contract: its run has nothing to issue.
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunsPath,
		map[string]any{"year": 2030, "from_month": "2030-09-01", "school_ids": []int64{f.office.schools[1]}})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	empty := f.noticeRun(t, decodeSchoolPortalData[careNoticeRunBody](t, rec.Body.Bytes()).ID)
	require.Zero(t, empty.PreviewCount)

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(empty.ID, "/discard"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "discarded", decodeSchoolPortalData[careNoticeRunBody](t, rec.Body.Bytes()).Status)
	var actor *int64
	require.NoError(t, f.db.NewRaw(`SELECT actor_account_id FROM parent_billing.care_notice_run_events
		WHERE run_id = ? AND action = 'discard'`, empty.ID).Scan(context.Background(), &actor))
	assert.NotNil(t, actor, "the trail names who discarded the run")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careNoticeRunPath(empty.ID, "/release"), careNoticeRelease(empty))
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.billing_run_discarded")
	_, err := f.db.ExecContext(context.Background(),
		`UPDATE parent_billing.care_notice_runs SET status = 'calculated', discarded_at = NULL WHERE id = ?`, empty.ID)
	assert.Error(t, err, "the trigger keeps a discarded notice run as it is")

	full := f.noticeRun(t, f.startNoticeRun(t, "2030-09-01", "").ID)
	require.Len(t, full.Notices, 1, "the year is free for a new run")
	code, body := f.discardRun(t, careNoticeRunPath(full.ID, "/discard"))
	assert.Equal(t, http.StatusConflict, code, body)
	assert.Contains(t, body, "parent_billing.billing_run_not_empty")
	assert.Equal(t, "calculated", f.noticeRun(t, full.ID).Status)
}
