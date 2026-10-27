package owners

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	deliveryPolicy "github.com/moto-nrw/project-phoenix/modules/delivery/devices"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

type recordedDocumentNotice struct {
	organizationID, documentID int64
	email                      string
}

type documentNoticesDouble struct{ got []recordedDocumentNotice }

func (d *documentNoticesDouble) NotifyBillingDocument(_ context.Context, organizationID, documentID int64, email string) error {
	d.got = append(d.got, recordedDocumentNotice{organizationID, documentID, email})
	return nil
}

func documentPushIntent(t *testing.T, organizationID, relatedID int64, notice parentbilling.BillingDocumentNotice) deliveryContract.ClaimedIntent {
	t.Helper()
	payload, err := json.Marshal(notice)
	require.NoError(t, err)
	return deliveryContract.ClaimedIntent{
		OrganizationID: organizationID, Transport: deliveryContract.TransportPush, Template: parentbilling.PushKindBillingDocument,
		RelatedEntityID: &relatedID, DeferredPayload: payload, PushRecipient: deliveryContract.PushRecipient{Deferred: true},
	}
}

// The worker hands the Träger's push of a new document to the parents
// portal and every other deferred push to the reminders; a push whose
// payload names another Träger or document is cancelled, never sent
// (ADR 0126).
func TestDeferredPushesRouteTheDocumentNoticeByKind(t *testing.T) {
	t.Parallel()
	documents := &documentNoticesDouble{}
	var reminders int
	route := deferredPushes(func(context.Context, deliveryContract.ClaimedIntent) error { reminders++; return nil }, documents)
	notice := parentbilling.BillingDocumentNotice{OrganizationID: 4, DocumentID: 31, RecipientEmail: "eva@example.org"}

	require.NoError(t, route(context.Background(), documentPushIntent(t, 4, 31, notice)))
	assert.Equal(t, []recordedDocumentNotice{{4, 31, "eva@example.org"}}, documents.got)
	assert.Zero(t, reminders)

	require.NoError(t, route(context.Background(), deliveryContract.ClaimedIntent{Template: reminderPushKind}))
	assert.Equal(t, 1, reminders, "every other kind stays with the reminders")

	for name, intent := range map[string]deliveryContract.ClaimedIntent{
		"another Träger":  documentPushIntent(t, 5, 31, notice),
		"another letter":  documentPushIntent(t, 4, 32, notice),
		"no Träger":       documentPushIntent(t, 0, 31, notice),
		"broken payload":  {OrganizationID: 4, Template: parentbilling.PushKindBillingDocument, DeferredPayload: []byte("{")},
		"no related item": {OrganizationID: 4, Template: parentbilling.PushKindBillingDocument, DeferredPayload: []byte(`{"organization_id":4}`)},
	} {
		err := route(context.Background(), intent)
		assert.ErrorIs(t, err, deliveryContract.ErrCancelled, name)
	}
	assert.Len(t, documents.got, 1, "a mismatched push reaches no parent")

	err := deferredPushes(nil, nil)(context.Background(), documentPushIntent(t, 4, 31, notice))
	assert.Error(t, err, "a worker without the parents portal retries instead of dropping the push")
}

type documentPreferencesDouble struct{ optedIn []int64 }

func (d documentPreferencesDouble) FilterOptedIn(_ context.Context, notificationType string, accountIDs []int64) ([]int64, error) {
	if notificationType != parentBillingDocumentType {
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
	push := parentportal.BillingDocumentPush{TenantID: 21, AccountID: 7301, StudentIDs: []int64{11, 12}, DocumentID: 31, Locale: "en"}

	silent := &documentNotificationsDouble{}
	require.NoError(t, NewParentDocumentNotifier(silent, documentPreferencesDouble{}).NotifyBillingDocument(context.Background(), push))
	assert.Empty(t, silent.events, "no consent, no push")

	router := &documentNotificationsDouble{}
	require.NoError(t, NewParentDocumentNotifier(router, documentPreferencesDouble{optedIn: []int64{7301}}).NotifyBillingDocument(context.Background(), push))
	require.Len(t, router.events, 1)
	event := router.events[0]
	assert.Equal(t, "parent_billing_document", event.Type)
	assert.Equal(t, "parent-billing-document:31:7301", event.IdempotencyKey)
	assert.Equal(t, parentbilling.PushRelatedBillingDocument, event.RelatedType)
	assert.Equal(t, int64(31), event.RelatedID)
	assert.Equal(t, deliveryPolicy.NotificationAudience{TenantID: 21, Scope: "guardian", GuardianAccountIDs: []int64{7301}, StudentIDs: []int64{11, 12}}, event.Audience)
	assert.Equal(t, "/documents", event.DeepLink)
	assert.Equal(t, "New document from the provider", event.Title)
	assert.Equal(t, "You have a new document in the parent portal.", event.Body)

	disabled := &documentNotificationsDouble{err: deliveryPolicy.ErrNotificationsDisabled}
	require.NoError(t, NewParentDocumentNotifier(disabled, documentPreferencesDouble{optedIn: []int64{7301}}).NotifyBillingDocument(context.Background(), push),
		"a school that switched notifications off leaves the document in the inbox without a push")
}
