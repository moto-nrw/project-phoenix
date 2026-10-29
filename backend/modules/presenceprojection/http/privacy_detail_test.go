package presenceprojectionhttp_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type failingPrivacyReader struct{ calls int }

func (f *failingPrivacyReader) ListPrivacyConsents(context.Context, int64) ([]studentpresence.PrivacyConsent, error) {
	f.calls++
	return nil, errors.New("consent storage unavailable")
}

func TestStudentDetailPrivacyConsent(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t)
	consent := testpkg.CreateTestPrivacyConsent(t, tc.db, "detail")
	path := fmt.Sprintf("/%d?include_privacy_consent=true", consent.StudentID)
	read := func(path string) string {
		t.Helper()
		rr := authExec(t, tc, testutil.NewRequest("GET", path, nil), testutil.AdminTestClaims(1), []string{"admin:*"})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		return rr.Body.String()
	}
	require.Contains(t, read(path), `"privacy_consent":{"accepted":true,"data_retention_days":30}`)
	child := testpkg.CreateTestStudent(t, tc.db, "No", "Consent", "1a")
	require.NoError(t, tc.settings().SetValue(testpkg.Ctx(t), "gdpr.privacy_consent_retention_days", 12, nil, nil))
	require.Contains(t, read(fmt.Sprintf("/%d?include_privacy_consent=true", child.ID)), `"privacy_consent":{"accepted":false,"data_retention_days":12}`)
	require.NotContains(t, read(fmt.Sprintf("/%d", child.ID)), `"privacy_consent"`)

	failure := &failingPrivacyReader{}
	tc.resource.PrivacyConsents = failure
	rr := authExec(t, tc, testutil.NewRequest("GET", path, nil), testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusInternalServerError, rr.Code)
	require.Equal(t, 1, failure.calls)
	// A missing child keeps priority and never reaches the failing consent port.
	rr = authExec(t, tc, testutil.NewRequest("GET", "/999999999?include_privacy_consent=true", nil), testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusNotFound, rr.Code)
	require.Equal(t, 1, failure.calls)

	otherTenant, _ := testpkg.CreateTestTenant(t, tc.db)
	foreignChild := testpkg.CreateTestStudentForTenant(t, tc.db, otherTenant, "Foreign", "Child", "1a")
	rr = authExec(t, tc, testutil.NewRequest("GET", fmt.Sprintf("/%d?include_privacy_consent=true", foreignChild.ID), nil), testutil.AdminTestClaims(1), []string{"admin:*"})
	require.Equal(t, http.StatusNotFound, rr.Code)
	require.Equal(t, 1, failure.calls)

	// users:read alone grants no individual access. Consent remains at the old
	// forbidden-read defaults, even with a configured default of 12 and port down.
	account := testpkg.CreateTestAccount(t, tc.db, "detail-no-staff@example.com")
	rr = authExec(t, tc, testutil.NewRequest("GET", path, nil), testutil.TeacherTestClaims(int(account.ID)), []string{"users:read"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Contains(t, rr.Body.String(), `"has_full_access":false`)
	require.Contains(t, rr.Body.String(), `"privacy_consent":{"accepted":false,"data_retention_days":30}`)
	require.Equal(t, 1, failure.calls)
	rr = authExec(t, tc, testutil.NewRequest("GET", path, nil), testutil.TeacherTestClaims(int(account.ID)), []string{"students:read"})
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	require.Contains(t, rr.Body.String(), `"privacy_consent":{"accepted":false,"data_retention_days":30}`)
	require.Equal(t, 1, failure.calls)
}
