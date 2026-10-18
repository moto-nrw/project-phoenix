package careplanhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type quotaErrorBody struct {
	Code    string         `json:"code"`
	Details map[string]int `json:"details"`
}

func requireQuotaResponse(t *testing.T, status int, body []byte, booked, occupied int) {
	t.Helper()
	require.Equal(t, http.StatusConflict, status, "Body: %s", body)
	var decoded quotaErrorBody
	require.NoError(t, json.Unmarshal(body, &decoded))
	assert.Equal(t, "students.child_quota_reached", decoded.Code)
	assert.Equal(t, map[string]int{"booked_places": booked, "occupied_places": occupied, "requested_places": 1}, decoded.Details)
}

func TestResumeCare_RefusesAFullKinderkontingent(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRouteWithCareLifecycle(t, false, fixedCalendarClock)
	today := studentsTestToday
	testpkg.CreateTestStudent(t, tc.db, "Schon", "Da", "1a")
	ended := testpkg.CreateTestStudent(t, tc.db, "Wieder", "Da", "1a")
	_, err := tc.db.NewUpdate().TableExpr("users.student_school_memberships").
		Set("enrolled_until = ?", today.AddDays(-1)).Set("status = ?", careplan.StudentStatusInactive).
		Where("student_profile_id = ? AND deleted_at IS NULL", ended.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	_, err = tc.db.NewRaw(`INSERT INTO users.student_care_exits (tenant_id, student_id, reason) VALUES (?, ?, 'moved_away')`,
		testpkg.Tenant(t), ended.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	testpkg.SetTestChildQuota(t, tc.db, 1, 1)

	path := fmt.Sprintf("/%d/care-end/resume", ended.ID)
	body := map[string]any{"new_start": today.String(), "checked": true}
	rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, http.MethodPost, path, body), testutil.AdminTestClaims(1), []string{"admin:*"})

	requireQuotaResponse(t, rr.Code, rr.Body.Bytes(), 1, 1)
	var until *calendar.Date
	require.NoError(t, tc.db.NewSelect().TableExpr("users.student_school_memberships").Column("enrolled_until").
		Where("student_profile_id = ? AND deleted_at IS NULL", ended.ID).Scan(testpkg.Ctx(t), &until))
	require.NotNil(t, until, "the care stays ended")
	assert.Equal(t, today.AddDays(-1), *until)
}
