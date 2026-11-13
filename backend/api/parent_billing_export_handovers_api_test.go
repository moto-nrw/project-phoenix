// Router-level tests of the handover of finance exports to the recipients of
// the Träger (#6824, ADR 0218; #6911, ADR 0222): the recipients without
// their passwords, one recipient per format, the rights, a handover that the
// production address policy refuses for a loopback recipient (a result, not
// an error), the protocol entry with its handover, the repeat guard per
// recipient and a flat query budget of the recipients page.
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
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

const (
	exportRecipientsPath  = "/traeger/export-recipient"
	exportRecipientPath   = exportRecipientsPath + "/"
	financeHandoversPath  = "/traeger/finance-handovers"
	handoverTestPassword  = "kein-echtes-sftp-passwort-4711"
	handoverFingerprint   = "SHA256:AbCdEfGhIjKlMnOpQrStUvWxYz0123456789+/AbCdE"
	handoverRecipientHost = "127.0.0.1"
)

type handoverResultBody struct {
	FileName      string `json:"file_name"`
	State         string `json:"state"`
	Reason        string `json:"reason"`
	RecipientName string `json:"recipient_name"`
	TargetHost    string `json:"target_host"`
}

type recipientBody struct {
	ID      int64    `json:"id"`
	Name    string   `json:"name"`
	Ready   bool     `json:"ready"`
	Formats []string `json:"formats"`
}

type recipientsPageBody struct {
	Recipients  []recipientBody `json:"recipients"`
	Assignments []struct {
		Format      string `json:"format"`
		RecipientID *int64 `json:"recipient_id"`
	} `json:"assignments"`
}

// addRecipient enters a recipient and answers the response and its id.
func addRecipient(t *testing.T, f careBillingFixture, body map[string]any) (*httptest.ResponseRecorder, int64) {
	t.Helper()
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, exportRecipientPath, body)
	if rec.Code != http.StatusCreated {
		return rec, 0
	}
	return rec, decodeSchoolPortalData[recipientBody](t, rec.Body.Bytes()).ID
}

// assignFormats sends each format to its recipient, 0 for the download.
func assignFormats(t *testing.T, f careBillingFixture, recipients map[string]int64) *httptest.ResponseRecorder {
	t.Helper()
	assignments := make([]map[string]any, 0, len(recipients))
	for format, id := range recipients {
		assignment := map[string]any{"format": format, "recipient_id": nil}
		if id > 0 {
			assignment["recipient_id"] = id
		}
		assignments = append(assignments, assignment)
	}
	return traegerJSON(t, f.traeger, f.office.token, http.MethodPut, exportRecipientsPath+"/formats", map[string]any{"assignments": assignments})
}

// saveRecipient enters the ready recipient that receives every format.
func saveRecipient(t *testing.T, f careBillingFixture, body map[string]any) int64 {
	t.Helper()
	rec, id := addRecipient(t, f, body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	rec = assignFormats(t, f, map[string]int64{"sollstellung": id, "datev": id, "pain008": id})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return id
}

func readyRecipient() map[string]any {
	return map[string]any{"name": "Kämmerei", "enabled": true, "host": handoverRecipientHost, "port": 22, "username": "kaemmerei",
		"password": handoverTestPassword, "remote_directory": "/eingang", "host_key_fingerprint": handoverFingerprint}
}

func recipientPath(id int64) string { return fmt.Sprintf("%s/%d", exportRecipientsPath, id) }

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

	rec, recipient := addRecipient(t, f, readyRecipient())
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), handoverTestPassword)
	rec = handOver(t, f, run, "sollstellung", nil)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), common.CodeParentBillingExportRecipientNotReady, "a ready recipient without the format makes no file")
	require.Equal(t, http.StatusOK, assignFormats(t, f, map[string]int64{"sollstellung": recipient, "pain008": recipient}).Code)
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
	assert.Equal(t, "Kämmerei", result.RecipientName)
	files := exportFiles(t, f, "")
	require.Len(t, files, 1)
	assert.Equal(t, result.FileName, files[0].FileName)
	assert.Contains(t, exportGet(t, f, "/files").Body.String(), `"handover":{"state":"failed","reason":"address_denied"`)
	assert.Contains(t, exportGet(t, f, "/files").Body.String(), `"recipient_name":"Kämmerei"`)

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
		recipient_id, recipient_name, state, settled_at) SELECT organization_id, id, 'kaemmerei.example.org', '/eingang', ?, 'Kämmerei', 'transferred', NOW()
		FROM parent_billing.export_files WHERE organization_id = ? AND file_number = 900`, recipient, f.office.organization).Exec(context.Background())
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

	page := traegerJSON(t, f.traeger, f.office.token, http.MethodGet, recipientPath(recipient), nil)
	require.Equal(t, http.StatusOK, page.Code)
	assert.Equal(t, 5, strings.Count(page.Body.String(), `"state":"failed"`), "every attempt is in the journal of its recipient, and the last one on it")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodDelete, recipientPath(recipient), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	removed := decodeSchoolPortalData[recipientsPageBody](t, rec.Body.Bytes())
	assert.Empty(t, removed.Recipients)
	for _, assignment := range removed.Assignments {
		assert.Nil(t, assignment.RecipientID, "every format goes back to the download: %s", assignment.Format)
	}
}

func TestFinanceExportHandoverRightsAndTraeger(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Übergabe Rechte")
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Mia", sepaDebitMonth, sjrServices()...)
	createMandate(t, f, mandateBody(contract, "SJR-HR-MIA"))
	run := releaseDebitRun(t, f, sepaDebitMonth)
	recipient := saveRecipient(t, f, readyRecipient())

	leadership := newTraegerOffice(t, f.db, f.traeger, "Träger Übergabe Leitung", identityaccess.TraegerRoleLeadership)
	for _, call := range []struct{ method, path string }{
		{http.MethodGet, exportRecipientPath}, {http.MethodPost, exportRecipientPath}, {http.MethodPut, exportRecipientsPath + "/formats"},
		{http.MethodGet, recipientPath(recipient)}, {http.MethodPut, recipientPath(recipient)}, {http.MethodDelete, recipientPath(recipient)},
		{http.MethodPost, fmt.Sprintf("%s/runs/care-runs/%d/sollstellung", financeHandoversPath, run)},
		{http.MethodPost, fmt.Sprintf("%s/runs/care-runs/%d/pain008", financeHandoversPath, run)},
	} {
		rec := traegerJSON(t, f.traeger, leadership.token, call.method, call.path, readyRecipient())
		assert.Equal(t, http.StatusForbidden, rec.Code, call.method+" "+call.path)
	}

	other := sepaDebitFixture(t, "Träger Übergabe Fremd")
	rec := traegerJSON(t, other.traeger, other.office.token, http.MethodGet, exportRecipientPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"recipients":[]`, "another Träger sees no recipient of this one")
	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		rec = traegerJSON(t, other.traeger, other.office.token, method, recipientPath(recipient), readyRecipient())
		assert.Equal(t, http.StatusNotFound, rec.Code, method)
		assert.Contains(t, rec.Body.String(), common.CodeParentBillingExportRecipientNotFound, method)
	}
	otherRecipient := saveRecipient(t, other, readyRecipient())
	rec = assignFormats(t, other, map[string]int64{"datev": recipient})
	assert.Equal(t, http.StatusNotFound, rec.Code, "a Träger never sends a format to another Träger's recipient")
	assert.Contains(t, rec.Body.String(), common.CodeParentBillingExportRecipientNotFound)
	assert.NotZero(t, otherRecipient)
	rec = handOver(t, other, run, "sollstellung", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), common.CodeParentBillingExportRunNotFound)

	invalid := readyRecipient()
	invalid["host_key_fingerprint"] = "MD5:00"
	delete(invalid, "password")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPut, recipientPath(recipient), invalid)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), common.CodeParentBillingExportRecipientInvalid)
	assert.Contains(t, rec.Body.String(), `"field":"host_key_fingerprint"`)
	rec, _ = addRecipient(t, f, readyRecipient())
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), common.CodeParentBillingExportRecipientNameTaken)
	assert.Contains(t, rec.Body.String(), `"field":"name"`)
	rec = assignFormats(t, f, map[string]int64{"cip": recipient})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), common.CodeParentBillingExportFormatsInvalid)
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, exportRecipientsPath+"/abc", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

// The recipients page reads the recipients with their formats and last
// handover and the journal in a fixed number of statements, however many
// recipients and handovers there are.
func TestExportRecipientsQueryBudget(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Übergabe Budget")
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Ben", sepaDebitMonth, sjrServices()...)
	createMandate(t, f, mandateBody(contract, "SJR-HB-BEN"))
	run := releaseDebitRun(t, f, sepaDebitMonth)
	saveRecipient(t, f, readyRecipient())
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
	for i := range 3 {
		require.Equal(t, http.StatusOK, handOver(t, f, run, "sollstellung", nil).Code)
		more := readyRecipient()
		more["name"] = fmt.Sprintf("Empfänger %d", i)
		rec, id := addRecipient(t, f, more)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		require.Equal(t, http.StatusOK, assignFormats(t, f, map[string]int64{"datev": id}).Code)
	}
	assert.Len(t, measure().Statements, len(first.Statements), "the recipients page is flat in recipients and handovers")
	querybudgets.AssertQueryBudget(t, "api.traeger.export_recipient", first)
}

// The Träger comes from the session alone: an organization in the query or
// the body never reaches another Träger's recipient or run.
func TestFinanceExportHandoverIgnoresARequestedOrganization(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Übergabe eigen")
	other := sepaDebitFixture(t, "Träger Übergabe fremd")
	input := readyRecipient()
	input["organization_id"] = other.office.organization
	rec, _ := addRecipient(t, f, input)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var foreign int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM config.traeger_export_recipients WHERE organization_id = ?`,
		other.office.organization).Scan(context.Background(), &foreign))
	assert.Zero(t, foreign, "the body names no Träger")

	rec = traegerJSON(t, other.traeger, other.office.token, http.MethodGet,
		fmt.Sprintf("%s?organization_id=%d", exportRecipientPath, f.office.organization), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"recipients":[]`, "the query names no Träger")

	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Mia", sepaDebitMonth, sjrServices()...)
	createMandate(t, f, mandateBody(contract, "SJR-H-MIA"))
	run := releaseDebitRun(t, f, sepaDebitMonth)
	saveRecipient(t, other, readyRecipient())
	rec = handOver(t, other, run, "pain008", map[string]any{"organization_id": f.office.organization})
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

// A Sollstellung whose mapping writes the file number into every row is
// still the same content: the protocol marks the repeat and a handover asks
// before it goes out again (ADR 0218 section 6).
func TestNumberedSollstellungRepeatsTheSameContent(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Übergabe nummeriert")
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Ben", sepaDebitMonth, sjrServices()...)
	createMandate(t, f, mandateBody(contract, "SJR-H-BEN"))
	run := releaseDebitRun(t, f, sepaDebitMonth)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPut, financeExportsPath+"/settings/sollstellung", map[string]any{
		"columns":   []map[string]any{{"field": "invoice_number", "header": "Nr"}, {"field": "file_number", "header": "Datei"}},
		"delimiter": ";", "decimal_separator": ",", "date_format": "dd.mm.yyyy", "encoding": "utf-8", "header_row": true,
		"file_name_pattern": "soll-{NR}.csv",
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	_, first := sollstellung(t, f, run)
	_, second := sollstellung(t, f, run)
	require.NotEqual(t, first, second, "the rows carry their file number")
	files := exportFiles(t, f, "")
	require.Len(t, files, 2)
	assert.Equal(t, files[1].FileNumber, files[0].RepeatOf, "the protocol knows the repeat")

	recipient := saveRecipient(t, f, readyRecipient())
	_, err := f.db.NewRaw(`INSERT INTO parent_billing.export_file_transfers (organization_id, export_file_id, target_host, target_directory,
		recipient_id, recipient_name, state, settled_at) SELECT organization_id, id, 'kaemmerei.example.org', '/eingang', ?, 'Kämmerei', 'transferred', NOW()
		FROM parent_billing.export_files WHERE organization_id = ? AND file_number = ?`, recipient, f.office.organization, files[1].FileNumber).
		Exec(context.Background())
	require.NoError(t, err)
	rec = handOver(t, f, run, "sollstellung", nil)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), common.CodeParentBillingExportAlreadyHandedOver)
}

// Each format goes to its own recipient, and the repeat guard asks per
// recipient: a file one recipient has does not stop its handover to another
// (#6911, ADR 0222).
func TestFinanceExportFormatsGoToTheirOwnRecipient(t *testing.T) {
	t.Parallel()
	f := sepaDebitFixture(t, "Träger Übergabe zwei Empfänger")
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Ida", sepaDebitMonth, sjrServices()...)
	createMandate(t, f, mandateBody(contract, "SJR-H-IDA"))
	run := releaseDebitRun(t, f, sepaDebitMonth)
	_, kaemmerei := addRecipient(t, f, readyRecipient())
	steuer := readyRecipient()
	steuer["name"], steuer["remote_directory"] = "Steuerbüro", "/buchhaltung"
	_, steuerbuero := addRecipient(t, f, steuer)
	require.Equal(t, http.StatusOK, assignFormats(t, f, map[string]int64{"sollstellung": kaemmerei, "pain008": steuerbuero}).Code)

	soll := decodeSchoolPortalData[handoverResultBody](t, handOver(t, f, run, "sollstellung", nil).Body.Bytes())
	assert.Equal(t, "Kämmerei", soll.RecipientName)
	pain := decodeSchoolPortalData[handoverResultBody](t, handOver(t, f, run, "pain008", nil).Body.Bytes())
	assert.Equal(t, "Steuerbüro", pain.RecipientName)

	// The Kämmerei has the Sollstellung: a second handover to it asks first.
	_, err := f.db.NewRaw(`INSERT INTO parent_billing.export_files (organization_id, file_number, format, run_kind, run_id, file_name,
		content_sha256, invoice_count, total_cents) SELECT organization_id, 901, format, run_kind, run_id, 'arrived.csv', content_sha256, 1, 0
		FROM parent_billing.export_files WHERE organization_id = ? AND format = 'sollstellung' ORDER BY id LIMIT 1`, f.office.organization).
		Exec(context.Background())
	require.NoError(t, err)
	_, err = f.db.NewRaw(`INSERT INTO parent_billing.export_file_transfers (organization_id, export_file_id, target_host, target_directory,
		recipient_id, recipient_name, state, settled_at) SELECT organization_id, id, 'kaemmerei.example.org', '/eingang', ?, 'Kämmerei', 'transferred', NOW()
		FROM parent_billing.export_files WHERE organization_id = ? AND file_number = 901`, kaemmerei, f.office.organization).Exec(context.Background())
	require.NoError(t, err)
	rec := handOver(t, f, run, "sollstellung", nil)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), common.CodeParentBillingExportAlreadyHandedOver)
	// The tax office never had it: once the Sollstellung goes there, no
	// question.
	require.Equal(t, http.StatusOK, assignFormats(t, f, map[string]int64{"sollstellung": steuerbuero}).Code)
	rec = handOver(t, f, run, "sollstellung", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, "Steuerbüro", decodeSchoolPortalData[handoverResultBody](t, rec.Body.Bytes()).RecipientName)

	// Removing the tax office sends its formats back to the download.
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodDelete, recipientPath(steuerbuero), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = handOver(t, f, run, "pain008", nil)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), common.CodeParentBillingExportRecipientNotReady)
	page := decodeSchoolPortalData[recipientsPageBody](t, traegerJSON(t, f.traeger, f.office.token, http.MethodGet, exportRecipientPath, nil).Body.Bytes())
	require.Len(t, page.Recipients, 1)
	assert.Equal(t, []string{}, page.Recipients[0].Formats, "the Kämmerei kept no format after the reassignment")
}
