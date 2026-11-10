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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
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

	// A new file number changes the bytes, but not the bookings. Both
	// the protocol and the invoice history must name the first download.
	rec = exportGet(t, f, datevPath)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	files = exportFiles(t, f, "")
	require.Len(t, files, 3)
	assert.Equal(t, before.FileNumbers.Next, files[0].RepeatOf)
	assert.NotEqual(t, files[2].Fingerprint, files[0].Fingerprint, "the audit hash covers the actual file metadata")
	rec = exportGet(t, f, fmt.Sprintf("/invoice-history/care-invoices/%d", invoice))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	history = decodeSchoolPortalData[exportHistoryBody](t, rec.Body.Bytes())
	last := history.Entries[len(history.Entries)-1].File
	require.NotNil(t, last)
	assert.Equal(t, before.FileNumbers.Next, last.RepeatOf)
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
