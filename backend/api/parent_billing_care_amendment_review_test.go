// Router-level tests for the second review of the changes of a single
// invoice (#6197): a recipient change whose basis an earlier revocation took
// away is a conflict, a module with the longest name can be taken off, the
// review list carries the reason of every changed line and an issued invoice
// still shows which position its release took off.
package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A→B, then B→C, then A→B revoked: the chain resolves to its end state,
// so the invoice goes to C. The release, the Storno, the reissue in the
// next run of the month and the send all use that resolution: the send of
// the reissued invoice is accepted and reaches C (#6197).
func TestCareRecipientChainResolvesToItsEndStateThroughReissueAndSend(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Empfänger Kette")
	brandCareTraeger(t, f)
	run, contracts := amendableRun(t, f, "Mia")
	mia := contracts["Mia"]
	recipient := func(first, email string) map[string]any {
		return map[string]any{"kind": "recipient", "reason": "Absprache",
			"recipient": map[string]any{"first_name": first, "last_name": "Beispiel", "email": email}}
	}
	code, review := f.amend(t, run.ID, mia, recipient("Jan", "jan@example.org"))
	require.Equal(t, http.StatusOK, code)
	first := amendedInvoice(t, review, mia).Amendments[0]
	code, _ = f.amend(t, run.ID, mia, recipient("Ole", "ole@example.org"))
	require.Equal(t, http.StatusOK, code)
	code, review = f.amend(t, run.ID, mia, map[string]any{"kind": "revoke", "revokes_id": first.ID, "reason": "Doch nicht"})
	require.Equal(t, http.StatusOK, code)
	invoice := amendedInvoice(t, review, mia)
	assert.Equal(t, "Ole", invoice.Recipient.FirstName, "the change in force applies, whatever it was made on")
	assert.Zero(t, review.AmendmentConflicts)

	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(review.careRunBody))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	issued := amendedInvoice(t, decodeSchoolPortalData[careAmendedRunBody](t, rec.Body.Bytes()), mia)
	assert.Equal(t, "ole@example.org", issued.Recipient.Email)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters"), careLettersInput())
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, fmt.Sprintf("/invoices/%d/mail", issued.ID)), nil)
	require.Equal(t, http.StatusAccepted, rec.Code, "the issued invoice goes out to C: %s", rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, fmt.Sprintf("/invoices/%d/cancellation", issued.ID)),
		map[string]any{"reason": "Falsches Modul"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	next := f.startRun(t, careMonth)
	preview := f.amended(t, next.ID)
	assert.Equal(t, "Ole", amendedInvoice(t, preview, mia).Recipient.FirstName, "the reissue takes the resolved recipient over")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(next.ID, "/release"), careRelease(preview.careRunBody))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	reissued := amendedInvoice(t, decodeSchoolPortalData[careAmendedRunBody](t, rec.Body.Bytes()), mia)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(next.ID, "/letters"), careLettersInput())
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(next.ID, fmt.Sprintf("/invoices/%d/mail", reissued.ID)), nil)
	require.Equal(t, http.StatusAccepted, rec.Code, "the reissued invoice goes out to C: %s", rec.Body.String())
	mails := careLetterMails(t, f, next.ID)
	require.Len(t, mails, 1)
	var email string
	require.NoError(t, f.db.NewRaw(`SELECT recipient->>'address' FROM platform.email_outbox WHERE id = ?`, mails[0].ID).
		Scan(context.Background(), &email))
	assert.Equal(t, "ole@example.org", email)
}

// A module name may have 200 characters; its removal line adds the prefix
// and is still stored. The review list names the reason of every changed
// line, and the issued invoice keeps the key of the position it took off.
func TestCareRemovalOfALongModuleIsStoredExportedAndKept(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Langes Modul")
	school := f.office.schools[0]
	long := strings.Repeat("M", 200)
	mia := careContract(t, f.db, f.office.organization, school, "Mia", careMonth,
		careService{"mon", careModuleUntil1400, long, "14:00"}, careService{"tue", careModuleUntil1630, "Betreuung bis 16:30 Uhr", "16:30"})
	prices := carePriceList(school, "2029-09-01", 3200)
	for _, price := range prices["prices"].([]map[string]any) {
		if price["care_offering_id"] == careModuleUntil1400 {
			price["module_name"] = long
		}
	}
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, prices)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	run := f.startRun(t, careMonth)
	monday := amendedInvoice(t, f.amended(t, run.ID), mia).Positions[0]
	require.Equal(t, long, monday.ModuleName)

	code, _ := f.amend(t, run.ID, mia, map[string]any{"kind": "remove_position", "position_key": monday.Key, "reason": "Montag fällt weg"})
	require.Equal(t, http.StatusOK, code, "the removal of a module with the longest name is stored")
	code, review := f.amend(t, run.ID, mia, map[string]any{"kind": "add_position", "label": "Ausflug", "amount_cents": 500, "reason": "Elternbrief"})
	require.Equal(t, http.StatusOK, code)
	list := reviewListCSV(t, f, run.ID)
	assert.Contains(t, list, "Ausflug (Grund: Elternbrief)", "the review list names the reason of an added position")
	assert.Contains(t, list, "(Grund: Montag fällt weg)", "and of a removed one")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(review.careRunBody))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	issued := amendedInvoice(t, decodeSchoolPortalData[careAmendedRunBody](t, rec.Body.Bytes()), mia)
	require.NotEmpty(t, issued.Positions)
	assert.Equal(t, monday.Key, issued.Positions[0].Key, "the issued position keeps the key its removal names")
}
