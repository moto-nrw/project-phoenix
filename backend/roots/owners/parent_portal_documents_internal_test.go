package owners

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
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
