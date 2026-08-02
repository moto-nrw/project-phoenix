// Router-level tests for /api/staff/{id}/stammdaten (#1423): the
// non-sensitive sections share the directory tier (users:read/users:update),
// the bank & tax section sits exclusively behind staff:financial, and the
// reveal endpoint serves plaintext only after the audited POST.
package staff_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStammdatenAPI_PermissionSplit(t *testing.T) {
	ctx := setupOverviewAPI(t)
	target := testpkg.CreateTestStaff(t, ctx.tc.db, "Stammdaten", fmt.Sprintf("API-%d", time.Now().UnixNano()))
	base := fmt.Sprintf("/staff/%d/stammdaten", target.ID)

	// Aggregate GET: directory read or time-tracking manager.
	rec := ctx.get(base, "users:read")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = ctx.get(base, "time_tracking:manage")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = ctx.get(base, "visits:read")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	// Section writes need users:update.
	personBody := `{"first_name":"Stammdaten","last_name":"Neu","birthday":"1991-06-02","gender":"diverse","note":"Test"}`
	rec = ctx.put(base+"/person", personBody, "users:read")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = ctx.put(base+"/person", personBody, "users:update")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// The financial section never opens for the directory tiers.
	rec = ctx.get(base+"/bank-steuer", "users:read", "users:update", "time_tracking:manage")
	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = ctx.put(base+"/bank-steuer", `{"iban":"DE89370400440532013000"}`, "users:update")
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

func TestStammdatenAPI_WireShapeAndValidation(t *testing.T) {
	ctx := setupOverviewAPI(t)
	target := testpkg.CreateTestStaff(t, ctx.tc.db, "Stammdaten", fmt.Sprintf("Wire-%d", time.Now().UnixNano()))
	base := fmt.Sprintf("/staff/%d/stammdaten", target.ID)

	rec := ctx.put(base+"/kontakt", `{"address_street":"Musterweg 1","address_postal_code":"48143","address_city":"Münster","phone":"+49 251 1","email":"k@example.com","emergency_contact_name":"Erika Muster","emergency_contact_phone":"+49 170 1","note":"Erstpflege"}`, "users:update")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = ctx.put(base+"/arbeitsvertrag", `{"entry_date":"2024-08-01","contract_end_date":null,"probation_end_date":"2025-01-31","weekly_hours":29.5,"employment_type":"part_time","note":"Vertrag"}`, "users:update")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = ctx.put(base+"/qualifikationen", `{"qualifikationen":[{"name":"Erste-Hilfe-Kurs","acquired_on":"2023-03-10","expires_on":"2026-03-10"},{"name":"Schwimmschein","acquired_on":null,"expires_on":null}],"note":"Nachweise"}`, "users:update")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	rec = ctx.get(base, "users:read")
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
	rec = ctx.put(base+"/person", `{"first_name":"","last_name":"X"}`, "users:update")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	rec = ctx.put(base+"/arbeitsvertrag", `{"weekly_hours":95}`, "users:update")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"stammdaten_invalid"`)
	rec = ctx.put(base+"/bank-steuer", `{"iban":"DE00123"}`, "staff:financial")
	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"stammdaten_invalid"`)
}
