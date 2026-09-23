package contracttest_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type detailFailDirectory struct {
	compose.ScheduleReviewDirectory
	students map[int64]compose.ReviewStudent
	err      error
	calls    int
}

func (d *detailFailDirectory) PersonNames(context.Context, []int64) (map[int64]compose.PersonName, error) {
	d.calls++
	return nil, d.err
}

func TestScheduleDetailAuthorizesBeforeLoadingNames(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := careplantest.NewCarePlan(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Detail", "Scope", "1a")
	account := testpkg.CreateTestAccount(t, db, "parent")
	request, err := module.CreateCareScheduleRequest(ctx, careplan.CareScheduleChangeRequest{
		StudentID: student.ID, SubmittedBy: account.ID, RequestKind: "weekly_schedule", Status: "pending", Payload: json.RawMessage(`{"weekdays":[]}`),
	})
	require.NoError(t, err)
	directory := &detailFailDirectory{students: map[int64]compose.ReviewStudent{student.ID: {ID: student.ID, PersonID: student.PersonID}}, err: errors.New("name lookup failed")}
	scope := compose.ReviewScope{}
	deps := compose.ScheduleReviewDependencies{People: directory, Bookings: struct{ compose.ReviewBookings }{}, Classes: struct{ compose.ReviewClassTimes }{},
		Scope:                 func(context.Context) (compose.ReviewScope, error) { return scope, nil },
		BookingsAuthoritative: func(context.Context) (bool, error) { return false, nil }, Today: func() careplan.Date { return "2026-09-20" },
	}
	query, err := compose.NewScheduleReviews(db, func(compose.Observation) {}, deps)
	require.NoError(t, err)
	_, err = query.GetForReview(ctx, request.ID)
	require.ErrorIs(t, err, carerequests.ErrCareRequestForbidden)
	require.Zero(t, directory.calls)
	scope.SchoolWide = true
	_, err = query.GetForReview(ctx, request.ID)
	require.ErrorIs(t, err, directory.err, "display read errors must not return a partial detail")
	require.Equal(t, 1, directory.calls)
	delete(directory.students, student.ID)
	_, err = query.GetForReview(ctx, request.ID)
	require.ErrorIs(t, err, careplan.ErrCareScheduleRequestNotFound)
	require.Equal(t, 1, directory.calls)
}

func (d *detailFailDirectory) FindStudents(context.Context, []int64) (map[int64]compose.ReviewStudent, error) {
	return d.students, nil
}
