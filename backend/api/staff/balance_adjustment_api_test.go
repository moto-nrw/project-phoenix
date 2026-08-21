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
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBalanceAdjustmentAPI(t *testing.T) {
	t.Parallel()

	tc := setupTestContext(t)
	suffix := time.Now().UnixNano()

	editorPerson, editorAccount := testpkg.CreateTestPersonWithAccount(t, tc.db, "Balance", fmt.Sprintf("Editor-%d", suffix))
	testpkg.CreateTestStaffForPerson(t, tc.db, editorPerson.ID)
	subject := testpkg.CreateTestStaff(t, tc.db, "Balance", fmt.Sprintf("Subject-%d", suffix))
	t.Cleanup(func() {
		_, _ = tc.db.Exec("DELETE FROM active.staff_balance_adjustments WHERE staff_id = ?", subject.ID)
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
	resetDate := today.AddDays(-1)
	adjustmentsPath := fmt.Sprintf("/staff/%d/time-tracking/adjustments", subject.ID)
	resetPath := fmt.Sprintf("/staff/%d/time-tracking/reset", subject.ID)
	// Anchor summary and list windows to resetDate, not today: on the 1st of a
	// month (or Jan 1) the adjustments land in the previous month/year and a
	// today-based window would miss them.
	summaryPath := fmt.Sprintf("/staff/%d/time-tracking/month-summary?year=%d&month=%d", subject.ID, resetDate.Year, int(resetDate.Month))
	listPath := fmt.Sprintf("%s?from=%d-01-01&to=%d-12-31", adjustmentsPath, resetDate.Year, today.Year)
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

	t.Run("payout cannot overdraw the account", func(t *testing.T) {
		rec := do(http.MethodPost, adjustmentsPath,
			fmt.Sprintf(`{"type":"payout","minutes_delta":-1,"effective_date":"%s","note":"Ungedeckter Abzug"}`, resetDate), token)
		require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"code":"balance_adjustment_exceeds_balance"`)
	})

	// Accrue four hours before exercising payout and reset behavior. The source
	// issue permits paying out plus-hours; an empty account is not a valid
	// payout fixture.
	tenantID := testpkg.Tenant(t)
	// Adjustments are effective at the start of their date, so fund the
	// payout with work completed on the preceding day.
	accrualDate := resetDate.AddDays(-1)
	checkIn := time.Date(accrualDate.Year, accrualDate.Month, accrualDate.Day, 8, 0, 0, 0, time.UTC)
	checkOut := checkIn.Add(4 * time.Hour)
	accrualSession := &activeModels.WorkSession{
		StaffID:      subject.ID,
		Date:         accrualDate,
		Status:       activeModels.WorkSessionStatusPresent,
		Source:       activeModels.WorkSessionSourceApp,
		CheckInTime:  checkIn,
		CheckOutTime: &checkOut,
		CreatedBy:    subject.ID,
	}
	accrualSession.SetTenantID(tenantID)
	_, err := tc.db.NewInsert().Model(accrualSession).Exec(testpkg.TenantContext(tenantID))
	require.NoError(t, err)

	t.Run("payout flows into month summary", func(t *testing.T) {
		rec := do(http.MethodPost, adjustmentsPath,
			fmt.Sprintf(`{"type":"payout","minutes_delta":-120,"effective_date":"%s","note":"Auszahlung Juli"}`, resetDate), token)
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
		// The subject accrued 240 minutes and paid out 120, so the reset must
		// remove the remaining positive 120-minute balance.
		rec := do(http.MethodPost, resetPath,
			fmt.Sprintf(`{"effective_date":"%s","carryover_minutes":0,"note":"Schuljahresende"}`, resetDate), token)
		require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"minutes_delta":-120`, "reset delta must invert the positive 120-minute balance")

		rec = do(http.MethodGet, summaryPath, "", token)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"closing_balance_minutes":0`)

		rec = do(http.MethodPost, resetPath,
			fmt.Sprintf(`{"effective_date":"%s","carryover_minutes":0,"note":"Doppelklick"}`, resetDate), token)
		require.Equal(t, http.StatusConflict, rec.Code, "second reset for the same date must conflict: %s", rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"code":"balance_already_reset"`)

		rec = do(http.MethodDelete, fmt.Sprintf("%s/%d", adjustmentsPath, payoutID), "", token)
		require.Equal(t, http.StatusConflict, rec.Code, "an adjustment used by a later reset must not be deleted: %s", rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"code":"dependent_balance_reset"`)
	})

	t.Run("writes before an existing reset conflict", func(t *testing.T) {
		rec := do(http.MethodPost, adjustmentsPath,
			fmt.Sprintf(`{"type":"comp_time","minutes_delta":-30,"effective_date":"%s","note":"Nachträglich"}`, resetDate), token)
		require.Equal(t, http.StatusConflict, rec.Code, "same-day writes after a reset must conflict: %s", rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"code":"dependent_balance_reset"`)

		rec = do(http.MethodPost, resetPath,
			fmt.Sprintf(`{"effective_date":"%s","carryover_minutes":0,"note":"Früherer Reset"}`, resetDate.AddDays(-1)), token)
		require.Equal(t, http.StatusConflict, rec.Code, "a reset before a later reset must conflict: %s", rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"code":"dependent_balance_reset"`)
	})

	t.Run("open-day resets rejected", func(t *testing.T) {
		for _, date := range []timezone.Date{today, today.AddDays(7)} {
			rec := do(http.MethodPost, resetPath,
				fmt.Sprintf(`{"effective_date":"%s","carryover_minutes":0,"note":"x"}`, date), token)
			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		}
	})

	t.Run("delete removes the adjustment", func(t *testing.T) {
		// The session must fund the comp-time booking below, so its window has
		// to be OVER when the test runs AND must lie entirely inside its own
		// calendar day. The month math caps a session both at "now"
		// (workedMinutesUpTo) and at the end of session.Date, so the previous
		// fixed 08:00-09:00 UTC window contributed nothing on any run before
		// 09:00 UTC: the balance stayed 0 after the reset and the booking was
		// rejected for lack of capacity.
		//
		// Four hours ending a minute ago satisfies both caps for any run from
		// 04:01 local onwards; earlier than that the window would reach back
		// past midnight and the day-end cap would trim most of it, so it moves
		// to the previous day instead (which is past, hence uncapped).
		sessionDate := today
		checkOut := timezone.Now().Add(-time.Minute)
		checkIn := checkOut.Add(-4 * time.Hour)
		if checkIn.Before(today.BerlinMidnight()) {
			sessionDate = today.AddDays(-1)
			checkIn = sessionDate.BerlinMidnight().Add(8 * time.Hour)
			checkOut = checkIn.Add(4 * time.Hour)
		}
		session := &activeModels.WorkSession{
			StaffID:      subject.ID,
			Date:         sessionDate,
			Status:       activeModels.WorkSessionStatusPresent,
			Source:       activeModels.WorkSessionSourceApp,
			CheckInTime:  checkIn,
			CheckOutTime: &checkOut,
			CreatedBy:    subject.ID,
		}
		session.SetTenantID(tenantID)
		_, err := tc.db.NewInsert().Model(session).Exec(testpkg.TenantContext(tenantID))
		require.NoError(t, err)

		rec := do(http.MethodPost, adjustmentsPath,
			fmt.Sprintf(`{"type":"comp_time","minutes_delta":-30,"effective_date":"%s","note":"Tippfehler"}`, today.AddDays(1)), token)
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
