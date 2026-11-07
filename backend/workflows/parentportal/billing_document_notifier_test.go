package parentportal_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	deliveryPolicy "github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

type documentPreferencesDouble struct{ optedIn []int64 }

func (d documentPreferencesDouble) FilterOptedIn(_ context.Context, notificationType string, accountIDs []int64) ([]int64, error) {
	if notificationType != "parent_billing_document" {
		return nil, errors.New("unexpected notification type " + notificationType)
	}
	var kept []int64
	for _, id := range accountIDs {
		for _, opted := range d.optedIn {
			if id == opted {
				kept = append(kept, id)
			}
		}
	}
	return kept, nil
}

type documentNotificationsDouble struct {
	events []deliveryPolicy.NotificationEvent
	err    error
}

func (d *documentNotificationsDouble) NotifyEvent(context.Context, deliveryPolicy.NotificationEvent) error {
	return errors.New("the document push is durable")
}

func (d *documentNotificationsDouble) NotifyEventDurably(_ context.Context, event deliveryPolicy.NotificationEvent) (int, error) {
	d.events = append(d.events, event)
	return 1, d.err
}

// The push goes through the school's router only with the parent's consent,
// in the parent's language, about the permitted children, naming no child
// and no amount, and opens the inbox.
func TestParentDocumentNotifierAsksForConsentAndNamesNoChild(t *testing.T) {
	t.Parallel()
	push := parentService.BillingDocumentPush{TenantID: 21, AccountID: 7301, StudentIDs: []int64{11, 12}, DocumentID: 31, Locale: "en"}

	silent := &documentNotificationsDouble{}
	require.NoError(t, owners.NewParentDocumentNotifier(silent, documentPreferencesDouble{}).NotifyBillingDocument(context.Background(), push))
	assert.Empty(t, silent.events, "no consent, no push")

	router := &documentNotificationsDouble{}
	require.NoError(t, owners.NewParentDocumentNotifier(router, documentPreferencesDouble{optedIn: []int64{7301}}).NotifyBillingDocument(context.Background(), push))
	require.Len(t, router.events, 1)
	event := router.events[0]
	assert.Equal(t, "parent_billing_document", event.Type)
	assert.Equal(t, "parent-billing-document:31:7301", event.IdempotencyKey)
	assert.Equal(t, "parent_billing_document", event.RelatedType)
	assert.Equal(t, int64(31), event.RelatedID)
	assert.Equal(t, deliveryPolicy.NotificationAudience{TenantID: 21, Scope: "guardian", GuardianAccountIDs: []int64{7301}, StudentIDs: []int64{11, 12}}, event.Audience)
	assert.Equal(t, "/documents", event.DeepLink)
	assert.Equal(t, "New document from the provider", event.Title)
	assert.Equal(t, "You have a new document in the parent portal.", event.Body)

	disabled := &documentNotificationsDouble{err: deliveryPolicy.ErrNotificationsDisabled}
	require.NoError(t, owners.NewParentDocumentNotifier(disabled, documentPreferencesDouble{optedIn: []int64{7301}}).NotifyBillingDocument(context.Background(), push),
		"a school that switched notifications off leaves the document in the inbox without a push")
}

// The pushes of a contract and of a release (#5941) share the letters'
// type, consent and copy; their keys and related types name their kind, so
// equal numeric IDs never collide. A school that switched notifications off
// enqueues nothing, which the release's fallback reads as not sent.
func TestTraegerDocumentNotifierKeysEachKindAndReportsWhatWentOut(t *testing.T) {
	t.Parallel()
	router := &documentNotificationsDouble{}
	var notifier parentService.TraegerDocumentNotifierPort = owners.NewParentDocumentNotifier(router, documentPreferencesDouble{optedIn: []int64{7301}})
	for _, kind := range []string{parentService.DocumentPushBilling, parentService.DocumentPushContract, parentService.DocumentPushRelease} {
		sent, err := notifier.NotifyTraegerDocument(context.Background(), parentService.BillingDocumentPush{Kind: kind, TenantID: 21, AccountID: 7301, StudentIDs: []int64{11}, DocumentID: 31})
		require.NoError(t, err)
		assert.True(t, sent, kind)
	}
	require.Len(t, router.events, 3)
	assert.Equal(t, []string{"parent-billing-document:31:7301", "parent-care-contract-document:31:7301", "parent-traeger-document-release:31:7301"},
		[]string{router.events[0].IdempotencyKey, router.events[1].IdempotencyKey, router.events[2].IdempotencyKey})
	assert.Equal(t, []string{"parent_billing_document", "care_contract_document", "traeger_document_release"},
		[]string{router.events[0].RelatedType, router.events[1].RelatedType, router.events[2].RelatedType})
	for _, event := range router.events {
		assert.Equal(t, "parent_billing_document", event.Type, "one consent for every kind")
		assert.Equal(t, "/documents", event.DeepLink)
	}

	disabled := &documentNotificationsDouble{err: deliveryPolicy.ErrNotificationsDisabled}
	sent, err := owners.NewParentDocumentNotifier(disabled, documentPreferencesDouble{optedIn: []int64{7301}}).
		NotifyTraegerDocument(context.Background(), parentService.BillingDocumentPush{Kind: parentService.DocumentPushRelease, TenantID: 21, AccountID: 7301, DocumentID: 31})
	require.NoError(t, err)
	assert.False(t, sent, "a school with notifications off sends nothing, so the next school is tried")
	sent, err = owners.NewParentDocumentNotifier(router, documentPreferencesDouble{}).
		NotifyTraegerDocument(context.Background(), parentService.BillingDocumentPush{Kind: parentService.DocumentPushRelease, TenantID: 21, AccountID: 7301, DocumentID: 31})
	require.NoError(t, err)
	assert.False(t, sent, "no consent, no push")
}
