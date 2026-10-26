// Router-level tests for the mail and the print of the confirmation letters
// of a released billing run (#4670, ADR 0123): every family once as a
// series, one family first or again, the state of the last mail of every
// family, all letters in one PDF, the right "Abrechnung", the Träger
// boundary and the record of every send and read.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

type letterMailingBody struct {
	DocumentID     int64      `json:"document_id"`
	Number         string     `json:"number"`
	RecipientEmail string     `json:"recipient_email"`
	Status         string     `json:"status"`
	Sends          int        `json:"sends"`
	Attempts       int        `json:"attempts"`
	RequestedAt    *time.Time `json:"requested_at"`
	SentAt         *time.Time `json:"sent_at"`
}

type letterSeriesBody struct {
	Queued       int `json:"queued"`
	Already      int `json:"already"`
	WithoutEmail int `json:"without_email"`
}

type queuedLetterMail struct {
	ID             int64           `bun:"id"`
	TenantID       *int64          `bun:"tenant_id"`
	Recipient      json.RawMessage `bun:"recipient"`
	Payload        json.RawMessage `bun:"payload"`
	IdempotencyKey string          `bun:"idempotency_key"`
}

func letterMailPath(runID, documentID int64) string {
	return billingRunPath(runID, fmt.Sprintf("/documents/%d/mailings", documentID))
}

// lettersOfRun is a released run of two families with their letters.
func lettersOfRun(t *testing.T, f reviewFixture, billing string) billingRunLettersBody {
	t.Helper()
	run := releasedBillingRun(t, f, billing)
	brandTraeger(t, f)
	return run
}

func letterMailings(t *testing.T, f reviewFixture, token string, runID int64) map[string]letterMailingBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, token, http.MethodGet, billingRunPath(runID, "/mailings"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := decodeSchoolPortalData[struct {
		Documents []letterMailingBody `json:"documents"`
	}](t, rec.Body.Bytes())
	byNumber := map[string]letterMailingBody{}
	for _, document := range body.Documents {
		byNumber[document.Number] = document
	}
	return byNumber
}

func queuedLetterMails(t *testing.T, f reviewFixture, runID int64) []queuedLetterMail {
	t.Helper()
	var mails []queuedLetterMail
	require.NoError(t, f.db.NewRaw(`SELECT id, tenant_id, recipient, payload, idempotency_key FROM platform.email_outbox
		WHERE organization_id = ? AND kind = 'parent_billing_letter' AND related_entity_type = 'parent_billing_run' AND related_entity_id = ?
		ORDER BY id`, f.office.organization, runID).Scan(context.Background(), &mails))
	return mails
}

func TestBillingLetterMailsReachEveryFamilyOnceAndShowTheirState(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Versand Ferien")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	run := lettersOfRun(t, f, billing)
	ctx := context.Background()

	// Without letters there is nothing to send or print.
	for _, request := range []struct{ method, path string }{
		{http.MethodGet, billingRunPath(run.ID, "/mailings")},
		{http.MethodPost, billingRunPath(run.ID, "/mailings")},
		{http.MethodPost, letterMailPath(run.ID, run.Documents[0].ID)},
		{http.MethodGet, billingRunPath(run.ID, "/letters/print")},
	} {
		rec := traegerJSON(t, f.traeger, billing, request.method, request.path, nil)
		require.Equal(t, http.StatusConflict, rec.Code, "%s %s: %s", request.method, request.path, rec.Body.String())
		code, _ := registrationRefusalOf(t, rec)
		assert.Equal(t, "parent_billing.billing_letters_missing", code, request.path)
	}
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/letters"), lettersBody("2099-01-15"))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	before := letterMailings(t, f, billing, run.ID)
	require.Len(t, before, 2)
	for number, mailing := range before {
		assert.Equal(t, "not_sent", mailing.Status, number)
		assert.Zero(t, mailing.Sends, number)
		assert.Nil(t, mailing.RequestedAt, number)
	}

	// The series puts one mail of the Träger per family on the outbox, with
	// the stored letter attached.
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/mailings"), nil)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	assert.Equal(t, letterSeriesBody{Queued: 2}, decodeSchoolPortalData[letterSeriesBody](t, rec.Body.Bytes()))
	mails := queuedLetterMails(t, f, run.ID)
	require.Len(t, mails, 2)
	for _, mail := range mails {
		assert.Nil(t, mail.TenantID, "a mail of the Träger has no school")
		var payload struct {
			DocumentID  int64  `json:"document_id"`
			TraegerName string `json:"traeger_name"`
			Number      string `json:"number"`
			Filename    string `json:"filename"`
			PDF         []byte `json:"pdf"`
		}
		require.NoError(t, json.Unmarshal(mail.Payload, &payload))
		var recipient struct {
			Address string `json:"address"`
		}
		require.NoError(t, json.Unmarshal(mail.Recipient, &recipient))
		assert.Equal(t, before[payload.Number].RecipientEmail, recipient.Address, "the mail goes to the family of the invoice")
		assert.Equal(t, fmt.Sprintf("parent-billing-letter:%d:1", payload.DocumentID), mail.IdempotencyKey)
		assert.NotEmpty(t, payload.TraegerName)
		var stored []byte
		require.NoError(t, f.db.NewRaw(`SELECT pdf FROM parent_billing.document_letters WHERE document_id = ?`, payload.DocumentID).Scan(ctx, &stored))
		assert.Equal(t, stored, payload.PDF, "the mail carries the stored letter")
		assert.Contains(t, payload.Filename, "bestaetigung-")
	}

	// A second series sends nothing twice.
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/mailings"), nil)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	assert.Equal(t, letterSeriesBody{Already: 2}, decodeSchoolPortalData[letterSeriesBody](t, rec.Body.Bytes()))
	assert.Len(t, queuedLetterMails(t, f, run.ID), 2)

	queued := letterMailings(t, f, billing, run.ID)
	first, second := queued["FE-0001"], queued["FE-0002"]
	assert.Equal(t, "queued", first.Status)
	assert.Equal(t, 1, first.Sends)
	assert.NotNil(t, first.RequestedAt)

	// While the mail waits, the family is not mailed again.
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, letterMailPath(run.ID, first.DocumentID), nil)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.letter_mail_pending", code)

	// Delivery gives up on the first mail and the mail server accepts the
	// second; the run shows both.
	setMailState := func(documentID int64, statement string) {
		t.Helper()
		_, err := f.db.ExecContext(ctx, `UPDATE platform.email_outbox SET `+statement+`
			WHERE organization_id = ? AND kind = 'parent_billing_letter' AND (payload->>'document_id')::bigint = ?`, f.office.organization, documentID)
		require.NoError(t, err)
	}
	setMailState(first.DocumentID, `status = 'dead_letter', dead_letter_at = NOW(), attempts = 5, last_error = 'mailbox unavailable'`)
	setMailState(second.DocumentID, `status = 'sent', sent_at = NOW(), attempts = 1`)
	states := letterMailings(t, f, billing, run.ID)
	assert.Equal(t, "failed", states["FE-0001"].Status)
	assert.Equal(t, 5, states["FE-0001"].Attempts)
	assert.Equal(t, "sent", states["FE-0002"].Status)
	assert.NotNil(t, states["FE-0002"].SentAt)

	// The failed letter is sent again with a new outbox entry.
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, letterMailPath(run.ID, first.DocumentID), nil)
	require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
	again := decodeSchoolPortalData[letterMailingBody](t, rec.Body.Bytes())
	assert.Equal(t, "queued", again.Status)
	assert.Equal(t, 2, again.Sends)
	mails = queuedLetterMails(t, f, run.ID)
	require.Len(t, mails, 3)
	assert.Equal(t, fmt.Sprintf("parent-billing-letter:%d:2", first.DocumentID), mails[2].IdempotencyKey)

	// Once Delivery no longer keeps a mail, its result is unknown.
	_, err := f.db.ExecContext(ctx, `DELETE FROM platform.email_outbox
		WHERE organization_id = ? AND kind = 'parent_billing_letter' AND (payload->>'document_id')::bigint = ?`, f.office.organization, second.DocumentID)
	require.NoError(t, err)
	assert.Equal(t, "unknown", letterMailings(t, f, billing, run.ID)["FE-0002"].Status)

	// All letters print as one PDF.
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, billingRunPath(run.ID, "/letters/print"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Header().Get("Content-Disposition"), `attachment; filename="bestaetigungsschreiben-`)
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	assert.True(t, bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")))
	pages := bytes.Count(rec.Body.Bytes(), []byte("/Type /Page")) - bytes.Count(rec.Body.Bytes(), []byte("/Type /Pages"))
	assert.GreaterOrEqual(t, pages, 2, "every letter is in the print")

	// Every send is stored with its account and address; every read stands
	// in the trail.
	var sends, withAccount int
	require.NoError(t, f.db.NewRaw(`SELECT count(*), count(requested_by_account_id) FROM parent_billing.document_mailings WHERE run_id = ?`, run.ID).
		Scan(ctx, &sends, &withAccount))
	assert.Equal(t, 3, sends)
	assert.Equal(t, 3, withAccount)
	var reads []string
	require.NoError(t, f.db.NewRaw(`SELECT COALESCE(details->>'mail', details->>'letter') FROM parent_billing.run_events
		WHERE run_id = ? AND action = 'read' AND (details ? 'mail' OR details->>'letter' = 'print') ORDER BY id`, run.ID).Scan(ctx, &reads))
	assert.Equal(t, []string{"status", "series", "series", "status", "status", "status", "print"}, reads)
}

func TestBillingLetterMailsNeedTheBillingRightAndStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Versand Recht")
	billing := traegerColleague(t, f, identityaccess.TraegerRoleBilling)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Versand Fremd", identityaccess.TraegerRoleBilling)
	run := lettersOfRun(t, f, billing)
	rec := traegerJSON(t, f.traeger, billing, http.MethodPost, billingRunPath(run.ID, "/letters"), lettersBody("2099-01-15"))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	document := run.Documents[0].ID

	requests := []struct{ method, path string }{
		{http.MethodGet, billingRunPath(run.ID, "/mailings")},
		{http.MethodPost, billingRunPath(run.ID, "/mailings")},
		{http.MethodPost, letterMailPath(run.ID, document)},
		{http.MethodGet, billingRunPath(run.ID, "/letters/print")},
	}
	for _, request := range requests {
		rec := traegerJSON(t, f.traeger, f.office.token, request.method, request.path, nil)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s without the right Abrechnung: %s", request.method, request.path, rec.Body.String())
	}
	for _, request := range requests {
		rec := traegerJSON(t, f.traeger, foreign.token, request.method, request.path, nil)
		assert.Equal(t, http.StatusNotFound, rec.Code, "%s %s of another Träger: %s", request.method, request.path, rec.Body.String())
	}
	var sends, mails, reads int
	require.NoError(t, f.db.NewRaw(`SELECT
			(SELECT count(*) FROM parent_billing.document_mailings WHERE run_id = ?),
			(SELECT count(*) FROM platform.email_outbox WHERE kind = 'parent_billing_letter' AND related_entity_id = ?),
			(SELECT count(*) FROM parent_billing.run_events WHERE run_id = ? AND (details ? 'mail' OR details->>'letter' = 'print'))`,
		run.ID, run.ID, run.ID).Scan(context.Background(), &sends, &mails, &reads))
	assert.Zero(t, sends, "refused requests stored no send")
	assert.Zero(t, mails, "refused requests queued no mail")
	assert.Zero(t, reads, "refused requests read nothing")

	// An invoice of another run is not found under this run.
	rec = traegerJSON(t, f.traeger, billing, http.MethodPost, letterMailPath(run.ID, document+1000), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.billing_letter_not_found", code)
}
