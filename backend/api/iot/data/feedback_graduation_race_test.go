// Kiosk feedback vs. a concurrent graduation (#405 review).
//
// The endpoint refuses a graduate, but the refusal is only worth as much as the
// window between the status read and the INSERT: a transition apply locks the
// student row, flips it to alumnus and reconciles the child's records, and the
// enrollment FK is a soft-delete reference that happily accepts the graduate.
// So the status has to be read under the row lock the apply itself takes, not
// from an unlocked snapshot.
package data_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dataAPI "github.com/moto-nrw/project-phoenix/api/iot/data"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/users/userstest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestSubmitFeedback_GraduatedAfterUnlockedRead(t *testing.T) {
	t.Parallel()

	ctx := setupFeedbackTestContext(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-race")
	student := testpkg.CreateTestStudent(t, ctx.db, "Feedback", "RaceGraduate", "4a")

	// What an unlocked read returned before the transition committed.
	snapshot := *student
	require.NotEqual(t, usersModel.StudentStatusAlumnus, snapshot.Status)

	_, err := ctx.db.NewUpdate().
		TableExpr(`users.students`).
		Set("status = ?", string(usersModel.StudentStatusAlumnus)).
		Where("id = ?", student.ID).
		Exec(t.Context())
	require.NoError(t, err)

	// The mock answers the unlocked lookup with the stale "active" snapshot and
	// delegates the locked one to the real service, which reads the committed
	// row. A handler that decides from the unlocked read writes the entry.
	realUsers := ctx.services.Users
	racingUsers := &userstest.PersonServiceMock{
		GetStudentByIDFn: func(context.Context, int64) (*usersModel.Student, error) {
			return &snapshot, nil
		},
		GetStudentByIDForUpdateFn: realUsers.GetStudentByIDForUpdate,
	}

	resource := dataAPI.NewFeedbackResource(ctx.services.IoT, racingUsers, ctx.services.Feedback, nil)

	body := map[string]interface{}{
		"student_id": student.ID,
		"value":      "positive",
	}
	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(resource.Router(), req)

	testutil.AssertNotFound(t, rr)

	count, err := ctx.db.NewSelect().
		TableExpr(`feedback.entries`).
		Where("student_id = ?", student.ID).
		Count(t.Context())
	require.NoError(t, err)
	assert.Zero(t, count, "no feedback entry may be written for a child graduated mid-request")
}

// TestSubmitFeedback_LockedLookupFallsBackToPlainStub pins the PersonServiceMock
// contract the test above relies on: a mock that stubs only the plain lookup
// must answer the locked one from the same stub. Without that fallback every
// existing test stubbing GetStudentByIDFn would silently start receiving a nil
// student from the locked read, and the handler's alumnus refusal would look
// like it had stopped working when in fact nothing was ever asked.
func TestSubmitFeedback_LockedLookupFallsBackToPlainStub(t *testing.T) {
	t.Parallel()

	ctx := setupFeedbackTestContext(t)

	testDevice := testpkg.CreateTestDevice(t, ctx.db, "feedback-test-device-fallback")
	student := testpkg.CreateTestStudent(t, ctx.db, "Feedback", "FallbackGraduate", "4a")

	graduated := *student
	graduated.Status = usersModel.StudentStatusAlumnus

	// Only the plain lookup is stubbed. The handler reads through
	// GetStudentByIDForUpdate, so the refusal below can only happen if the mock
	// routed that call to this stub.
	stubbedUsers := &userstest.PersonServiceMock{
		GetStudentByIDFn: func(context.Context, int64) (*usersModel.Student, error) {
			return &graduated, nil
		},
	}

	resource := dataAPI.NewFeedbackResource(ctx.services.IoT, stubbedUsers, ctx.services.Feedback, nil)

	body := map[string]interface{}{
		"student_id": student.ID,
		"value":      "positive",
	}
	req := testutil.NewAuthenticatedRequest(t, "POST", "/feedback", body,
		testutil.WithDeviceContext(testDevice),
	)

	rr := testutil.ExecuteRequest(resource.Router(), req)

	testutil.AssertNotFound(t, rr)
}
