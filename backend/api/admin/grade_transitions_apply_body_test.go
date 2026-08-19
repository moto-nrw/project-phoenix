// Apply-body handling for the grade-transition apply endpoint (#405 review).
//
// The body is optional — an absent or empty one means "no preview to confirm
// against" and skips the stale-preview check. A MALFORMED body must not be
// treated that way: swallowing the decode error left expected_fingerprint empty,
// which ApplyChecked reads as opting out of the check, so the destructive
// transition ran unguarded on a cohort that may have changed since the admin
// confirmed it.
package admin_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/models/education"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestGradeTransitionResource_Apply_BodyHandling(t *testing.T) {
	tc := setupTestContext(t)

	account := testpkg.CreateTestAccount(t, tc.db, "apply-body-test@example.com")

	router := tc.resource.Router()
	token := mintAdminToken(t, account.ID)

	// newApplyRequest posts a RAW body so a deliberately broken payload survives
	// to the handler (the shared helper marshals its argument and would reject
	// invalid JSON in the test itself).
	newApplyRequest := func(transitionID int64, rawBody string) *http.Request {
		url := fmt.Sprintf("/%d/apply", transitionID)
		var req *http.Request
		if rawBody == "" {
			req = httptest.NewRequest(http.MethodPost, url, nil)
		} else {
			req = httptest.NewRequest(http.MethodPost, url, strings.NewReader(rawBody))
		}
		req.Header.Set("Content-Type", "application/json")
		testutil.WithJWTBearer(token)(req)
		return req
	}

	newTransition := func(t *testing.T) int64 {
		t.Helper()
		suffix := uuid.Must(uuid.NewV4()).String()[:8]
		fromClass := fmt.Sprintf("1body-%s", suffix)
		toClass := fmt.Sprintf("2body-%s", suffix)

		_ = testpkg.CreateTestStudent(t, tc.db, "ApplyBody", "Test", fromClass)

		transition := testpkg.CreateTestGradeTransition(t, tc.db, "2025-2026", account.ID)
		testpkg.CreateTestGradeTransitionMapping(t, tc.db, transition.ID, fromClass, &toClass)
		return transition.ID
	}

	t.Run("truncated JSON body is rejected and the transition stays applicable", func(t *testing.T) {
		transitionID := newTransition(t)

		rr := testutil.ExecuteRequest(router, newApplyRequest(transitionID, `{"expected_fingerprint": `))
		require.Equal(t, http.StatusBadRequest, rr.Code, "malformed body must not apply the transition")

		// Still in draft: nothing was applied behind the broken request.
		var status string
		require.NoError(t, tc.db.NewSelect().
			TableExpr("education.grade_transitions").
			Column("status").
			Where("id = ?", transitionID).
			Scan(testpkg.TenantContext(1), &status))
		assert.Equal(t, education.TransitionStatusDraft, status)
	})

	t.Run("wrong-typed fingerprint field is rejected", func(t *testing.T) {
		transitionID := newTransition(t)

		rr := testutil.ExecuteRequest(router, newApplyRequest(transitionID, `{"expected_fingerprint": 42}`))
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("empty body still applies (no preview to confirm)", func(t *testing.T) {
		transitionID := newTransition(t)

		rr := testutil.ExecuteRequest(router, newApplyRequest(transitionID, ""))
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})

	t.Run("empty JSON object still applies (no preview to confirm)", func(t *testing.T) {
		transitionID := newTransition(t)

		rr := testutil.ExecuteRequest(router, newApplyRequest(transitionID, `{}`))
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	})
}
