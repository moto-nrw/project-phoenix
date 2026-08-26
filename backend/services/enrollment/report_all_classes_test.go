package enrollment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	baseModels "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// fakeAllClassesStudentRepo serves per-class student sets, mimicking the real
// repo where ListSchoolClasses returns distinct non-empty classes in byte
// order (so "10a" comes before "1a" here — the service must re-sort).
type fakeAllClassesStudentRepo struct {
	userModels.StudentRepository
	students []*userModels.Student
}

func (r *fakeAllClassesStudentRepo) ListWithOptions(_ context.Context, _ *baseModels.QueryOptions) ([]*userModels.Student, error) {
	return r.students, nil
}

func allClassesTestService() *reportService {
	students := []*userModels.Student{
		{Model: baseModels.Model{ID: 3}, PersonID: 13, SchoolClass: "10a"},
		{Model: baseModels.Model{ID: 2}, PersonID: 12, SchoolClass: "1a"},
		{Model: baseModels.Model{ID: 1}, PersonID: 11, SchoolClass: "1a"},
		{Model: baseModels.Model{ID: 4}, PersonID: 14, SchoolClass: "2b"},
	}
	persons := map[int64]*userModels.Person{
		11: {FirstName: "Mila", LastName: "Anders"},
		12: {FirstName: "Finn", LastName: "Becker"},
		13: {FirstName: "Ida", LastName: "Conrad"},
		14: {FirstName: "Emma", LastName: "Dreyer"},
	}
	svc := classRosterTestService(nil, persons, &fakeClassRosterRequestRepo{}, &fakeClassRosterChildRepo{})
	svc.StudentRepo = &fakeAllClassesStudentRepo{students: students}
	return svc
}

func TestClassRosterAllClassesSortsClassFirstThenName(t *testing.T) {
	t.Parallel()

	svc := allClassesTestService()

	report, err := svc.ClassRoster(context.Background(), ClassRosterFilters{PhaseID: 55, AllClasses: true})

	require.NoError(t, err)
	require.Len(t, report.Rows, 4)
	got := make([][2]string, 0, len(report.Rows))
	for _, row := range report.Rows {
		got = append(got, [2]string{row.SchoolClass, row.LastName})
	}
	assert.Equal(t, [][2]string{
		{"1a", "Anders"},
		{"1a", "Becker"},
		{"2b", "Dreyer"},
		{"10a", "Conrad"},
	}, got)
	assert.True(t, report.Filters.AllClasses)
	assert.Equal(t, 4, report.Totals.Students)
}

// fakeCaseVariantStudentRepo ensures one bulk read cannot duplicate a student
// merely because class labels differ in case.
type fakeCaseVariantStudentRepo struct {
	userModels.StudentRepository
	students  []*userModels.Student
	listCalls int
}

func (r *fakeCaseVariantStudentRepo) ListWithOptions(_ context.Context, _ *baseModels.QueryOptions) ([]*userModels.Student, error) {
	r.listCalls++
	return r.students, nil
}

func TestClassRosterAllClassesDeduplicatesCaseVariantClasses(t *testing.T) {
	t.Parallel()

	persons := map[int64]*userModels.Person{
		11: {FirstName: "Mila", LastName: "Anders"},
		12: {FirstName: "Finn", LastName: "Becker"},
		13: {FirstName: "Ida", LastName: "Conrad"},
	}
	svc := classRosterTestService(nil, persons, &fakeClassRosterRequestRepo{}, &fakeClassRosterChildRepo{})
	repo := &fakeCaseVariantStudentRepo{
		students: []*userModels.Student{
			{Model: baseModels.Model{ID: 1}, PersonID: 11, SchoolClass: "1a"},
			{Model: baseModels.Model{ID: 2}, PersonID: 12, SchoolClass: "1A"},
			{Model: baseModels.Model{ID: 3}, PersonID: 13, SchoolClass: "2b"},
		},
	}
	svc.StudentRepo = repo

	report, err := svc.ClassRoster(context.Background(), ClassRosterFilters{PhaseID: 55, AllClasses: true})

	require.NoError(t, err)
	require.Len(t, report.Rows, 3)
	assert.Equal(t, 3, report.Totals.Students)
	assert.Equal(t, 1, repo.listCalls, "all classes load in one query")
	seen := map[int64]bool{}
	for _, row := range report.Rows {
		assert.False(t, seen[row.StudentID], "student %d appears more than once", row.StudentID)
		seen[row.StudentID] = true
	}
}

func TestClassRosterFilterValidation(t *testing.T) {
	t.Parallel()

	svc := allClassesTestService()

	_, err := svc.ClassRoster(context.Background(), ClassRosterFilters{PhaseID: 55, SchoolClass: "1a", AllClasses: true})
	require.ErrorIs(t, err, ErrReportInvalidFilter)

	_, err = svc.ClassRoster(context.Background(), ClassRosterFilters{PhaseID: 55})
	require.ErrorIs(t, err, ErrReportInvalidFilter)
}
