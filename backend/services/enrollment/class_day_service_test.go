package enrollment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	baseModels "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

func TestClassDayWeekdayKey(t *testing.T) {
	assert.Equal(t, "mon", classDayWeekdayKey(timezone.NewDate(2026, 8, 3)))
	assert.Equal(t, "tue", classDayWeekdayKey(timezone.NewDate(2026, 8, 4)))
	assert.Equal(t, "wed", classDayWeekdayKey(timezone.NewDate(2026, 8, 5)))
	assert.Equal(t, "thu", classDayWeekdayKey(timezone.NewDate(2026, 8, 6)))
	assert.Equal(t, "fri", classDayWeekdayKey(timezone.NewDate(2026, 8, 7)))
	assert.Equal(t, "", classDayWeekdayKey(timezone.NewDate(2026, 8, 8)))
	assert.Equal(t, "", classDayWeekdayKey(timezone.NewDate(2026, 8, 9)))
}

func TestBuildClassDayReportProjection(t *testing.T) {
	rows := []ClassRosterRow{
		{
			StudentID:  1,
			FirstName:  "Mila",
			LastName:   "Anders",
			Registered: true,
			OfferingsByDay: map[string][]string{
				"wed": {"Ganztag"},
			},
			PickupByDay:  map[string]string{"wed": "15:00"},
			ArrivalByDay: map[string]string{"wed": "07:30"},
			Departure:    "Wird abgeholt",
		},
		{
			StudentID:  2,
			FirstName:  "Finn",
			LastName:   "Becker",
			Registered: false,
			Departure:  "Geht alleine",
		},
		{
			StudentID:  3,
			FirstName:  "Ida",
			LastName:   "Conrad",
			Registered: true,
			OfferingsByDay: map[string][]string{
				"wed": {"Randstunde"},
			},
		},
	}
	statuses := map[int64]string{3: activeModels.StudentStatusDaySick}

	report := buildClassDayReport("1a", timezone.NewDate(2026, 8, 5), "Schuljahr 2026/27", rows, statuses)

	require.Len(t, report.Rows, 3)
	assert.Equal(t, "wed", report.Weekday)
	assert.True(t, report.SchoolDay)
	assert.Equal(t, "Schuljahr 2026/27", report.PhaseName)

	assert.True(t, report.Rows[0].StaysToday)
	assert.Equal(t, []string{"Ganztag"}, report.Rows[0].Offerings)
	assert.Equal(t, "15:00", report.Rows[0].Pickup)
	assert.Equal(t, "07:30", report.Rows[0].Arrival)
	assert.Equal(t, "Wird abgeholt", report.Rows[0].Departure)

	assert.False(t, report.Rows[1].StaysToday)
	assert.Empty(t, report.Rows[1].Offerings)

	// A reported sick day wins over the enrollment: the student is absent,
	// not staying, even though the weekday has an offering.
	assert.False(t, report.Rows[2].StaysToday)
	assert.Equal(t, activeModels.StudentStatusDaySick, report.Rows[2].Status)

	assert.Equal(t, ClassDayTotals{Students: 3, Staying: 1, Leaving: 1, Absent: 1}, report.Totals)
}

func TestBuildClassDayReportWeekend(t *testing.T) {
	rows := []ClassRosterRow{{
		StudentID:      1,
		Registered:     true,
		OfferingsByDay: map[string][]string{"mon": {"Ganztag"}},
	}}

	report := buildClassDayReport("1a", timezone.NewDate(2026, 8, 8), "", rows, nil)

	assert.False(t, report.SchoolDay)
	assert.Equal(t, "", report.Weekday)
	require.Len(t, report.Rows, 1)
	assert.False(t, report.Rows[0].StaysToday)
	assert.Equal(t, ClassDayTotals{Students: 1, Leaving: 1}, report.Totals)
}

// fakeClassDayPhaseRepo serves a fixed phase list for classDayPhase.
type fakeClassDayPhaseRepo struct {
	enrollmentModels.PhaseRepository
	phases []*enrollmentModels.Phase
}

func (r *fakeClassDayPhaseRepo) ListByTenant(_ context.Context) ([]*enrollmentModels.Phase, error) {
	return r.phases, nil
}

func (r *fakeClassDayPhaseRepo) FindByID(_ context.Context, id int64) (*enrollmentModels.Phase, error) {
	for _, phase := range r.phases {
		if phase != nil && phase.ID == id {
			return phase, nil
		}
	}
	return nil, ErrReportPhaseNotFound
}

// fakeClassDayStatusRepo serves fixed status-day rows.
type fakeClassDayStatusRepo struct {
	activeModels.StudentStatusDayRepository
	entries []*activeModels.StudentStatusDay
}

func (r *fakeClassDayStatusRepo) FindActiveByStudentIDsAndDate(_ context.Context, _ []int64, _ timezone.Date) ([]*activeModels.StudentStatusDay, error) {
	return r.entries, nil
}

func TestClassDayPhasePrefersActiveCoveringPhase(t *testing.T) {
	svc := &reportService{ReportServiceConfig: ReportServiceConfig{PhaseRepo: &fakeClassDayPhaseRepo{phases: []*enrollmentModels.Phase{
		{Model: baseModels.Model{ID: 1}, Name: "Altjahr", ServiceStartDate: timezone.NewDate(2025, 8, 1), ServiceEndDate: timezone.NewDate(2026, 7, 31)},
		{Model: baseModels.Model{ID: 2}, Name: "Inaktiv", ServiceStartDate: timezone.NewDate(2026, 8, 1), ServiceEndDate: timezone.NewDate(2027, 7, 31)},
		{Model: baseModels.Model{ID: 3}, Name: "Aktuell", IsActive: true, ServiceStartDate: timezone.NewDate(2026, 8, 1), ServiceEndDate: timezone.NewDate(2027, 7, 31)},
	}}}}

	phaseID, phaseName, err := svc.classDayPhase(context.Background(), timezone.NewDate(2026, 8, 5))

	require.NoError(t, err)
	assert.Equal(t, int64(3), phaseID)
	assert.Equal(t, "Aktuell", phaseName)
}

func TestClassDayPhaseNoneCovering(t *testing.T) {
	svc := &reportService{ReportServiceConfig: ReportServiceConfig{PhaseRepo: &fakeClassDayPhaseRepo{phases: []*enrollmentModels.Phase{
		{Model: baseModels.Model{ID: 1}, ServiceStartDate: timezone.NewDate(2025, 8, 1), ServiceEndDate: timezone.NewDate(2026, 7, 31)},
	}}}}

	phaseID, phaseName, err := svc.classDayPhase(context.Background(), timezone.NewDate(2026, 8, 5))

	require.NoError(t, err)
	assert.Equal(t, int64(0), phaseID)
	assert.Equal(t, "", phaseName)
}

func TestClassDayWithoutPhaseListsFullClass(t *testing.T) {
	svc := classRosterTestService(
		[]*userModels.Student{
			{Model: baseModels.Model{ID: 1}, PersonID: 11, SchoolClass: "1a"},
			{Model: baseModels.Model{ID: 2}, PersonID: 12, SchoolClass: "1a"},
		},
		map[int64]*userModels.Person{
			11: {FirstName: "Mila", LastName: "Anders"},
			12: {FirstName: "Finn", LastName: "Becker"},
		},
		&fakeClassRosterRequestRepo{},
		&fakeClassRosterChildRepo{},
	)
	svc.PhaseRepo = &fakeClassDayPhaseRepo{}
	svc.StudentStatusDayRepo = &fakeClassDayStatusRepo{entries: []*activeModels.StudentStatusDay{
		{StudentID: 2, Status: activeModels.StudentStatusDayExcused},
	}}

	report, err := svc.ClassDay(context.Background(), "1a", timezone.NewDate(2026, 8, 5), 42)

	require.NoError(t, err)
	require.Len(t, report.Rows, 2)
	assert.Equal(t, "", report.PhaseName)
	assert.False(t, report.Rows[0].Registered)
	assert.Equal(t, "Anders", report.Rows[0].LastName)
	assert.Equal(t, activeModels.StudentStatusDayExcused, report.Rows[1].Status)
	assert.Equal(t, ClassDayTotals{Students: 2, Leaving: 1, Absent: 1}, report.Totals)
}

func TestClassDayStatusPrecedenceSickWins(t *testing.T) {
	svc := &reportService{ReportServiceConfig: ReportServiceConfig{StudentStatusDayRepo: &fakeClassDayStatusRepo{entries: []*activeModels.StudentStatusDay{
		{StudentID: 1, Status: activeModels.StudentStatusDayExcused},
		{StudentID: 1, Status: activeModels.StudentStatusDaySick},
		{StudentID: 2, Status: activeModels.StudentStatusDayClassTrip},
		{StudentID: 2, Status: activeModels.StudentStatusDayExcused},
	}}}}

	statuses, err := svc.classDayStatuses(context.Background(), []int64{1, 2}, timezone.NewDate(2026, 8, 5))

	require.NoError(t, err)
	assert.Equal(t, activeModels.StudentStatusDaySick, statuses[1])
	assert.Equal(t, activeModels.StudentStatusDayClassTrip, statuses[2])
}

func TestClassDayRequiresSchoolClass(t *testing.T) {
	svc := &reportService{}

	_, err := svc.ClassDay(context.Background(), "  ", timezone.NewDate(2026, 8, 5), 42)

	require.ErrorIs(t, err, ErrReportInvalidFilter)
}
