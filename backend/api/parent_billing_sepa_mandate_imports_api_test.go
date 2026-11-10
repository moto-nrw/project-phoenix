// Router-level tests of the import of existing SEPA mandates (#6198): the
// preview with its mapping, the import of the ready rows through the
// mandate owner, a repeated import without duplicates, the trail with the
// file's name and line, the right "Abrechnung", the Träger boundary and
// the upload checks.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/encoding/charmap"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

const (
	sepaImportPath        = sepaMandatesPath + "/imports"
	sepaImportPreviewPath = sepaImportPath + "/preview"
)

type sepaImportBody struct {
	Encoding    string   `json:"encoding"`
	Delimiter   string   `json:"delimiter"`
	Headers     []string `json:"headers"`
	Unmapped    []string `json:"unmapped"`
	Applied     bool     `json:"applied"`
	Fingerprint string   `json:"fingerprint"`
	Mapping     struct {
		PayerKey string `json:"payer_key"`
	} `json:"mapping"`
	Counts map[string]int `json:"counts"`
	Rows   []struct {
		Line            int     `json:"line"`
		Status          string  `json:"status"`
		Reference       string  `json:"reference"`
		IBANMasked      string  `json:"iban_masked"`
		ContractID      int64   `json:"contract_id"`
		PayerLastName   string  `json:"payer_last_name"`
		Sequence        string  `json:"sequence"`
		LastCollectedOn *string `json:"last_collected_on"`
		MandateID       int64   `json:"mandate_id"`
		Problems        []struct {
			Field string `json:"field"`
			Code  string `json:"code"`
		} `json:"problems"`
	} `json:"rows"`
}

// importForm posts a file and the optional fields as multipart form.
func importForm(t *testing.T, router http.Handler, token, path, name string, content []byte, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	part, err := form.CreateFormFile("file", name)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	for key, value := range fields {
		require.NoError(t, form.WriteField(key, value))
	}
	require.NoError(t, form.Close())
	req := httptest.NewRequest(http.MethodPost, path, &body)
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// applyImport takes over a file like the office does: it previews the file
// with the same fields and imports with the fingerprint of that preview.
func applyImport(t *testing.T, router http.Handler, token, name string, content []byte, fields map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	rec := importForm(t, router, token, sepaImportPreviewPath, name, content, fields)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	confirmed := map[string]string{"fingerprint": decodeSchoolPortalData[sepaImportBody](t, rec.Body.Bytes()).Fingerprint}
	for key, value := range fields {
		confirmed[key] = value
	}
	return importForm(t, router, token, sepaImportPath, name, content, confirmed)
}

func partyEmail(t *testing.T, f careBillingFixture, contract int64) string {
	t.Helper()
	var email string
	require.NoError(t, f.db.NewRaw(`SELECT email FROM care_contract.contract_parties WHERE contract_id = ?`, contract).Scan(context.Background(), &email))
	return email
}

// The SJR case: an export with German headers in Windows-1252 and
// semicolons. moto maps the columns itself; the office says the file holds
// the Träger's mandates and answers the one row the file leaves unclear.
// Used mandates come as RCUR with their last collection, the reference and
// date stay, and a second import of the same file adds nothing.
func TestSEPAMandateImportTakesOverOldMandatesOnce(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mandatsimport SJR")
	school := f.office.schools[0]
	used := careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	fresh := careContract(t, f.db, f.office.organization, school, "Noah", careMonth, sjrServices()...)
	unclear := careContract(t, f.db, f.office.organization, school, "Lia", careMonth, sjrServices()...)
	saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 14, "missing_mandate": "transfer"})

	csv := strings.Join([]string{
		"Mandatsreferenz;Unterschriftsdatum;IBAN;Kontoinhaber;E-Mail;Lastschriftart;Letzter Einzug",
		fmt.Sprintf("OGS-0001;15.08.2024;DE89 3704 0044 0532 0130 00;Jürgen Müller;%s;Folgelastschrift;01.10.2026", partyEmail(t, f, used)),
		fmt.Sprintf("OGS-0002;01.09.2026;%s;Eva Beispiel;%s;FRST;", sepaIBAN, partyEmail(t, f, fresh)),
		fmt.Sprintf("OGS-0003;01.09.2025;%s;Eva Beispiel;%s;;", sepaIBAN, partyEmail(t, f, unclear)),
		fmt.Sprintf("OGS-0004;31.02.2025;DE89370400440532013001;;%s;RCUR;", "unbekannt@example.org"),
	}, "\r\n") + "\r\n"
	content, err := charmap.Windows1252.NewEncoder().Bytes([]byte(csv))
	require.NoError(t, err)

	// Without the choice of the creditor the preview names the gap and reads
	// no row; importing then is refused.
	rec := importForm(t, f.traeger, f.office.token, sepaImportPreviewPath, "ogs-connect.csv", content, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview := decodeSchoolPortalData[sepaImportBody](t, rec.Body.Bytes())
	assert.Equal(t, "windows-1252", preview.Encoding)
	assert.Equal(t, ";", preview.Delimiter)
	assert.Equal(t, []string{"creditor_id"}, preview.Unmapped)
	assert.Empty(t, preview.Rows)
	rec = importForm(t, f.traeger, f.office.token, sepaImportPath, "ogs-connect.csv", content, nil)
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.sepa_import_mapping_invalid", code)

	mapping := `{"columns":{"reference":"Mandatsreferenz","signed_on":"Unterschriftsdatum","iban":"IBAN","account_holder":"Kontoinhaber",` +
		`"payer":"E-Mail","sequence":"Lastschriftart","last_collected_on":"Letzter Einzug"},"payer_key":"email","creditor_from_traeger":true,"confirmed_creditor_id":"DE98ZZZ09999999999"}`
	rec = importForm(t, f.traeger, f.office.token, sepaImportPreviewPath, "ogs-connect.csv", content, map[string]string{"mapping": mapping})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotContains(t, rec.Body.String(), "0532013000", "the preview shows no IBAN")
	assert.NotContains(t, rec.Body.String(), "0532013001", "nor an invalid one")
	preview = decodeSchoolPortalData[sepaImportBody](t, rec.Body.Bytes())
	require.Len(t, preview.Rows, 4)
	assert.Equal(t, map[string]int{"ready": 2, "clarify": 1, "duplicate": 0, "conflict": 0, "invalid": 1, "imported": 0,
		"debtor_numbers": 0, "debtor_conflicts": 0}, preview.Counts)
	assert.Equal(t, "clarify", preview.Rows[2].Status)
	assert.Equal(t, 4, preview.Rows[2].Line)
	assert.Equal(t, "invalid", preview.Rows[3].Status)
	assert.Equal(t, 5, preview.Rows[3].Line)
	assert.Equal(t, "DE•• •••• •••• •••• ••30 00", preview.Rows[0].IBANMasked)
	var stored int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.sepa_mandates WHERE organization_id = ?`,
		f.office.organization).Scan(context.Background(), &stored))
	assert.Zero(t, stored, "the preview stores nothing")

	answers := map[string]string{"mapping": mapping, "sequences": `{"4":"recurring"}`}
	rec = applyImport(t, f.traeger, f.office.token, "ogs-connect.csv", content, answers)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	result := decodeSchoolPortalData[sepaImportBody](t, rec.Body.Bytes())
	assert.True(t, result.Applied)
	assert.Equal(t, 3, result.Counts["imported"])
	assert.Equal(t, 1, result.Counts["invalid"])

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, sepaMandatesPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	byReference := map[string]sepaMandateBody{}
	for _, mandate := range decodeSchoolPortalData[[]sepaMandateBody](t, rec.Body.Bytes()) {
		byReference[mandate.Reference] = mandate
	}
	require.Len(t, byReference, 3)
	assert.Equal(t, "recurring", byReference["OGS-0001"].Sequence)
	assert.Equal(t, "2024-08-15", byReference["OGS-0001"].SignedOn, "the original date stays")
	assert.Equal(t, "Jürgen Müller", byReference["OGS-0001"].AccountHolder)
	assert.Equal(t, "import", byReference["OGS-0001"].Source)
	assert.Equal(t, sepaCreditorID, byReference["OGS-0001"].CreditorID)
	assert.Equal(t, "first", byReference["OGS-0002"].Sequence)
	assert.Equal(t, "recurring", byReference["OGS-0003"].Sequence, "the office's answer")
	// The mandates fit the debit file (#4697): the used ones are RCUR and
	// run 36 months from their last collection, or from the import when the
	// file does not tell it; none counts as expired.
	var expiry []struct {
		Reference string    `json:"reference"`
		ExpiresOn string    `json:"expires_on"`
		CreatedAt time.Time `json:"created_at"`
	}
	require.NoError(t, json.Unmarshal(mustData(t, rec.Body.Bytes()), &expiry))
	expires, takenOver := map[string]string{}, map[string]calendar.Date{}
	for _, mandate := range expiry {
		expires[mandate.Reference], takenOver[mandate.Reference] = mandate.ExpiresOn, calendar.DateFromTime(mandate.CreatedAt)
	}
	assert.Equal(t, "2029-10-01", expires["OGS-0001"], "36 months from the last collection")
	importedOn := takenOver["OGS-0003"]
	assert.Equal(t, parentbilling.MandateExpiresOn(calendar.NewDate(2025, 9, 1), &importedOn).String(), expires["OGS-0003"],
		"used elsewhere without a date: 36 months from the import")
	assert.Equal(t, "2029-09-01", expires["OGS-0002"], "never collected: 36 months from the signature")
	var lastCollected string
	require.NoError(t, f.db.NewRaw(`SELECT last_collected_on::text FROM parent_billing.sepa_mandates WHERE id = ?`,
		byReference["OGS-0001"].ID).Scan(context.Background(), &lastCollected))
	assert.Equal(t, "2026-10-01", lastCollected)
	var origin string
	require.NoError(t, f.db.NewRaw(`SELECT action || ':' || source || ':' || import_line || ':' || import_file
		FROM parent_billing.sepa_mandate_events WHERE mandate_id = ? AND import_file IS NOT NULL`,
		byReference["OGS-0003"].ID).Scan(context.Background(), &origin))
	assert.Equal(t, "create:import:4:ogs-connect.csv", origin, "the trail keeps the file and the line")
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet,
		fmt.Sprintf("%s/%d/history", sepaMandatesPath, byReference["OGS-0003"].ID), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	events := decodeSchoolPortalData[[]struct {
		ImportFile string `json:"import_file"`
		ImportLine int    `json:"import_line"`
	}](t, rec.Body.Bytes())
	require.NotEmpty(t, events)
	assert.Equal(t, "ogs-connect.csv", events[0].ImportFile)
	assert.Equal(t, 4, events[0].ImportLine)
	for _, event := range events[1:] {
		assert.Empty(t, event.ImportFile, "only the creation records the import origin")
		assert.Zero(t, event.ImportLine)
	}

	// The same file again: every taken-over row is a duplicate, nothing new.
	rec = applyImport(t, f.traeger, f.office.token, "ogs-connect.csv", content, answers)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	again := decodeSchoolPortalData[sepaImportBody](t, rec.Body.Bytes())
	assert.Equal(t, 3, again.Counts["duplicate"])
	assert.Zero(t, again.Counts["imported"])
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.sepa_mandates WHERE organization_id = ?`,
		f.office.organization).Scan(context.Background(), &stored))
	assert.Equal(t, 3, stored, "a repeated import adds no mandate")

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, sepaImportPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var log []struct {
		File      string `json:"file"`
		AccountID *int64 `json:"account_id"`
		Mandates  int    `json:"mandates"`
	}
	require.NoError(t, json.Unmarshal(mustData(t, rec.Body.Bytes()), &log))
	require.Len(t, log, 1, "the repeated import took over nothing")
	assert.Equal(t, "ogs-connect.csv", log[0].File)
	assert.Equal(t, 3, log[0].Mandates)
	assert.NotNil(t, log[0].AccountID)
}

func mustData(t *testing.T, body []byte) []byte {
	t.Helper()
	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &envelope))
	return envelope.Data
}

// Conflicts with stored mandates are reported, never overwritten: another
// reference for a family with a mandate in force, a reference the Träger
// gave another family, a mandate of another creditor. A contract of another
// Träger is no payer here.
func TestSEPAMandateImportReportsConflictsAndKeepsTheTraeger(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mandatsimport Konflikt")
	school := f.office.schools[0]
	withMandate := careContract(t, f.db, f.office.organization, school, "Mia", careMonth, sjrServices()...)
	other := careContract(t, f.db, f.office.organization, school, "Noah", careMonth, sjrServices()...)
	saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 14, "missing_mandate": "transfer"})
	createMandate(t, f, mandateBody(withMandate, "SJR-K-1"))
	foreign := newCareBillingFixture(t, "Träger Mandatsimport Fremd")
	foreignContract := careContract(t, foreign.db, foreign.office.organization, foreign.office.schools[0], "Ben", careMonth, sjrServices()...)

	csv := strings.Join([]string{
		"Referenz,Datum,IBAN,Kontoinhaber,Vertragsnummer,Gläubiger-ID,Sequenz",
		fmt.Sprintf("SJR-K-2,2026-09-01,%s,Eva Beispiel,%d,%s,RCUR", sepaIBAN, withMandate, sepaCreditorID),
		fmt.Sprintf("SJR-K-1,2026-08-01,%s,Eva Beispiel,%d,%s,RCUR", sepaIBAN, other, sepaCreditorID),
		fmt.Sprintf("SJR-K-3,2026-09-01,%s,Eva Beispiel,%d,DE98ABC09999999999,RCUR", sepaIBAN, other),
		fmt.Sprintf("SJR-K-4,2026-09-01,%s,Eva Beispiel,%d,%s,RCUR", sepaIBAN, foreignContract, sepaCreditorID),
	}, "\n")
	mapping := `{"columns":{"reference":"Referenz","signed_on":"Datum","iban":"IBAN","account_holder":"Kontoinhaber",` +
		`"payer":"Vertragsnummer","creditor_id":"Gläubiger-ID","sequence":"Sequenz"},"payer_key":"contract_id"}`
	rec := applyImport(t, f.traeger, f.office.token, "mandate.csv", []byte(csv), map[string]string{"mapping": mapping})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	result := decodeSchoolPortalData[sepaImportBody](t, rec.Body.Bytes())
	require.Len(t, result.Rows, 4)
	codes := func(i int) []string {
		var got []string
		for _, problem := range result.Rows[i].Problems {
			got = append(got, problem.Field+":"+problem.Code)
		}
		return got
	}
	assert.Equal(t, "conflict", result.Rows[0].Status)
	assert.Equal(t, []string{"payer:payer_has_mandate"}, codes(0))
	assert.Equal(t, "conflict", result.Rows[1].Status)
	assert.Equal(t, []string{"reference:reference_taken"}, codes(1))
	assert.Equal(t, []string{"creditor_id:creditor_mismatch"}, codes(2))
	assert.Equal(t, "invalid", result.Rows[3].Status, "a contract of another Träger is no payer here")
	assert.Equal(t, []string{"payer:payer_unknown"}, codes(3))
	assert.Empty(t, result.Rows[3].PayerLastName, "and names no one of it")
	assert.Zero(t, result.Counts["imported"])

	var mandates int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.sepa_mandates WHERE organization_id IN (?, ?)`,
		f.office.organization, foreign.office.organization).Scan(context.Background(), &mandates))
	assert.Equal(t, 1, mandates, "nothing is overwritten or added")
}

func TestSEPAMandateImportNeedsTheRightAndAFileOfText(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mandatsimport Recht")
	saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 14, "missing_mandate": "transfer"})
	csv := []byte("Mandatsreferenz;IBAN\nSJR-1;" + sepaIBAN + "\n")

	password := testpkg.NewTestPassword()
	account := traegerAccount(t, f.db, password)
	testpkg.CreateTestTraegerMember(t, f.db, f.office.organization, account.ID, identityaccess.TraegerRoleAdministration)
	code, answer := traegerSignIn(t, f.traeger, account.Email, password, 0)
	require.Equal(t, http.StatusOK, code)
	for _, path := range []string{sepaImportPreviewPath, sepaImportPath} {
		rec := importForm(t, f.traeger, answer.AccessToken, path, "mandate.csv", csv, nil)
		assert.Equal(t, http.StatusForbidden, rec.Code, "%s without the right: %s", path, rec.Body.String())
	}
	rec := traegerJSON(t, f.traeger, answer.AccessToken, http.MethodGet, sepaImportPath, nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	for name, test := range map[string]struct {
		file    string
		content []byte
		status  int
		code    string
	}{
		"an executable named .exe": {"mandate.exe", csv, http.StatusBadRequest, "parent_billing.sepa_import_file_invalid"},
		"a zip named .csv":         {"mandate.csv", []byte("PK\x03\x04\x14\x00\x00\x00binary"), http.StatusBadRequest, "parent_billing.sepa_import_file_invalid"},
		"too large":                {"mandate.csv", bytes.Repeat([]byte("a;b\n"), 3<<18), http.StatusRequestEntityTooLarge, "parent_billing.sepa_import_file_too_large"},
	} {
		rec := importForm(t, f.traeger, f.office.token, sepaImportPreviewPath, test.file, test.content, nil)
		require.Equal(t, test.status, rec.Code, "%s: %s", name, rec.Body.String())
		got, _ := registrationRefusalOf(t, rec)
		assert.Equal(t, test.code, got, name)
	}
}

// The log reads the same statements for any number of imports.
func TestSEPAMandateImportLogQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mandatsimport Budget")
	saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 14, "missing_mandate": "transfer"})
	measure := func() testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, sepaImportPath, nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.traeger, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	importFor := func(child string) {
		contract := careContract(t, f.db, f.office.organization, f.office.schools[0], child, careMonth, sjrServices()...)
		csv := fmt.Sprintf("Mandatsreferenz;Mandatsdatum;IBAN;Kontoinhaber;Vertragsnummer;Gläubiger-ID;Sequenz\nSJR-B-%s;01.09.2026;%s;Eva Beispiel;%d;%s;FRST\n",
			child, sepaIBAN, contract, sepaCreditorID)
		rec := applyImport(t, f.traeger, f.office.token, "mandate.csv", []byte(csv), nil)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), `"imported":1`)
	}
	importFor("Mia")
	first := measure()
	importFor("Noah")
	importFor("Lia")
	assert.Len(t, measure().Statements, len(first.Statements), "the log reads the same statements for any number of imports")
	querybudgets.AssertQueryBudget(t, "api.traeger.sepa_mandate_imports", first)
}

// The office confirms that a file without a creditor column holds the
// Träger's mandates for the creditor identifier it saw. When the identifier
// changes before the import, that confirmation no longer holds: the import
// is refused and the preview asks again, so no mandate is bound to an
// identifier nobody confirmed.
func TestSEPAMandateImportNeedsTheConfirmationOfTheCurrentCreditor(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mandatsimport Gläubiger")
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Mia", careMonth, sjrServices()...)
	saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 14, "missing_mandate": "transfer"})
	csv := fmt.Sprintf("Mandatsreferenz;Mandatsdatum;IBAN;Kontoinhaber;E-Mail;Sequenz\nSJR-G-1;01.09.2026;%s;Eva Beispiel;%s;FRST\n",
		sepaIBAN, partyEmail(t, f, contract))
	mapping := `{"columns":{"reference":"Mandatsreferenz","signed_on":"Mandatsdatum","iban":"IBAN","account_holder":"Kontoinhaber",` +
		`"payer":"E-Mail","sequence":"Sequenz"},"payer_key":"email","creditor_from_traeger":true,"confirmed_creditor_id":"` + sepaCreditorID + `"}`
	rec := importForm(t, f.traeger, f.office.token, sepaImportPreviewPath, "mandate.csv", []byte(csv), map[string]string{"mapping": mapping})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Empty(t, decodeSchoolPortalData[sepaImportBody](t, rec.Body.Bytes()).Unmapped, "confirmed for the current identifier")

	saveCreditor(t, f, map[string]any{"creditor_id": "DE98ABC09999999999", "prenotification_days": 14, "missing_mandate": "transfer"})
	rec = importForm(t, f.traeger, f.office.token, sepaImportPath, "mandate.csv", []byte(csv), map[string]string{"mapping": mapping})
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	code, fields := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.sepa_import_mapping_invalid", code)
	assert.Equal(t, []string{"creditor_id"}, fields)
	rec = importForm(t, f.traeger, f.office.token, sepaImportPreviewPath, "mandate.csv", []byte(csv), map[string]string{"mapping": mapping})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview := decodeSchoolPortalData[sepaImportBody](t, rec.Body.Bytes())
	assert.Equal(t, []string{"creditor_id"}, preview.Unmapped, "the preview asks for the new identifier")
	var stored int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.sepa_mandates WHERE organization_id = ?`,
		f.office.organization).Scan(context.Background(), &stored))
	assert.Zero(t, stored, "no mandate for an unconfirmed identifier")
}

// A stored mandate with the same reference, family, date and creditor but
// another IBAN or account holder is a conflict, never a duplicate: the
// import opens the stored account in memory through the mandate owner and
// compares. No answer repeats either IBAN.
func TestSEPAMandateImportReportsAnotherAccountOfTheSameMandate(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mandatsimport Konto")
	contract := careContract(t, f.db, f.office.organization, f.office.schools[0], "Mia", careMonth, sjrServices()...)
	saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 14, "missing_mandate": "transfer"})
	const otherIBAN = "DE02120300000000202051"
	row := func(iban, holder string) []byte {
		return []byte(fmt.Sprintf("Mandatsreferenz;Mandatsdatum;IBAN;Kontoinhaber;E-Mail;Gläubiger-ID;Sequenz\nSJR-A-1;01.09.2026;%s;%s;%s;%s;RCUR\n",
			iban, holder, partyEmail(t, f, contract), sepaCreditorID))
	}
	rec := applyImport(t, f.traeger, f.office.token, "mandate.csv", row(sepaIBAN, "Eva Beispiel"), nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	require.Equal(t, 1, decodeSchoolPortalData[sepaImportBody](t, rec.Body.Bytes()).Counts["imported"])

	for name, test := range map[string]struct {
		content []byte
		status  string
		problem string
	}{
		"same account":       {row(sepaIBAN, "Eva Beispiel"), "duplicate", ""},
		"another IBAN":       {row(otherIBAN, "Eva Beispiel"), "conflict", "iban:account_differs"},
		"another holder":     {row(sepaIBAN, "Jan Beispiel"), "conflict", "account_holder:account_differs"},
		"another IBAN again": {row("de02 1203 0000 0000 2020 51", "Eva Beispiel"), "conflict", "iban:account_differs"},
	} {
		rec := importForm(t, f.traeger, f.office.token, sepaImportPreviewPath, "mandate.csv", test.content, nil)
		require.Equal(t, http.StatusOK, rec.Code, "%s: %s", name, rec.Body.String())
		assert.NotContains(t, rec.Body.String(), "0532013000", name)
		assert.NotContains(t, rec.Body.String(), "0000202051", name)
		preview := decodeSchoolPortalData[sepaImportBody](t, rec.Body.Bytes())
		require.Len(t, preview.Rows, 1, name)
		assert.Equal(t, test.status, preview.Rows[0].Status, name)
		var problems []string
		for _, problem := range preview.Rows[0].Problems {
			problems = append(problems, problem.Field+":"+problem.Code)
		}
		if test.problem == "" {
			assert.Empty(t, problems, name)
		} else {
			assert.Equal(t, []string{test.problem}, problems, name)
		}
	}
	var mandates int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.sepa_mandates WHERE organization_id = ?`,
		f.office.organization).Scan(context.Background(), &mandates))
	assert.Equal(t, 1, mandates, "nothing is overwritten or added")
}

// The import takes over the preview the office confirmed, not what the
// file would give now: a mandate revoked between preview and import turns a
// conflicting row ready, and the import refuses until the office looked
// again (#6198, like the review of a monthly run).
func TestSEPAMandateImportTakesOverOnlyTheConfirmedPreview(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mandatsimport Vorschau")
	withMandate := careContract(t, f.db, f.office.organization, f.office.schools[0], "Mia", careMonth, sjrServices()...)
	fresh := careContract(t, f.db, f.office.organization, f.office.schools[0], "Noah", careMonth, sjrServices()...)
	saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 14, "missing_mandate": "transfer"})
	stored := createMandate(t, f, mandateBody(withMandate, "SJR-V-1"))
	csv := []byte(strings.Join([]string{
		"Mandatsreferenz;Mandatsdatum;IBAN;Kontoinhaber;E-Mail;Gläubiger-ID;Sequenz",
		fmt.Sprintf("OGS-V-1;01.09.2026;%s;Eva Beispiel;%s;%s;RCUR", sepaIBAN, partyEmail(t, f, withMandate), sepaCreditorID),
		fmt.Sprintf("OGS-V-2;01.09.2026;%s;Eva Beispiel;%s;%s;RCUR", sepaIBAN, partyEmail(t, f, fresh), sepaCreditorID),
	}, "\n"))
	rec := importForm(t, f.traeger, f.office.token, sepaImportPreviewPath, "mandate.csv", csv, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview := decodeSchoolPortalData[sepaImportBody](t, rec.Body.Bytes())
	require.Equal(t, 1, preview.Counts["ready"], "the office confirms one mandate")
	require.Equal(t, 1, preview.Counts["conflict"])
	require.NotEmpty(t, preview.Fingerprint)

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, fmt.Sprintf("%s/%d/revoke", sepaMandatesPath, stored.ID),
		map[string]any{"revoked_on": "2026-10-01"})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = importForm(t, f.traeger, f.office.token, sepaImportPath, "mandate.csv", csv, map[string]string{"fingerprint": preview.Fingerprint})
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	code, _ := registrationRefusalOf(t, rec)
	assert.Equal(t, "parent_billing.sepa_import_changed", code)
	rec = importForm(t, f.traeger, f.office.token, sepaImportPath, "mandate.csv", csv, nil)
	require.Equal(t, http.StatusConflict, rec.Code, "an import without a confirmed preview: %s", rec.Body.String())
	var imported int
	require.NoError(t, f.db.NewRaw(`SELECT count(*) FROM parent_billing.sepa_mandates WHERE organization_id = ? AND source = 'import'`,
		f.office.organization).Scan(context.Background(), &imported))
	assert.Zero(t, imported, "nothing beyond the confirmed preview is taken over")

	rec = applyImport(t, f.traeger, f.office.token, "mandate.csv", csv, nil)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	assert.Equal(t, 2, decodeSchoolPortalData[sepaImportBody](t, rec.Body.Bytes()).Counts["imported"], "after a new look both rows")
}
