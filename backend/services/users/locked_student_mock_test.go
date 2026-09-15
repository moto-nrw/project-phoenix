package users_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/services/users/userstest"

	usersModel "github.com/moto-nrw/project-phoenix/models/users"
)

// Migrated from the IoT route suite: the fallback is a contract of this mock,
// not of the kiosk's public locked-student interface.
func TestLockedStudentLookupFallsBackToPlainStub(t *testing.T) {
	t.Parallel()
	graduate := &usersModel.Student{Status: usersModel.StudentStatusAlumnus}
	stub := &userstest.PersonServiceMock{GetStudentByIDFn: func(context.Context, int64) (*usersModel.Student, error) {
		return graduate, nil
	}}
	student, err := stub.GetStudentByIDForUpdate(context.Background(), graduate.ID)
	if err != nil || student != graduate || !student.IsAlumnus() {
		t.Fatalf("locked lookup must use the plain stub when no locked stub was supplied: %v, %v", student, err)
	}
}
