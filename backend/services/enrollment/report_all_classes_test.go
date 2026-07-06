package enrollment

import (
	"context"
	"strings"
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
	byClass map[string][]*userModels.Student
	classes []string
}

func (r *fakeAllClassesStudentRepo) ListSchoolClasses(_ context.Context) ([]string, error) {
	return r.classes, nil
}

func (r *fakeAllClassesStudentRepo) FindBySchoolClass(_ context.Context, schoolClass string) ([]*userModels.Student, error) {
	return r.byClass[schoolClass], nil
}

func allClassesTestService() *reportService {
	students := map[string][]*userModels.Student{
		"10a": {{Model: baseModels.Model{ID: 3}, PersonID: 13, SchoolClass: "10a"}},
		"1a": {
			{Model: baseModels.Model{ID: 2}, PersonID: 12, SchoolClass: "1a"},
			{Model: baseModels.Model{ID: 1}, PersonID: 11, SchoolClass: "1a"},
		},
		"2b": {{Model: baseModels.Model{ID: 4}, PersonID: 14, SchoolClass: "2b"}},
	}
	persons := map[int64]*userModels.Person{
		11: {FirstName: "Mila", LastName: "Anders"},
		12: {FirstName: "Finn", LastName: "Becker"},
		13: {FirstName: "Ida", LastName: "Conrad"},
		14: {FirstName: "Emma", LastName: "Dreyer"},
	}
	svc := classRosterTestService(nil, persons, &fakeClassRosterRequestRepo{}, &fakeClassRosterChildRepo{})
	svc.StudentRepo = &fakeAllClassesStudentRepo{
		byClass: students,
		classes: []string{"10a", "1a", "2b"},
	}
	return svc
}

func TestClassRosterAllClassesSortsClassFirstThenName(t *testing.T) {
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

// fakeCaseVariantStudentRepo mimics the real repo: ListSchoolClasses returns
// case-sensitive DISTINCT labels while FindBySchoolClass matches LOWER(TRIM).
type fakeCaseVariantStudentRepo struct {
	userModels.StudentRepository
	byClassKey map[string][]*userModels.Student // keyed by lower(trim(label))
	classes    []string
	findCalls  int
}

func (r *fakeCaseVariantStudentRepo) ListSchoolClasses(_ context.Context) ([]string, error) {
	return r.classes, nil
}

func (r *fakeCaseVariantStudentRepo) FindBySchoolClass(_ context.Context, schoolClass string) ([]*userModels.Student, error) {
	r.findCalls++
	return r.byClassKey[strings.ToLower(strings.TrimSpace(schoolClass))], nil
}

func TestClassRosterAllClassesDeduplicatesCaseVariantClasses(t *testing.T) {
	persons := map[int64]*userModels.Person{
		11: {FirstName: "Mila", LastName: "Anders"},
		12: {FirstName: "Finn", LastName: "Becker"},
		13: {FirstName: "Ida", LastName: "Conrad"},
	}
	svc := classRosterTestService(nil, persons, &fakeClassRosterRequestRepo{}, &fakeClassRosterChildRepo{})
	repo := &fakeCaseVariantStudentRepo{
		byClassKey: map[string][]*userModels.Student{
			"1a": {
				{Model: baseModels.Model{ID: 1}, PersonID: 11, SchoolClass: "1a"},
				{Model: baseModels.Model{ID: 2}, PersonID: 12, SchoolClass: "1A"},
			},
			"2b": {{Model: baseModels.Model{ID: 3}, PersonID: 13, SchoolClass: "2b"}},
		},
		classes: []string{"1A", "1a", "2b"},
	}
	svc.StudentRepo = repo

	report, err := svc.ClassRoster(context.Background(), ClassRosterFilters{PhaseID: 55, AllClasses: true})

	require.NoError(t, err)
	require.Len(t, report.Rows, 3)
	assert.Equal(t, 3, report.Totals.Students)
	assert.Equal(t, 2, repo.findCalls, "one FindBySchoolClass call per logical class")
	seen := map[int64]bool{}
	for _, row := range report.Rows {
		assert.False(t, seen[row.StudentID], "student %d appears more than once", row.StudentID)
		seen[row.StudentID] = true
	}
}

func TestClassRosterFilterValidation(t *testing.T) {
	svc := allClassesTestService()

	_, err := svc.ClassRoster(context.Background(), ClassRosterFilters{PhaseID: 55, SchoolClass: "1a", AllClasses: true})
	require.ErrorIs(t, err, ErrReportInvalidFilter)

	_, err = svc.ClassRoster(context.Background(), ClassRosterFilters{PhaseID: 55})
	require.ErrorIs(t, err, ErrReportInvalidFilter)
}
