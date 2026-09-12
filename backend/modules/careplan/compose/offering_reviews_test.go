package compose

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type offeringDirectoryFixture struct{ reviewDirectoryFixture }

func (offeringDirectoryFixture) SearchStudentIDs(context.Context, string, []int64) ([]int64, error) {
	return []int64{}, nil
}

type offeringEnrollmentFixture struct {
	child    OfferingReviewChild
	request  OfferingReviewRequest
	phase    OfferingReviewPhase
	bookings []OfferingReviewBooking
	dates    map[int64]careplan.Date
	calls    int
	err      error
}

func (f *offeringEnrollmentFixture) Children(context.Context, []int64) ([]OfferingReviewChild, error) {
	f.calls++
	return []OfferingReviewChild{f.child}, f.err
}
func (f *offeringEnrollmentFixture) Requests(context.Context, []int64) ([]OfferingReviewRequest, error) {
	f.calls++
	return []OfferingReviewRequest{f.request}, nil
}
func (f *offeringEnrollmentFixture) Phases(context.Context, []int64) ([]OfferingReviewPhase, error) {
	f.calls++
	return []OfferingReviewPhase{f.phase}, nil
}
func (f *offeringEnrollmentFixture) Selections(_ context.Context, dates map[int64]careplan.Date) ([]OfferingReviewBooking, error) {
	f.calls++
	f.dates = dates
	return f.bookings, nil
}

type offeringCoursesFixture struct {
	groups map[int64][]OfferingReviewCourse
	day    careplan.Date
	err    error
}

func (f *offeringCoursesFixture) CourseGroups(_ context.Context, _ []OfferingReviewCourseRef, day careplan.Date) (map[int64][]OfferingReviewCourse, error) {
	f.day = day
	return f.groups, f.err
}

func TestOfferingNativeReviewScopeCursorDatesDiffAndCount(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	studentID, childID, accountID := insertOfferingChangeFixture(t, db, ctx, testpkg.Tenant(t))
	hiddenID, hiddenChildID, hiddenAccountID := insertOfferingChangeFixture(t, db, ctx, testpkg.Tenant(t))
	var facts struct {
		PersonID  int64
		RequestID int64
		PhaseID   int64
	}
	err := tenant.WithinTenant(ctx, mustTenantID(t, testpkg.Tenant(t)), func(txCtx context.Context) error {
		return transactionDB(t, txCtx).NewRaw(`SELECT s.person_id,c.request_id,r.phase_id FROM users.students s JOIN enrollment.request_children c ON c.created_student_id=s.id JOIN enrollment.requests r ON r.id=c.request_id WHERE s.id=? AND c.id=?`, studentID, childID).Scan(txCtx, &facts)
	})
	require.NoError(t, err)
	offering := createOffering(t, ctx, module, offeringFields(facts.PhaseID, "Native course"))
	payload := json.RawMessage(fmt.Sprintf(`{"offerings":[{"offering_id":%d}]}`, offering.ID))
	create := func(student, child, account int64) careplan.OfferingChangeRequest {
		row, err := module.CreateOfferingChange(ctx, careplan.OfferingChangeRequest{StudentID: student, RequestChildID: child, SubmittedBy: account, Payload: payload, EffectiveFrom: "2030-08-01", Status: "pending"})
		require.NoError(t, err)
		return row
	}
	visible := create(studentID, childID, accountID)
	hidden := create(hiddenID, hiddenChildID, hiddenAccountID)
	group := testpkg.CreateTestEducationGroup(t, db, "Offering review")
	directory := offeringDirectoryFixture{reviewDirectoryFixture{
		students: map[int64]ReviewStudent{studentID: {ID: studentID, PersonID: facts.PersonID, GroupID: &group.ID}, hiddenID: {ID: hiddenID}},
		names:    map[int64]PersonName{facts.PersonID: {FirstName: "Care", LastName: "Change"}},
	}}
	grade := int16(2)
	enrollment := &offeringEnrollmentFixture{
		child:   OfferingReviewChild{ID: childID, RequestID: facts.RequestID, Grade: &grade, SchoolClass: " 2A "},
		request: OfferingReviewRequest{ID: facts.RequestID, PhaseID: facts.PhaseID},
		phase:   OfferingReviewPhase{ID: facts.PhaseID, Start: "2030-09-01", End: "2031-07-31", SelectionMode: "optional"},
	}
	courses := &offeringCoursesFixture{groups: map[int64][]OfferingReviewCourse{offering.ID: {{Active: true, Grades: []int{2}, SchoolClasses: []string{"2a"}}}}}
	deps := OfferingReviewDependencies{People: directory, Enrollment: enrollment, Courses: courses, Scope: func(context.Context) (ReviewScope, error) { return ReviewScope{GroupIDs: []int64{group.ID}}, nil }, Today: func() careplan.Date { return "2030-08-15" }}
	query, err := NewOfferingReviews(db, func(Observation) {}, deps)
	require.NoError(t, err)
	rows, cursor, err := query.ListPending(ctx, careplan.RequestQueueFilter{Limit: 1})
	require.NoError(t, err)
	require.Empty(t, rows)
	require.NotNil(t, cursor)
	require.Equal(t, hidden.ID, cursor.ID)
	rows, next, err := query.ListPending(ctx, careplan.RequestQueueFilter{Limit: 1, BeforeID: cursor.ID, BeforeInstant: cursor.UpdatedAt, UrgentDate: "2030-08-20"})
	require.NoError(t, err)
	require.Nil(t, next)
	require.Len(t, rows, 1)
	require.Equal(t, visible.ID, rows[0].Request.ID)
	require.Equal(t, "Care Change", rows[0].StudentName)
	require.Equal(t, "2030-08-01", rows[0].RequestedEffectiveFrom)
	require.Equal(t, "2030-09-01", rows[0].Request.EffectiveFrom)
	require.Equal(t, "2030-09-01", rows[0].EarliestEffectiveFrom)
	require.Equal(t, "2031-07-31", rows[0].LatestEffectiveFrom)
	require.Equal(t, careplan.Date("2030-09-01"), enrollment.dates[childID])
	require.Equal(t, careplan.Date("2030-08-20"), courses.day)
	require.Len(t, rows[0].Diff, 1)
	require.True(t, rows[0].Diff[0].IsCourse)
	require.Equal(t, "not_booked", rows[0].Diff[0].OldState)
	require.Equal(t, "booked", rows[0].Diff[0].NewState)
	beforeCount := enrollment.calls
	count, err := query.PendingCount(ctx, careplan.Date("2030-08-20"))
	require.NoError(t, err)
	require.Equal(t, 1, count)
	require.Equal(t, beforeCount, enrollment.calls, "badge count never loads enrollment aggregates")
	rows, _, err = query.ListPending(ctx, careplan.RequestQueueFilter{Search: "no match"})
	require.NoError(t, err)
	require.Empty(t, rows)
	courses.err = errors.New("course preview unavailable")
	rows, _, err = query.ListPending(ctx, careplan.RequestQueueFilter{})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Empty(t, rows[0].Diff)
	require.Empty(t, rows[0].EarliestEffectiveFrom, "global optional diff failures also discard phase bounds")
	courses.err = nil
	departed := directory.students[studentID]
	departed.EnrolledUntil = "2030-08-19"
	directory.students[studentID] = departed
	rows, _, err = query.ListPending(ctx, careplan.RequestQueueFilter{UrgentDate: "2030-08-20"})
	require.NoError(t, err)
	require.Empty(t, rows, "visibility uses the shared review day")
	require.NoError(t, module.DecideOfferingChange(ctx, careplan.DecideOfferingChange{ID: visible.ID, Status: "rejected"}))
	require.NoError(t, module.UpdateOfferingChangeSnapshot(ctx, visible.ID, json.RawMessage(fmt.Sprintf(`{"diff":[{"offering_id":%d,"label":"Frozen course","old_state":"not_booked","new_state":"booked","is_course":true}]}`, offering.ID))))
	history, _, err := query.ListHistory(ctx, careplan.RequestQueueFilter{})
	require.NoError(t, err)
	require.Len(t, history, 1, "care-ended children remain in history")
	require.Equal(t, "Frozen course", history[0].Diff[0].Label)
	require.True(t, history[0].Diff[0].IsCourse)
	require.NoError(t, module.DecideOfferingChange(ctx, careplan.DecideOfferingChange{ID: hidden.ID, Status: "withdrawn"}))
	history, cursor, err = query.ListHistory(ctx, careplan.RequestQueueFilter{Limit: 1})
	require.NoError(t, err)
	require.Empty(t, history)
	require.NotNil(t, cursor, "history cursor precedes scope filtering")
	deps.Courses = nil
	_, err = NewOfferingReviews(db, func(Observation) {}, deps)
	require.Error(t, err)
}
