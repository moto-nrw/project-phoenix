//go:build testhooks

package shifttypes_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The cleanup lookup is a later observation, not a reuse of the live directory.
// A failure on a retired employment must still abort the read transaction.
func TestStaffDocumentReadRetainsRetiredEmploymentFailure(t *testing.T) {
	t.Parallel()
	f := newStaffGrowthFixture(t)
	retired := testpkg.CreateTestStaff(t, f.db, "Retired", "ReadFailure")
	_, err := f.db.ExecContext(testpkg.Ctx(t), "UPDATE users.staff_school_memberships SET deleted_at = NOW() WHERE id = ?", retired.ID)
	require.NoError(t, err)
	_, err = f.db.ExecContext(testpkg.Ctx(t), fmt.Sprintf(`
		CREATE FUNCTION public.reject_retired_profile_read(membership bigint) RETURNS boolean
		LANGUAGE plpgsql VOLATILE AS $body$
		BEGIN
			IF membership = %d THEN RAISE EXCEPTION 'retired employment read refused'; END IF;
			RETURN true;
		END $body$;
		CREATE POLICY reject_retired_profile_read ON users.staff_employment_profiles
		AS RESTRICTIVE FOR SELECT TO phoenix_tenant
		USING (public.reject_retired_profile_read(membership_id));`, retired.ID))
	require.NoError(t, err)
	claims := testutil.DefaultTestClaims()
	claims.ID = int(f.account)
	claims.Permissions = []string{"staff:documents"}
	path := fmt.Sprintf("/staff/%d/documents", f.staff)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+testutil.MintTestJWT(t, claims))
	counter := testpkg.CaptureQueries(t, f.db)
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	counter.Stop()
	t.Logf("status=%d body=%s statements=%q", rec.Code, rec.Body.String(), counter.Queries())
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
	require.JSONEq(t, `{"type":"https://moto-app.de/help/fehlermeldungen#anleitung-unerwarteter-fehler","title":"Internal Server Error","status":500,"detail":"Internal Server Error","instance":"","code":"general.server"}`, rec.Body.String())
	require.NotContains(t, rec.Body.String(), "Staff documents retrieved successfully")
	profiles := 0
	for _, query := range counter.Queries() {
		if strings.Contains(query, `"users"."staff_employment_profiles"`) {
			profiles++
		}
	}
	require.Equal(t, 2, profiles, "the live existence guard and later retired cleanup each observe employment")
}
