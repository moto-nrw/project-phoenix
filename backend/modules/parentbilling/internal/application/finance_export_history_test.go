package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
)

type historyStore struct {
	FinanceExportStore
	steps []parentbilling.InvoiceHistoryEntry
	reads *[]historyRead
}

// historyRead is one InsertRunEvents call of a history read.
type historyRead struct {
	organization, account int64
	action                string
	holiday, care         map[int64]map[string]any
}

func (s historyStore) InvoiceHistory(context.Context, int64, string, int64) (string, []parentbilling.InvoiceHistoryEntry, []parentbilling.ExportProtocolEntry, error) {
	return "RE-1", s.steps, nil, nil
}

func (s historyStore) InsertRunEvents(_ context.Context, organization, account int64, action string, holiday, care map[int64]map[string]any) error {
	if s.reads != nil {
		*s.reads = append(*s.reads, historyRead{organization, account, action, holiday, care})
	}
	return nil
}

// issuedIn is the issue step of an invoice of run.
func issuedIn(run int64) parentbilling.InvoiceHistoryEntry {
	return parentbilling.InvoiceHistoryEntry{Kind: parentbilling.HistoryIssued, At: time.Date(2026, 11, 1, 9, 0, 0, 0, time.UTC), RunID: run}
}

type historyPost struct {
	states            []parentbilling.BillingLetterMailState
	err               error
	care, holiday     int
	organization, run int64
}

func (p *historyPost) CareLetterMailStates(_ context.Context, organization, run int64) ([]parentbilling.BillingLetterMailState, error) {
	p.care++
	p.organization, p.run = organization, run
	return p.states, p.err
}
func (p *historyPost) LetterMailStates(_ context.Context, organization, run int64) ([]parentbilling.BillingLetterMailState, error) {
	p.holiday++
	p.organization, p.run = organization, run
	return p.states, p.err
}

func TestInvoiceHistorySeparatesRequestedMailFromSuccessfulSend(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{parentbilling.HistoryCareInvoice, parentbilling.HistoryHolidayDocument} {
		t.Run(kind, func(t *testing.T) {
			requested := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
			accepted := requested.Add(time.Minute)
			account := sjrReviewer.AccountID
			post := &historyPost{states: []parentbilling.BillingLetterMailState{
				{DeliveryID: 11, Status: parentbilling.LetterMailQueued},
				{DeliveryID: 12, Status: parentbilling.LetterMailFailed, Outcome: "blocked"},
				{DeliveryID: 13, Status: parentbilling.LetterMailFailed, Outcome: "send_failed"},
				{DeliveryID: 14, Status: parentbilling.LetterMailUnknown},
				{DeliveryID: 15, Status: parentbilling.LetterMailSent, SentAt: &accepted, Outcome: "accepted"},
				{DeliveryID: 99, Status: parentbilling.LetterMailSent, SentAt: &accepted},
			}}
			store := historyStore{steps: []parentbilling.InvoiceHistoryEntry{issuedIn(7)}}
			for id := int64(11); id <= 16; id++ {
				store.steps = append(store.steps, parentbilling.InvoiceHistoryEntry{Kind: parentbilling.HistoryMailRequested, At: requested,
					AccountID: &account, RunID: 7, DeliveryID: id, Note: "invoice"})
			}
			history, err := newFinanceExportsForTest(store, post).InvoiceHistory(context.Background(), sjrReviewer, kind, 1)
			require.NoError(t, err)
			require.Len(t, history.Entries, 8, "the issue, six requests, one accepted mail; no send invented for a missing outbox entry")
			assert.Equal(t, parentbilling.HistoryIssued, history.Entries[0].Kind)
			for _, entry := range history.Entries[1:7] {
				assert.Equal(t, parentbilling.HistoryMailRequested, entry.Kind)
			}
			assert.Equal(t, parentbilling.HistorySent, history.Entries[7].Kind)
			assert.Equal(t, accepted, history.Entries[7].At)
			assert.True(t, history.Entries[7].ByYou)
			assert.Equal(t, sjrReviewer.OrganizationID, post.organization)
			assert.Equal(t, store.steps[1].RunID, post.run)
			if kind == parentbilling.HistoryCareInvoice {
				assert.Equal(t, 1, post.care)
				assert.Zero(t, post.holiday)
			} else {
				assert.Equal(t, 1, post.holiday)
				assert.Zero(t, post.care)
			}
			post.err = errors.New("mail states unavailable")
			_, err = newFinanceExportsForTest(store, post).InvoiceHistory(context.Background(), sjrReviewer, kind, 1)
			require.ErrorIs(t, err, post.err)
		})
	}
}

// runPost answers the mail states of each run apart.
type runPost struct {
	byRun map[int64][]parentbilling.BillingLetterMailState
}

func (p runPost) CareLetterMailStates(_ context.Context, _, run int64) ([]parentbilling.BillingLetterMailState, error) {
	return p.byRun[run], nil
}
func (p runPost) LetterMailStates(_ context.Context, _, run int64) ([]parentbilling.BillingLetterMailState, error) {
	return p.byRun[run], nil
}

// A Storno mailed in another run than the invoice still shows its send
// (review of d4eefbc966): the states of every run a mailing names are read.
func TestInvoiceHistoryReadsTheMailStatesOfEveryRun(t *testing.T) {
	t.Parallel()
	requested := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	accepted := requested.Add(time.Minute)
	post := runPost{byRun: map[int64][]parentbilling.BillingLetterMailState{
		7: {{DeliveryID: 21, Status: parentbilling.LetterMailSent, SentAt: &accepted}},
		8: {{DeliveryID: 22, Status: parentbilling.LetterMailSent, SentAt: &accepted}},
	}}
	store := historyStore{steps: []parentbilling.InvoiceHistoryEntry{
		issuedIn(7),
		{Kind: parentbilling.HistoryMailRequested, At: requested, RunID: 7, DeliveryID: 21, Note: "invoice"},
		{Kind: parentbilling.HistoryMailRequested, At: requested, RunID: 8, DeliveryID: 22, Note: "cancellation"},
	}}
	history, err := newFinanceExportsForTest(store, post).InvoiceHistory(context.Background(), sjrReviewer, parentbilling.HistoryHolidayDocument, 1)
	require.NoError(t, err)
	var sent []string
	for _, entry := range history.Entries {
		if entry.Kind == parentbilling.HistorySent {
			sent = append(sent, entry.Note)
		}
	}
	assert.ElementsMatch(t, []string{"invoice", "cancellation"}, sent)
}

// Reading an invoice's history is a read of personal billing data: it
// lands in the trail of the invoice's run, monthly or holiday, as the
// family view records its reads (#4693). Without recordHistoryRead no
// event is written and this test fails.
func TestInvoiceHistoryRecordsTheReadInTheTrailOfItsRun(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{parentbilling.HistoryCareInvoice, parentbilling.HistoryHolidayDocument} {
		t.Run(kind, func(t *testing.T) {
			var reads []historyRead
			store := historyStore{steps: []parentbilling.InvoiceHistoryEntry{issuedIn(31)}, reads: &reads}
			_, err := newFinanceExportsForTest(store, runPost{}).InvoiceHistory(context.Background(), sjrReviewer, kind, 404)
			require.NoError(t, err)
			require.Len(t, reads, 1)
			read := reads[0]
			assert.Equal(t, sjrReviewer.OrganizationID, read.organization)
			assert.Equal(t, sjrReviewer.AccountID, read.account)
			assert.Equal(t, "read", read.action)
			trail, other := read.holiday, read.care
			if kind == parentbilling.HistoryCareInvoice {
				trail, other = read.care, read.holiday
			}
			assert.Empty(t, other, "only the trail of the invoice's own kind of run")
			assert.Equal(t, map[int64]map[string]any{31: {"view": "invoice_history", "document_ids": []int64{404}}}, trail)
		})
	}
}

// actorNamesDouble answers the names of the Träger-Büro and records each
// call.
type actorNamesDouble struct {
	names map[int64]string
	calls []actorNamesCall
}

type actorNamesCall struct {
	organization int64
	accounts     []int64
}

func (d *actorNamesDouble) TraegerActorNames(_ context.Context, organization int64, accounts []int64) (map[int64]string, error) {
	d.calls = append(d.calls, actorNamesCall{organization, accounts})
	return d.names, nil
}

// The history names a colleague's steps and exported files with the
// colleague's name in one call for the reviewer's Träger; the reviewer's own
// steps stay "by you" and need no name (#4693).
func TestInvoiceHistoryNamesColleaguesInOneCall(t *testing.T) {
	t.Parallel()
	own, colleague, unnamed := sjrReviewer.AccountID, sjrReviewer.AccountID+1, sjrReviewer.AccountID+2
	at := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	store := historyFilesStore{historyStore: historyStore{steps: []parentbilling.InvoiceHistoryEntry{
		issuedIn(7),
		{Kind: parentbilling.HistoryMailRequested, At: at, AccountID: &colleague, RunID: 7, DeliveryID: 1},
		{Kind: parentbilling.HistoryMailRequested, At: at, AccountID: &own, RunID: 7, DeliveryID: 2},
		{Kind: parentbilling.HistoryMailRequested, At: at, AccountID: &unnamed, RunID: 7, DeliveryID: 3},
	}}, files: []parentbilling.ExportProtocolEntry{{ID: 5, ExportedBy: &colleague, ExportedAt: at.Add(time.Hour)}}}
	names := &actorNamesDouble{names: map[int64]string{colleague: "Jana Krause"}}
	exports := newFinanceExportsWith(store, &historyPost{}, names, &datevSettingsDouble{})
	history, err := exports.InvoiceHistory(context.Background(), sjrReviewer, parentbilling.HistoryCareInvoice, 1)
	require.NoError(t, err)
	require.Len(t, names.calls, 1, "one call for the whole history")
	assert.Equal(t, sjrReviewer.OrganizationID, names.calls[0].organization, "the names of the reviewer's Träger")
	assert.ElementsMatch(t, []int64{colleague, unnamed}, names.calls[0].accounts, "the colleagues, not the reviewer")
	byDelivery := map[int64]parentbilling.InvoiceHistoryEntry{}
	for _, entry := range history.Entries {
		if entry.Kind == parentbilling.HistoryMailRequested {
			byDelivery[entry.DeliveryID] = entry
		}
		if entry.Kind == parentbilling.HistoryExported {
			require.NotNil(t, entry.File)
			assert.Equal(t, "Jana Krause", entry.AccountName)
			assert.Equal(t, "Jana Krause", entry.File.ExportedByName)
			assert.False(t, entry.File.ExportedByYou)
		}
	}
	assert.Equal(t, "Jana Krause", byDelivery[1].AccountName)
	assert.True(t, byDelivery[2].ByYou)
	assert.Empty(t, byDelivery[2].AccountName, "the reviewer's own step stays by you")
	assert.Empty(t, byDelivery[3].AccountName, "a colleague without a name stays unnamed")

	names.calls = nil
	alone := historyStore{steps: []parentbilling.InvoiceHistoryEntry{issuedIn(7),
		{Kind: parentbilling.HistoryMailRequested, At: at, AccountID: &own, RunID: 7, DeliveryID: 2}}}
	_, err = newFinanceExportsWith(alone, &historyPost{}, names, &datevSettingsDouble{}).InvoiceHistory(context.Background(), sjrReviewer, parentbilling.HistoryCareInvoice, 1)
	require.NoError(t, err)
	assert.Empty(t, names.calls, "a history without a colleague reads no name")
}

// historyFilesStore answers the history with export files.
type historyFilesStore struct {
	historyStore
	files []parentbilling.ExportProtocolEntry
}

func (s historyFilesStore) InvoiceHistory(context.Context, int64, string, int64) (string, []parentbilling.InvoiceHistoryEntry, []parentbilling.ExportProtocolEntry, error) {
	return "RE-1", s.steps, s.files, nil
}
