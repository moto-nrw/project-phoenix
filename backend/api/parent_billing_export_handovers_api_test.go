// Router-level tests of the handover of finance exports to the recipient of
// the Träger (#6824, ADR 0218): the recipient without its password, the
// rights, a handover that the production address policy refuses for a
// loopback recipient (a result, not an error), the protocol entry with its
// handover, the repeat guard and a flat query budget of the recipient page.
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

const (
	exportRecipientPath   = "/traeger/export-recipient/"
	financeHandoversPath  = "/traeger/finance-handovers"
	handoverTestPassword  = "kein-echtes-sftp-passwort-4711"
	handoverFingerprint   = "SHA256:AbCdEfGhIjKlMnOpQrStUvWxYz0123456789+/AbCdE"
	handoverRecipientHost = "127.0.0.1"
)

type handoverResultBody struct {
	FileName   string `json:"file_name"`
	State      string `json:"state"`
	Reason     string `json:"reason"`
	TargetHost string `json:"target_host"`
}

func saveRecipient(t *testing.T, f careBillingFixture, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return traegerJSON(t, f.traeger, f.office.token, http.MethodPut, exportRecipientPath, body)
}

func readyRecipient() map[string]any {
	return map[string]any{"enabled": true, "host": handoverRecipientHost, "port": 22, "username": "kaemmerei",
		"password": handoverTestPassword, "remote_directory": "/eingang", "host_key_fingerprint": handoverFingerprint}
}

func handOver(t *testing.T, f careBillingFixture, run int64, format string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return traegerJSON(t, f.traeger, f.office.token, http.MethodPost, fmt.Sprintf("%s/runs/care-runs/%d/%s", financeHandoversPath, run, format), body)
}

func TestFinanceExportHandoverOfARun(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Übergabe SJR")
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Lia", sepaDebitMonth, sjrServices()...)
	createMandate(t, f, mandateBody(contract, "SJR-H-LIA"))
	run := releaseDebitRun(t, f, sepaDebitMonth)

	rec := handOver(t, f, run, "sollstellung", nil)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), common.CodeParentBillingExportRecipientNotReady)
	assert.Empty(t, exportFiles(t, f, ""), "no recipient, no file and no number")

	rec = saveRecipient(t, f, readyRecipient())
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), handoverTestPassword)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, exportRecipientPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), handoverTestPassword, "the password never leaves")
	assert.Contains(t, rec.Body.String(), `"password_set":true`)
	assert.Contains(t, rec.Body.String(), `"ready":true`)

	// The production address policy refuses the loopback recipient: the
	// handover is a recorded failure, with its file in the protocol.
	rec = handOver(t, f, run, "sollstellung", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	result := decodeSchoolPortalData[handoverResultBody](t, rec.Body.Bytes())
	assert.Equal(t, "failed", result.State)
	assert.Equal(t, "address_denied", result.Reason)
	assert.Equal(t, handoverRecipientHost, result.TargetHost)
	files := exportFiles(t, f, "")
	require.Len(t, files, 1)
	assert.Equal(t, result.FileName, files[0].FileName)
	assert.Contains(t, exportGet(t, f, "/files").Body.String(), `"handover":{"state":"failed","reason":"address_denied"`)

	// A failed handover did not arrive: the next one needs no repeat. One
	// that arrived does.
	rec = handOver(t, f, run, "sollstellung", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	_, err := f.db.NewRaw(`UPDATE parent_billing.export_file_transfers SET state = 'pending', failure_reason = '', settled_at = NULL
		WHERE organization_id = ?`, f.office.organization).Exec(context.Background())
	require.Error(t, err, "a settled handover never changes")
	_, err = f.db.NewRaw(`INSERT INTO parent_billing.export_files (organization_id, file_number, format, run_kind, run_id, file_name,
		content_sha256, invoice_count, total_cents) SELECT organization_id, 900, format, run_kind, run_id, 'arrived.csv', content_sha256, 1, 0
		FROM parent_billing.export_files WHERE organization_id = ? ORDER BY id LIMIT 1`, f.office.organization).Exec(context.Background())
	require.NoError(t, err)
	_, err = f.db.NewRaw(`INSERT INTO parent_billing.export_file_transfers (organization_id, export_file_id, target_host, target_directory,
		state, settled_at) SELECT organization_id, id, 'kaemmerei.example.org', '/eingang', 'transferred', NOW()
		FROM parent_billing.export_files WHERE organization_id = ? AND file_number = 900`, f.office.organization).Exec(context.Background())
	require.NoError(t, err)
	before := len(exportFiles(t, f, ""))
	rec = handOver(t, f, run, "sollstellung", nil)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), common.CodeParentBillingExportAlreadyHandedOver)
	assert.Len(t, exportFiles(t, f, ""), before, "a refused repeat takes no file number")
	rec = handOver(t, f, run, "sollstellung", map[string]any{"repeat": true})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// The PAIN.008 file goes the same way, behind "Abrechnung".
	rec = handOver(t, f, run, "pain008", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.True(t, strings.HasPrefix(decodeSchoolPortalData[handoverResultBody](t, rec.Body.Bytes()).FileName, "lastschrift-"))

	page := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, exportRecipientPath, nil)
	require.Equal(t, http.StatusOK, page.Code)
	assert.Equal(t, 4, strings.Count(page.Body.String(), `"state":"failed"`), "every attempt is in the journal")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodDelete, exportRecipientPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"configured":false`)
}

func TestFinanceExportHandoverRightsAndTraeger(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Übergabe Rechte")
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Mia", sepaDebitMonth, sjrServices()...)
	createMandate(t, f, mandateBody(contract, "SJR-HR-MIA"))
	run := releaseDebitRun(t, f, sepaDebitMonth)
	require.Equal(t, http.StatusOK, saveRecipient(t, f, readyRecipient()).Code)

	leadership := newTraegerOffice(t, f.db, f.traeger, "Träger Übergabe Leitung", identityaccess.TraegerRoleLeadership)
	for _, call := range []struct{ method, path string }{
		{http.MethodGet, exportRecipientPath}, {http.MethodPut, exportRecipientPath}, {http.MethodDelete, exportRecipientPath},
		{http.MethodPost, fmt.Sprintf("%s/runs/care-runs/%d/sollstellung", financeHandoversPath, run)},
		{http.MethodPost, fmt.Sprintf("%s/runs/care-runs/%d/pain008", financeHandoversPath, run)},
	} {
		rec := traegerJSON(t, f.traeger, leadership.token, call.method, call.path, readyRecipient())
		assert.Equal(t, http.StatusForbidden, rec.Code, call.method+" "+call.path)
	}

	other := sepaDebitFixture(t, "Träger Übergabe Fremd")
	rec := traegerJSON(t, other.traeger, other.office.token, http.MethodGet, exportRecipientPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"configured":false`, "another Träger sees no recipient of this one")
	require.Equal(t, http.StatusOK, saveRecipient(t, other, readyRecipient()).Code)
	rec = handOver(t, other, run, "sollstellung", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), common.CodeParentBillingExportRunNotFound)

	invalid := readyRecipient()
	invalid["host_key_fingerprint"] = "MD5:00"
	delete(invalid, "password")
	rec = saveRecipient(t, f, invalid)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), common.CodeParentBillingExportRecipientInvalid)
	assert.Contains(t, rec.Body.String(), `"field":"host_key_fingerprint"`)
}

// The recipient page reads the recipient and its journal in a fixed number
// of statements, however many handovers the journal holds.
func TestExportRecipientQueryBudget(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Übergabe Budget")
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Ben", sepaDebitMonth, sjrServices()...)
	createMandate(t, f, mandateBody(contract, "SJR-HB-BEN"))
	run := releaseDebitRun(t, f, sepaDebitMonth)
	require.Equal(t, http.StatusOK, saveRecipient(t, f, readyRecipient()).Code)
	measure := func() testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, exportRecipientPath, nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.traeger, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	require.Equal(t, http.StatusOK, handOver(t, f, run, "sollstellung", nil).Code)
	first := measure()
	for range 3 {
		require.Equal(t, http.StatusOK, handOver(t, f, run, "sollstellung", nil).Code)
	}
	assert.Len(t, measure().Statements, len(first.Statements), "the recipient page is flat in handovers")
	querybudgets.AssertQueryBudget(t, "api.traeger.export_recipient", first)
}
