// Router-level tests of the names in the finance exports (#4693): a person
// of the Abrechnung without "Konten verwalten" reads who of the colleagues
// exported a file in the protocol, the runs and the history of an invoice,
// by the name the colleague carries at this Träger and never by address; a
// name the same person carries at another Träger never appears, and an
// erased colleague is named by nobody.
package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

type namedExportEntry struct {
	FileNumber     int64  `json:"file_number"`
	ExportedBy     *int64 `json:"exported_by_account_id"`
	ExportedByYou  bool   `json:"exported_by_you"`
	ExportedByName string `json:"exported_by_name"`
}

type namedHistory struct {
	Entries []struct {
		Kind        string            `json:"kind"`
		AccountID   *int64            `json:"account_id"`
		ByYou       bool              `json:"by_you"`
		AccountName string            `json:"account_name"`
		File        *namedExportEntry `json:"file"`
	} `json:"entries"`
}

// exportColleague signs a colleague with the role Abrechnung in to the
// Träger of f and answers the fixture acting as the colleague.
func exportColleague(t *testing.T, f careBillingFixture, name string) (careBillingFixture, *testpkg.AccountFixture) {
	t.Helper()
	account, answer := traegerMember(t, f.db, f.traeger, f.office.organization, identityaccess.TraegerRoleBilling)
	// The name lands in the ledger of the Träger-Büro, which the fixture's
	// Träger does not clean up itself.
	t.Cleanup(func() {
		_, err := f.db.ExecContext(context.Background(), `DELETE FROM audit.traeger_account_changes WHERE organization_id = ? AND actor_account_id = ?`,
			f.office.organization, account.ID)
		require.NoError(t, err)
	})
	if name != "" {
		rec := traegerJSON(t, f.traeger, answer.AccessToken, http.MethodPut, "/traeger/profile/name", map[string]any{"name": name})
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	}
	colleague := f
	colleague.office.token = answer.AccessToken
	return colleague, account
}

func namedFiles(t *testing.T, f careBillingFixture) []namedExportEntry {
	t.Helper()
	rec := exportGet(t, f, "/files")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[struct {
		Files []namedExportEntry `json:"files"`
	}](t, rec.Body.Bytes()).Files
}

func TestFinanceExportsNameTheColleagueWithoutTheAddress(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Namen Export")
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Mia", sepaDebitMonth, sjrServices()...)
	createMandate(t, f, mandateBody(contract, "SJR-N-MIA"))
	run := releaseDebitRun(t, f, sepaDebitMonth)
	var invoice int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM parent_billing.care_invoices WHERE run_id = ?`, run).Scan(context.Background(), &invoice))
	colleague, account := exportColleague(t, f, "Jana Krause")
	// The same person is named differently at another Träger.
	other := traegerOrganization(t, f.db, "Träger Namen Fremd", true)
	testpkg.CreateTestTraegerMember(t, f.db, other, account.ID, identityaccess.TraegerRoleBilling)
	_, err := f.db.ExecContext(context.Background(), `UPDATE platform.organization_accounts SET display_name = 'Fremder Name'
		WHERE organization_id = ? AND account_id = ?`, other, account.ID)
	require.NoError(t, err)

	sollstellung(t, colleague, run)
	sollstellung(t, f, run)
	files := namedFiles(t, f)
	require.Len(t, files, 2)
	assert.True(t, files[0].ExportedByYou, "the reviewer's own file stays by you")
	assert.Empty(t, files[0].ExportedByName)
	assert.False(t, files[1].ExportedByYou)
	assert.Equal(t, "Jana Krause", files[1].ExportedByName, "the colleague is named at this Träger")
	assert.Equal(t, []namedExportEntry{{FileNumber: files[0].FileNumber, ExportedBy: files[0].ExportedBy},
		{FileNumber: files[1].FileNumber, ExportedBy: files[1].ExportedBy, ExportedByYou: true}},
		namedFiles(t, colleague), "the colleague sees the reviewer, who has no name, unnamed and the own file by you")

	rec := exportGet(t, colleague, "/runs")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	runs := decodeSchoolPortalData[struct {
		Runs []struct {
			LastSollstellung *namedExportEntry `json:"last_sollstellung"`
		} `json:"runs"`
	}](t, rec.Body.Bytes()).Runs
	require.Len(t, runs, 1)
	require.NotNil(t, runs[0].LastSollstellung)
	assert.False(t, runs[0].LastSollstellung.ExportedByYou, "the last file is the reviewer's")

	rec = exportGet(t, f, fmt.Sprintf("/invoice-history/care-invoices/%d", invoice))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	history := decodeSchoolPortalData[namedHistory](t, rec.Body.Bytes())
	var named int
	for _, entry := range history.Entries {
		if entry.Kind == "exported" && !entry.ByYou {
			named++
			assert.Equal(t, "Jana Krause", entry.AccountName)
			require.NotNil(t, entry.File)
			assert.Equal(t, "Jana Krause", entry.File.ExportedByName)
		}
	}
	assert.Equal(t, 1, named, "the history names the colleague's export")
	for _, body := range []string{rec.Body.String(), exportGet(t, f, "/files").Body.String(), exportGet(t, f, "/runs").Body.String()} {
		assert.NotContains(t, body, account.Email, "no address of a colleague")
		assert.NotContains(t, body, "Fremder Name", "no name of another Träger")
	}

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, "/traeger/accounts/", nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, "the Abrechnung does not read the member list")

	// The erasure of an account deletes its memberships, a deleted account
	// leaves no account on the file.
	_, err = f.db.ExecContext(context.Background(), `DELETE FROM platform.organization_accounts WHERE organization_id = ? AND account_id = ?`,
		f.office.organization, account.ID)
	require.NoError(t, err)
	files = namedFiles(t, f)
	assert.Empty(t, files[1].ExportedByName, "an erased colleague is named by nobody")
	_, err = f.db.ExecContext(context.Background(), `UPDATE parent_billing.export_files SET exported_by_account_id = NULL
		WHERE organization_id = ? AND exported_by_account_id = ?`, f.office.organization, account.ID)
	require.NoError(t, err)
	files = namedFiles(t, f)
	assert.Nil(t, files[1].ExportedBy)
	assert.Empty(t, files[1].ExportedByName)
	assert.False(t, files[1].ExportedByYou)
}

// exportMeasurement measures the statements of a GET of the finance exports
// as the reviewer of f.
func exportMeasurement(t *testing.T, f careBillingFixture) func(path string) testpkg.QueryMeasurement {
	return func(path string) testpkg.QueryMeasurement {
		t.Helper()
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, financeExportsPath+path, nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.traeger, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
}

// The names of the colleagues come in one statement for the whole page or
// history, however many colleagues exported.
func TestFinanceExportsNameColleaguesInOneStatement(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Namen Budget")
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Mia", sepaDebitMonth, sjrServices()...)
	createMandate(t, f, mandateBody(contract, "SJR-NB-MIA"))
	run := releaseDebitRun(t, f, sepaDebitMonth)
	var invoice int64
	require.NoError(t, f.db.NewRaw(`SELECT id FROM parent_billing.care_invoices WHERE run_id = ?`, run).Scan(context.Background(), &invoice))
	measure := exportMeasurement(t, f)
	history := fmt.Sprintf("/invoice-history/care-invoices/%d", invoice)
	first, _ := exportColleague(t, f, "Jana Krause")
	sollstellung(t, first, run)
	firstRuns, firstFiles, firstHistory := measure("/runs"), measure("/files"), measure(history)
	for _, name := range []string{"Tobias Engel", "Lena Vogt"} {
		colleague, _ := exportColleague(t, f, name)
		sollstellung(t, colleague, run)
	}
	assert.Len(t, measure("/runs").Statements, len(firstRuns.Statements), "the run list is flat in colleagues")
	assert.Len(t, measure("/files").Statements, len(firstFiles.Statements), "the protocol is flat in colleagues")
	assert.Len(t, measure(history).Statements, len(firstHistory.Statements), "the history is flat in colleagues")
	querybudgets.AssertQueryBudget(t, "api.traeger.finance_export_runs_named", firstRuns)
	querybudgets.AssertQueryBudget(t, "api.traeger.finance_export_files_named", firstFiles)
	querybudgets.AssertQueryBudget(t, "api.traeger.finance_export_invoice_history_named", firstHistory)
}
