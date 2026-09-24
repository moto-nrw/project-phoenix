// The HTTP surface of the Kinderkontingent (#3567): manual creation and
// resuming care answer a full Kinderkontingent with 409 and a stable code,
// and nothing is written. The Datenverwaltung reads the Kinderkontingent next
// to the Kontingentzahl (#3569).
package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	studentsAPI "github.com/moto-nrw/project-phoenix/api/students"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func setChildQuota(t *testing.T, tc *testContext, bundles, bundleSize int) {
	t.Helper()
	_, err := tc.db.NewUpdate().TableExpr("platform.schools").
		Set("child_quota_bundles = ?", bundles).Set("child_quota_bundle_size = ?", bundleSize).
		Where("id = ?", testpkg.Tenant(t)).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
}

func liveMemberships(t *testing.T, tc *testContext) int {
	t.Helper()
	count, err := tc.db.NewSelect().TableExpr("users.student_school_memberships").
		Where("tenant_id = ?", testpkg.Tenant(t)).Where("deleted_at IS NULL").Count(testpkg.Ctx(t))
	require.NoError(t, err)
	return count
}

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

func TestCreateStudent_RefusesAFullKinderkontingent(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t)
	testpkg.CreateTestStudent(t, tc.db, "Schon", "Da", "1a")
	setChildQuota(t, tc, 1, 1)

	body := map[string]any{"first_name": "Zu", "last_name": "Viel", "school_class": "1a"}
	rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, http.MethodPost, "/", body), testutil.AdminTestClaims(1), []string{"admin:*"})

	requireQuotaResponse(t, rr.Code, rr.Body.Bytes(), 1, 1)
	assert.Equal(t, 1, liveMemberships(t, tc), "the refused child left no membership behind")
	var people int
	require.NoError(t, tc.db.NewSelect().TableExpr("users.persons").ColumnExpr("COUNT(*)").
		Where("tenant_id = ?", testpkg.Tenant(t)).Where("first_name = ? AND last_name = ?", "Zu", "Viel").
		Scan(testpkg.Ctx(t), &people))
	assert.Zero(t, people, "the whole creation rolled back")
}

func TestCreateStudent_FillsTheLastPlace(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t)
	testpkg.CreateTestStudent(t, tc.db, "Schon", "Da", "1a")
	setChildQuota(t, tc, 1, 2)

	body := map[string]any{"first_name": "Letzter", "last_name": "Platz", "school_class": "1a"}
	rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, http.MethodPost, "/", body), testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusCreated, rr.Code, "Body: %s", rr.Body.String())
	assert.Equal(t, 2, liveMemberships(t, tc))
}

func TestResumeCare_RefusesAFullKinderkontingent(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	wireCareLifecycle(t, tc)
	today := studentsTestToday
	testpkg.CreateTestStudent(t, tc.db, "Schon", "Da", "1a")
	ended := testpkg.CreateTestStudent(t, tc.db, "Wieder", "Da", "1a")
	_, err := tc.db.NewUpdate().TableExpr("users.student_school_memberships").
		Set("enrolled_until = ?", today.AddDays(-1)).Set("status = ?", string(userModels.StudentStatusInactive)).
		Where("student_profile_id = ? AND deleted_at IS NULL", ended.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	_, err = tc.db.NewRaw(`INSERT INTO users.student_care_exits (tenant_id, student_id, reason) VALUES (?, ?, 'moved_away')`,
		testpkg.Tenant(t), ended.ID).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
	setChildQuota(t, tc, 1, 1)

	path := fmt.Sprintf("/%d/care-end/resume", ended.ID)
	body := map[string]any{"new_start": today.String(), "checked": true}
	rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, http.MethodPost, path, body), testutil.AdminTestClaims(1), []string{"admin:*"})

	requireQuotaResponse(t, rr.Code, rr.Body.Bytes(), 1, 1)
	var until *timezone.Date
	require.NoError(t, tc.db.NewSelect().TableExpr("users.student_school_memberships").Column("enrolled_until").
		Where("student_profile_id = ? AND deleted_at IS NULL", ended.ID).Scan(testpkg.Ctx(t), &until))
	require.NotNil(t, until, "the care stays ended")
	assert.Equal(t, today.AddDays(-1), *until)
}

// childQuotaReader binds the School Membership capability to the students
// port the way the serving root does.
type childQuotaReader struct {
	membership schoolmembership.ChildQuotaUsages
}

func (r childQuotaReader) ChildQuotaUsage(ctx context.Context) (studentsAPI.ChildQuotaUsage, bool, error) {
	usage, limited, err := r.membership.ChildQuotaUsage(ctx)
	return studentsAPI.ChildQuotaUsage{Booked: usage.Booked, Occupied: usage.Occupied}, limited, err
}

func newChildQuotaReader(t *testing.T, db *bun.DB) childQuotaReader {
	t.Helper()
	membership, err := repositories.NewSchoolMembership(db)
	require.NoError(t, err)
	usages, ok := membership.(schoolmembership.ChildQuotaUsages)
	require.True(t, ok, "the School Membership module answers the Kinderkontingent read")
	return childQuotaReader{membership: usages}
}

func getChildQuota(t *testing.T, tc *testContext, perms []string) (int, map[string]any) {
	t.Helper()
	rr := authExec(t, tc, testutil.NewAuthenticatedRequest(t, http.MethodGet, "/child-quota", nil), testutil.AdminTestClaims(1), perms)
	var body struct {
		Data map[string]any `json:"data"`
	}
	if rr.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body), "Body: %s", rr.Body.String())
	}
	return rr.Code, body.Data
}

func TestChildQuota_ReportsTheKinderkontingentNextToTheKontingentzahl(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	testpkg.CreateTestStudent(t, tc.db, "Erstes", "Kind", "1a")
	testpkg.CreateTestStudent(t, tc.db, "Zweites", "Kind", "1a")
	setChildQuota(t, tc, 2, 25)

	status, data := getChildQuota(t, tc, []string{"users:delete"})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, map[string]any{"limited": true, "booked_places": float64(50), "occupied_places": float64(2)}, data)
}

func TestChildQuota_ReportsNoLimitWithoutAKinderkontingent(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	testpkg.CreateTestStudent(t, tc.db, "Ohne", "Grenze", "1a")

	status, data := getChildQuota(t, tc, []string{"users:delete"})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, map[string]any{"limited": false}, data)
}

func TestChildQuota_IsClosedToRolesWithoutDatenverwaltung(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	setChildQuota(t, tc, 1, 50)

	for _, perms := range [][]string{{"users:read", "users:create", "users:update"}, {}} {
		status, _ := getChildQuota(t, tc, perms)
		assert.Equal(t, http.StatusForbidden, status, "permissions %v", perms)
	}
}

func TestChildQuota_CountsOnlyTheCallersSchool(t *testing.T) {
	t.Parallel()
	tc := setupStudentsRoute(t, fixedCalendarClock)
	testpkg.CreateTestStudent(t, tc.db, "Eigenes", "Kind", "1a")
	setChildQuota(t, tc, 2, 25)
	otherSchool, _ := testpkg.CreateTestTenant(t, tc.db)
	for _, name := range []string{"Fremd1", "Fremd2", "Fremd3"} {
		testpkg.CreateTestStudentForTenant(t, tc.db, otherSchool, name, "Kind", "1a")
	}
	_, err := tc.db.NewUpdate().TableExpr("platform.schools").
		Set("child_quota_bundles = ?", 1).Set("child_quota_bundle_size = ?", 3).
		Where("id = ?", otherSchool).Exec(testpkg.Ctx(t))
	require.NoError(t, err)

	status, data := getChildQuota(t, tc, []string{"users:delete"})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, map[string]any{"limited": true, "booked_places": float64(50), "occupied_places": float64(1)}, data)
}
