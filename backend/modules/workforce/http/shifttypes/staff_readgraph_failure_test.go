//go:build testhooks

package shifttypes_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moto-ogs/project-phoenix/api/testutil"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The cleanup lookup is a later observation, not a reuse of the live directory.
// E1 (#5786): an unused retired Employment read must not abort the transaction.
func TestStaffDocumentReadIgnoresUnusedRetiredEmploymentFailure(t *testing.T) {
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
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "Staff documents retrieved successfully")
	require.NotContains(t, rec.Body.String(), "general.server")
	profiles := 0
	for _, query := range counter.Queries() {
		if strings.Contains(query, `"users"."staff_employment_profiles"`) {
			profiles++
		}
	}
	require.Equal(t, 1, profiles, "the live guard still reads Employment; ID-only cleanup does not")
}
