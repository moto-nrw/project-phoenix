package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

type historyStore struct {
	FinanceExportStore
	steps []parentbilling.InvoiceHistoryEntry
}

func (s historyStore) InvoiceHistory(context.Context, int64, string, int64) (string, []parentbilling.InvoiceHistoryEntry, []parentbilling.ExportProtocolEntry, error) {
	return "RE-1", s.steps, nil, nil
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
			store := historyStore{}
			for id := int64(11); id <= 16; id++ {
				store.steps = append(store.steps, parentbilling.InvoiceHistoryEntry{Kind: parentbilling.HistoryMailRequested, At: requested,
					AccountID: &account, RunID: 7, DeliveryID: id, Note: "invoice"})
			}
			history, err := NewFinanceExports(store, post).InvoiceHistory(context.Background(), sjrReviewer, kind, 1)
			require.NoError(t, err)
			require.Len(t, history.Entries, 7, "six requests, one accepted mail; no send invented for a missing outbox entry")
			for _, entry := range history.Entries[:6] {
				assert.Equal(t, parentbilling.HistoryMailRequested, entry.Kind)
			}
			assert.Equal(t, parentbilling.HistorySent, history.Entries[6].Kind)
			assert.Equal(t, accepted, history.Entries[6].At)
			assert.True(t, history.Entries[6].ByYou)
			assert.Equal(t, sjrReviewer.OrganizationID, post.organization)
			assert.Equal(t, store.steps[0].RunID, post.run)
			if kind == parentbilling.HistoryCareInvoice {
				assert.Equal(t, 1, post.care)
				assert.Zero(t, post.holiday)
			} else {
				assert.Equal(t, 1, post.holiday)
				assert.Zero(t, post.care)
			}
			post.err = errors.New("mail states unavailable")
			_, err = NewFinanceExports(store, post).InvoiceHistory(context.Background(), sjrReviewer, kind, 1)
			require.ErrorIs(t, err, post.err)
		})
	}
}
