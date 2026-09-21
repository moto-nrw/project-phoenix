package active

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStudentEnrolledOn(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, 8, 20)
	before := date.AddDays(-1)
	after := date.AddDays(1)

	tests := []struct {
		name    string
		student *StudentRecord
		want    bool
	}{
		{name: "inside enrollment interval", student: &StudentRecord{EnrolledFrom: &before, EnrolledUntil: &after}, want: true},
		{name: "before enrollment", student: &StudentRecord{EnrolledFrom: &after, Lifecycle: StudentLifecycleOther}},
		{name: "immediately active before enrollment", student: &StudentRecord{EnrolledFrom: &after, Lifecycle: StudentLifecycleActive}, want: true},
		{name: "after enrollment", student: &StudentRecord{EnrolledUntil: &before, Lifecycle: StudentLifecycleActive}},
		{name: "inactive legacy student", student: &StudentRecord{Lifecycle: StudentLifecycleInactive}},
		{name: "active legacy student", student: &StudentRecord{Lifecycle: StudentLifecycleActive}, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, studentEnrolledOn(test.student, date, date))
		})
	}
}

func TestFilterOverviewStudentIDsKeepsUnresolvedPeopleWithoutNameFilter(t *testing.T) {
	t.Parallel()

	students := map[int64]*StudentRecord{
		1: {ID: 1, PersonID: 11},
		2: {ID: 2, PersonID: 22},
	}
	persons := map[int64]*PersonName{11: {ID: 11, FirstName: "Mia", LastName: "Muster"}}

	assert.Equal(t, []int64{1, 2}, filterOverviewStudentIDs([]int64{1, 2}, students, persons, ""))
	assert.Equal(t, []int64{1}, filterOverviewStudentIDs([]int64{1, 2}, students, persons, "mia"))
}

type cappedOverviewRepository struct {
	StudentStatusDayOverviewRepository
	total       int
	listCalled  bool
	listOptions *QueryOptions
}

func (r *cappedOverviewRepository) CountWithOptions(context.Context, *QueryOptions) (int, error) {
	return r.total, nil
}

func (r *cappedOverviewRepository) ListOverviewWithOptions(_ context.Context, options *QueryOptions, _ []int64) ([]*absencerecords.StudentStatusDay, error) {
	r.listCalled = true
	r.listOptions = options
	return []*absencerecords.StudentStatusDay{{ID: 1, StudentID: 1}}, nil
}

type overviewPeopleStub struct {
	students []*StudentRecord
	persons  map[int64]*PersonName
}

func (s overviewPeopleStub) GetStudentsByGroupIDs(context.Context, []int64) ([]*StudentRecord, error) {
	return s.students, nil
}

func (s overviewPeopleStub) GetByIDs(context.Context, []int64) (map[int64]*PersonName, error) {
	return s.persons, nil
}

func TestGetOverviewPaginatesLargeResultSets(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, 8, 20)
	repo := &cappedOverviewRepository{total: 10_001}
	people := overviewPeopleStub{
		students: []*StudentRecord{{ID: 1, PersonID: 11, Lifecycle: StudentLifecycleActive}},
		persons:  map[int64]*PersonName{11: {ID: 11, FirstName: "Mia", LastName: "Muster"}},
	}

	overview, err := NewStudentStatusDayOverviewService(repo, people).GetOverview(
		context.Background(), nil, date, date, date,
		StatusDayOverviewFilters{Page: 1, PageSize: 100},
	)

	require.NoError(t, err)
	require.NotNil(t, overview)
	assert.True(t, repo.listCalled)
	require.NotNil(t, repo.listOptions)
	require.NotNil(t, repo.listOptions.Pagination)
	assert.Equal(t, Pagination{Page: 1, PageSize: 100}, *repo.listOptions.Pagination)
}
