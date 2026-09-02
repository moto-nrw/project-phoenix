package active

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
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
		student *userModels.Student
		want    bool
	}{
		{name: "inside enrollment interval", student: &userModels.Student{EnrolledFrom: &before, EnrolledUntil: &after}, want: true},
		{name: "before enrollment", student: &userModels.Student{EnrolledFrom: &after, Status: userModels.StudentStatusPending}},
		{name: "immediately active before enrollment", student: &userModels.Student{EnrolledFrom: &after, Status: userModels.StudentStatusActive}, want: true},
		{name: "after enrollment", student: &userModels.Student{EnrolledUntil: &before, Status: userModels.StudentStatusActive}},
		{name: "inactive legacy student", student: &userModels.Student{Status: userModels.StudentStatusInactive}},
		{name: "active legacy student", student: &userModels.Student{Status: userModels.StudentStatusActive}, want: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, studentEnrolledOn(test.student, date, date))
		})
	}
}

func TestFilterOverviewStudentIDsKeepsUnresolvedPeopleWithoutNameFilter(t *testing.T) {
	t.Parallel()

	students := map[int64]*userModels.Student{
		1: {ID: 1, PersonID: 11},
		2: {ID: 2, PersonID: 22},
	}
	persons := map[int64]*userModels.Person{11: {ID: 11, FirstName: "Mia", LastName: "Muster"}}

	assert.Equal(t, []int64{1, 2}, filterOverviewStudentIDs([]int64{1, 2}, students, persons, ""))
	assert.Equal(t, []int64{1}, filterOverviewStudentIDs([]int64{1, 2}, students, persons, "mia"))
}

type cappedOverviewRepository struct {
	activeModels.StudentStatusDayOverviewRepository
	total       int
	listCalled  bool
	listOptions *modelBase.QueryOptions
}

func (r *cappedOverviewRepository) CountWithOptions(context.Context, *modelBase.QueryOptions) (int, error) {
	return r.total, nil
}

func (r *cappedOverviewRepository) ListOverviewWithOptions(_ context.Context, options *modelBase.QueryOptions, _ []int64) ([]*activeModels.StudentStatusDay, error) {
	r.listCalled = true
	r.listOptions = options
	return []*activeModels.StudentStatusDay{{ID: 1, StudentID: 1}}, nil
}

type overviewPeopleStub struct {
	students []*userModels.Student
	persons  map[int64]*userModels.Person
}

func (s overviewPeopleStub) GetStudentsByGroupIDs(context.Context, []int64) ([]*userModels.Student, error) {
	return s.students, nil
}

func (s overviewPeopleStub) GetByIDs(context.Context, []int64) (map[int64]*userModels.Person, error) {
	return s.persons, nil
}

func TestGetOverviewPaginatesLargeResultSets(t *testing.T) {
	t.Parallel()

	date := timezone.NewDate(2026, 8, 20)
	repo := &cappedOverviewRepository{total: 10_001}
	people := overviewPeopleStub{
		students: []*userModels.Student{{ID: 1, PersonID: 11, Status: userModels.StudentStatusActive}},
		persons:  map[int64]*userModels.Person{11: {ID: 11, FirstName: "Mia", LastName: "Muster"}},
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
	assert.Equal(t, modelBase.Pagination{Page: 1, PageSize: 100}, *repo.listOptions.Pagination)
}
