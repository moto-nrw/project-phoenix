// Router-level tests for POST /api/staff/{id}/absences/rebook (#3258): the
// Leitung turns past Freizeitausgleich days into days of a school-defined
// type with an allowance, as in the Wissingen call of 16.09.2026.
package timetracking

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type rebookFixture struct {
	tc        *testContext
	token     string
	subjectID int64
	typeID    int64
	absences  []int64
}

// setupRebookFixture gives the subject a 8-hour Monday-to-Friday schedule
// and three Freizeitausgleich Fridays in August 2026, plus a
// "Krank-Urlaubstage" type with the given 2026 allowance.
func setupRebookFixture(t *testing.T, entitledDays float64) rebookFixture {
	t.Helper()
	tc, token, subjectID, _ := setupAbsenceAdminTest(t, func() time.Time {
		return testpkg.Date(2026, time.September, 17).BerlinMidnight().Add(12 * time.Hour)
	})
	validFrom := testpkg.Date(2026, time.January, 1)
	for weekday := range 5 {
		testpkg.CreateTestStaffWorkScheduleForTenant(t, tc.db, testpkg.Tenant(t), subjectID, weekday, 480, validFrom)
	}
	typeID := insertAbsenceType(t, tc, fmt.Sprintf("Krank-Urlaubstage-%d", time.Now().UnixNano()), true)
	_, err := tc.db.NewRaw(`
		INSERT INTO active.staff_absence_type_allowances (tenant_id, staff_id, absence_type_id, year, entitled_days)
		VALUES (?, ?, ?, 2026, ?)`,
		testpkg.Tenant(t), subjectID, typeID, entitledDays).Exec(testpkg.Ctx(t))
	require.NoError(t, err)

	fixture := rebookFixture{tc: tc, token: token, subjectID: subjectID, typeID: typeID}
	for _, day := range []int{7, 14, 21} {
		friday := testpkg.Date(2026, time.August, day)
		fixture.absences = append(fixture.absences, insertAbsenceRow(t, tc, map[string]any{
			"staff_id":     subjectID,
			"absence_type": "comp_time",
			"date_start":   friday.String(),
			"date_end":     friday.String(),
			"status":       "reported",
			"created_by":   subjectID,
			"requested_at": time.Now(),
		}))
	}
	return fixture
}

func (f rebookFixture) rebook(t *testing.T, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return testutil.ExecuteRequest(f.tc.router, testutil.NewAuthenticatedRequest(
		t, http.MethodPost, fmt.Sprintf("/staff/%d/absences/rebook", f.subjectID),
		body, testutil.WithJWTBearer(f.token)))
}

func (f rebookFixture) body(reason string, dryRun bool) map[string]any {
	ids := make([]string, 0, len(f.absences))
	for _, id := range f.absences {
		ids = append(ids, strconv.FormatInt(id, 10))
	}
	return map[string]any{
		"absence_ids":     ids,
		"absence_type":    "other",
		"absence_type_id": strconv.FormatInt(f.typeID, 10),
		"reason":          reason,
		"dry_run":         dryRun,
	}
}

func (f rebookFixture) augustBalance(t *testing.T) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/staff/%d/time-tracking/month-summary?year=2026&month=8", f.subjectID), nil)
	req.Header.Set("Authorization", "Bearer "+f.token)
	rec := httptest.NewRecorder()
	f.tc.router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var body struct {
		Data struct {
			BalanceMinutes int `json:"balance_minutes"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Data.BalanceMinutes
}

func (f rebookFixture) storedTypes(t *testing.T) []string {
	t.Helper()
	var types []string
	require.NoError(t, f.tc.db.NewRaw(`
		SELECT absence_type || ':' || COALESCE(absence_type_id::text, '')
		FROM active.staff_absences WHERE tenant_id = ? AND id IN (?) ORDER BY id`,
		testpkg.Tenant(t), bun.List(f.absences)).Scan(testpkg.Ctx(t), &types))
	return types
}

type rebookResponse struct {
	Data struct {
		Absences []struct {
			ID               int64  `json:"id"`
			AbsenceType      string `json:"absence_type"`
			AbsenceTypeID    string `json:"absence_type_id"`
			AbsenceTypeLabel string `json:"absence_type_label"`
		} `json:"absences"`
		Days                float64 `json:"days"`
		BalanceDeltaMinutes int     `json:"balance_delta_minutes"`
		Allowances          []struct {
			Year          int     `json:"year"`
			RemainingDays float64 `json:"remaining_days"`
			BookingDays   float64 `json:"booking_days"`
		} `json:"allowances"`
		AllowanceExceeded bool `json:"allowance_exceeded"`
		Applied           bool `json:"applied"`
	} `json:"data"`
}

func decodeRebook(t *testing.T, rec *httptest.ResponseRecorder) rebookResponse {
	t.Helper()
	var body rebookResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), rec.Body.String())
	return body
}

func TestAdminRebookAbsences_CompTimeToAllowanceType(t *testing.T) {
	t.Parallel()
	f := setupRebookFixture(t, 10)
	balanceBefore := f.augustBalance(t)

	// The preview names every effect and writes nothing.
	preview := f.rebook(t, f.body("", true))
	require.Equal(t, http.StatusOK, preview.Code, preview.Body.String())
	previewBody := decodeRebook(t, preview)
	assert.False(t, previewBody.Data.Applied)
	assert.Equal(t, 3.0, previewBody.Data.Days)
	assert.Equal(t, 3*480, previewBody.Data.BalanceDeltaMinutes)
	assert.False(t, previewBody.Data.AllowanceExceeded)
	require.Len(t, previewBody.Data.Allowances, 1)
	assert.Equal(t, 2026, previewBody.Data.Allowances[0].Year)
	assert.Equal(t, 3.0, previewBody.Data.Allowances[0].BookingDays)
	assert.Equal(t, 7.0, previewBody.Data.Allowances[0].RemainingDays)
	assert.Equal(t, []string{"comp_time:", "comp_time:", "comp_time:"}, f.storedTypes(t))

	// The write needs a reason.
	unreasoned := f.rebook(t, f.body("  ", false))
	require.Equal(t, http.StatusConflict, unreasoned.Code, unreasoned.Body.String())
	assert.Contains(t, unreasoned.Body.String(), `"absence_rebooking_blocked"`)

	applied := f.rebook(t, f.body("Kontingent angelegt, Freitage waren Krank-Urlaubstage", false))
	require.Equal(t, http.StatusOK, applied.Code, applied.Body.String())
	appliedBody := decodeRebook(t, applied)
	assert.True(t, appliedBody.Data.Applied)
	require.Len(t, appliedBody.Data.Absences, 3)
	for _, absence := range appliedBody.Data.Absences {
		assert.Equal(t, "other", absence.AbsenceType)
		assert.Equal(t, strconv.FormatInt(f.typeID, 10), absence.AbsenceTypeID)
		assert.NotEmpty(t, absence.AbsenceTypeLabel)
	}
	typeRef := "other:" + strconv.FormatInt(f.typeID, 10)
	assert.Equal(t, []string{typeRef, typeRef, typeRef}, f.storedTypes(t))

	// The Stundenkonto gets the deducted targets back, exactly as previewed.
	assert.Equal(t, balanceBefore+3*480, f.augustBalance(t))

	// A second rebooking of the same entries reports that they already
	// carry the type instead of booking the allowance twice.
	again := f.rebook(t, f.body("", true))
	require.Equal(t, http.StatusConflict, again.Code, again.Body.String())
	assert.Contains(t, again.Body.String(), "hat diese Art schon")

	// Every entry appears with its reason and both types in the audit log.
	var rows []struct {
		Reason   string `bun:"reason"`
		FromType string `bun:"from_type"`
		ToType   string `bun:"to_type"`
		ToLabel  string `bun:"to_label"`
		ActorID  *int64 `bun:"actor_staff_id"`
	}
	require.NoError(t, f.tc.db.NewRaw(`
		SELECT reason, detail->>'from_absence_type' AS from_type,
		       detail->>'to_absence_type' AS to_type,
		       detail->>'to_absence_type_label' AS to_label, actor_staff_id
		FROM audit.time_tracking_audit_log
		WHERE tenant_id = ? AND source = 'absence' AND staff_id = ?`,
		testpkg.Tenant(t), f.subjectID).Scan(testpkg.Ctx(t), &rows))
	require.Len(t, rows, 3)
	for _, row := range rows {
		assert.Equal(t, "Kontingent angelegt, Freitage waren Krank-Urlaubstage", row.Reason)
		assert.Equal(t, "comp_time", row.FromType)
		assert.Equal(t, "other", row.ToType)
		assert.Contains(t, row.ToLabel, "Krank-Urlaubstage")
		require.NotNil(t, row.ActorID, "the Leitung is named as actor")
		assert.NotEqual(t, f.subjectID, *row.ActorID)
	}
}

func TestAdminRebookAbsences_RejectsBlockingOverlap(t *testing.T) {
	t.Parallel()

	f := setupRebookFixture(t, 10)
	friday := testpkg.Date(2026, time.August, 7)
	insertAbsenceRow(t, f.tc, map[string]any{
		"staff_id":     f.subjectID,
		"absence_type": "training",
		"date_start":   friday.String(),
		"date_end":     friday.String(),
		"status":       "reported",
		"created_by":   f.subjectID,
		"requested_at": time.Now(),
	})

	rec := f.rebook(t, f.body("", true))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"absence_rebooking_blocked"`)
	assert.Contains(t, rec.Body.String(), "überschneidet sich")
	assert.Equal(t, []string{"comp_time:", "comp_time:", "comp_time:"}, f.storedTypes(t))
}

func TestAdminRebookAbsences_AllowanceShortfallIsShownAndBlocks(t *testing.T) {
	t.Parallel()
	f := setupRebookFixture(t, 2)

	preview := f.rebook(t, f.body("", true))
	require.Equal(t, http.StatusOK, preview.Code, preview.Body.String())
	body := decodeRebook(t, preview)
	assert.True(t, body.Data.AllowanceExceeded)
	require.Len(t, body.Data.Allowances, 1)
	assert.Equal(t, -1.0, body.Data.Allowances[0].RemainingDays)

	rec := f.rebook(t, f.body("Kontingent angelegt", false))
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"absence_allowance_exceeded"`)
	assert.Equal(t, []string{"comp_time:", "comp_time:", "comp_time:"}, f.storedTypes(t))
}

func TestAdminRebookAbsences_ClosedMonthBlocks(t *testing.T) {
	t.Parallel()
	f := setupRebookFixture(t, 10)
	_, err := f.tc.db.NewRaw(`
		INSERT INTO active.staff_month_balance_snapshots
			(tenant_id, staff_id, year, month, closing_balance_minutes, closed_by, close_reason)
		VALUES (?, ?, 2026, 8, 0, ?, 'Monatsabschluss')`,
		testpkg.Tenant(t), f.subjectID, f.subjectID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)

	for _, dryRun := range []bool{true, false} {
		rec := f.rebook(t, f.body("Kontingent angelegt", dryRun))
		require.Equal(t, http.StatusConflict, rec.Code, "dry_run=%v: %s", dryRun, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"absence_rebooking_blocked"`)
		assert.Contains(t, rec.Body.String(), "August 2026 ist abgeschlossen")
	}
	assert.Equal(t, []string{"comp_time:", "comp_time:", "comp_time:"}, f.storedTypes(t))
}

func TestAdminRebookAbsences_RejectsSickReportsAndForeignEntries(t *testing.T) {
	t.Parallel()
	f := setupRebookFixture(t, 10)

	monday := testpkg.Date(2026, time.August, 3)
	sickID := insertAbsenceRow(t, f.tc, map[string]any{
		"staff_id":     f.subjectID,
		"absence_type": "sick",
		"date_start":   monday.String(),
		"date_end":     monday.String(),
		"status":       "reported",
		"created_by":   f.subjectID,
		"requested_at": time.Now(),
	})
	body := f.body("Korrektur", false)
	body["absence_ids"] = []string{strconv.FormatInt(sickID, 10)}
	rec := f.rebook(t, body)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), "Krankmeldung vom 03.08.2026")

	// Rebooking into sick would skip the plan cascade.
	intoSick := f.body("Korrektur", false)
	intoSick["absence_type"], intoSick["absence_type_id"] = "sick", nil
	rec = f.rebook(t, intoSick)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"absence_rebooking_blocked"`)

	// Another person's entry is not reachable through this staff member.
	other := testpkg.CreateTestStaff(t, f.tc.db, "Rebook", fmt.Sprintf("Other-%d", time.Now().UnixNano()))
	foreignID := insertAbsenceRow(t, f.tc, map[string]any{
		"staff_id":     other.ID,
		"absence_type": "comp_time",
		"date_start":   monday.String(),
		"date_end":     monday.String(),
		"status":       "reported",
		"created_by":   other.ID,
		"requested_at": time.Now(),
	})
	foreign := f.body("Korrektur", false)
	foreign["absence_ids"] = []string{strconv.FormatInt(foreignID, 10)}
	rec = f.rebook(t, foreign)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	assert.Equal(t, []string{"comp_time:", "comp_time:", "comp_time:"}, f.storedTypes(t))
}

func TestAdminRebookAbsences_RequiresTimeTrackingManage(t *testing.T) {
	t.Parallel()
	f := setupRebookFixture(t, 10)
	claims := testutil.DefaultTestClaims()
	claims.Permissions = []string{"vacation:approve"}
	rec := testutil.ExecuteRequest(f.tc.router, testutil.NewAuthenticatedRequest(
		t, http.MethodPost, fmt.Sprintf("/staff/%d/absences/rebook", f.subjectID),
		f.body("Korrektur", false), testutil.WithJWTBearer(testutil.MintTestJWT(t, claims))))
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

func TestAdminRebookAbsences_IntoVacationChecksTheVacationAccount(t *testing.T) {
	t.Parallel()
	f := setupRebookFixture(t, 10)
	quota := putVacationQuota(t, f.tc, f.token, f.subjectID, 2026, 2)
	require.Equal(t, http.StatusOK, quota.Code, quota.Body.String())

	body := f.body("Urlaub statt Freizeitausgleich", true)
	body["absence_type"], body["absence_type_id"] = "vacation", nil
	preview := f.rebook(t, body)
	require.Equal(t, http.StatusOK, preview.Code, preview.Body.String())
	var previewBody struct {
		Data struct {
			Vacation []struct {
				Year            int     `json:"year"`
				RemainingBefore float64 `json:"remaining_before"`
				RemainingAfter  float64 `json:"remaining_after"`
			} `json:"vacation"`
			VacationExceeded    bool `json:"vacation_exceeded"`
			BalanceDeltaMinutes int  `json:"balance_delta_minutes"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(preview.Body.Bytes(), &previewBody))
	assert.True(t, previewBody.Data.VacationExceeded)
	require.Len(t, previewBody.Data.Vacation, 1)
	assert.Equal(t, 2.0, previewBody.Data.Vacation[0].RemainingBefore)
	assert.Equal(t, -1.0, previewBody.Data.Vacation[0].RemainingAfter)
	assert.Equal(t, 3*480, previewBody.Data.BalanceDeltaMinutes)

	body["dry_run"] = false
	rec := f.rebook(t, body)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"vacation_quota_exceeded"`)

	// Two of the three Fridays fit; they become direct vacation with their
	// working days, so the vacation account counts them.
	body["absence_ids"] = []string{strconv.FormatInt(f.absences[0], 10), strconv.FormatInt(f.absences[1], 10)}
	rec = f.rebook(t, body)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var workingDays []float64
	require.NoError(t, f.tc.db.NewRaw(`
		SELECT working_days FROM active.staff_absences
		WHERE tenant_id = ? AND id IN (?) AND absence_type = 'vacation' AND status = 'reported'`,
		testpkg.Tenant(t), bun.List(f.absences[:2])).Scan(testpkg.Ctx(t), &workingDays))
	assert.Equal(t, []float64{1, 1}, workingDays)
	assert.Equal(t, []string{"vacation:", "vacation:", "comp_time:"}, f.storedTypes(t))
}
