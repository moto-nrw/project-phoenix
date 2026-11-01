// Router-level tests for /api/staff/{id}/stammdaten (#1423): the
// non-sensitive sections are HR-file data and sit behind staff:stammdaten /
// time_tracking:manage (deliberately NOT users:read or users:update — see the
// router comment and #2906), the bank & tax section sits exclusively behind
// staff:financial, and the reveal endpoint serves plaintext only after the
// audited POST.
package timetracking

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStammdatenAPI_PermissionSplit(t *testing.T) {
	t.Parallel()

	ctx := setupOverviewAPI(t)
	target := testpkg.CreateTestStaff(t, ctx.tc.db, "Stammdaten", fmt.Sprintf("API-%d", time.Now().UnixNano()))
	base := fmt.Sprintf("/staff/%d/stammdaten", target.ID)

	// Aggregate GET: personnel administrators and time-tracking managers only.
	// users:read (the list tier) must NOT open the HR sections — and neither
	// may users:update, which the plain Betreuer role holds for the child-data
	// surfaces (#2906).
	rec := ctx.get(base, "users:read")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = ctx.get(base, "users:read", "users:update", "users:absence")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = ctx.get(base, "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = ctx.get(base, "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = ctx.get(base, "visits:read")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	// Section writes need staff:stammdaten.
	personBody := `{"first_name":"Stammdaten","last_name":"Neu","birthday":"1991-06-02","gender":"diverse","note":"Test"}`
	rec = ctx.put(base+"/person", personBody, "users:read")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = ctx.put(base+"/person", personBody, "users:read", "users:update")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = ctx.put(base+"/person", personBody, "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Every section write is closed to the Betreuer tier (#2906).
	for _, section := range []string{"kontakt", "arbeitsvertrag", "qualifikationen"} {
		rec = ctx.put(base+"/"+section, `{"note":"Test"}`, "users:read", "users:update")
		require.Equal(t, http.StatusForbidden, rec.Code, section+": "+rec.Body.String())
	}

	// The financial section never opens for the directory or personnel tiers.
	rec = ctx.get(base+"/bank-steuer", "users:read", "staff:stammdaten", "time_tracking:manage")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = ctx.put(base+"/bank-steuer", `{"iban":"DE89370400440532013000"}`, "staff:stammdaten")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = ctx.post(base+"/bank-steuer/reveal", `{}`, "time_tracking:manage")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	// staff:financial reads and writes it.
	rec = ctx.put(base+"/bank-steuer", `{"iban":"DE89 3704 0044 0532 0130 00","tax_id":"12345678911","social_security_number":"65170839J003","note":"Lohnbüro"}`, "staff:financial")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = ctx.get(base+"/bank-steuer", "staff:financial")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"iban_masked":"•••• 3000"`)
	assert.NotContains(t, rec.Body.String(), "DE89370400440532013000", "masked read must not leak the IBAN")
	assert.NotContains(t, rec.Body.String(), "12345678911", "masked read must not leak the Steuer-ID")

	rec = ctx.post(base+"/bank-steuer/reveal", `{}`, "staff:financial")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"iban":"DE89370400440532013000"`)
	assert.Contains(t, rec.Body.String(), `"social_security_number":"65170839J003"`)
}

func TestStammdatenAPI_FinancialWriteAllowsAccountWithoutStaffMapping(t *testing.T) {
	t.Parallel()

	ctx := setupOverviewAPI(t)
	target := testpkg.CreateTestStaff(t, ctx.tc.db, "Stammdaten", fmt.Sprintf("Payroll-%d", time.Now().UnixNano()))
	account := testpkg.CreateTestAccount(t, ctx.tc.db, fmt.Sprintf("payroll-%d@example.test", time.Now().UnixNano()))

	claims := testutil.DefaultTestClaims()
	claims.ID = int(account.ID)
	claims.Permissions = []string{"staff:financial"}
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/staff/%d/stammdaten/bank-steuer", target.ID), bytes.NewBufferString(`{"iban":"DE89370400440532013000"}`))
	req.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	ctx.tc.router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestStammdatenAPI_WireShapeAndValidation(t *testing.T) {
	t.Parallel()

	ctx := setupOverviewAPI(t)
	target := testpkg.CreateTestStaff(t, ctx.tc.db, "Stammdaten", fmt.Sprintf("Wire-%d", time.Now().UnixNano()))
	base := fmt.Sprintf("/staff/%d/stammdaten", target.ID)

	rec := ctx.put(base+"/kontakt", `{"address_street":"Musterweg 1","address_postal_code":"48143","address_city":"Münster","phone":"+49 251 1","email":"k@example.com","emergency_contact_name":"Erika Muster","emergency_contact_phone":"+49 170 1","note":"Erstpflege"}`, "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = ctx.put(base+"/arbeitsvertrag", `{"entry_date":"2024-08-01","contract_end_date":null,"probation_end_date":"2025-01-31","weekly_hours":29.5,"employment_type":"part_time","note":"Vertrag"}`, "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = ctx.put(base+"/qualifikationen", `{"qualifikationen":[{"name":"Erste-Hilfe-Kurs","acquired_on":"2023-03-10","expires_on":"2026-03-10"},{"name":"Schwimmschein","acquired_on":null,"expires_on":null}],"note":"Nachweise"}`, "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = ctx.get(base, "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	body := rec.Body.String()
	for _, fragment := range []string{
		`"address_street":"Musterweg 1"`,
		`"entry_date":"2024-08-01"`,
		`"probation_end_date":"2025-01-31"`,
		`"weekly_hours":29.5`,
		`"employment_type":"part_time"`,
		`"name":"Erste-Hilfe-Kurs"`,
		`"expires_on":"2026-03-10"`,
	} {
		assert.Contains(t, body, fragment)
	}
	assert.NotContains(t, body, `"iban"`, "the aggregate GET must not expose financial fields")

	// Validation → 400 with the stable code.
	rec = ctx.put(base+"/person", `{"first_name":"","last_name":"X"}`, "staff:stammdaten")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	rec = ctx.put(base+"/arbeitsvertrag", `{"weekly_hours":95}`, "staff:stammdaten")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"workforce.stammdaten_invalid"`)
	rec = ctx.put(base+"/bank-steuer", `{"iban":"DE00123"}`, "staff:financial")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"workforce.stammdaten_invalid"`)
}

func TestStammdatenAPI_RejectsDuplicateQualificationIDs(t *testing.T) {
	t.Parallel()

	ctx := setupOverviewAPI(t)
	target := testpkg.CreateTestStaff(t, ctx.tc.db, "Stammdaten", fmt.Sprintf("Duplicate-%d", time.Now().UnixNano()))
	base := fmt.Sprintf("/staff/%d/stammdaten", target.ID)

	rec := ctx.put(base+"/qualifikationen", `{"qualifikationen":[{"name":"Erste Hilfe"}],"note":"Nachweis"}`, "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var listed struct {
		Data struct {
			Qualifikationen []struct {
				ID string `json:"id"`
			} `json:"qualifikationen"`
		} `json:"data"`
	}
	rec = ctx.get(base, "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &listed))
	require.Len(t, listed.Data.Qualifikationen, 1)

	rec = ctx.put(base+"/qualifikationen", fmt.Sprintf(`{"qualifikationen":[{"id":%q,"name":"Erste Hilfe"},{"id":%q,"name":"Erste Hilfe"}],"note":"Nachweis"}`, listed.Data.Qualifikationen[0].ID, listed.Data.Qualifikationen[0].ID), "staff:stammdaten")
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "workforce.stammdaten_invalid")
}

func TestStammdatenAPIUsesTheCurrentQualificationTypeName(t *testing.T) {
	t.Parallel()

	ctx := setupOverviewAPI(t)
	target := testpkg.CreateTestStaff(t, ctx.tc.db, "Nachweis", fmt.Sprintf("Aktuell-%d", time.Now().UnixNano()))
	base := fmt.Sprintf("/staff/%d/stammdaten", target.ID)

	rec := ctx.get("/staff/qualification-types", "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var listed qualificationTypesBody
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &listed))
	require.NotEmpty(t, listed.Data)
	typeID := listed.Data[0].ID

	rec = ctx.put(base+"/qualifikationen", fmt.Sprintf(`{"qualifikationen":[{"type_id":%q,"name":"Alte Erste Hilfe"}],"note":"Nachweis"}`, typeID), "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = ctx.put("/staff/qualification-types/"+typeID, `{"name":"Erste Hilfe neu","validity_months":24,"reminder_lead_weeks":8,"mandatory":true,"archived":false}`, "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = ctx.get(base, "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"name":"Erste Hilfe neu"`)
}

// A retired membership remains readable through the same permission gates,
// while direct writes must fail before they can alter its retained history.
func TestFormerEmploymentPersonnelAndTimesAreReadOnly(t *testing.T) {
	t.Parallel()
	ctx := setupOverviewAPI(t)
	target := testpkg.CreateTestStaff(t, ctx.tc.db, "Former", "Employment")
	base := fmt.Sprintf("/staff/%d", target.ID)
	rec := ctx.put(base+"/stammdaten/kontakt", `{"phone":"0123456789","note":"Kontakt"}`, "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = ctx.put(base+"/stammdaten/qualifikationen", `{"qualifikationen":[{"name":"Erste Hilfe"}],"note":"Nachweis"}`, "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	day := testpkg.Date(2026, time.July, 1)
	insertWorkSession(t, ctx.tc, target.ID, day.String(), day.BerlinMidnight().Add(8*time.Hour), day.BerlinMidnight().Add(12*time.Hour))
	var sessionID int64
	require.NoError(t, ctx.tc.db.NewRaw("SELECT id FROM active.work_sessions WHERE staff_id = ?", target.ID).Scan(testpkg.Ctx(t), &sessionID))
	_, err := ctx.tc.db.ExecContext(testpkg.Ctx(t), "UPDATE users.staff_school_memberships SET deleted_at = NOW() WHERE id = ?", target.ID)
	require.NoError(t, err)

	rec = ctx.get(base+"/stammdaten", "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "0123456789")
	assert.Contains(t, rec.Body.String(), "Erste Hilfe")
	rec = ctx.get(base+"/stammdaten", "users:read")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = ctx.get(base+"/stammdaten/bank-steuer", "staff:financial")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = ctx.get(base+"/time-tracking/history?from=2026-07-01&to=2026-07-31", "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"check_in_time":"2026-07-01`)
	rec = ctx.get(base+"/time-tracking/history?from=2026-07-01&to=2026-07-31", "users:read")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	for _, section := range []string{"kontakt", "arbeitsvertrag", "qualifikationen"} {
		rec = ctx.put(base+"/stammdaten/"+section, `{"note":"Änderung"}`, "staff:stammdaten")
		require.Equal(t, http.StatusNotFound, rec.Code, section+": "+rec.Body.String())
	}
	rec = ctx.put(base+"/stammdaten/person", `{"first_name":"Changed","last_name":"Person","note":"Änderung"}`, "staff:stammdaten")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	rec = ctx.put(base+"/stammdaten/bank-steuer", `{"note":"Änderung"}`, "staff:financial")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	for _, suffix := range []string{"time-tracking/sessions", "time-tracking/adjustments", "time-tracking/reset", "time-tracking/opening", "time-tracking/month-close/reopen", "absences"} {
		rec = ctx.post(base+"/"+suffix, `{}`, "time_tracking:manage")
		require.Equal(t, http.StatusNotFound, rec.Code, suffix+": "+rec.Body.String())
	}
	rec = ctx.put(fmt.Sprintf("%s/time-tracking/sessions/%d", base, sessionID), `{"notes":"Änderung"}`, "time_tracking:manage")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	rec = ctx.get(base+"/stammdaten", "staff:stammdaten")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "0123456789", "failed writes preserve the retained contact")
	assert.Contains(t, rec.Body.String(), "Erste Hilfe", "failed writes preserve qualifications")

	_, err = ctx.tc.db.ExecContext(testpkg.Ctx(t), "UPDATE users.staff_school_memberships SET personal_data_erased_at = NOW() WHERE id = ?", target.ID)
	require.NoError(t, err)
	rec = ctx.get(base+"/stammdaten", "staff:stammdaten")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	rec = ctx.get(base+"/time-tracking/history?from=2026-07-01&to=2026-07-31", "time_tracking:manage")
	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}
