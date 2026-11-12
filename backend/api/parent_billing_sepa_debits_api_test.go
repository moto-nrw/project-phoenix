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
	"regexp"
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
	// The creation time names the second the file was written; the rest is
	// the same file.
	withoutTime := regexp.MustCompile(`<CreDtTm>[^<]*</CreDtTm>`)
	again := debitFile(t, f, run)
	assert.Equal(t, withoutTime.ReplaceAllString(file, ""), withoutTime.ReplaceAllString(again, ""),
		"a second download gives the same file")
	// The last download is kept for the Rückgabe (ADR 0187): sealed, with
	// its checksum, not yet submitted.
	var kept struct {
		Sealed    []byte  `bun:"file_sealed"`
		SHA256    string  `bun:"file_sha256"`
		Submitted *string `bun:"submitted_at"`
	}
	const keptFile = `SELECT file_sealed, file_sha256, submitted_at::text FROM parent_billing.sepa_debit_files WHERE organization_id = ? AND message_id = ?`
	require.NoError(t, f.db.NewRaw(keptFile, f.office.organization, overview.MessageID).Scan(context.Background(), &kept))
	digest := testpkg.ContentFingerprint([]byte(again))
	assert.Equal(t, digest, kept.SHA256, "the checksum of the last download")
	assert.NotContains(t, string(kept.Sealed), sepaIBAN, "the stored file is sealed")
	assert.Nil(t, kept.Submitted)
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
	require.NoError(t, f.db.NewRaw(keptFile, f.office.organization, overview.MessageID).Scan(context.Background(), &kept))
	assert.NotNil(t, kept.Submitted, "the confirmation marks the kept file submitted")
	assert.Equal(t, digest, kept.SHA256, "and keeps it")

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
	// A one-off mandate claimed by two runs goes into neither file
	// (TestSEPADebitOneOffMandateGoesIntoOneFileAcrossRuns); the race is the
	// FRST hand-over of a recurring mandate.
	for _, kind := range []string{"recurring"} {
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
			var submitted int
			require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM parent_billing.sepa_debits WHERE organization_id = ? AND mandate_id = ? AND status = 'submitted'`, f.office.organization, mandate.ID).Scan(ctx, &submitted))
			assert.ElementsMatch(t, []int{http.StatusOK, http.StatusConflict}, codes)
			assert.Equal(t, 1, submitted, "only one FRST file may be confirmed")
		})
	}
}

// A one-off mandate authorizes one collection. Two released runs that both
// carry an invoice of it leave both out of their files, with the reason,
// instead of exporting the mandate twice; a cancelled one frees the other.
func TestSEPADebitOneOffMandateGoesIntoOneFileAcrossRuns(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Einzelmandat zwei Läufe")
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Ada", sepaDebitMonth, sjrServices()...)
	input := mandateBody(contract, "SJR-OOFF-1")
	input["kind"], input["sequence"] = "one_off", "first"
	createMandate(t, f, input)
	first := releaseDebitRun(t, f, sepaDebitMonth)
	assert.Equal(t, 1, debitRun(t, f, first).Count, "one run alone may collect the one-off mandate")
	second := releaseDebitRun(t, f, sepaDebitNextMonth)
	for _, run := range []int64{first, second} {
		overview := debitRun(t, f, run)
		assert.Zero(t, overview.Count, "run %d: the one-off mandate is claimed by two runs", run)
		require.Len(t, overview.Left, 1)
		assert.Equal(t, "one_off_multiple", overview.Left[0].Left)
	}
	var invoice int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM parent_billing.care_invoices WHERE run_id = ? AND care_contract_id = ?`,
		second, contract).Scan(context.Background(), &invoice))
	_, err := f.db.ExecContext(context.Background(), `INSERT INTO parent_billing.care_invoice_cancellations (organization_id, run_id,
		invoice_id, number, number_year, number_sequence, issued_on, reason, total_cents)
		VALUES (?, ?, ?, 'ST-OOFF', 2029, 9, DATE '2029-07-10', 'Doppelt', -7750)`, f.office.organization, second, invoice)
	require.NoError(t, err)
	assert.Equal(t, 1, debitRun(t, f, first).Count, "the cancelled invoice frees the authorization for the other run")
}

// A family with a Jahresbescheid pays its installment, a family with a
// monthly invoice its invoice (#4697, ADR 0182): Noah's October is billed by
// a monthly invoice, Mia's by her notice. The installments due in October
// have their own file; the file of the October run holds Noah's invoice,
// that of October's installments Mia's installment, and no family is in
// both. The confirmation writes the installment as a collection of Mia's
// mandate, and her mandate's history shows it.
func TestSEPADebitFilesCollectTheInstallmentAndTheInvoiceOfAMonthOnce(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Lastschrift Jahresbescheid")
	school := f.office.schools[0]
	noah := careContract(t, f.db, f.office.organization, school, "Noah", sepaDebitMonth, sjrServices()...)
	createMandate(t, f, mandateBody(noah, "SJR-JB-NOAH"))
	october := releaseDebitRun(t, f, "2029-10-01")

	mia := careContract(t, f.db, f.office.organization, school, "Mia", sepaDebitMonth, sjrServices()...)
	miaMandate := createMandate(t, f, mandateBody(mia, "SJR-JB-MIA"))
	writeLegalRemedy(t, f.db, f.office.organization)
	notices := f.startNoticeRun(t, "2029-10-01", "2029-09-24")
	released := f.releaseNoticeRun(t, f.noticeRun(t, notices.ID))
	var miaNotice int64
	for _, notice := range released.Notices {
		if notice.ContractID == mia {
			miaNotice = notice.ID
		}
	}
	require.NotZero(t, miaNotice, "Mia's year is assessed by a notice")

	type fileDebit struct {
		sepaDebitBody
		ContractID  int64  `json:"contract_id"`
		NoticeID    int64  `json:"notice_id"`
		NoticeMonth string `json:"notice_month"`
	}
	type fileBody struct {
		MessageID string      `json:"message_id"`
		Debits    []fileDebit `json:"debits"`
	}
	read := func(path string) fileBody {
		rec := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, path, nil)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.NotContains(t, rec.Body.String(), sepaIBAN)
		return decodeSchoolPortalData[fileBody](t, rec.Body.Bytes())
	}
	installments := sepaDebitsPath + "/notice-months/202910"
	runFile, rateFile := read(sepaDebitRunPath(october, "")), read(installments)
	require.Len(t, runFile.Debits, 1)
	assert.Equal(t, noah, runFile.Debits[0].ContractID, "the run's file holds Noah's invoice")
	assert.Zero(t, runFile.Debits[0].NoticeID)
	require.Len(t, rateFile.Debits, 1)
	rate := rateFile.Debits[0]
	assert.Equal(t, mia, rate.ContractID, "the installments' file holds Mia's installment")
	assert.Equal(t, miaNotice, rate.NoticeID)
	assert.Equal(t, "2029-10-01", rate.NoticeMonth)
	assert.Equal(t, int64(7750), rate.AmountCents, "what the family pays after the cost bearers, as the notice sets it")
	assert.Equal(t, "2029-10-01", rate.DueOn)

	req := httptest.NewRequest(http.MethodGet, installments+"/file", nil)
	req.Header.Set("Authorization", "Bearer "+f.office.token)
	out := httptest.NewRecorder()
	f.traeger.ServeHTTP(out, req)
	require.Equal(t, http.StatusOK, out.Code, out.Body.String())
	file := out.Body.String()
	assert.Contains(t, file, fmt.Sprintf("<EndToEndId>MOTO-N%d-202910</EndToEndId>", miaNotice))
	assert.Contains(t, file, "<MndtId>SJR-JB-MIA</MndtId>")
	assert.NotContains(t, file, "<MndtId>SJR-JB-NOAH</MndtId>", "Noah is in the run's file only")
	assert.NotContains(t, debitFile(t, f, october), "<MndtId>SJR-JB-MIA</MndtId>", "Mia is in the installments' file only")

	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, installments+"/submission",
		map[string]any{"message_id": rateFile.MessageID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var collected string
	require.NoError(t, f.db.NewRaw(`SELECT concat_ws('|', status, run_kind, run_id, notice_month, submitted_amount_cents, mandate_id)
		FROM parent_billing.sepa_debits WHERE care_notice_id = ?`, miaNotice).Scan(context.Background(), &collected))
	assert.Equal(t, fmt.Sprintf("submitted|notice|202910|2029-10-01|7750|%d", miaMandate.ID), collected)
	var last string
	require.NoError(t, f.db.NewRaw(`SELECT last_collected_on::text FROM parent_billing.sepa_mandates WHERE id = ?`, miaMandate.ID).
		Scan(context.Background(), &last))
	assert.Equal(t, "2029-10-01", last)
	assert.Empty(t, read(installments).Debits, "the installment is collected once")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, installments+"/submission",
		map[string]any{"message_id": rateFile.MessageID})
	assert.Equal(t, http.StatusConflict, rec.Code, "a second confirmation collects nothing: %s", rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, fmt.Sprintf("%s/%d/collections", sepaMandatesPath, miaMandate.ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"notice_month":"2029-10-01"`, "the installment is in the mandate's history")
}

// A stored download holds the families' accounts until its debits go,
// whatever message the debits end up with (ADR 0187): a changed download
// keeps a file of its own, the confirmation drops the downloads it
// superseded, and deleting the debits leaves no stored file. The pending
// installment of a downloaded file of installments counts as a member too.
func TestSEPADebitFilesGoWithTheirDebitsWhateverTheMessage(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Lastschriftdatei veraltet")
	school := f.office.schools[0]
	lia := careContract(t, f.db, f.office.organization, school, "Lia", sepaDebitMonth, sjrServices()...)
	createMandate(t, f, mandateBody(lia, "SJR-OLD-LIA"))
	run := releaseDebitRun(t, f, sepaDebitMonth)
	ctx := context.Background()
	kept := func() []string {
		var messages []string
		require.NoError(t, f.db.NewRaw(`SELECT message_id FROM parent_billing.sepa_debit_files WHERE organization_id = ? ORDER BY message_id`,
			f.office.organization).Scan(ctx, &messages))
		return messages
	}

	a := debitRun(t, f, run).MessageID
	debitFile(t, f, run)
	// A change of the creditor account is a different file.
	_, err := f.db.ExecContext(ctx, `UPDATE platform.traeger_bank_accounts SET iban = ? WHERE organization_id = ?`,
		sepaChangedIBAN, f.office.organization)
	require.NoError(t, err)
	b := debitRun(t, f, run).MessageID
	require.NotEqual(t, a, b)
	debitFile(t, f, run)
	assert.ElementsMatch(t, []string{a, b}, kept(), "both downloads are kept until the confirmation")

	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, sepaDebitRunPath(run, "/submission"),
		map[string]any{"message_id": b})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, []string{b}, kept(), "the confirmation drops the download it superseded")

	// A newer download of a run that is downloaded again before its old
	// file is confirmed: deleting the debits drops every file they are in.
	next := releaseDebitRun(t, f, sepaDebitNextMonth)
	c := debitRun(t, f, next).MessageID
	debitFile(t, f, next)
	_, err = f.db.ExecContext(ctx, `UPDATE platform.traeger_bank_accounts SET iban = ? WHERE organization_id = ?`,
		sepaTraegerIBAN, f.office.organization)
	require.NoError(t, err)
	require.NotEqual(t, c, debitRun(t, f, next).MessageID)
	debitFile(t, f, next)
	_, err = f.db.ExecContext(ctx, `DELETE FROM parent_billing.sepa_debits WHERE organization_id = ?`, f.office.organization)
	require.NoError(t, err)
	assert.Empty(t, kept(), "no stored file outlives its debits")
}

// The pending installment of a downloaded file of installments has no
// debit row yet: its file goes with the notice (ADR 0187).
func TestSEPADebitFileOfInstallmentsGoesWithItsNotice(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Lastschriftdatei Bescheid")
	school := f.office.schools[0]
	mia := careContract(t, f.db, f.office.organization, school, "Mia", sepaDebitMonth, sjrServices()...)
	createMandate(t, f, mandateBody(mia, "SJR-OLD-MIA"))
	writeLegalRemedy(t, f.db, f.office.organization)
	notices := f.startNoticeRun(t, "2029-10-01", "2029-09-24")
	f.releaseNoticeRun(t, f.noticeRun(t, notices.ID))
	req := httptest.NewRequest(http.MethodGet, sepaDebitsPath+"/notice-months/202910/file", nil)
	req.Header.Set("Authorization", "Bearer "+f.office.token)
	out := httptest.NewRecorder()
	f.traeger.ServeHTTP(out, req)
	require.Equal(t, http.StatusOK, out.Code, out.Body.String())
	ctx := context.Background()
	count := func() int {
		var n int
		require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.sepa_debit_files WHERE organization_id = ?`,
			f.office.organization).Scan(ctx, &n))
		return n
	}
	require.Equal(t, 1, count())
	_, err := f.db.ExecContext(ctx, `DELETE FROM parent_billing.care_notices WHERE organization_id = ?`, f.office.organization)
	require.NoError(t, err)
	assert.Zero(t, count(), "the file of a pending installment goes with its notice")
}
