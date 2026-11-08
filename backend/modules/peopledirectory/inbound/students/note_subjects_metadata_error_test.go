package students_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// The note subjects of an enrolled child read the active enrollments and then
// their groups with categories (#5751). A failure of that metadata read is a
// server error, not a successful answer without it; a child without
// enrollments never reaches the read and keeps answering. The permission
// fault is confined to this test's disposable database clone.
func TestNoteSubjectsKeepTheGroupMetadataErrorContract(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	tc := setupStudentsRoute(t)
	_, activities := api.NewActiveTestTimetable(testpkg.NewDatabase(t, tc.db), testpkg.TenantRuntime(t, tc.db))
	tc.resource.ActiveEnrollments = students20Enrollments{activities}
	ctx := testpkg.Ctx(t)
	enrolled := testpkg.CreateTestStudent(t, tc.db, "Metadata", "Enrolled", "M51")
	idle := testpkg.CreateTestStudent(t, tc.db, "Metadata", "Idle", "M51")
	group := testpkg.CreateTestActivityGroup(t, tc.db, "Metadata AG")
	_, err := tc.db.NewInsert().Model(&testpkg.StudentEnrollment{
		TenantID: testpkg.Tenant(t), StudentID: enrolled.ID, ActivityGroupID: group.ID, ValidFrom: calendar.TodayDate().AddDays(-1),
	}).Exec(ctx)
	require.NoError(t, err)
	actor := testpkg.CreateTestAccount(t, tc.db, "m51-notes@example.com")

	_, err = tc.db.NewRaw(`REVOKE SELECT ON activities.categories FROM phoenix_tenant`).Exec(ctx)
	require.NoError(t, err)
	var canRead bool
	require.NoError(t, tc.db.NewRaw(`SELECT has_table_privilege('phoenix_tenant', 'activities.categories', 'SELECT')`).Scan(ctx, &canRead))
	require.False(t, canRead, "the fault must actually deny the tenant role")

	router := testutil.NewRouter()
	router.Mount("/api/students", tc.router())
	get := func(studentID int64) (int, map[string]json.RawMessage) {
		req := testutil.NewRequest(http.MethodGet, fmt.Sprintf("/api/students/%d/note-subjects", studentID), nil)
		rr := testutil.ExecuteWithAuthPermissions(t, router, req, testutil.AdminTestClaims(int(actor.ID)), []string{"admin:*"})
		var body map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body), rr.Body.String())
		return rr.Code, body
	}

	status, body := get(enrolled.ID)
	require.Equal(t, http.StatusInternalServerError, status, "a failed group metadata read fails the request")
	require.JSONEq(t, `"general.server"`, string(body["code"]))

	status, _ = get(idle.ID)
	require.Equal(t, http.StatusOK, status, "a child without enrollments never reads the group metadata")
}
