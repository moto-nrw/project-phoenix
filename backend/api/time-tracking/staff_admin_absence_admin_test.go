// Router-level tests for the #1843 admin absence endpoints:
// POST   /api/staff/{id}/absences        (file a sick report for someone)
// DELETE /api/staff/{id}/absences/{absenceId} (undo it, reversing the cascade)
//
// Requests flow through the production middleware chain (JWT → tenant →
// permission → tenant tx), so these also prove the cascade commits/reverts
// inside the request transaction. Fixtures via testpkg helpers.
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
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupAbsenceAdminTest builds the router context plus an editor (admin
// account linked to a staff row, required by resolveEditorStaffID) and a
// subject staff member with one plain shift tomorrow.
func setupAbsenceAdminTest(t *testing.T, clocks ...func() time.Time) (tc *testContext, token string, subjectID int64, shiftID int64) {
	t.Helper()
	tc = setupStaffRoute(t, clocks...)
	suffix := time.Now().UnixNano()

	editorPerson, editorAccount := testpkg.CreateTestPersonWithAccount(t, tc.db, "Absence", fmt.Sprintf("Editor-%d", suffix))
	testpkg.CreateTestStaffForPerson(t, tc.db, editorPerson.ID)
	subject := testpkg.CreateTestStaff(t, tc.db, "Absence", fmt.Sprintf("Subject-%d", suffix))

	today := timezone.TodayDate()
	if len(clocks) > 0 {
		today = timezone.DateFromTime(clocks[0]())
	}
	tomorrow := today.AddDays(1)
	shift := &scheduleModels.StaffShift{
		StaffID:   subject.ID,
		Date:      tomorrow,
		StartTime: time.Date(1, 1, 1, 8, 0, 0, 0, time.UTC),
		EndTime:   time.Date(1, 1, 1, 12, 0, 0, 0, time.UTC),
		CreatedBy: subject.ID,
	}
	shift.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repositories.NewFactory(tc.db).StaffShift.Create(testpkg.Ctx(t), shift))

	claims := testutil.DefaultTestClaims()
	claims.ID = int(editorAccount.ID)
	claims.Permissions = []string{"time_tracking:manage"}
	return tc, testutil.MintTestJWT(t, claims), subject.ID, shift.ID
}

func postAbsence(t *testing.T, tc *testContext, token string, staffID int64, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/staff/%d/absences", staffID), bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	tc.router.ServeHTTP(rec, req)
	return rec
}

func TestAdminCreateStaffAbsence_SickCascades(t *testing.T) {
	t.Parallel()

	tc, token, subjectID, shiftID := setupAbsenceAdminTest(t, func() time.Time {
		return timezone.NewDate(2026, 8, 24).BerlinMidnight().Add(12 * time.Hour)
	})
	tomorrow := timezone.NewDate(2026, 8, 24).AddDays(1)

	rec := postAbsence(t, tc, token, subjectID, map[string]any{
		"absence_type": "sick",
		"date_start":   tomorrow.String(),
		"date_end":     tomorrow.String(),
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	// Absence persisted with subject/creator separation.
	ctx := testpkg.Ctx(t)
	repos := repositories.NewFactory(tc.db)
	absences, err := repos.StaffAbsence.GetByStaffAndDateRange(ctx, subjectID, tomorrow, tomorrow)
	require.NoError(t, err)
	require.Len(t, absences, 1)
	assert.Equal(t, activeModels.AbsenceTypeSick, absences[0].AbsenceType)
	assert.NotEqual(t, subjectID, absences[0].CreatedBy, "creator must be the editor, not the subject")

	// The cascade cancelled + stamped the shift inside the request tx.
	shift, err := repos.StaffShift.FindByID(ctx, shiftID)
	require.NoError(t, err)
	assert.True(t, shift.Cancelled)
	require.NotNil(t, shift.SickAbsenceID)
	assert.Equal(t, absences[0].ID, *shift.SickAbsenceID)

	// DELETE undoes it: absence gone, shift reactivated.
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/staff/%d/absences/%d", subjectID, absences[0].ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	tc.router.ServeHTTP(rec2, req)
	require.Equal(t, http.StatusNoContent, rec2.Code, rec2.Body.String())

	remaining, err := repos.StaffAbsence.GetByStaffAndDateRange(ctx, subjectID, tomorrow, tomorrow)
	require.NoError(t, err)
	assert.Empty(t, remaining)
	shift, err = repos.StaffShift.FindByID(ctx, shiftID)
	require.NoError(t, err)
	assert.False(t, shift.Cancelled)
	assert.Nil(t, shift.SickAbsenceID)
}

func TestAdminCreateStaffAbsence_CompTimeAllowedForManager(t *testing.T) {
	t.Parallel()

	tc, token, subjectID, _ := setupAbsenceAdminTest(t)
	tomorrow := timezone.NewDate(2026, 8, 24).AddDays(1)

	rec := postAbsence(t, tc, token, subjectID, map[string]any{
		"absence_type": "comp_time",
		"date_start":   tomorrow.String(),
		"date_end":     tomorrow.String(),
		"half_day":     true,
		"note":         "Überstundenabbau",
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	ctx := testpkg.Ctx(t)
	absences, err := repositories.NewFactory(tc.db).StaffAbsence.GetByStaffAndDateRange(
		ctx,
		subjectID,
		tomorrow,
		tomorrow,
	)
	require.NoError(t, err)
	require.Len(t, absences, 1)
	assert.Equal(t, activeModels.AbsenceTypeCompTime, absences[0].AbsenceType)
	assert.True(t, absences[0].HalfDay)
	assert.Equal(t, activeModels.AbsenceStatusReported, absences[0].Status)
}

func TestAdminCreateStaffAbsence_RejectsMultiDayHalfDayCompTime(t *testing.T) {
	t.Parallel()

	tc, token, subjectID, _ := setupAbsenceAdminTest(t)
	tomorrow := timezone.TodayDate().AddDays(1)

	rec := postAbsence(t, tc, token, subjectID, map[string]any{
		"absence_type": "comp_time",
		"date_start":   tomorrow.String(),
		"date_end":     tomorrow.AddDays(1).String(),
		"half_day":     true,
		"note":         "Ungültiger Freizeitausgleich",
	})

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestAdminCreateStaffAbsence_RequiresPermission(t *testing.T) {
	t.Parallel()

	tc, _, subjectID, _ := setupAbsenceAdminTest(t)
	tomorrow := timezone.TodayDate().AddDays(1)

	noPermToken := authToken(t, "schedules:read")
	rec := postAbsence(t, tc, noPermToken, subjectID, map[string]any{
		"absence_type": "sick",
		"date_start":   tomorrow.String(),
		"date_end":     tomorrow.String(),
	})
	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestStaffAbsenceReads_AllowTimeTrackingManage(t *testing.T) {
	t.Parallel()

	tc := setupStaffRoute(t)
	staff := testpkg.CreateTestStaff(t, tc.db, "Absence", fmt.Sprintf("Reader-%d", time.Now().UnixNano()))
	token := authToken(t, "time_tracking:manage")
	year := timezone.TodayDate().Year()

	paths := []string{
		fmt.Sprintf("/staff/%d/absences?from=%d-01-01&to=%d-12-31", staff.ID, year, year),
		fmt.Sprintf("/staff/%d/vacation/quota?year=%d", staff.ID, year),
	}
	for _, path := range paths {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		tc.router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "%s: %s", path, rec.Body.String())
	}
}

func TestAdminCreateStaffAbsence_UnknownStaff(t *testing.T) {
	t.Parallel()

	tc, token, _, _ := setupAbsenceAdminTest(t)
	tomorrow := timezone.TodayDate().AddDays(1)

	rec := postAbsence(t, tc, token, 99999999, map[string]any{
		"absence_type": "sick",
		"date_start":   tomorrow.String(),
		"date_end":     tomorrow.String(),
	})
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminCreateStaffAbsence_RejectsVacationType(t *testing.T) {
	t.Parallel()

	tc, token, subjectID, _ := setupAbsenceAdminTest(t)
	tomorrow := timezone.TodayDate().AddDays(1)

	rec := postAbsence(t, tc, token, subjectID, map[string]any{
		"absence_type": "vacation",
		"date_start":   tomorrow.String(),
		"date_end":     tomorrow.String(),
	})
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAdminCreateStaffAbsence_RejectsBlockedCustomAllowanceOverrun(t *testing.T) {
	t.Parallel()

	tc, token, subjectID, _ := setupAbsenceAdminTest(t)
	ctx := testpkg.Ctx(t)
	absenceType := &activeModels.StaffAbsenceType{
		Name:             "Sonderurlaub",
		BaseType:         activeModels.AbsenceTypeOther,
		IsActive:         true,
		AllowanceEnabled: true,
		OverrunPolicy:    activeModels.AbsenceTypeOverrunBlock,
	}
	absenceType.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repositories.NewFactory(tc.db).StaffAbsenceType.Create(ctx, absenceType))

	tomorrow := timezone.TodayDate().AddDays(1)
	rec := postAbsence(t, tc, token, subjectID, map[string]any{
		"absence_type":    "other",
		"absence_type_id": absenceType.ID,
		"date_start":      tomorrow.String(),
		"date_end":        tomorrow.String(),
	})

	assert.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
}

// #2873: the comp-time preview endpoint returns the Stundenkonto projection
// for the modal. The subject has no schedule targets, so every value is zero
// — the point here is the wire contract and the manager permission gate.
func TestAdminCompTimePreview_ReturnsProjection(t *testing.T) {
	t.Parallel()

	tc, token, subjectID, _ := setupAbsenceAdminTest(t)
	day := timezone.NewDate(2026, 9, 7)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf(
		"/staff/%d/time-tracking/comp-time-preview?date_start=%s&date_end=%s&half_day=false",
		subjectID, day.String(), day.String(),
	), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	tc.router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var body struct {
		Data struct {
			CurrentBalanceMinutes   int `json:"current_balance_minutes"`
			DeductionMinutes        int `json:"deduction_minutes"`
			ProjectedBalanceMinutes int `json:"projected_balance_minutes"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, 0, body.Data.DeductionMinutes)
	assert.Equal(t, body.Data.CurrentBalanceMinutes, body.Data.ProjectedBalanceMinutes)

	// Malformed dates are a client error, not a 500.
	badReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf(
		"/staff/%d/time-tracking/comp-time-preview?date_start=heute&date_end=%s",
		subjectID, day.String(),
	), nil)
	badReq.Header.Set("Authorization", "Bearer "+token)
	badRec := httptest.NewRecorder()
	tc.router.ServeHTTP(badRec, badReq)
	assert.Equal(t, http.StatusBadRequest, badRec.Code, badRec.Body.String())

	// half_day accepts only true|false — anything else is a client error, not
	// a silent full-day preview.
	for _, halfDay := range []string{"1", "invalid"} {
		hdReq := httptest.NewRequest(http.MethodGet, fmt.Sprintf(
			"/staff/%d/time-tracking/comp-time-preview?date_start=%s&date_end=%s&half_day=%s",
			subjectID, day.String(), day.String(), halfDay,
		), nil)
		hdReq.Header.Set("Authorization", "Bearer "+token)
		hdRec := httptest.NewRecorder()
		tc.router.ServeHTTP(hdRec, hdReq)
		assert.Equal(t, http.StatusBadRequest, hdRec.Code, "half_day=%s: %s", halfDay, hdRec.Body.String())
	}
}
