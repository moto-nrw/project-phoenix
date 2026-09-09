package data

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type teacherDirectoryStub struct {
	teachers []DeviceTeacherResponse
	err      error
}

func (s teacherDirectoryStub) Teachers(context.Context) ([]DeviceTeacherResponse, error) {
	return s.teachers, s.err
}

func (teacherDirectoryStub) Students(context.Context, []int64) ([]TeacherStudentResponse, error) {
	panic("unexpected student roster lookup")
}

func (teacherDirectoryStub) Activities(context.Context) ([]TeacherActivityResponse, error) {
	panic("unexpected activity roster lookup")
}

func requestWithDeviceContext() *testpkg.HTTPRequest {
	req := httptest.NewRequest("GET", "/", nil)
	testutil.WithDeviceIdentity(0, "dev-1")(req)
	return req
}

func TestGetAvailableTeachers_RendersDirectoryErrors(t *testing.T) {
	t.Parallel()
	resource := &Resource{runtime: testRuntime(), Directory: teacherDirectoryStub{err: errors.New("teacher repository unavailable")}}
	router := resource.TeachersRouter()
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, requestWithDeviceContext())
	assert.Equal(t, 500, rr.Code)
}

// Roster membership and incomplete-row filtering are tested at the directory
// seam. This test pins the HTTP representation of its result.
func TestGetAvailableTeachers_RendersTeacherRoster(t *testing.T) {
	t.Parallel()
	resource := &Resource{runtime: testRuntime(), Directory: teacherDirectoryStub{teachers: []DeviceTeacherResponse{
		{StaffID: 11, PersonID: 21, FirstName: "Legacy", LastName: "Teacher", DisplayName: "Legacy Teacher"},
	}}}
	router := resource.TeachersRouter()
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, requestWithDeviceContext())
	require.Equal(t, 200, rr.Code)
	assert.Contains(t, rr.Body.String(), "Legacy Teacher")
	assert.Contains(t, rr.Body.String(), `"staff_id":11`)
	assert.NotContains(t, rr.Body.String(), `"staff_id":12`)
}
