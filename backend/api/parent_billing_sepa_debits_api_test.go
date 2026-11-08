// Router-level tests of the SEPA direct debit file (#4697, ADR 0181): the
// release fixes the account of the mandate, the file of a released run
// takes only valid mandates, a download changes nothing, the confirmation
// collects once and turns FRST into RCUR, the right "Abrechnung" and the
// Träger boundary, and flat query budgets.
package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

const (
	sepaDebitsPath = "/traeger/sepa-debits"
	// sepaDebitMonth lies within 36 months of the mandates the tests sign.
	sepaDebitMonth     = "2029-06-01"
	sepaDebitNextMonth = "2029-07-01"
	sepaTraegerIBAN    = "DE75512108001245126199"
	sepaChangedIBAN    = "AT611904300234573201"
)

type sepaDebitBody struct {
	ID            int64   `json:"id"`
	MandateID     int64   `json:"mandate_id"`
	InvoiceNumber string  `json:"invoice_number"`
	RecipientName string  `json:"recipient_name"`
	AmountCents   int64   `json:"amount_cents"`
	DueOn         string  `json:"due_on"`
	SequenceType  string  `json:"sequence_type"`
	Status        string  `json:"status"`
	Left          string  `json:"left"`
	SubmittedAt   *string `json:"submitted_at"`
}

type sepaDebitRunBody struct {
	CreditorID     string          `json:"creditor_id"`
	CreditorName   string          `json:"creditor_name"`
	CreditorIBAN   string          `json:"creditor_iban_masked"`
	AccountMissing bool            `json:"account_missing"`
	MessageID      string          `json:"message_id"`
	Count          int             `json:"count"`
	TotalCents     int64           `json:"total_cents"`
	Debits         []sepaDebitBody `json:"debits"`
	Submitted      []sepaDebitBody `json:"submitted"`
	Left           []sepaDebitBody `json:"left"`
	Groups         []struct {
		SequenceType string `json:"sequence_type"`
		CollectOn    string `json:"collect_on"`
		Count        int    `json:"count"`
	} `json:"groups"`
}

func sepaDebitRunPath(run int64, suffix string) string {
	return fmt.Sprintf("%s/care-runs/%d%s", sepaDebitsPath, run, suffix)
}

// sepaDebitFixture is a Träger that collects the care by direct debit, with
// its creditor identifier and a default bank account.
func sepaDebitFixture(t *testing.T, name string) careBillingFixture {
	t.Helper()
	f := newCareBillingFixture(t, name)
	testpkg.OwnTraegerMasterData(t, f.db, f.office.organization)
	_, err := f.db.ExecContext(context.Background(), `INSERT INTO platform.traeger_bank_accounts
		(organization_id, label, account_holder, iban, bic, is_default) VALUES (?, 'Beiträge', 'Stadtjugendring Pforzheim e. V.', ?, 'SOLADES1PFO', true)`,
		f.office.organization, sepaTraegerIBAN)
	require.NoError(t, err)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, careTariffsPath, carePriceList(f.office.schools[0], sepaDebitMonth, 3200))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	setTerms(t, f.traeger, f.office.token, "care", map[string]any{"payment_method": "direct_debit", "due_days": 10})
	saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 3, "missing_mandate": "transfer"})
	return f
}

// releaseDebitRun releases the run of a month with the debit on the 4th.
func releaseDebitRun(t *testing.T, f careBillingFixture, month string) int64 {
	t.Helper()
	run := f.startRun(t, month)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, runDueDatesPathOf("care", run.ID),
		dueDates(month[:8]+"02", month[:8]+"04"))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview := f.run(t, run.ID)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(preview))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return run.ID
}

func debitRun(t *testing.T, f careBillingFixture, run int64) sepaDebitRunBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, sepaDebitRunPath(run, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), sepaIBAN, "the overview names no IBAN of a family")
	assert.NotContains(t, rec.Body.String(), sepaTraegerIBAN, "and the Träger's account only masked")
	return decodeSchoolPortalData[sepaDebitRunBody](t, rec.Body.Bytes())
}

func debitFile(t *testing.T, f careBillingFixture, run int64) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, sepaDebitRunPath(run, "/file"), nil)
	req.Header.Set("Authorization", "Bearer "+f.office.token)
	rec := httptest.NewRecorder()
	f.traeger.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"), "nobody caches the file")
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "attachment")
	return rec.Body.String()
}

// The SJR case end to end: the release fixes the account, a later change
// of the mandate does not reach the file, a download changes nothing, the
// confirmation collects once and turns the next run's FRST into RCUR.
func TestSEPADebitFileOfAReleasedRun(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Lastschriftdatei SJR")
	school := f.office.schools[0]
	lia := careContract(t, f.db, f.office.organization, school, "Lia", sepaDebitMonth, sjrServices()...)
	noah := careContract(t, f.db, f.office.organization, school, "Noah", sepaDebitMonth, sjrServices()...)
	careContract(t, f.db, f.office.organization, school, "Ida", sepaDebitMonth, sjrServices()...) // no mandate: transfer
	first := mandateBody(lia, "SJR-D-LIA")
	first["sequence"] = "first"
	liaMandate := createMandate(t, f, first)
	noahMandate := createMandate(t, f, mandateBody(noah, "SJR-D-NOAH"))

	run := releaseDebitRun(t, f, sepaDebitMonth)
	next := releaseDebitRun(t, f, sepaDebitNextMonth)

	overview := debitRun(t, f, run)
	assert.Equal(t, sepaCreditorID, overview.CreditorID)
	assert.Equal(t, "Stadtjugendring Pforzheim e. V.", overview.CreditorName)
	assert.Equal(t, "DE•• •••• •••• •••• ••61 99", overview.CreditorIBAN)
	assert.False(t, overview.AccountMissing)
	assert.Equal(t, 2, overview.Count, "only the families with a mandate; the transfer is no debit")
	assert.Equal(t, int64(2*7750), overview.TotalCents)
	sequences := map[int64]string{}
	for _, debit := range overview.Debits {
		sequences[debit.MandateID] = debit.SequenceType
		assert.Equal(t, "2029-06-04", debit.DueOn)
		assert.Equal(t, "pending", debit.Status)
	}
	assert.Equal(t, map[int64]string{liaMandate.ID: "FRST", noahMandate.ID: "RCUR"}, sequences)
	require.Len(t, overview.Groups, 2, "one batch per sequence type")
	assert.NotEmpty(t, overview.MessageID)

	// The account at the release counts: a change of the IBAN afterwards
	// reaches only the next release, never this file.
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, fmt.Sprintf("%s/%d", sepaMandatesPath, noahMandate.ID),
		map[string]any{"account_holder": "Jan Beispiel", "iban": sepaChangedIBAN, "sequence": "recurring"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	file := debitFile(t, f, run)
	assert.Contains(t, file, "<IBAN>"+sepaIBAN+"</IBAN>")
	assert.NotContains(t, file, sepaChangedIBAN)
	assert.Contains(t, file, "<Nm>Eva Beispiel</Nm>")
	assert.NotContains(t, file, "Jan Beispiel")
	assert.Contains(t, file, "<IBAN>"+sepaTraegerIBAN+"</IBAN>")
	assert.Contains(t, file, "<MsgId>"+overview.MessageID+"</MsgId>")
	assert.Contains(t, mandateEvents(t, f, noahMandate.ID), "reveal:office", "the download is in the trail")
	assert.Equal(t, file, debitFile(t, f, run), "a second download gives the same file")
	var state string
	const mandateState = `SELECT sequence || '|' || COALESCE(last_collected_on::text, '-') FROM parent_billing.sepa_mandates WHERE id = ?`
	require.NoError(t, f.db.NewRaw(mandateState, liaMandate.ID).Scan(context.Background(), &state))
	assert.Equal(t, "first|-", state, "a download collects nothing")

	// The confirmation names the file the office saw.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, sepaDebitRunPath(run, "/submission"),
		map[string]any{"message_id": "MOTO-M1-0000000000000000"})
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.sepa_debits_changed")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, sepaDebitRunPath(run, "/submission"),
		map[string]any{"message_id": overview.MessageID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	submitted := decodeSchoolPortalData[sepaDebitRunBody](t, rec.Body.Bytes())
	assert.Zero(t, submitted.Count)
	assert.Len(t, submitted.Submitted, 2)
	require.NoError(t, f.db.NewRaw(mandateState, liaMandate.ID).Scan(context.Background(), &state))
	assert.Equal(t, "recurring|2029-06-04", state, "the first collection turns FRST into RCUR and is the last collection")
	assert.Contains(t, mandateEvents(t, f, liaMandate.ID), "collect:office")

	// The next run was released before: its pending FRST is now RCUR.
	for _, debit := range debitRun(t, f, next).Debits {
		assert.Equal(t, "RCUR", debit.SequenceType, "debit %s of the next run", debit.InvoiceNumber)
	}
	// A second confirmation collects nothing.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, sepaDebitRunPath(run, "/submission"),
		map[string]any{"message_id": overview.MessageID})
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "parent_billing.sepa_debits_empty")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, sepaDebitRunPath(run, "/file"), nil)
	assert.Equal(t, http.StatusConflict, rec.Code, "no file without a pending debit: %s", rec.Body.String())

	// The history per mandate and invoice.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, fmt.Sprintf("%s/%d/collections", sepaMandatesPath, liaMandate.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	history := decodeSchoolPortalData[[]sepaDebitBody](t, rec.Body.Bytes())
	require.Len(t, history, 2)
	assert.Equal(t, "2029-07-04", history[0].DueOn, "newest first")
	assert.Equal(t, "pending", history[0].Status)
	assert.Equal(t, "submitted", history[1].Status)
	assert.NotNil(t, history[1].SubmittedAt)
	assert.NotContains(t, rec.Body.String(), sepaIBAN)
}

// Only valid mandates: a revoked mandate and a cancelled invoice leave
// their debit out of the file, each with its reason; a new creditor
// identifier leaves out the mandates of the old one.
func TestSEPADebitFileTakesOnlyValidMandates(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Lastschriftdatei gültig")
	school := f.office.schools[0]
	revoked := careContract(t, f.db, f.office.organization, school, "Ben", sepaDebitMonth, sjrServices()...)
	cancelled := careContract(t, f.db, f.office.organization, school, "Ada", sepaDebitMonth, sjrServices()...)
	kept := careContract(t, f.db, f.office.organization, school, "Ole", sepaDebitMonth, sjrServices()...)
	revokedMandate := createMandate(t, f, mandateBody(revoked, "SJR-V-BEN"))
	createMandate(t, f, mandateBody(cancelled, "SJR-V-ADA"))
	createMandate(t, f, mandateBody(kept, "SJR-V-OLE"))
	run := releaseDebitRun(t, f, sepaDebitMonth)

	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, fmt.Sprintf("%s/%d/revoke", sepaMandatesPath, revokedMandate.ID),
		map[string]any{"revoked_on": "2026-10-15"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var invoice int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM parent_billing.care_invoices WHERE run_id = ? AND care_contract_id = ?`,
		run, cancelled).Scan(context.Background(), &invoice))
	_, err := f.db.ExecContext(context.Background(), `INSERT INTO parent_billing.care_invoice_cancellations (organization_id, run_id,
		invoice_id, number, number_year, number_sequence, issued_on, reason, total_cents)
		VALUES (?, ?, ?, 'ST-1', 2029, 1, DATE '2029-06-10', 'Vertrag beendet', -7750)`, f.office.organization, run, invoice)
	require.NoError(t, err)

	overview := debitRun(t, f, run)
	assert.Equal(t, 1, overview.Count)
	left := map[string]bool{}
	for _, debit := range overview.Left {
		left[debit.Left] = true
	}
	assert.Equal(t, map[string]bool{"revoked": true, "cancelled": true}, left)
	file := debitFile(t, f, run)
	assert.Equal(t, 1, strings.Count(file, "<DrctDbtTxInf>"))
	assert.Contains(t, file, "<MndtId>SJR-V-OLE</MndtId>")

	saveCreditor(t, f, map[string]any{"creditor_id": "DE86ZZZ01111111111", "prenotification_days": 3, "missing_mandate": "transfer"})
	overview = debitRun(t, f, run)
	assert.Zero(t, overview.Count, "a mandate of the earlier creditor identifier is no proof for the new one")
	assert.Len(t, overview.Left, 3)
}

func TestSEPADebitsNeedTheRightAndStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Lastschriftdatei Grenze")
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Mia", sepaDebitMonth, sjrServices()...)
	createMandate(t, f, mandateBody(contract, "SJR-DG-1"))
	run := releaseDebitRun(t, f, sepaDebitMonth)
	overview := debitRun(t, f, run)
	require.Equal(t, 1, overview.Count)

	password := testpkg.NewTestPassword()
	account := traegerAccount(t, f.db, password)
	testpkg.CreateTestTraegerMember(t, f.db, f.office.organization, account.ID, identityaccess.TraegerRoleAdministration)
	code, answer := traegerSignIn(t, f.traeger, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Lastschriftdatei Fremd", identityaccess.TraegerRoleBilling)
	requests := []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, sepaDebitRunPath(run, ""), nil},
		{http.MethodGet, sepaDebitRunPath(run, "/file"), nil},
		{http.MethodPost, sepaDebitRunPath(run, "/submission"), map[string]any{"message_id": overview.MessageID}},
	}
	for _, request := range requests {
		rec := traegerJSON(t, f.traeger, answer.AccessToken, request.method, request.path, request.body)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s %s without the right: %s", request.method, request.path, rec.Body.String())
		rec = traegerJSON(t, f.traeger, foreign.token, request.method, request.path, request.body)
		assert.Equal(t, http.StatusNotFound, rec.Code, "%s %s of another Träger: %s", request.method, request.path, rec.Body.String())
		assert.NotContains(t, rec.Body.String(), sepaIBAN)
	}
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, fmt.Sprintf("%s/school-runs/%d", sepaDebitsPath, run), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, "an unknown kind of run: %s", rec.Body.String())
	var status string
	require.NoError(t, f.db.NewRaw(`SELECT status FROM parent_billing.sepa_debits WHERE run_id = ? AND run_kind = 'care'`, run).
		Scan(context.Background(), &status))
	assert.Equal(t, "pending", status, "nobody else submitted the debit")
}

// The overview of a run and the history of a mandate read the same
// statements for any number of debits.
func TestSEPADebitsQueryBudgetsAreFlat(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Lastschriftdatei Budget")
	school := f.office.schools[0]
	measure := func(path string) testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.traeger, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	contract := careContract(t, f.db, f.office.organization, school, "Mia", sepaDebitMonth, sjrServices()...)
	mandate := createMandate(t, f, mandateBody(contract, "SJR-DB-1"))
	single := releaseDebitRun(t, f, sepaDebitMonth)
	firstRun := measure(sepaDebitRunPath(single, ""))
	collections := fmt.Sprintf("%s/%d/collections", sepaMandatesPath, mandate.ID)
	firstHistory := measure(collections)

	for _, child := range []string{"Noah", "Lia"} {
		other := careContract(t, f.db, f.office.organization, school, child, sepaDebitMonth, sjrServices()...)
		createMandate(t, f, mandateBody(other, "SJR-DB-"+child))
	}
	several := releaseDebitRun(t, f, sepaDebitNextMonth)
	require.Equal(t, 3, debitRun(t, f, several).Count)
	assert.Len(t, measure(sepaDebitRunPath(several, "")).Statements, len(firstRun.Statements), "the overview is flat in the debits")
	assert.Len(t, measure(collections).Statements, len(firstHistory.Statements), "the history is flat in the debits")
	querybudgets.AssertQueryBudget(t, "api.traeger.sepa_debits", firstRun)
	querybudgets.AssertQueryBudget(t, "api.traeger.sepa_mandate_collections", firstHistory)
}

// Hold the shared mandate until both confirmations are waiting. With
// debit-first locking both requests already own their run's debit, causing
// the FRST propagation to deadlock; mandate-first locking serializes them.
func TestSEPADebitConfirmationsShareMandatesSafely(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	for _, kind := range []string{"recurring", "one_off"} {
		t.Run(kind, func(t *testing.T) {
			f := sepaDebitFixture(t, "Träger gleichzeitige Bestätigung "+kind)
			contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Lia", sepaDebitMonth, sjrServices()...)
			input := mandateBody(contract, "SJR-CONCURRENT")
			input["kind"], input["sequence"] = kind, "first"
			mandate := createMandate(t, f, input)
			runs := []int64{releaseDebitRun(t, f, sepaDebitMonth), releaseDebitRun(t, f, sepaDebitNextMonth)}
			tokens := []string{debitRun(t, f, runs[0]).MessageID, debitRun(t, f, runs[1]).MessageID}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			gate, err := f.db.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = gate.Rollback() }()
			_, err = gate.ExecContext(ctx, `SELECT id FROM parent_billing.sepa_mandates WHERE organization_id = ? AND id = ? FOR UPDATE`,
				f.office.organization, mandate.ID)
			require.NoError(t, err)
			results := make(chan *httptest.ResponseRecorder, 2)
			for i, run := range runs {
				go func(run int64, token string) {
					req := httptest.NewRequest(http.MethodPost, sepaDebitRunPath(run, "/submission"), strings.NewReader(fmt.Sprintf(`{"message_id":%q}`, token)))
					req = req.WithContext(ctx)
					req.Header.Set("Authorization", "Bearer "+f.office.token)
					req.Header.Set("Content-Type", "application/json")
					rec := httptest.NewRecorder()
					f.traeger.ServeHTTP(rec, req)
					results <- rec
				}(run, tokens[i])
			}
			assert.Eventually(t, func() bool {
				var waiting int
				err := f.db.NewRaw(`SELECT COUNT(*) FROM pg_stat_activity WHERE datname = current_database() AND wait_event_type = 'Lock'`).Scan(ctx, &waiting)
				return err == nil && waiting == 2
			}, 5*time.Second, 10*time.Millisecond, "both confirmations must reach the shared mandate")
			require.NoError(t, gate.Commit())
			codes := []int{}
			for range runs {
				select {
				case rec := <-results:
					codes = append(codes, rec.Code)
					if rec.Code != http.StatusOK {
						assert.Contains(t, rec.Body.String(), "parent_billing.sepa_debits_", rec.Body.String())
					}
				case <-ctx.Done():
					t.Fatal("confirmations did not finish")
				}
			}
			assert.ElementsMatch(t, []int{http.StatusOK, http.StatusConflict}, codes)
			var submitted int
			require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM parent_billing.sepa_debits WHERE organization_id = ? AND mandate_id = ? AND status = 'submitted'`, f.office.organization, mandate.ID).Scan(ctx, &submitted))
			assert.Equal(t, 1, submitted, "only one FRST or one-off file may be confirmed")
		})
	}
}
