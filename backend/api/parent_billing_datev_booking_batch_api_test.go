// Router-level tests of the DATEV booking batch of a run (#4694) in the
// frame of the finance exports (#4693): the export page reads the check of
// the settings, the download waits for it without taking a number, names
// the invoices DATEV would refuse, takes the next file number of the Träger
// shared with the Sollstellung, and reaches the protocol, the run list and
// the history of each invoice; the rights and the Träger hold.
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

	"github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	configModels "github.com/moto-ogs/project-phoenix/modules/settings/records"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

type exportSettingsDatevBody struct {
	FileNumbers struct {
		Next int64 `json:"next"`
	} `json:"file_numbers"`
	Datev datevReadinessBody `json:"datev"`
}

func exportSettingsDatev(t *testing.T, f careBillingFixture) exportSettingsDatevBody {
	t.Helper()
	rec := exportGet(t, f, "/settings")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[exportSettingsDatevBody](t, rec.Body.Bytes())
}

// setDatevSettings fills the DATEV settings of the fixture's Träger as its
// administration.
func setDatevSettings(t *testing.T, f careBillingFixture) {
	t.Helper()
	admin := datevMember(t, f.db, f.traeger, f.office.organization, identityaccess.TraegerRoleAdministration)
	for key, value := range map[string]string{
		configModels.KeyParentBillingDatevConsultantNumber: "29098",
		configModels.KeyParentBillingDatevClientNumber:     "55003",
		configModels.KeyParentBillingDatevAccountLength:    "4",
		configModels.KeyParentBillingDatevRevenueAccount:   "4400",
	} {
		rec := traegerJSON(t, f.traeger, admin, http.MethodPut, "/traeger/settings/values/"+key, map[string]any{"value": value})
		require.Equal(t, http.StatusOK, rec.Code, "%s: %s", key, rec.Body.String())
	}
}

func setDebtor(t *testing.T, f careBillingFixture, contract int64, debtor string) {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, familyBillingPath,
		map[string]any{"contract_id": contract, "recipient_kind": "applicant", "debtor_number": debtor})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func datevFileCount(t *testing.T, f careBillingFixture) int {
	t.Helper()
	var files int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM parent_billing.export_files WHERE organization_id = ? AND format = 'datev'`,
		f.office.organization).Scan(context.Background(), &files))
	return files
}

func TestDatevBatchOfAReleasedRun(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger DATEV Stapel")
	lia := careContract(t, f.db, f.office.organization, f.office.schools[0], "Lia", sepaDebitMonth, sjrServices()...)
	setDebtor(t, f, lia, "10001")
	run := releaseDebitRun(t, f, sepaDebitMonth)
	var invoice int64
	var number string
	require.NoError(t, f.db.NewRaw(`SELECT id, number FROM parent_billing.care_invoices WHERE run_id = ?`, run).
		Scan(context.Background(), &invoice, &number))
	datevPath := fmt.Sprintf("/runs/care-runs/%d/datev", run)

	// Invariant: no file while the check names a setting, and no number.
	before := exportSettingsDatev(t, f)
	assert.False(t, before.Datev.Ready, "the export page reads the check")
	assert.Contains(t, before.Datev.MissingSettings, configModels.KeyParentBillingDatevConsultantNumber)
	rec := exportGet(t, f, datevPath)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), common.CodeParentBillingDatevNotReady)
	assert.Contains(t, rec.Body.String(), configModels.KeyParentBillingDatevRevenueAccount, "details name the missing keys")
	assert.Equal(t, before.FileNumbers.Next, exportSettingsDatev(t, f).FileNumbers.Next, "a refused download takes no number")
	assert.Zero(t, datevFileCount(t, f))

	setDatevSettings(t, f)
	assert.True(t, exportSettingsDatev(t, f).Datev.Ready)
	rec = exportGet(t, f, datevPath)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "text/csv; charset=windows-1252", rec.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	first := fmt.Sprintf("EXTF_Buchungsstapel_%d.csv", before.FileNumbers.Next)
	assert.Contains(t, rec.Header().Get("Content-Disposition"), first, "the file number of the Träger names the file")
	lines := strings.Split(strings.TrimSuffix(rec.Body.String(), "\r\n"), "\r\n")
	require.Len(t, lines, 3, "header, columns and the invoice")
	assert.True(t, strings.HasPrefix(lines[0], `"EXTF";700;21;"Buchungsstapel";13;`), lines[0])
	assert.Contains(t, lines[2], `;"S";`, "an invoice charges the debtor")
	assert.Contains(t, lines[2], ";10001;4400;", "the debtor of the invoice against the revenue account of the settings")
	assert.Contains(t, lines[2], `;"`+number+`";`, "the invoice number is Belegfeld 1")

	// Invariant: one counter across the formats of the Träger.
	soll, _ := sollstellung(t, f, run)
	assert.Equal(t, fmt.Sprintf("sollstellung-%d.csv", before.FileNumbers.Next+1), soll)

	files := exportFiles(t, f, "")
	require.Len(t, files, 2)
	assert.Equal(t, "datev", files[1].Format)
	assert.Equal(t, first, files[1].FileName)
	assert.Equal(t, 1, files[1].InvoiceCount)
	runs := decodeSchoolPortalData[struct {
		Runs []struct {
			LastDatev *exportEntryBody `json:"last_datev"`
		} `json:"runs"`
	}](t, exportGet(t, f, "/runs").Body.Bytes()).Runs
	require.Len(t, runs, 1)
	require.NotNil(t, runs[0].LastDatev)
	assert.Equal(t, first, runs[0].LastDatev.FileName)

	// Invariant: every invoice of the file names it in its history.
	rec = exportGet(t, f, fmt.Sprintf("/invoice-history/care-invoices/%d", invoice))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	history := decodeSchoolPortalData[exportHistoryBody](t, rec.Body.Bytes())
	formats := []string{}
	for _, entry := range history.Entries {
		if entry.File != nil {
			formats = append(formats, entry.File.Format)
		}
	}
	assert.Equal(t, []string{"datev", "sollstellung"}, formats)

	// The invoice is in a DATEV file now (#6433): a second standard
	// download makes no file and takes no number.
	next := exportSettingsDatev(t, f).FileNumbers.Next
	rec = exportGet(t, f, datevPath)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), common.CodeParentBillingDatevNoNewBookings)
	assert.Equal(t, next, exportSettingsDatev(t, f).FileNumbers.Next, "no number without a file")
	assert.Len(t, exportFiles(t, f, ""), 2)

	// "All again" repeats the bookings on purpose. A new file number
	// changes the bytes, but not the bookings: the protocol and the invoice
	// history name the first download and mark the repeat.
	rec = exportGet(t, f, datevPath+"?mode=all")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	files = exportFiles(t, f, "")
	require.Len(t, files, 3)
	assert.Equal(t, before.FileNumbers.Next, files[0].RepeatOf)
	assert.True(t, files[0].DatevAll, "the protocol marks the file of all bookings again")
	assert.False(t, files[2].DatevAll)
	assert.NotEqual(t, files[2].Fingerprint, files[0].Fingerprint, "the audit hash covers the actual file metadata")
	rec = exportGet(t, f, fmt.Sprintf("/invoice-history/care-invoices/%d", invoice))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	history = decodeSchoolPortalData[exportHistoryBody](t, rec.Body.Bytes())
	last := history.Entries[len(history.Entries)-1].File
	require.NotNil(t, last)
	assert.Equal(t, before.FileNumbers.Next, last.RepeatOf)
	rec = exportGet(t, f, datevPath+"?mode=again")
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

// A debtor DATEV refuses names the invoice and takes no number.
func TestDatevBatchNamesTheInvoicesWithoutAValidDebtor(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger DATEV Debitor")
	mia := careContract(t, f.db, f.office.organization, f.office.schools[0], "Mia", sepaDebitMonth, sjrServices()...)
	setDebtor(t, f, mia, "10 4711")
	run := releaseDebitRun(t, f, sepaDebitMonth)
	var number string
	require.NoError(t, f.db.NewRaw(`SELECT number FROM parent_billing.care_invoices WHERE run_id = ?`, run).Scan(context.Background(), &number))
	setDatevSettings(t, f)
	next := exportSettingsDatev(t, f).FileNumbers.Next

	rec := exportGet(t, f, fmt.Sprintf("/runs/care-runs/%d/datev", run))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), common.CodeParentBillingDatevBookingsInvalid)
	assert.Contains(t, rec.Body.String(), `"document_number":"`+number+`","field":"debtor_number","reason":"invalid"`)
	assert.Equal(t, next, exportSettingsDatev(t, f).FileNumbers.Next)
	assert.Zero(t, datevFileCount(t, f))
	_, content := sollstellung(t, f, run)
	assert.Contains(t, content, number, "the tax office can still use the Sollstellung to book the refused run by hand")
}

// Only the right "Finanz-Export" downloads the batch; another Träger's run
// is not found, also when its own settings are complete.
func TestDatevBatchRightsAndTraeger(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger DATEV Rechte")
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Ida", sepaDebitMonth, sjrServices()...)
	setDebtor(t, f, contract, "10001")
	run := releaseDebitRun(t, f, sepaDebitMonth)
	setDatevSettings(t, f)
	path := fmt.Sprintf("/runs/care-runs/%d/datev", run)

	leadership := newTraegerOffice(t, f.db, f.traeger, "Träger DATEV Leitung", identityaccess.TraegerRoleLeadership)
	rec := exportGet(t, careBillingFixture{db: f.db, traeger: f.traeger, office: leadership}, path)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	other := sepaDebitFixture(t, "Träger DATEV Fremd")
	setDatevSettings(t, other)
	rec = exportGet(t, other, path)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), common.CodeParentBillingExportRunNotFound)
	assert.Zero(t, datevFileCount(t, f))
	assert.Zero(t, datevFileCount(t, other))
}

type datevRunsBody struct {
	Ready bool `json:"ready"`
	Runs  []struct {
		RunKind     string   `json:"run_kind"`
		RunID       int64    `json:"run_id"`
		NewBookings int      `json:"new_bookings"`
		FiscalYears []string `json:"fiscal_years"`
	} `json:"runs"`
}

func datevNewBookings(t *testing.T, f careBillingFixture, run int64) int {
	t.Helper()
	rec := exportGet(t, f, fmt.Sprintf("/datev-runs?care=%d", run))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	runs := decodeSchoolPortalData[datevRunsBody](t, rec.Body.Bytes()).Runs
	require.Len(t, runs, 1)
	return runs[0].NewBookings
}

func cancelCareInvoice(t *testing.T, f careBillingFixture, run, invoice int64) {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run, fmt.Sprintf("/invoices/%d/cancellation", invoice)),
		map[string]any{"reason": "Falsche Debitorennummer"})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

// A Storno of an invoice that went to DATEV is a booking of its own in the
// next file (#6433): Haben on the debtor of the invoice, the invoice is not
// booked again, and the run counts it as its one new booking.
func TestDatevBatchBooksALaterStornoOnce(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger DATEV Storno")
	lia := careContract(t, f.db, f.office.organization, f.office.schools[0], "Lia", sepaDebitMonth, sjrServices()...)
	setDebtor(t, f, lia, "10001")
	run := releaseDebitRun(t, f, sepaDebitMonth)
	var invoice int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM parent_billing.care_invoices WHERE run_id = ?`, run).Scan(context.Background(), &invoice))
	setDatevSettings(t, f)
	datevPath := fmt.Sprintf("/runs/care-runs/%d/datev", run)
	assert.Equal(t, 1, datevNewBookings(t, f, run))
	rec := exportGet(t, f, datevPath)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Zero(t, datevNewBookings(t, f, run))

	cancelCareInvoice(t, f, run, invoice)
	assert.Equal(t, 1, datevNewBookings(t, f, run), "the Storno is new")
	rec = exportGet(t, f, datevPath)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	lines := strings.Split(strings.TrimSuffix(rec.Body.String(), "\r\n"), "\r\n")
	require.Len(t, lines, 3, "header, columns and only the Storno")
	assert.Contains(t, lines[2], `;"H";`, "the Storno relieves the debtor")
	assert.Contains(t, lines[2], ";10001;4400;")
	assert.Zero(t, datevNewBookings(t, f, run))
	rec = exportGet(t, f, datevPath)
	assert.Equal(t, http.StatusConflict, rec.Code, "the Storno goes to DATEV once: %s", rec.Body.String())
}

// An invoice whose debtor DATEV refuses locked its run (#6427). Its Storno
// frees it: the pair never reached DATEV and cancels out, so the run has no
// booking left and the next one books the reissue (#6433).
func TestDatevBatchOfARefusedDebtorIsFreedByItsStorno(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger DATEV Sperre")
	mia := careContract(t, f.db, f.office.organization, f.office.schools[0], "Mia", sepaDebitMonth, sjrServices()...)
	ida := careContract(t, f.db, f.office.organization, f.office.schools[0], "Ida", sepaDebitMonth, sjrServices()...)
	setDebtor(t, f, mia, "10 4711")
	setDebtor(t, f, ida, "10002")
	run := releaseDebitRun(t, f, sepaDebitMonth)
	var refused int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM parent_billing.care_invoices WHERE run_id = ? AND debtor_number = '10 4711'`, run).
		Scan(context.Background(), &refused))
	setDatevSettings(t, f)
	datevPath := fmt.Sprintf("/runs/care-runs/%d/datev", run)
	rec := exportGet(t, f, datevPath)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), common.CodeParentBillingDatevBookingsInvalid)

	cancelCareInvoice(t, f, run, refused)
	assert.Equal(t, 1, datevNewBookings(t, f, run), "only Ida's invoice is left to book")
	rec = exportGet(t, f, datevPath)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	lines := strings.Split(strings.TrimSuffix(rec.Body.String(), "\r\n"), "\r\n")
	require.Len(t, lines, 3, "header, columns and Ida's invoice; the cancelled pair is left out")
	assert.Contains(t, lines[2], ";10002;4400;")
	_, content := sollstellung(t, f, run)
	assert.Contains(t, content, "10 4711", "the Sollstellung keeps the invoice and its Storno")
}

// The DATEV counts of the export page read the same statements for one run
// as for several (#6433, query budget).
func TestDatevRunsQueryBudget(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger DATEV Budget")
	lia := careContract(t, f.db, f.office.organization, f.office.schools[0], "Lia", sepaDebitMonth, sjrServices()...)
	setDebtor(t, f, lia, "10001")
	first := releaseDebitRun(t, f, sepaDebitMonth)
	setDatevSettings(t, f)
	measure := func(path string) testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, financeExportsPath+path, nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.traeger, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	one := measure(fmt.Sprintf("/datev-runs?care=%d", first))
	second := releaseDebitRun(t, f, sepaDebitNextMonth)
	assert.Len(t, measure(fmt.Sprintf("/datev-runs?care=%d,%d&holiday=%d", first, second, second+1000)).Statements, len(one.Statements),
		"flat in the number of runs")
	querybudgets.AssertQueryBudget(t, "api.traeger.finance_export_datev_runs", one)

	rec := exportGet(t, f, "/datev-runs?care=x")
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}
