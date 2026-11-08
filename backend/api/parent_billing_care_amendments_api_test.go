// Router-level tests for the changes of a single invoice in the review of a
// monthly run (#6197, ADR 0119 addendum 2026-11-08 "Änderungen je
// Rechnung"): amount, recipient, address and positions change only the
// invoice of this run, by entries that are only appended; every change
// enters the fingerprint, so a release of the state before refuses; the
// release copies the changes onto the issued invoice, its letter and the
// review list; after the release nothing changes, and the new invoice after
// a Storno takes the changes over; the right "Abrechnung", the Träger
// boundary and the switches of the Träger hold.
package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type careAmendmentBody struct {
	ID                int64  `json:"id"`
	Kind              string `json:"kind"`
	Label             string `json:"label"`
	AmountCents       int64  `json:"amount_cents"`
	Reason            string `json:"reason"`
	AccountID         *int64 `json:"account_id"`
	Carried           bool   `json:"carried"`
	CarriedFromNumber string `json:"carried_from_number"`
	Conflict          string `json:"conflict"`
	Before            struct {
		TotalCents *int64 `json:"total_cents"`
	} `json:"before"`
	Revocation *struct {
		Reason string `json:"reason"`
	} `json:"revocation"`
}

type careAmendedInvoiceBody struct {
	ID         int64 `json:"id"`
	ContractID int64 `json:"contract_id"`
	TotalCents int64 `json:"total_cents"`
	Recipient  struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Email     string `json:"email"`
		Address   *struct {
			Street     string `json:"street"`
			PostalCode string `json:"postal_code"`
			City       string `json:"city"`
		} `json:"address"`
	} `json:"recipient"`
	Positions []struct {
		Key         string `json:"key"`
		ModuleName  string `json:"module_name"`
		AmountCents int64  `json:"amount_cents"`
	} `json:"positions"`
	Amendments []careAmendmentBody `json:"amendments"`
}

type careAmendedRunBody struct {
	careRunBody
	Invoices  []careAmendedInvoiceBody `json:"invoices"`
	Amendable struct {
		Amount, Recipient, Address, Positions bool
	} `json:"amendable"`
	AmendmentConflicts int `json:"amendment_conflicts"`
}

func (f careBillingFixture) amended(t *testing.T, id int64) careAmendedRunBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(id, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[careAmendedRunBody](t, rec.Body.Bytes())
}

// amend posts a change on the review the run shows now.
func (f careBillingFixture) amend(t *testing.T, runID, contractID int64, change map[string]any) (int, careAmendedRunBody) {
	t.Helper()
	body := map[string]any{"contract_id": contractID, "fingerprint": f.amended(t, runID).Fingerprint}
	for key, value := range change {
		body[key] = value
	}
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(runID, "/amendments"), body)
	if rec.Code != http.StatusOK {
		return rec.Code, careAmendedRunBody{}
	}
	return rec.Code, decodeSchoolPortalData[careAmendedRunBody](t, rec.Body.Bytes())
}

func amendedInvoice(t *testing.T, run careAmendedRunBody, contractID int64) careAmendedInvoiceBody {
	t.Helper()
	for _, invoice := range run.Invoices {
		if invoice.ContractID == contractID {
			return invoice
		}
	}
	require.FailNow(t, "no invoice for the contract")
	return careAmendedInvoiceBody{}
}

// amendableRun prices the first school and starts a run of the month with
// one invoice per child: 32 € Monday plus 45,50 € Tuesday = 77,50 €.
func amendableRun(t *testing.T, f careBillingFixture, children ...string) (careRunBody, map[string]int64) {
	t.Helper()
	school := f.office.schools[0]
	contracts := map[string]int64{}
	for _, child := range children {
		contracts[child] = careContract(t, f.db, f.office.organization, school, child, careMonth, sjrServices()...)
	}
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(school, "2029-09-01", 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return f.startRun(t, careMonth), contracts
}

func reviewListCSV(t *testing.T, f careBillingFixture, runID int64) string {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(runID, "/review-export?format=csv"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return rec.Body.String()
}

// Each kind of change shows in the preview and the review list, holds for
// this invoice only and goes onto the issued invoice and its letter.
func TestCareAmendmentsChangeOneInvoiceUpToItsLetter(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Änderung je Rechnung")
	brandCareTraeger(t, f)
	run, contracts := amendableRun(t, f, "Mia", "Lea")
	mia, lea := contracts["Mia"], contracts["Lea"]
	review := f.amended(t, run.ID)
	assert.True(t, review.Amendable.Amount && review.Amendable.Recipient && review.Amendable.Address && review.Amendable.Positions,
		"every change is on by default")
	require.Equal(t, int64(7750), amendedInvoice(t, review, mia).TotalCents)
	tuesday := amendedInvoice(t, review, mia).Positions[1]
	require.Equal(t, "Betreuung bis 16:30 Uhr", tuesday.ModuleName)

	code, review := f.amend(t, run.ID, mia, map[string]any{"kind": "amount", "total_cents": 7000, "reason": "Kulanz wegen Schließtag"})
	require.Equal(t, http.StatusOK, code)
	invoice := amendedInvoice(t, review, mia)
	assert.Equal(t, int64(7000), invoice.TotalCents, "the amount change adds a correction line")
	require.Len(t, invoice.Amendments, 1)
	assert.Equal(t, int64(-750), invoice.Amendments[0].AmountCents)
	assert.Equal(t, int64(7750), *invoice.Amendments[0].Before.TotalCents, "the entry keeps the old total")
	assert.NotNil(t, invoice.Amendments[0].AccountID, "the entry names who changed it")
	assert.Len(t, invoice.Positions, 2, "the computed positions stay")

	code, review = f.amend(t, run.ID, mia, map[string]any{"kind": "add_position", "label": "Ausflug Wildpark", "amount_cents": 1200, "reason": "Elternbrief vom 3.1."})
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, int64(8200), amendedInvoice(t, review, mia).TotalCents)
	code, review = f.amend(t, run.ID, mia, map[string]any{"kind": "remove_position", "position_key": tuesday.Key, "reason": "Dienstag nicht gebucht"})
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, int64(3650), amendedInvoice(t, review, mia).TotalCents, "the removal takes the Tuesday off")
	code, _ = f.amend(t, run.ID, mia, map[string]any{"kind": "remove_position", "position_key": tuesday.Key, "reason": "Noch einmal"})
	assert.Equal(t, http.StatusConflict, code, "a position goes off once")

	code, review = f.amend(t, run.ID, mia, map[string]any{"kind": "recipient", "reason": "Rechnung an den Vater",
		"recipient": map[string]any{"first_name": "Jan", "last_name": "Beispiel", "email": "jan@example.org"}})
	require.Equal(t, http.StatusOK, code)
	code, review = f.amend(t, run.ID, mia, map[string]any{"kind": "address", "reason": "Umzug",
		"address": map[string]any{"street": "Neue Straße 4", "postal_code": "75172", "city": "Pforzheim"}})
	require.Equal(t, http.StatusOK, code)
	invoice = amendedInvoice(t, review, mia)
	assert.Equal(t, "Jan", invoice.Recipient.FirstName)
	require.NotNil(t, invoice.Recipient.Address)
	assert.Equal(t, "Neue Straße 4", invoice.Recipient.Address.Street)
	assert.Equal(t, int64(3650), invoice.TotalCents, "a recipient change leaves the total")
	other := amendedInvoice(t, review, lea)
	assert.Equal(t, "Eva", other.Recipient.FirstName, "the other invoice keeps its recipient")
	assert.Equal(t, int64(7750), other.TotalCents)
	assert.Empty(t, other.Amendments)

	list := reviewListCSV(t, f, run.ID)
	for _, want := range []string{"Korrektur: Kulanz wegen Schließtag", "Ausflug Wildpark", "Entfällt: Betreuung bis 16:30 Uhr", "Jan Beispiel"} {
		assert.Contains(t, list, want, "the review list shows the change")
	}

	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(review.careRunBody))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	released := decodeSchoolPortalData[careAmendedRunBody](t, rec.Body.Bytes())
	issued := amendedInvoice(t, released, mia)
	assert.Equal(t, int64(3650), issued.TotalCents, "the release issues the reviewed total")
	assert.Len(t, issued.Amendments, 5, "the issued invoice names every change")
	var stored struct {
		Total     int64  `bun:"total_cents"`
		FirstName string `bun:"recipient_first_name"`
		Street    string `bun:"recipient_address_street"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT total_cents, recipient_first_name, recipient_address_street
		FROM parent_billing.care_invoices WHERE id = ?`, issued.ID).Scan(context.Background(), &stored))
	assert.Equal(t, int64(3650), stored.Total)
	assert.Equal(t, "Jan", stored.FirstName)
	assert.Equal(t, "Neue Straße 4", stored.Street)
	var lines int64
	require.NoError(t, f.db.NewRaw(`SELECT COALESCE(SUM(amount_cents), 0) FROM parent_billing.care_invoice_corrections WHERE invoice_id = ?`,
		issued.ID).Scan(context.Background(), &lines))
	assert.Equal(t, int64(3650-7750), lines, "the correction lines add up to the difference")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters"), careLettersInput())
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, careRunPath(run.ID, fmt.Sprintf("/invoices/%d/letter", issued.ID)), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.True(t, strings.HasPrefix(rec.Body.String(), "%PDF"), "the letter of the changed invoice renders")

	var families int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.family_billing WHERE organization_id = ?`, f.office.organization).
		Scan(context.Background(), &families))
	assert.Zero(t, families, "a recipient change of the run is no family deviation (#4689)")
	nextMonth := f.startRun(t, "2030-02-01")
	assert.Equal(t, "Eva", amendedInvoice(t, f.amended(t, nextMonth.ID), mia).Recipient.FirstName,
		"the next month goes to the contract party again")
}

// A change after the review makes the release of the reviewed state refuse;
// a change made on an old review is refused too, so two changes at once
// never overwrite each other unseen.
func TestCareAmendmentInvalidatesTheReviewedState(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Änderung Fingerabdruck")
	run, contracts := amendableRun(t, f, "Mia")
	reviewed := f.amended(t, run.ID)
	code, _ := f.amend(t, run.ID, contracts["Mia"], map[string]any{"kind": "amount", "total_cents": 5000, "reason": "Kulanz"})
	require.Equal(t, http.StatusOK, code)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(reviewed.careRunBody))
	assert.Equal(t, http.StatusConflict, rec.Code, "the release of the state before the change: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.billing_run_changed")

	stale := map[string]any{"contract_id": contracts["Mia"], "fingerprint": reviewed.Fingerprint, "kind": "amount",
		"total_cents": 6000, "reason": "Alter Stand"}
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/amendments"), stale)
	assert.Equal(t, http.StatusConflict, rec.Code, "a change on an old review: %s", rec.Body.String())

	current := f.amended(t, run.ID)
	codes := make([]int, 2)
	var wg sync.WaitGroup
	for i, total := range []int64{4000, 4500} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/amendments"),
				map[string]any{"contract_id": contracts["Mia"], "fingerprint": current.Fingerprint, "kind": "amount",
					"total_cents": total, "reason": "Gleichzeitig"})
			codes[i] = rec.Code
		}()
	}
	wg.Wait()
	assert.ElementsMatch(t, []int{http.StatusOK, http.StatusConflict}, codes, "two changes on one review: one wins, one is refused")
	assert.Len(t, amendedInvoice(t, f.amended(t, run.ID), contracts["Mia"]).Amendments, 2)

	final := f.amended(t, run.ID)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(final.careRunBody))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// A change whose basis changed is shown as a conflict and blocks the
// release until it is revoked; a revocation is another entry.
func TestCareAmendmentInConflictBlocksTheReleaseUntilRevoked(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Änderung Konflikt")
	run, contracts := amendableRun(t, f, "Mia")
	mia := contracts["Mia"]
	code, review := f.amend(t, run.ID, mia, map[string]any{"kind": "amount", "total_cents": 7000, "reason": "Kulanz"})
	require.Equal(t, http.StatusOK, code)
	entry := amendedInvoice(t, review, mia).Amendments[0]

	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(f.office.schools[0], "2029-09-01", 3500))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	review = f.amended(t, run.ID)
	invoice := amendedInvoice(t, review, mia)
	assert.Equal(t, "basis_changed", invoice.Amendments[0].Conflict, "the total the change corrected is gone")
	assert.Equal(t, int64(8050), invoice.TotalCents, "a change in conflict is not applied")
	assert.Equal(t, 1, review.AmendmentConflicts)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(review.careRunBody))
	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.care_amendments_unresolved")

	code, review = f.amend(t, run.ID, mia, map[string]any{"kind": "revoke", "revokes_id": entry.ID, "reason": "Preis geändert"})
	require.Equal(t, http.StatusOK, code)
	invoice = amendedInvoice(t, review, mia)
	require.NotNil(t, invoice.Amendments[0].Revocation)
	assert.Equal(t, "Preis geändert", invoice.Amendments[0].Revocation.Reason)
	code, _ = f.amend(t, run.ID, mia, map[string]any{"kind": "revoke", "revokes_id": entry.ID, "reason": "Noch einmal"})
	assert.Equal(t, http.StatusConflict, code, "a change is revoked once")
	var rows int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.care_run_amendments WHERE run_id = ?`, run.ID).
		Scan(context.Background(), &rows))
	assert.Equal(t, 2, rows, "the revocation is appended, the change stays")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(review.careRunBody))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// Changes are checked against the invoice: a total below zero, an unknown
// position, a recipient without a valid mail address, an empty reason.
func TestCareAmendmentIsCheckedAgainstTheInvoice(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Änderung Prüfung")
	run, contracts := amendableRun(t, f, "Mia")
	mia := contracts["Mia"]
	for _, change := range []map[string]any{
		{"kind": "amount", "total_cents": -1, "reason": "Unter null"},
		{"kind": "amount", "total_cents": 7750, "reason": "Unverändert"},
		{"kind": "amount", "total_cents": 7000, "reason": " "},
		{"kind": "add_position", "label": "", "amount_cents": 100, "reason": "Ohne Text"},
		{"kind": "recipient", "reason": "Ohne Mail", "recipient": map[string]any{"first_name": "Jan", "last_name": "B", "email": "kein-mail"}},
		{"kind": "address", "reason": "Ohne PLZ", "address": map[string]any{"street": "Weg 1", "postal_code": "1", "city": "Pforzheim"}},
		{"kind": "unknown", "reason": "Unbekannt"},
	} {
		code, _ := f.amend(t, run.ID, mia, change)
		assert.Equal(t, http.StatusBadRequest, code, "%v", change)
	}
	code, _ := f.amend(t, run.ID, mia, map[string]any{"kind": "remove_position", "position_key": "care:1:1:2030-01-01", "reason": "Gibt es nicht"})
	assert.Equal(t, http.StatusConflict, code, "an unknown position")
	code, _ = f.amend(t, run.ID, mia+1000000, map[string]any{"kind": "amount", "total_cents": 100, "reason": "Fremd"})
	assert.Equal(t, http.StatusConflict, code, "an invoice the preview does not have")
	assert.Empty(t, amendedInvoice(t, f.amended(t, run.ID), mia).Amendments, "no refused change is stored")
}

// After the release no invoice changes: the API answers 409 and the
// database refuses an entry of a released run and any update of an entry.
func TestCareAmendmentIsRefusedAfterTheRelease(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Änderung nach Ausstellung")
	run, contracts := amendableRun(t, f, "Mia")
	code, review := f.amend(t, run.ID, contracts["Mia"], map[string]any{"kind": "amount", "total_cents": 7000, "reason": "Kulanz"})
	require.Equal(t, http.StatusOK, code)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(review.careRunBody))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/amendments"),
		map[string]any{"contract_id": contracts["Mia"], "fingerprint": review.Fingerprint, "kind": "amount", "total_cents": 100, "reason": "Zu spät"})
	assert.Equal(t, http.StatusConflict, rec.Code, "a released run takes no change: %s", rec.Body.String())
	_, err := f.db.ExecContext(context.Background(), `INSERT INTO parent_billing.care_run_amendments (organization_id, run_id,
		care_contract_id, service, kind, amount_cents, label, reason) VALUES (?, ?, ?, 'care', 'add_position', 100, 'Zu spät', 'Zu spät')`,
		f.office.organization, run.ID, contracts["Mia"])
	assert.ErrorContains(t, err, "never changes", "the database refuses an entry of a released run for every role")
	_, err = f.db.ExecContext(context.Background(), `UPDATE parent_billing.care_run_amendments SET reason = 'Anders' WHERE run_id = ?`, run.ID)
	assert.ErrorContains(t, err, "only ever appended")
	_, err = f.db.ExecContext(context.Background(), `UPDATE parent_billing.care_invoice_corrections SET reason = 'Anders'
		WHERE organization_id = ?`, f.office.organization)
	assert.ErrorContains(t, err, "never changes", "an issued correction never changes")
}

// The new invoice after a Storno takes the changes of the cancelled one
// over (ADR 0119 addendum 2026-11-04); the run may revoke a carried change.
func TestCareReissueCarriesTheChangesOfTheCancelledInvoice(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Änderung Neuausstellung")
	brandCareTraeger(t, f)
	run, contracts := amendableRun(t, f, "Mia")
	mia := contracts["Mia"]
	code, _ := f.amend(t, run.ID, mia, map[string]any{"kind": "amount", "total_cents": 7000, "reason": "Kulanz"})
	require.Equal(t, http.StatusOK, code)
	code, review := f.amend(t, run.ID, mia, map[string]any{"kind": "recipient", "reason": "An den Vater",
		"recipient": map[string]any{"first_name": "Jan", "last_name": "Beispiel", "email": "jan@example.org"}})
	require.Equal(t, http.StatusOK, code)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(review.careRunBody))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	issued := amendedInvoice(t, decodeSchoolPortalData[careAmendedRunBody](t, rec.Body.Bytes()), mia)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters"), careLettersInput())
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, fmt.Sprintf("/invoices/%d/cancellation", issued.ID)),
		map[string]any{"reason": "Falsches Modul"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	next := f.startRun(t, careMonth)
	preview := amendedInvoice(t, f.amended(t, next.ID), mia)
	assert.Equal(t, int64(7000), preview.TotalCents, "the new invoice takes the amount change over")
	assert.Equal(t, "Jan", preview.Recipient.FirstName, "and the recipient change")
	require.Len(t, preview.Amendments, 2)
	for _, carried := range preview.Amendments {
		assert.True(t, carried.Carried)
		assert.Equal(t, issued.Number(t, f), carried.CarriedFromNumber)
	}

	code, review = f.amend(t, next.ID, mia, map[string]any{"kind": "revoke", "revokes_id": preview.Amendments[1].ID,
		"revokes_carried": true, "reason": "Wieder an die Mutter"})
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "Eva", amendedInvoice(t, review, mia).Recipient.FirstName, "a carried change can be revoked")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(next.ID, "/release"), careRelease(review.careRunBody))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	reissued := amendedInvoice(t, decodeSchoolPortalData[careAmendedRunBody](t, rec.Body.Bytes()), mia)
	assert.Equal(t, int64(7000), reissued.TotalCents)
	require.Len(t, reissued.Amendments, 1)
	assert.Equal(t, issued.Number(t, f), reissued.Amendments[0].CarriedFromNumber, "the issued correction names where it came from")
}

// Number reads the number of an issued invoice.
func (invoice careAmendedInvoiceBody) Number(t *testing.T, f careBillingFixture) string {
	t.Helper()
	var number string
	require.NoError(t, f.db.NewRaw(`SELECT number FROM parent_billing.care_invoices WHERE id = ?`, invoice.ID).Scan(context.Background(), &number))
	return number
}

// The change needs the right "Abrechnung" and a run of the own Träger; a
// kind the Träger switched off is refused.
func TestCareAmendmentNeedsTheRightTheTraegerAndItsSwitch(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Änderung Recht")
	run, contracts := amendableRun(t, f, "Mia")
	review := f.amended(t, run.ID)
	body := map[string]any{"contract_id": contracts["Mia"], "fingerprint": review.Fingerprint, "kind": "amount", "total_cents": 7000, "reason": "Kulanz"}

	password := testpkg.NewTestPassword()
	account := traegerAccount(t, f.db, password)
	testpkg.CreateTestTraegerMember(t, f.db, f.office.organization, account.ID, identityaccess.TraegerRoleAdministration)
	code, answer := traegerSignIn(t, f.traeger, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	rec := traegerJSON(t, f.traeger, answer.AccessToken, http.MethodPost, careRunPath(run.ID, "/amendments"), body)
	assert.Equal(t, http.StatusForbidden, rec.Code, "without the right: %s", rec.Body.String())
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Änderung Fremd", identityaccess.TraegerRoleBilling)
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodPost, careRunPath(run.ID, "/amendments"), body)
	assert.Equal(t, http.StatusNotFound, rec.Code, "a run of another Träger: %s", rec.Body.String())

	_, err := f.db.ExecContext(context.Background(), `INSERT INTO config.traeger_setting_values (organization_id, setting_key, value)
		VALUES (?, 'parent_billing.care_review_amount', 'false'::jsonb)`, f.office.organization)
	require.NoError(t, err)
	review = f.amended(t, run.ID)
	assert.False(t, review.Amendable.Amount)
	assert.True(t, review.Amendable.Positions, "the other switches stay on")
	body["fingerprint"] = review.Fingerprint
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/amendments"), body)
	assert.Equal(t, http.StatusForbidden, rec.Code, "a change the Träger switched off: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.care_amendment_disabled")
	assert.Empty(t, amendedInvoice(t, f.amended(t, run.ID), contracts["Mia"]).Amendments, "no refused change is stored")
}
