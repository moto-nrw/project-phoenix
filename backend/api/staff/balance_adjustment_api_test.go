// Router-level tests for the Stundenkonto lifecycle endpoints (#1420):
// payout/comp-time adjustments, the school-year reset with its double-click
// guard, and their effect on the month-summary carry chain.
package staff_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBalanceAdjustmentAPI(t *testing.T) {
	tc := setupTestContext(t)
	suffix := time.Now().UnixNano()

	editorPerson, editorAccount := testpkg.CreateTestPersonWithAccount(t, tc.db, "Balance", fmt.Sprintf("Editor-%d", suffix))
	editorStaff := testpkg.CreateTestStaffForPerson(t, tc.db, editorPerson.ID)
	subject := testpkg.CreateTestStaff(t, tc.db, "Balance", fmt.Sprintf("Subject-%d", suffix))
	t.Cleanup(func() {
		_, _ = tc.db.Exec("DELETE FROM active.staff_balance_adjustments WHERE staff_id = ?", subject.ID)
		testpkg.CleanupStaffFixtures(t, tc.db, subject.ID)
		testpkg.CleanupStaffFixtures(t, tc.db, editorStaff.ID)
		testpkg.CleanupTableRecords(t, tc.db, "users.persons", editorPerson.ID)
		testpkg.CleanupTableRecords(t, tc.db, "auth.accounts", editorAccount.ID)
	})

	claims := testutil.DefaultTestClaims()
	claims.ID = int(editorAccount.ID)
	claims.Permissions = []string{"time_tracking:manage"}
	token := testutil.MintTestJWT(t, claims)

	do := func(method, path, body, tok string) *httptest.ResponseRecorder {
		var reader *strings.Reader
		if body == "" {
			reader = strings.NewReader("{}")
		} else {
			reader = strings.NewReader(body)
		}
		req := httptest.NewRequest(method, path, reader)
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		tc.router.ServeHTTP(rec, req)
		return rec
	}

	today := timezone.TodayDate()
	adjustmentsPath := fmt.Sprintf("/staff/%d/time-tracking/adjustments", subject.ID)
	resetPath := fmt.Sprintf("/staff/%d/time-tracking/reset", subject.ID)
	summaryPath := fmt.Sprintf("/staff/%d/time-tracking/month-summary?year=%d&month=%d", subject.ID, today.Year, int(today.Month))
	listPath := fmt.Sprintf("%s?from=%d-01-01&to=%d-12-31", adjustmentsPath, today.Year, today.Year)
	var payoutID int64

	t.Run("requires manage permission", func(t *testing.T) {
		ownToken := authToken(t, "time_tracking:own")
		rec := do(http.MethodPost, adjustmentsPath, `{"type":"payout","minutes_delta":-60,"effective_date":"2026-01-01","note":"x"}`, ownToken)
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	})

	t.Run("missing and cross-tenant staff are not addressable", func(t *testing.T) {
		missingStaffID := subject.ID + 9_000_000_000
		missingAdjustmentsPath := fmt.Sprintf("/staff/%d/time-tracking/adjustments", missingStaffID)
		missingResetPath := fmt.Sprintf("/staff/%d/time-tracking/reset", missingStaffID)

		rec := do(http.MethodGet, missingAdjustmentsPath+"?from=2026-01-01&to=2026-12-31", "", token)
		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
		rec = do(http.MethodPost, missingAdjustmentsPath,
			fmt.Sprintf(`{"type":"payout","minutes_delta":-60,"effective_date":"%s","note":"x"}`, today), token)
		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
		rec = do(http.MethodDelete, missingAdjustmentsPath+"/1", "", token)
		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
		rec = do(http.MethodPost, missingResetPath,
			fmt.Sprintf(`{"effective_date":"%s","carryover_minutes":0,"note":"x"}`, today), token)
		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

		foreignTenantID := testpkg.UniqueTestTenantID(t)
		testpkg.EnsureTestTenant(t, tc.db, foreignTenantID)
		foreignStaff := testpkg.CreateTestStaffForTenant(
			t,
			tc.db,
			foreignTenantID,
			"Foreign",
			fmt.Sprintf("Subject-%d", suffix),
		)
		t.Cleanup(func() {
			testpkg.CleanupStaffFixtures(t, tc.db, foreignStaff.ID)
			_, err := tc.db.Exec("DELETE FROM platform.schools WHERE id = ?", foreignTenantID)
			require.NoError(t, err)
			_, err = tc.db.Exec("DELETE FROM platform.organizations WHERE id = ?", foreignTenantID)
			require.NoError(t, err)
		})

		foreignPath := fmt.Sprintf("/staff/%d/time-tracking/adjustments", foreignStaff.ID)
		rec = do(http.MethodGet, foreignPath+"?from=2026-01-01&to=2026-12-31", "", token)
		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
		rec = do(http.MethodPost, foreignPath,
			fmt.Sprintf(`{"type":"payout","minutes_delta":-60,"effective_date":"%s","note":"x"}`, today), token)
		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	})

	t.Run("create validation", func(t *testing.T) {
		rec := do(http.MethodPost, adjustmentsPath,
			fmt.Sprintf(`{"type":"payout","minutes_delta":60,"effective_date":"%s","note":"x"}`, today), token)
		require.Equal(t, http.StatusBadRequest, rec.Code, "positive delta must be rejected: %s", rec.Body.String())

		rec = do(http.MethodPost, adjustmentsPath,
			fmt.Sprintf(`{"type":"payout","minutes_delta":-60,"effective_date":"%s","note":"  "}`, today), token)
		require.Equal(t, http.StatusBadRequest, rec.Code, "blank note must be rejected: %s", rec.Body.String())

		rec = do(http.MethodPost, adjustmentsPath,
			fmt.Sprintf(`{"type":"reset","minutes_delta":-60,"effective_date":"%s","note":"x"}`, today), token)
		require.Equal(t, http.StatusBadRequest, rec.Code, "reset type must not be creatable directly: %s", rec.Body.String())

		rec = do(http.MethodPost, adjustmentsPath,
			fmt.Sprintf(`{"type":"payout","minutes_delta":-60001,"effective_date":"%s","note":"x"}`, today), token)
		require.Equal(t, http.StatusBadRequest, rec.Code, "payouts above 1,000 hours must be rejected: %s", rec.Body.String())

		rec = do(http.MethodPost, resetPath,
			fmt.Sprintf(`{"effective_date":"%s","carryover_minutes":-1,"note":"x"}`, today), token)
		require.Equal(t, http.StatusBadRequest, rec.Code, "negative carryover must be rejected: %s", rec.Body.String())

		rec = do(http.MethodPost, resetPath,
			fmt.Sprintf(`{"effective_date":"%s","carryover_minutes":600001,"note":"x"}`, today), token)
		require.Equal(t, http.StatusBadRequest, rec.Code, "carryover above 10,000 hours must be rejected: %s", rec.Body.String())
	})

	t.Run("payout flows into month summary", func(t *testing.T) {
		rec := do(http.MethodPost, adjustmentsPath,
			fmt.Sprintf(`{"type":"payout","minutes_delta":-120,"effective_date":"%s","note":"Auszahlung Juli"}`, today), token)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		var created struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
		payoutID = created.Data.ID

		rec = do(http.MethodGet, listPath, "", token)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"payout"`)

		rec = do(http.MethodGet, summaryPath, "", token)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"adjustment_minutes":-120`)
	})

	t.Run("reset zeroes the closing balance and conflicts on repeat", func(t *testing.T) {
		// The subject has no schedule and no sessions, so before the reset the
		// closing balance is exactly the payout: -120.
		rec := do(http.MethodPost, resetPath,
			fmt.Sprintf(`{"effective_date":"%s","carryover_minutes":0,"note":"Schuljahresende"}`, today), token)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"minutes_delta":120`, "reset delta must invert the -120 balance")

		rec = do(http.MethodGet, summaryPath, "", token)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"closing_balance_minutes":0`)

		rec = do(http.MethodPost, resetPath,
			fmt.Sprintf(`{"effective_date":"%s","carryover_minutes":0,"note":"Doppelklick"}`, today), token)
		require.Equal(t, http.StatusConflict, rec.Code, "second reset for the same date must conflict: %s", rec.Body.String())

		rec = do(http.MethodDelete, fmt.Sprintf("%s/%d", adjustmentsPath, payoutID), "", token)
		require.Equal(t, http.StatusConflict, rec.Code, "an adjustment used by a later reset must not be deleted: %s", rec.Body.String())
	})

	t.Run("future reset rejected", func(t *testing.T) {
		rec := do(http.MethodPost, resetPath,
			fmt.Sprintf(`{"effective_date":"%s","carryover_minutes":0,"note":"x"}`, today.AddDays(7)), token)
		require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	})

	t.Run("delete removes the adjustment", func(t *testing.T) {
		rec := do(http.MethodPost, adjustmentsPath,
			fmt.Sprintf(`{"type":"comp_time","minutes_delta":-30,"effective_date":"%s","note":"Tippfehler"}`, today), token)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		var created struct {
			Data struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

		rec = do(http.MethodDelete, fmt.Sprintf("%s/%d", adjustmentsPath, created.Data.ID), "", token)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

		rec = do(http.MethodDelete, fmt.Sprintf("%s/%d", adjustmentsPath, created.Data.ID), "", token)
		require.Equal(t, http.StatusNotFound, rec.Code, "second delete must 404: %s", rec.Body.String())
	})
}
