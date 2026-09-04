package data_test

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeacherStudentsQueryBudget(t *testing.T) {
	t.Parallel()
	tc := setupDataRoute(t)
	device := testpkg.CreateTestDevice(t, tc.db, "teacher-students-budget")
	teacherIDs := make([]string, 0, 8)
	add := func(count int) {
		for i := range count {
			teacher, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, fmt.Sprintf("Teacher%d", len(teacherIDs)+i), "Budget")
			testpkg.EnsureAccountTenant(t, tc.db, account.ID, teacher.GetTenantID())
			group := testpkg.CreateTestEducationGroup(t, tc.db, fmt.Sprintf("TeacherBudget%d", len(teacherIDs)+i))
			testpkg.CreateTestGroupTeacher(t, tc.db, group.ID, teacher.ID)
			student := testpkg.CreateTestStudent(t, tc.db, fmt.Sprintf("Student%d", len(teacherIDs)+i), "Budget", "1a")
			_, err := tc.db.NewUpdate().Model(student).ModelTableExpr(`users.students AS "student"`).
				Set("group_id = ?", group.ID).Where("id = ?", student.ID).Exec(testpkg.Ctx(t))
			require.NoError(t, err)
			teacherIDs = append(teacherIDs, strconv.FormatInt(teacher.StaffID, 10))
		}
	}
	add(3)
	counter := testpkg.CaptureQueriesForContext(t, tc.db)
	run := func() []string {
		counter.Reset()
		req := testutil.NewAuthenticatedRequest(t, http.MethodGet, "/students?teacher_ids="+strings.Join(teacherIDs, ","), nil,
			testutil.WithDeviceContext(device),
		)
		req = req.WithContext(counter.Context(req.Context()))
		rr := testutil.ExecuteRequest(tc.resource.Router(), req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)
		return counter.Operation("SELECT")
	}
	small := run()
	add(5)
	large := run()
	assert.Equal(t, len(small), len(large))
	testpkg.AssertQueryBudget(t, "api.iot.teacher_students.reads", large)
}
