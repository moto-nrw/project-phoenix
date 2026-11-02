package data_test

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	dataAPI "github.com/moto-nrw/project-phoenix/modules/devicescan/http/data"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The legacy teacher-students budget intentionally reaches the filtered
// error-to-empty fallback. This separate positive path must return the real
// children, not merely HTTP200 with an empty payload and zero SQL.
func TestTeacherStudentsPositiveQueryBudget(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	directory, err := api.NewIoTTeacherStudentsTestDirectory(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	resource := dataAPI.NewResource(directory, nil, nil, testRuntime())
	router := dataTenantRoot(db, resource.Router())
	device := testpkg.CreateTestDevice(t, db, "teacher-students-positive-budget")
	teacherIDs := []string{}
	want := map[int64]devicescan.DirectoryStudent{}
	add := func(count int) {
		for range count {
			i := len(teacherIDs)
			teacher, account := testpkg.CreateTestTeacherWithAccount(t, db, fmt.Sprintf("PositiveTeacher%d", i), "Budget")
			testpkg.EnsureAccountTenant(t, db, account.ID, teacher.TenantID)
			group := testpkg.CreateTestEducationGroup(t, db, fmt.Sprintf("PositiveTeacherGroup%d", i))
			testpkg.CreateTestGroupTeacher(t, db, group.ID, teacher.ID)
			first := fmt.Sprintf("PositiveChild%d", i)
			student := testpkg.CreateTestStudent(t, db, first, "Budget", "1a")
			testpkg.AssignStudentToGroup(t, db, student.ID, group.ID)
			teacherIDs = append(teacherIDs, strconv.FormatInt(teacher.StaffID, 10))
			want[student.ID] = devicescan.DirectoryStudent{
				StudentID: student.ID, PersonID: student.PersonID, FirstName: first,
				LastName: "Budget", SchoolClass: "1a", GroupName: group.Name,
			}
		}
	}
	_ = testpkg.CreateTestStudent(t, db, "Unassigned", "Budget", "1a")
	add(3)
	counter := testpkg.CaptureQueriesForContext(t, db)
	run := func() []string {
		request := testutil.NewAuthenticatedRequest(t, "GET", "/students?teacher_ids="+strings.Join(teacherIDs, ","), nil,
			testutil.WithDeviceContext(device))
		request = request.WithContext(counter.Context(request.Context()))
		counter.Reset()
		response := testutil.ExecuteRequest(router, request)
		testutil.AssertSuccessResponse(t, response, 200)
		var body struct {
			Data []devicescan.DirectoryStudent `json:"data"`
		}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
		require.Len(t, body.Data, len(want), "the filtered lookup must not silently degrade to an empty response")
		got := make(map[int64]devicescan.DirectoryStudent, len(body.Data))
		for _, row := range body.Data {
			require.NotContains(t, got, row.StudentID, "no duplicate children")
			got[row.StudentID] = row
		}
		assert.Equal(t, want, got, "only assigned children with exact fixture fields may be returned")
		require.Zero(t, counter.WriteRows())
		for _, operation := range []string{"INSERT", "UPDATE", "DELETE", "MERGE"} {
			require.Empty(t, counter.Operation(operation))
		}
		t.Logf("teachers=%d children=%d statements=%d", len(teacherIDs), len(body.Data), counter.Total())
		querybudgets.AssertQueryBudget(t, "api.iot.teacher_students.positive", counter.Queries())
		return counter.Queries()
	}
	small := run()
	add(5)
	large := run()
	require.NotEmpty(t, small)
	assert.Len(t, large, len(small), "teacher and child growth must not add queries")
}
