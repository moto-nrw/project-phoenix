// Router-level test for the push of a new document (#4671, ADR 0126): the
// send of a confirmation letter puts the Träger's deferred push on the
// outbox in the same Träger transaction, once per letter.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

type queuedDocumentNotice struct {
	TenantID       *int64          `bun:"tenant_id"`
	Recipient      json.RawMessage `bun:"recipient"`
	Payload        json.RawMessage `bun:"payload"`
	IdempotencyKey string          `bun:"idempotency_key"`
	RelatedID      int64           `bun:"related_entity_id"`
}

func TestBillingLetterSendPushesTheInboxOncePerLetter(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Postfach Push")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	run := lettersOfRun(t, f, billing)
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/letters"), lettersBody("2099-01-15"))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	notices := func() []queuedDocumentNotice {
		t.Helper()
		var queued []queuedDocumentNotice
		require.NoError(t, f.db.NewRaw(`SELECT tenant_id, recipient, payload, idempotency_key, related_entity_id FROM platform.push_outbox
			WHERE organization_id = ? AND kind = 'parent_billing_document' ORDER BY related_entity_id`, f.office.organization).
			Scan(context.Background(), &queued))
		return queued
	}

	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/mailings"), nil)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	queued := notices()
	require.Len(t, queued, 2, "one push of the Träger per sent letter")
	mailings := letterMailings(t, f, billing, run.ID)
	for _, notice := range queued {
		assert.Nil(t, notice.TenantID, "a push of the Träger names no school")
		assert.JSONEq(t, `{"deferred":true}`, string(notice.Recipient), "the devices are found when the worker sends")
		assert.Equal(t, fmt.Sprintf("parent-billing-document:%d", notice.RelatedID), notice.IdempotencyKey)
		var payload struct {
			OrganizationID int64  `json:"organization_id"`
			DocumentID     int64  `json:"document_id"`
			RecipientEmail string `json:"recipient_email"`
		}
		require.NoError(t, json.Unmarshal(notice.Payload, &payload))
		assert.Equal(t, f.office.organization, payload.OrganizationID)
		assert.Equal(t, notice.RelatedID, payload.DocumentID)
		var recipient string
		for _, mailing := range mailings {
			if mailing.DocumentID == payload.DocumentID {
				recipient = mailing.RecipientEmail
			}
		}
		assert.Equal(t, recipient, payload.RecipientEmail, "the push names the address of the invoice")
	}

	// Sending a letter again mails it again but pushes nothing new.
	first := mailings["FE-0001"]
	_, err := f.db.ExecContext(context.Background(), `UPDATE platform.email_outbox SET status = 'sent', sent_at = NOW()
		WHERE organization_id = ? AND kind = 'parent_billing_letter'`, f.office.organization)
	require.NoError(t, err)
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, letterMailPath(run.ID, first.DocumentID), nil)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	assert.Len(t, notices(), 2, "only the first send of a letter pushes")
}
