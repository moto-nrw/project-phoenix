package statistics

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
)

type courseRepoStub struct {
	instances     []scheduleModels.CourseInstanceRow
	participation []scheduleModels.CourseParticipationRow
	instanceErr   error
	participErr   error
}

func (s courseRepoStub) CourseInstances(context.Context, timezone.Date, timezone.Date) ([]scheduleModels.CourseInstanceRow, error) {
	return s.instances, s.instanceErr
}

func (s courseRepoStub) CourseParticipation(context.Context, timezone.Date, timezone.Date) ([]scheduleModels.CourseParticipationRow, error) {
	return s.participation, s.participErr
}

func courseFilters() Filters {
	return Filters{From: timezone.NewDate(2026, 6, 1), To: timezone.NewDate(2026, 6, 30)}
}

func courseFixture() courseRepoStub {
	return courseRepoStub{
		instances: []scheduleModels.CourseInstanceRow{
			{CourseID: 10, Name: "Fußball", CategoryName: "AG", MaxParticipants: 4, HeldInstances: 8, CancelledInstances: 2},
			{CourseID: 20, Name: "Ärztespiel", CategoryName: "AG", HeldInstances: 5},
		},
		participation: []scheduleModels.CourseParticipationRow{
			{CourseID: 10, StudentID: 1, PresentDays: 6, AbsentDays: 2, OpenDays: 0},
			{CourseID: 10, StudentID: 2, PresentDays: 8, AbsentDays: 0, OpenDays: 0},
			{CourseID: 20, StudentID: 1, PresentDays: 3, AbsentDays: 1, OpenDays: 1},
		},
	}
}

func courseStudents() []StudentRow {
	return []StudentRow{
		{StudentID: 1, FirstName: "Emma", LastName: "Bauer", SchoolClass: "1a", GroupName: "Bärengruppe"},
		{StudentID: 2, FirstName: "Finn", LastName: "Ahrens", SchoolClass: "2b", GroupName: "Bärengruppe"},
	}
}

// The quota divides by decided slots only, cancelled occurrences are reported
// beside it and open ones stay out of both numerator and denominator.
func TestCourseSection_QuotaUsesDecidedSlotsOnly(t *testing.T) {
	t.Parallel()
	svc := &service{cfg: Config{Courses: courseFixture(), Now: fixedNow}}

	courses, _, totals, err := svc.courseSection(context.Background(), courseFilters(), courseStudents())
	require.NoError(t, err)
	require.Len(t, courses, 2)

	// "Ärztespiel" sorts before "Fußball" once the umlaut is folded.
	arzt, fussball := courses[0], courses[1]
	require.Equal(t, "Ärztespiel", arzt.Name)
	require.Equal(t, "Fußball", fussball.Name)

	assert.Equal(t, 8, fussball.HeldInstances)
	assert.Equal(t, 2, fussball.CancelledInstances)
	assert.Equal(t, 14, fussball.PresentDays)
	assert.Equal(t, 2, fussball.AbsentDays)
	require.NotNil(t, fussball.ParticipationRate)
	assert.InDelta(t, 87.5, *fussball.ParticipationRate, 0.001)

	// One open slot: 3 present of 4 decided = 75 %, not 60 % of 5.
	assert.Equal(t, 1, arzt.OpenDays)
	require.NotNil(t, arzt.ParticipationRate)
	assert.InDelta(t, 75.0, *arzt.ParticipationRate, 0.001)

	assert.Equal(t, 2, totals.StudentCount, "a child in two courses is one child")
	assert.Equal(t, 13, totals.HeldInstances)
	require.NotNil(t, totals.ParticipationRate)
	assert.InDelta(t, 85.0, *totals.ParticipationRate, 0.001)
}

// Occupancy compares enrolled children against the Teilnehmergrenze and stays
// empty for a course without one.
func TestCourseSection_OccupancyOnlyWithLimit(t *testing.T) {
	t.Parallel()
	svc := &service{cfg: Config{Courses: courseFixture(), Now: fixedNow}}

	courses, _, _, err := svc.courseSection(context.Background(), courseFilters(), courseStudents())
	require.NoError(t, err)

	assert.Nil(t, courses[0].OccupancyPercent, "no Teilnehmergrenze, no Belegung")
	require.NotNil(t, courses[1].OccupancyPercent)
	assert.InDelta(t, 50.0, *courses[1].OccupancyPercent, 0.001, "2 of 4 seats")
}

// The section reports exactly the population the attendance section reports:
// a child filtered out there must not reappear here.
func TestCourseSection_IgnoresChildrenOutsideThePopulation(t *testing.T) {
	t.Parallel()
	fixture := courseFixture()
	fixture.participation = append(fixture.participation,
		scheduleModels.CourseParticipationRow{CourseID: 10, StudentID: 999, PresentDays: 8})
	svc := &service{cfg: Config{Courses: fixture, Now: fixedNow}}

	courses, childRows, totals, err := svc.courseSection(context.Background(), courseFilters(), courseStudents())
	require.NoError(t, err)

	assert.Equal(t, 2, courses[1].StudentCount)
	assert.Equal(t, 14, courses[1].PresentDays, "the alien child's days never enter the course")
	assert.Equal(t, 2, totals.StudentCount)
	for _, row := range childRows {
		assert.NotEqual(t, int64(999), row.StudentID)
	}
}

// The child view carries one row per child and course, ordered by child.
func TestCourseSection_ChildRowsPerCourse(t *testing.T) {
	t.Parallel()
	svc := &service{cfg: Config{Courses: courseFixture(), Now: fixedNow}}

	_, childRows, _, err := svc.courseSection(context.Background(), courseFilters(), courseStudents())
	require.NoError(t, err)
	require.Len(t, childRows, 3)

	assert.Equal(t, "Ahrens", childRows[0].LastName)
	assert.Equal(t, "Fußball", childRows[0].CourseName)
	assert.Equal(t, "Bauer", childRows[1].LastName)
	assert.Equal(t, "Ärztespiel", childRows[1].CourseName, "a child's courses sort by name")
	assert.Equal(t, "Fußball", childRows[2].CourseName)
	require.NotNil(t, childRows[2].ParticipationRate)
	assert.InDelta(t, 75.0, *childRows[2].ParticipationRate, 0.001)
}

// A repository failure withholds the report instead of showing empty tables.
func TestCourseSection_RepositoryErrorSurfaces(t *testing.T) {
	t.Parallel()
	fixture := courseFixture()
	fixture.participErr = errors.New("boom")
	svc := &service{cfg: Config{Courses: fixture, Now: fixedNow}}

	_, _, _, err := svc.courseSection(context.Background(), courseFilters(), courseStudents())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load course participation")
}

// Sections limit what compute() spends time on; empty means everything.
func TestFiltersWants(t *testing.T) {
	t.Parallel()
	assert.True(t, Filters{}.wants(SectionCourses))
	assert.True(t, Filters{Sections: []Section{SectionCourses}}.wants(SectionCourses))
	assert.False(t, Filters{Sections: []Section{SectionRooms}}.wants(SectionCourses))
}
