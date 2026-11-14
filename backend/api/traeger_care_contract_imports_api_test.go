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

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

const (
	contractImportPath        = "/traeger/contract-imports"
	contractImportPreviewPath = "/traeger/contract-imports/preview"
)

type contractImportBody struct {
	Headers  []string `json:"headers"`
	Unmapped []string `json:"unmapped"`
	Mapping  struct {
		SchoolID   int64  `json:"school_id"`
		ModuleName string `json:"module_name"`
	} `json:"mapping"`
	Schools []struct {
		ID      int64 `json:"id"`
		Modules []struct {
			Name          string `json:"name"`
			HasConditions bool   `json:"has_conditions"`
		} `json:"modules"`
	} `json:"schools"`
	DebtorNumberScope string `json:"debtor_number_scope"`
	Fingerprint       string `json:"fingerprint"`
	Applied           bool   `json:"applied"`
	Counts            struct {
		Ready, Duplicate, Conflict, Invalid, Imported int
		DebtorNumbers                                 int `json:"debtor_numbers"`
	} `json:"counts"`
	Rows []struct {
		Line               int      `json:"line"`
		Status             string   `json:"status"`
		SchoolID           int64    `json:"school_id"`
		ModuleNames        []string `json:"module_names"`
		Weekdays           []string `json:"weekdays"`
		EndsOn             *string  `json:"ends_on"`
		ContractID         int64    `json:"contract_id"`
		DebtorNumber       string   `json:"debtor_number"`
		DebtorNumberStatus string   `json:"debtor_number_status"`
		Problems           []struct {
			Field string `json:"field"`
			Code  string `json:"code"`
		} `json:"problems"`
	} `json:"rows"`
}

// contractImportFile is a file of the school's children in the layout an
// office exports: a row per child with its module, start, payer and debtor
// number.
func contractImportFile(school string, rows ...string) []byte {
	lines := []string{"Vorname Kind;Nachname Kind;Geburtsdatum;Schule;Modul;Vertragsbeginn;Zahler;E-Mail;Debitor-Nr."}
	for _, row := range rows {
		lines = append(lines, strings.ReplaceAll(row, "{school}", school))
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}

func contractImportSchoolName(t *testing.T, f careContractFixture) string {
	t.Helper()
	var name string
	require.NoError(t, f.db.NewRaw(`SELECT name FROM platform.schools WHERE id = ?`, f.school).Scan(context.Background(), &name))
	return name
}

func applyContractImport(t *testing.T, f careContractFixture, token, name string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	rec := importForm(t, f.traeger, token, contractImportPreviewPath, name, content, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	fingerprint := decodeSchoolPortalData[contractImportBody](t, rec.Body.Bytes()).Fingerprint
	return importForm(t, f.traeger, token, contractImportPath, name, content, map[string]string{"fingerprint": fingerprint})
}

// A Träger takes over the children and contracts it had before moto: the
// preview answers every row and stores nothing, the import concludes the
// ready rows on paper with their first version in force, keeps the debtor
// number of the payer and records the file; a second import of the file
// stores nothing twice (#4723).
func TestCareContractImportTakesOverExistingContractsOnce(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertragsimport")
	token := f.office.token
	school := contractImportSchoolName(t, f)
	file := contractImportFile(school,
		"Mia;Beispiel;03.05.2019;{school};Offene Ganztagsschule;01.08.2027;Eva Beispiel;eva.beispiel@example.org;70001",
		"Ben;Beispiel;11.11.2020;{school};Offene Ganztagsschule;01.09.2027;Eva Beispiel;eva.beispiel@example.org;70001",
		"Ole;Anders;;{school};Hort;01.08.2027;Anders;;70003",
		"mia;BEISPIEL;03.05.2019;{school};Offene Ganztagsschule;01.08.2027;Eva Beispiel;eva.beispiel@example.org;70001")

	// Without conditions for the module no row is taken over, and the
	// preview says so.
	rec := importForm(t, f.traeger, token, contractImportPreviewPath, "kinder.csv", file, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview := decodeSchoolPortalData[contractImportBody](t, rec.Body.Bytes())
	require.Empty(t, preview.Unmapped)
	require.Len(t, preview.Rows, 4)
	assert.Equal(t, "module", preview.Rows[0].Problems[0].Field)
	assert.Equal(t, "conditions_missing", preview.Rows[0].Problems[0].Code)
	assert.Equal(t, "payer", preview.DebtorNumberScope)
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)

	rec = importForm(t, f.traeger, token, contractImportPreviewPath, "kinder.csv", file, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	preview = decodeSchoolPortalData[contractImportBody](t, rec.Body.Bytes())
	assert.False(t, preview.Applied)
	assert.Equal(t, 2, preview.Counts.Ready)
	assert.Equal(t, 1, preview.Counts.Invalid)
	assert.Equal(t, 1, preview.Counts.Conflict)
	assert.Equal(t, 2, preview.Counts.DebtorNumbers, "both siblings carry the number of their payer")
	assert.Equal(t, []string{"mon", "tue", "wed"}, preview.Rows[0].Weekdays, "the module's own days")
	assert.Equal(t, "2028-07-31", *preview.Rows[0].EndsOn, "the contract ends with the phase of its module")
	assert.Equal(t, "new", preview.Rows[0].DebtorNumberStatus)
	assert.Equal(t, "unknown", preview.Rows[2].Problems[0].Code)
	assert.Equal(t, "repeated", preview.Rows[3].Problems[0].Code)
	var stored int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM care_contract.contracts WHERE organization_id = ?`, f.office.organization).Scan(context.Background(), &stored))
	assert.Zero(t, stored, "a preview stores nothing")

	// An import with another fingerprint than the preview's is refused.
	rec = importForm(t, f.traeger, token, contractImportPath, "kinder.csv", file, map[string]string{"fingerprint": "stale"})
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "care_contract.import_changed")

	rec = importForm(t, f.traeger, token, contractImportPath, `C:\Export\kinder.csv`, file, map[string]string{"fingerprint": preview.Fingerprint})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	applied := decodeSchoolPortalData[contractImportBody](t, rec.Body.Bytes())
	assert.True(t, applied.Applied)
	assert.Equal(t, 2, applied.Counts.Imported)
	require.Equal(t, "imported", applied.Rows[0].Status)
	contract := applied.Rows[0].ContractID
	require.Positive(t, contract)

	// The imported contract is a contract like any other: active, concluded
	// on paper, its first version in force with the booked days.
	rec = traegerJSON(t, f.traeger, token, http.MethodGet, contractPath(contract, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := decodeSchoolPortalData[careContractDetailBody](t, rec.Body.Bytes())
	assert.Equal(t, "aktiv", detail.Status)
	assert.Equal(t, "wirksam", detail.Version.Status)
	assert.Equal(t, 1, detail.Version.Number)
	assert.Equal(t, "Mia", detail.Version.ChildFirstName)
	assert.Equal(t, "2019-05-03", detail.Version.ChildBirthDate)
	assert.Len(t, detail.Version.Services, 3)
	assert.Equal(t, int64(9500), *detail.Version.PriceCents)
	assert.Equal(t, 1, detail.Version.Terms.NoticeMonths)
	var method, source, file1 string
	var line int
	require.NoError(t, f.db.NewRaw(`SELECT c.conclusion_method, v.source, e.details->>'import_file', (e.details->>'import_line')::int
		FROM care_contract.contracts AS c
		JOIN care_contract.contract_versions AS v ON v.contract_id = c.id
		JOIN care_contract.contract_events AS e ON e.contract_id = c.id AND e.action = 'create'
		WHERE c.id = ?`, contract).Scan(context.Background(), &method, &source, &file1, &line))
	assert.Equal(t, []any{"papier", "import", "kinder.csv", 2}, []any{method, source, file1, line}, "the trail keeps the file without its path")
	var number string
	require.NoError(t, f.db.NewRaw(`SELECT debtor_number FROM parent_billing.family_billing WHERE organization_id = ? AND family_email = ?`,
		f.office.organization, "eva.beispiel@example.org").Scan(context.Background(), &number))
	assert.Equal(t, "70001", number)
	var logged int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM audit.data_access_log WHERE tenant_id = ? AND metadata->>'view' = 'care_contract_import'`,
		f.school).Scan(context.Background(), &logged))
	assert.Equal(t, 1, logged, "the school's access log names the import")

	// The same file again takes over nothing.
	rec = applyContractImport(t, f, token, "kinder.csv", file)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	again := decodeSchoolPortalData[contractImportBody](t, rec.Body.Bytes())
	assert.Zero(t, again.Counts.Imported)
	assert.Equal(t, 2, again.Counts.Duplicate, "both children have their contract")
	assert.Equal(t, contract, again.Rows[0].ContractID)
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM care_contract.contracts WHERE organization_id = ?`, f.office.organization).Scan(context.Background(), &stored))
	assert.Equal(t, 2, stored)

	rec = traegerJSON(t, f.traeger, token, http.MethodGet, contractImportPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	log := decodeSchoolPortalData[[]struct {
		File      string `json:"file"`
		AccountID *int64 `json:"account_id"`
		Contracts int    `json:"contracts"`
	}](t, rec.Body.Bytes())
	require.Len(t, log, 1)
	assert.Equal(t, "kinder.csv", log[0].File)
	assert.Equal(t, 2, log[0].Contracts)
	assert.NotNil(t, log[0].AccountID)
}

// The import needs the right "Verträge verwalten", reads only files that
// are text, and never reaches a school of another Träger.
func TestCareContractImportNeedsTheRightAndStaysWithTheTraeger(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertragsimport Grenze")
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	file := contractImportFile(contractImportSchoolName(t, f),
		"Mia;Beispiel;03.05.2019;{school};Offene Ganztagsschule;01.08.2027;Beispiel;eva.beispiel@example.org;")

	billing := traegerColleague(t, f.reviewFixture, identityaccess.TraegerRoleBilling)
	assert.Equal(t, http.StatusForbidden, importForm(t, f.traeger, billing, contractImportPreviewPath, "kinder.csv", file, nil).Code)
	assert.Equal(t, http.StatusForbidden, importForm(t, f.traeger, billing, contractImportPath, "kinder.csv", file, nil).Code)
	assert.Equal(t, http.StatusForbidden, traegerJSON(t, f.traeger, billing, http.MethodGet, contractImportPath, nil).Code)

	for name, upload := range map[string]struct {
		file    string
		content []byte
		status  int
		code    string
	}{
		"a zip named .csv": {"kinder.csv", []byte("PK\x03\x04\x14\x00\x00\x00binary"), http.StatusBadRequest, "care_contract.import_file_invalid"},
		"a spreadsheet":    {"kinder.xlsx", file, http.StatusBadRequest, "care_contract.import_file_invalid"},
		"a header only":    {"kinder.csv", []byte("Vorname;Nachname\n"), http.StatusBadRequest, "care_contract.import_file_invalid"},
		"too large":        {"kinder.csv", []byte(strings.Repeat("a;b\n", 600_000)), http.StatusRequestEntityTooLarge, "care_contract.import_file_too_large"},
	} {
		rec := importForm(t, f.traeger, f.office.token, contractImportPreviewPath, upload.file, upload.content, nil)
		assert.Equal(t, upload.status, rec.Code, "%s: %s", name, rec.Body.String())
		assert.Contains(t, rec.Body.String(), upload.code, name)
	}
	rec := importForm(t, f.traeger, f.office.token, contractImportPreviewPath, "kinder.csv", file, map[string]string{"mapping": `{"columns":{"school":"Ort"}}`})
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "care_contract.field_invalid")

	// Another Träger knows neither the school by its name nor by its ID,
	// and its log stays empty.
	other := newTraegerOffice(t, f.db, f.traeger, "Träger Vertragsimport Fremd", identityaccess.TraegerRoleAdministration)
	testpkg.OwnTraegerCareContracts(t, f.db, other.organization)
	rec = importForm(t, f.traeger, other.token, contractImportPreviewPath, "kinder.csv", file, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	foreign := decodeSchoolPortalData[contractImportBody](t, rec.Body.Bytes())
	require.Len(t, foreign.Rows, 1)
	assert.Equal(t, "invalid", foreign.Rows[0].Status)
	assert.Equal(t, "unknown", foreign.Rows[0].Problems[0].Code)
	rec = importForm(t, f.traeger, other.token, contractImportPreviewPath, "kinder.csv", file,
		map[string]string{"mapping": fmt.Sprintf(`{"columns":{"child_first_name":"Vorname Kind","child_last_name":"Nachname Kind","effective_from":"Vertragsbeginn","party_last_name":"Zahler","module":"Modul"},"school_id":%d}`, f.school)})
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	require.Equal(t, http.StatusCreated, applyContractImport(t, f, f.office.token, "kinder.csv", file).Code)
	rec = traegerJSON(t, f.traeger, other.token, http.MethodGet, contractImportPath, nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.JSONEq(t, `[]`, string(decodeSchoolPortalData[rawJSON](t, rec.Body.Bytes())))
}

type rawJSON []byte

func (r *rawJSON) UnmarshalJSON(data []byte) error {
	*r = append((*r)[:0], data...)
	return nil
}

func TestCareContractImportLogQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertragsimport Budget")
	token := f.office.token
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	school := contractImportSchoolName(t, f)
	measure := func() testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, contractImportPath, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.traeger, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	importFor := func(children ...string) {
		rows := make([]string, len(children))
		for i, child := range children {
			rows[i] = child + ";Budget;;{school};Offene Ganztagsschule;01.08.2027;Budget;;"
		}
		rec := applyContractImport(t, f, token, children[0]+".csv", contractImportFile(school, rows...))
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		require.Contains(t, rec.Body.String(), fmt.Sprintf(`"imported":%d`, len(children)))
	}
	importFor("Mia")
	first := measure()
	importFor("Noah", "Lia", "Ole")
	assert.Len(t, measure().Statements, len(first.Statements), "the log reads the same statements for any number of imports")
	querybudgets.AssertQueryBudget(t, "api.traeger.care_contract_imports", first)
}
