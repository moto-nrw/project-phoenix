package compose

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type reviewDirectoryFixture struct {
	students map[int64]ReviewStudent
	names    map[int64]PersonName
}

func (d reviewDirectoryFixture) ReviewFields(_ context.Context, changes []MasterDataFieldChange) (map[int64]MasterDataFieldFacts, error) {
	facts := make(map[int64]MasterDataFieldFacts, len(changes))
	for _, change := range changes {
		student, found := d.students[change.StudentID]
		if !found {
			continue
		}
		name := d.names[student.PersonID]
		facts[change.RequestID] = MasterDataFieldFacts{Student: &student, FirstName: name.FirstName, LastName: name.LastName, BulkEligible: true}
	}
	return facts, nil
}

func (d reviewDirectoryFixture) FindStudents(context.Context, []int64) (map[int64]ReviewStudent, error) {
	return d.students, nil
}
func (d reviewDirectoryFixture) PersonNames(context.Context, []int64) (map[int64]PersonName, error) {
	return d.names, nil
}
func (d reviewDirectoryFixture) ReviewerNames(context.Context, []int64) (map[int64]PersonName, error) {
	return map[int64]PersonName{}, nil
}

func TestMasterDataNativeQueuePreservesCursorScopeAndCareEnd(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, "parent")
	visible := testpkg.CreateTestStudent(t, db, "Visible", "Review", "1a")
	hidden := testpkg.CreateTestStudent(t, db, "Hidden", "Review", "1a")
	group := testpkg.CreateTestEducationGroup(t, db, "ReviewScope")
	create := func(studentID int64) careplan.StudentDataChangeRequest {
		row, err := module.CreateStudentDataRequest(ctx, careplan.StudentDataChangeRequest{
			StudentID: studentID, SubmittedBy: account.ID, Target: "person", FieldKey: "first_name",
			OldValue: json.RawMessage(`"Visible"`), NewValue: json.RawMessage(`"Changed"`), Status: "pending",
		})
		require.NoError(t, err)
		return row
	}
	first := create(visible.ID)
	last := create(hidden.ID)
	directory := reviewDirectoryFixture{
		students: map[int64]ReviewStudent{
			visible.ID: {ID: visible.ID, PersonID: visible.PersonID, GroupID: &group.ID},
			hidden.ID:  {ID: hidden.ID, PersonID: hidden.PersonID},
		},
		names: map[int64]PersonName{visible.PersonID: {FirstName: "Visible", LastName: "Review"}},
	}
	reads, err := NewStudentDataRequestQueries(db, func(Observation) {})
	require.NoError(t, err)
	queue, err := NewMasterDataReviews(MasterDataReviewDependencies{
		Requests: reads, People: directory,
		Scope: func(context.Context) (ReviewScope, error) { return ReviewScope{GroupIDs: []int64{group.ID}}, nil },
		Today: func() careplan.Date { return "2026-09-12" },
	})
	require.NoError(t, err)
	items, cursor, err := queue.ListPending(ctx, careplan.RequestQueueFilter{Limit: 1})
	require.NoError(t, err)
	require.Empty(t, items, "newest request is outside the caller's group")
	require.NotNil(t, cursor, "scope filtering must not discard the storage cursor")
	require.Equal(t, last.ID, cursor.ID)
	items, next, err := queue.ListPending(ctx, careplan.RequestQueueFilter{Limit: 1, BeforeID: cursor.ID, BeforeInstant: cursor.UpdatedAt})
	require.NoError(t, err)
	require.Nil(t, next)
	require.Len(t, items, 1)
	require.Equal(t, first.ID, items[0].Request.ID)
	require.True(t, items[0].BulkEligible)

	student := directory.students[visible.ID]
	student.EnrolledUntil = "2026-09-11"
	directory.students[visible.ID] = student
	items, _, err = queue.ListPending(ctx, careplan.RequestQueueFilter{})
	require.NoError(t, err)
	require.Empty(t, items)
	items, _, err = queue.ListPending(ctx, careplan.RequestQueueFilter{UrgentDate: "2026-09-11"})
	require.NoError(t, err)
	require.Len(t, items, 1, "the shared request day takes precedence over the clock")
	student.Alumnus = true
	directory.students[visible.ID] = student
	items, _, err = queue.ListPending(ctx, careplan.RequestQueueFilter{UrgentDate: "2026-09-11"})
	require.NoError(t, err)
	require.Empty(t, items)

	student.Alumnus = false
	directory.students[visible.ID] = student
	require.NoError(t, module.DecideStudentDataRequest(ctx, careplan.StudentDataRequestDecision{
		ID: first.ID, Status: "approved", ReviewedBy: account.ID,
	}))
	history, _, err := queue.ListHistory(ctx, careplan.RequestQueueFilter{})
	require.NoError(t, err)
	require.Len(t, history, 1, "history retains a child whose care has ended")
	require.Equal(t, first.ID, history[0].Request.ID)
	require.Equal(t, "Visible", history[0].FirstName)
	require.Equal(t, "Unbekannt", history[0].ReviewerName)
	require.True(t, history[0].Request.CanCorrect())
	student.Alumnus = true
	directory.students[visible.ID] = student
	history, _, err = queue.ListHistory(ctx, careplan.RequestQueueFilter{})
	require.NoError(t, err)
	require.Empty(t, history, "history must still exclude former pupils")
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, _, err = queue.ListPending(cancelled, careplan.RequestQueueFilter{})
	require.ErrorIs(t, err, context.Canceled)
}
