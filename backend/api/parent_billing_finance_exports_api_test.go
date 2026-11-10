// Router-level tests of the finance exports (#4693): the debtor number of
// the family reaches the monthly invoice and the Sollstellung, every
// download gets the next file number of the Träger in its name and an entry
// in the protocol (also the PAIN.008 file), the start value is set once, the
// history of an invoice names every export, the Träger rights, and flat
// query budgets.
package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

const financeExportsPath = "/traeger/finance-exports"

type exportEntryBody struct {
	FileNumber    int64   `json:"file_number"`
	Format        string  `json:"format"`
	RunKind       string  `json:"run_kind"`
	RunLabel      string  `json:"run_label"`
	FileName      string  `json:"file_name"`
	InvoiceCount  int     `json:"invoice_count"`
	TotalCents    int64   `json:"total_cents"`
	Reference     string  `json:"reference"`
	RepeatOf      int64   `json:"repeat_of"`
	ExportedByYou bool    `json:"exported_by_you"`
	SubmittedAt   *string `json:"submitted_at"`
}

type exportRunBody struct {
	RunKind          string           `json:"run_kind"`
	RunID            int64            `json:"run_id"`
	Label            string           `json:"label"`
	InvoiceCount     int              `json:"invoice_count"`
	Exports          int              `json:"exports"`
	LastSollstellung *exportEntryBody `json:"last_sollstellung"`
}

type exportHistoryBody struct {
	Number  string `json:"number"`
	Entries []struct {
		Kind      string           `json:"kind"`
		ByYou     bool             `json:"by_you"`
		Positions int              `json:"positions"`
		Note      string           `json:"note"`
		File      *exportEntryBody `json:"file"`
	} `json:"entries"`
}

func exportGet(t *testing.T, f careBillingFixture, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, financeExportsPath+path, nil)
	req.Header.Set("Authorization", "Bearer "+f.office.token)
	rec := httptest.NewRecorder()
	f.traeger.ServeHTTP(rec, req)
	return rec
}

func exportRuns(t *testing.T, f careBillingFixture) []exportRunBody {
	t.Helper()
	rec := exportGet(t, f, "/runs")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[struct {
		Runs []exportRunBody `json:"runs"`
	}](t, rec.Body.Bytes()).Runs
}

func exportFiles(t *testing.T, f careBillingFixture, query string) []exportEntryBody {
	t.Helper()
	rec := exportGet(t, f, "/files"+query)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[struct {
		Files []exportEntryBody `json:"files"`
	}](t, rec.Body.Bytes()).Files
}

// sollstellung downloads the Sollstellung of a run and answers the file name
// and the content.
func sollstellung(t *testing.T, f careBillingFixture, run int64) (string, string) {
	t.Helper()
	rec := exportGet(t, f, fmt.Sprintf("/runs/care-runs/%d/sollstellung", run))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	disposition := rec.Header().Get("Content-Disposition")
	start := strings.Index(disposition, `filename="`)
	require.GreaterOrEqual(t, start, 0, disposition)
	name := disposition[start+len(`filename="`):]
	return name[:strings.Index(name, `"`)], rec.Body.String()
}

// The SJR case end to end: the debtor number of the family on the invoice
// and in the file, the start value 110, two Sollstellungen and the PAIN.008
// file with their own numbers, the protocol and the history of the invoice.
func TestFinanceExportsOfAReleasedRun(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Sollstellung SJR")
	lia := careContract(t, f.db, f.office.organization, f.office.schools[0], "Lia", sepaDebitMonth, sjrServices()...)
	createMandate(t, f, mandateBody(lia, "SJR-X-LIA"))
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, familyBillingPath,
		map[string]any{"contract_id": lia, "recipient_kind": "applicant", "debtor_number": " 10 4711 "})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"debtor_number":"10 4711"`, "a family with only a debtor number keeps it")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, financeExportsPath+"/settings/file-number-start", map[string]any{"start": 110})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"file_numbers":{"next":110,"started":true}`)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, financeExportsPath+"/settings/file-number-start", map[string]any{"start": 5})
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), common.CodeParentBillingExportFileNumberStartLocked)

	run := releaseDebitRun(t, f, sepaDebitMonth)
	var invoice int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM parent_billing.care_invoices WHERE run_id = ?`, run).Scan(context.Background(), &invoice))
	var debtor string
	require.NoError(t, f.db.NewRaw(`SELECT debtor_number FROM parent_billing.care_invoices WHERE id = ?`, invoice).Scan(context.Background(), &debtor))
	assert.Equal(t, "10 4711", debtor, "the release copies the debtor number of the family")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, billingFamiliesPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "10 4711", "the families show the debtor number of a monthly invoice")

	runs := exportRuns(t, f)
	require.Len(t, runs, 1)
	assert.Equal(t, run, runs[0].RunID)
	assert.Nil(t, runs[0].LastSollstellung, "not yet exported")

	first, content := sollstellung(t, f, run)
	assert.Equal(t, "sollstellung-110.csv", first)
	assert.Contains(t, content, "10 4711;")
	second, again := sollstellung(t, f, run)
	assert.Equal(t, "sollstellung-111.csv", second, "a repeated download is a new file with the next number")
	assert.Equal(t, content, again)
	debit := debitRun(t, f, run)
	req := httptest.NewRequest(http.MethodGet, sepaDebitRunPath(run, "/file"), nil)
	req.Header.Set("Authorization", "Bearer "+f.office.token)
	painRec := httptest.NewRecorder()
	f.traeger.ServeHTTP(painRec, req)
	require.Equal(t, http.StatusOK, painRec.Code, painRec.Body.String())
	assert.Contains(t, painRec.Header().Get("Content-Disposition"), "lastschrift-112-"+debit.MessageID+".xml",
		"the PAIN.008 file counts on the same numbers")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, sepaDebitRunPath(run, "/submission"), map[string]any{"message_id": debit.MessageID})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	files := exportFiles(t, f, "")
	require.Len(t, files, 3)
	assert.Equal(t, []int64{112, 111, 110}, []int64{files[0].FileNumber, files[1].FileNumber, files[2].FileNumber}, "newest first")
	assert.Equal(t, "pain008", files[0].Format)
	assert.NotNil(t, files[0].SubmittedAt, "the confirmation reaches the protocol")
	assert.Equal(t, int64(110), files[1].RepeatOf, "the same content as file 110")
	assert.Zero(t, files[2].RepeatOf)
	assert.True(t, files[2].ExportedByYou)
	assert.Equal(t, 1, files[2].InvoiceCount)
	assert.Equal(t, "Monatsbeiträge Juni 2029", files[2].RunLabel)
	assert.Len(t, exportFiles(t, f, fmt.Sprintf("?run_kind=care-runs&run_id=%d", run)), 3)

	runs = exportRuns(t, f)
	require.NotNil(t, runs[0].LastSollstellung)
	assert.Equal(t, int64(111), runs[0].LastSollstellung.FileNumber)
	assert.Equal(t, 3, runs[0].Exports)

	rec = exportGet(t, f, fmt.Sprintf("/invoice-history/care-invoices/%d", invoice))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	history := decodeSchoolPortalData[exportHistoryBody](t, rec.Body.Bytes())
	kinds := []string{}
	for _, entry := range history.Entries {
		kinds = append(kinds, entry.Kind)
	}
	assert.Equal(t, []string{"issued", "exported", "exported", "exported", "submitted"}, kinds)
	assert.Positive(t, history.Entries[0].Positions)
	require.NotNil(t, history.Entries[1].File)
	assert.Equal(t, "sollstellung-110.csv", history.Entries[1].File.FileName)
	assert.Equal(t, debit.MessageID, history.Entries[4].Note)
	var reads int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM parent_billing.care_run_events
		WHERE organization_id = ? AND run_id = ? AND action = 'read' AND actor_account_id IS NOT NULL
			AND details->>'view' = 'invoice_history' AND details->'document_ids' = to_jsonb(ARRAY[?::bigint])`,
		f.office.organization, run, invoice).Scan(context.Background(), &reads))
	assert.Equal(t, 1, reads, "the read of the history is in the trail of the invoice's run")

	page := decodeSchoolPortalData[struct {
		Total       int  `json:"total"`
		NotExported int  `json:"not_exported"`
		NextOffset  *int `json:"next_offset"`
	}](t, exportGet(t, f, "/runs").Body.Bytes())
	assert.Equal(t, 1, page.Total)
	assert.Zero(t, page.NotExported)
	assert.Nil(t, page.NextOffset, "one run is one page")
	assert.Empty(t, exportFiles(t, f, "?offset=3"), "no file after the last one")
	assert.Len(t, exportFiles(t, f, "?offset=2"), 1, "the oldest file on its own page")
	for _, offset := range []string{"-1", "x"} {
		rec = exportGet(t, f, "/files?offset="+offset)
		assert.Equal(t, http.StatusBadRequest, rec.Code, offset)
	}
}

// Only the right "Finanz-Export" reads and downloads the exports; the
// history of an invoice needs "Abrechnung"; another Träger's run and invoice
// are not found.
func TestFinanceExportsRightsAndTraeger(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Sollstellung Rechte")
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Mia", sepaDebitMonth, sjrServices()...)
	createMandate(t, f, mandateBody(contract, "SJR-R-MIA"))
	run := releaseDebitRun(t, f, sepaDebitMonth)
	var invoice int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM parent_billing.care_invoices WHERE run_id = ?`, run).Scan(context.Background(), &invoice))

	leadership := newTraegerOffice(t, f.db, f.traeger, "Träger Sollstellung Leitung", identityaccess.TraegerRoleLeadership)
	for _, path := range []string{"/settings", "/runs", "/files", fmt.Sprintf("/runs/care-runs/%d/sollstellung", run),
		fmt.Sprintf("/invoice-history/care-invoices/%d", invoice)} {
		rec := exportGet(t, careBillingFixture{db: f.db, traeger: f.traeger, office: leadership}, path)
		assert.Equal(t, http.StatusForbidden, rec.Code, path)
	}
	other := sepaDebitFixture(t, "Träger Sollstellung Fremd")
	rec := exportGet(t, other, fmt.Sprintf("/runs/care-runs/%d/sollstellung", run))
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), common.CodeParentBillingExportRunNotFound)
	rec = exportGet(t, other, fmt.Sprintf("/invoice-history/care-invoices/%d", invoice))
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Empty(t, exportRuns(t, other))
	var files int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM parent_billing.export_files WHERE organization_id IN (?, ?)`,
		f.office.organization, other.office.organization).Scan(context.Background(), &files))
	assert.Zero(t, files, "a refused download leaves no protocol entry")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, financeExportsPath+"/settings/sollstellung", map[string]any{
		"columns": []map[string]any{{"field": "iban", "header": "IBAN"}}, "delimiter": ";", "decimal_separator": ",",
		"date_format": "dd.mm.yyyy", "encoding": "utf-8", "header_row": true, "file_name_pattern": "s.csv",
	})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), common.CodeParentBillingExportMappingInvalid)
}

// The run list, the protocol and the history of an invoice read the same
// number of statements for one file as for several.
func TestFinanceExportsQueryBudgets(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Sollstellung Budget")
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Mia", sepaDebitMonth, sjrServices()...)
	createMandate(t, f, mandateBody(contract, "SJR-B-MIA"))
	run := releaseDebitRun(t, f, sepaDebitMonth)
	var invoice int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM parent_billing.care_invoices WHERE run_id = ?`, run).Scan(context.Background(), &invoice))
	measure := func(path string) testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, financeExportsPath+path, nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.traeger, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	history := fmt.Sprintf("/invoice-history/care-invoices/%d", invoice)
	sollstellung(t, f, run)
	firstRuns, firstFiles, firstHistory := measure("/runs"), measure("/files"), measure(history)
	releaseDebitRun(t, f, sepaDebitNextMonth)
	for range 3 {
		sollstellung(t, f, run)
	}
	assert.Len(t, measure("/runs").Statements, len(firstRuns.Statements), "the run list is flat in runs and files")
	assert.Len(t, measure("/files").Statements, len(firstFiles.Statements), "the protocol is flat in files")
	assert.Len(t, measure(history).Statements, len(firstHistory.Statements), "the history is flat in exports")
	querybudgets.AssertQueryBudget(t, "api.traeger.finance_export_runs", firstRuns)
	querybudgets.AssertQueryBudget(t, "api.traeger.finance_export_files", firstFiles)
	querybudgets.AssertQueryBudget(t, "api.traeger.finance_export_invoice_history", firstHistory)
}

func assertInvoiceMailHistory(t *testing.T, router http.Handler, token, kind string, id int64, requested, sent int) {
	t.Helper()
	rec := traegerJSON(t, router, token, http.MethodGet, fmt.Sprintf("%s/invoice-history/%s/%d", financeExportsPath, kind, id), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	history := decodeSchoolPortalData[exportHistoryBody](t, rec.Body.Bytes())
	counts := map[string]int{}
	for _, entry := range history.Entries {
		counts[entry.Kind]++
	}
	assert.Equal(t, requested, counts["mail_requested"], "mail requests")
	assert.Equal(t, sent, counts["sent"], "accepted sends")
}
