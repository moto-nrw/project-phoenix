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
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
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

	report := buildClassDayReport("1a", timezone.NewDate(2026, 8, 5), "Schuljahr 2026/27", rows, statuses, map[int64]string{1: "Abholung"}, nil, map[int64]string{1: "16:00"})

	require.Len(t, report.Rows, 3)
	assert.Equal(t, "wed", report.Weekday)
	assert.True(t, report.SchoolDay)
	assert.Equal(t, "Schuljahr 2026/27", report.PhaseName)

	assert.True(t, report.Rows[0].StaysToday)
	assert.Equal(t, []string{"Ganztag"}, report.Rows[0].Offerings)
	// The effective (materialized-plan) pickup time beats the form answer.
	assert.Equal(t, "16:00", report.Rows[0].Pickup)
	assert.Equal(t, "07:30", report.Rows[0].Arrival)
	// The day-specific departure replaces the roster's week summary.
	assert.Equal(t, "Abholung", report.Rows[0].Departure)
	assert.Equal(t, "Geht alleine", report.Rows[1].Departure)

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

	report := buildClassDayReport("1a", timezone.NewDate(2026, 8, 8), "", rows, nil, nil, nil, nil)

	assert.False(t, report.SchoolDay)
	assert.Equal(t, "", report.Weekday)
	require.Len(t, report.Rows, 1)
	assert.False(t, report.Rows[0].StaysToday)
	// Kein Schultag: nur der Klassenverband ist eine ehrliche Zahl.
	assert.Equal(t, ClassDayTotals{Students: 1}, report.Totals)
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

func TestClassDayPhasesReturnsEveryCoveringPhaseActiveFirst(t *testing.T) {
	svc := &reportService{ReportServiceConfig: ReportServiceConfig{PhaseRepo: &fakeClassDayPhaseRepo{phases: []*enrollmentModels.Phase{
		{Model: baseModels.Model{ID: 1}, Name: "Altjahr", ServiceStartDate: timezone.NewDate(2025, 8, 1), ServiceEndDate: timezone.NewDate(2026, 7, 31)},
		{Model: baseModels.Model{ID: 2}, Name: "Schuljahr", IsActive: true, ServiceStartDate: timezone.NewDate(2026, 8, 1), ServiceEndDate: timezone.NewDate(2027, 7, 31)},
		{Model: baseModels.Model{ID: 3}, Name: "Ferien", IsActive: true, ServiceStartDate: timezone.NewDate(2026, 8, 3), ServiceEndDate: timezone.NewDate(2026, 8, 14)},
		{Model: baseModels.Model{ID: 4}, Name: "InaktivAlt", ServiceStartDate: timezone.NewDate(2026, 8, 1), ServiceEndDate: timezone.NewDate(2027, 7, 31)},
	}}}}

	phases, err := svc.classDayPhases(context.Background(), timezone.NewDate(2026, 8, 5))

	require.NoError(t, err)
	// EVERY covering phase, not one "best": a child enrolled only in the
	// Schuljahr phase must not vanish because a Ferien phase also covers
	// the date. Active first, then latest start.
	require.Len(t, phases, 3)
	assert.Equal(t, []int64{3, 2, 4}, []int64{phases[0].id, phases[1].id, phases[2].id})
}

func TestClassDayPhasesNoneCovering(t *testing.T) {
	svc := &reportService{ReportServiceConfig: ReportServiceConfig{PhaseRepo: &fakeClassDayPhaseRepo{phases: []*enrollmentModels.Phase{
		{Model: baseModels.Model{ID: 1}, ServiceStartDate: timezone.NewDate(2025, 8, 1), ServiceEndDate: timezone.NewDate(2026, 7, 31)},
	}}}}

	phases, err := svc.classDayPhases(context.Background(), timezone.NewDate(2026, 8, 5))

	require.NoError(t, err)
	assert.Empty(t, phases)
}

func TestMergeClassDayRostersUnionsRegistrations(t *testing.T) {
	schoolYear := []ClassRosterRow{
		{StudentID: 1, LastName: "Anders", Registered: true, EnrollmentSummary: "Ganztag", OfferingsByDay: map[string][]string{"wed": {"Ganztag"}}, PickupByDay: map[string]string{"wed": "16:00"}},
		{StudentID: 2, LastName: "Becker", Registered: false, EnrollmentSummary: "Keine Anmeldung"},
	}
	ferien := []ClassRosterRow{
		{StudentID: 1, LastName: "Anders", Registered: false, EnrollmentSummary: "Keine Anmeldung"},
		{StudentID: 2, LastName: "Becker", Registered: true, EnrollmentSummary: "Ferienbetreuung", OfferingsByDay: map[string][]string{"wed": {"Ferienbetreuung"}}},
	}

	merged := mergeClassDayRosters([][]ClassRosterRow{schoolYear, ferien})

	require.Len(t, merged, 2)
	// Registered in EITHER covering phase counts.
	assert.True(t, merged[0].Registered)
	assert.Equal(t, []string{"Ganztag"}, merged[0].OfferingsByDay["wed"])
	assert.Equal(t, "16:00", merged[0].PickupByDay["wed"])
	assert.True(t, merged[1].Registered)
	assert.Equal(t, "Ferienbetreuung", merged[1].EnrollmentSummary)
	assert.Equal(t, []string{"Ferienbetreuung"}, merged[1].OfferingsByDay["wed"])
}

func TestClassDayRequiresConfiguredDependencies(t *testing.T) {
	svc := &reportService{ReportServiceConfig: ReportServiceConfig{PhaseRepo: &fakeClassDayPhaseRepo{}}}

	_, err := svc.ClassDay(context.Background(), "1a", timezone.NewDate(2026, 8, 5), 42, "lehrkraft")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
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
	wireClassDayDeps(svc)

	report, err := svc.ClassDay(context.Background(), "1a", timezone.NewDate(2026, 8, 5), 42, "lehrkraft")

	require.NoError(t, err)
	require.Len(t, report.Rows, 2)
	assert.Equal(t, "", report.PhaseName)
	assert.False(t, report.Rows[0].Registered)
	assert.Equal(t, "Anders", report.Rows[0].LastName)
	assert.Equal(t, activeModels.StudentStatusDayExcused, report.Rows[1].Status)
	// Without a covering phase the stays/leaves split is unknowable: the
	// flag says so and the counters stay zero instead of claiming everyone
	// goes home.
	assert.False(t, report.EnrollmentKnown)
	assert.Equal(t, ClassDayTotals{Students: 2, Absent: 1}, report.Totals)
}

// fakeCareDayService serves fixed care-day statuses.
type fakeCareDayService struct {
	scheduleService.CareDayService
	statuses map[int64]scheduleService.CareDayStatus
}

func (f *fakeCareDayService) ResolveForDate(_ context.Context, _ []int64, _ timezone.Date) (map[int64]scheduleService.CareDayStatus, error) {
	return f.statuses, nil
}

// fakePickupScheduleService / fakeArrivalScheduleService serve fixed
// effective times for the bulk read the class day view uses.
type fakePickupScheduleService struct {
	scheduleService.PickupScheduleService
	byStudent map[int64]*scheduleService.EffectivePickupTime
}

func (f *fakePickupScheduleService) GetBulkEffectivePickupTimesForDate(_ context.Context, _ []int64, _ timezone.Date) (map[int64]*scheduleService.EffectivePickupTime, error) {
	return f.byStudent, nil
}

type fakeArrivalScheduleService struct {
	scheduleService.ArrivalScheduleService
	byStudent map[int64]*scheduleService.EffectiveArrivalTime
}

func (f *fakeArrivalScheduleService) GetBulkEffectiveArrivalTimesForDate(_ context.Context, _ []int64, _ timezone.Date) (map[int64]*scheduleService.EffectiveArrivalTime, error) {
	return f.byStudent, nil
}

// wireClassDayDeps attaches the four now-required status/schedule deps.
func wireClassDayDeps(svc *reportService) {
	if svc.StudentStatusDayRepo == nil {
		svc.StudentStatusDayRepo = &fakeClassDayStatusRepo{}
	}
	svc.CareDaySvc = &fakeCareDayService{}
	svc.PickupScheduleSvc = &fakePickupScheduleService{}
	svc.ArrivalScheduleSvc = &fakeArrivalScheduleService{}
}

func TestClassDayCancellationsUseCareDayService(t *testing.T) {
	svc := &reportService{ReportServiceConfig: ReportServiceConfig{CareDaySvc: &fakeCareDayService{statuses: map[int64]scheduleService.CareDayStatus{
		1: scheduleService.CareDayCancelled,
		2: scheduleService.CareDayScheduled,
		3: scheduleService.CareDayNotScheduled,
	}}}}

	cancelled, err := svc.classDayCancellations(context.Background(), []int64{1, 2, 3}, timezone.NewDate(2026, 8, 5))

	require.NoError(t, err)
	assert.Equal(t, map[int64]bool{1: true}, cancelled)
}

// capturingChildOfferingRepo records the date the offering links were
// requested for.
type capturingChildOfferingRepo struct {
	enrollmentModels.RequestChildOfferingRepository
	seenDate timezone.Date
}

func (r *capturingChildOfferingRepo) ListByRequestChildIDsAtDate(_ context.Context, _ []int64, date timezone.Date) ([]*enrollmentModels.RequestChildOffering, error) {
	r.seenDate = date
	return nil, nil
}

func TestClassRosterOfferingDatePinsSelection(t *testing.T) {
	svc := classRosterTestService(
		[]*userModels.Student{{Model: baseModels.Model{ID: 1}, PersonID: 11, SchoolClass: "1a"}},
		map[int64]*userModels.Person{11: {FirstName: "Mila", LastName: "Anders"}},
		&fakeClassRosterRequestRepo{},
		&fakeClassRosterChildRepo{},
	)
	capture := &capturingChildOfferingRepo{}
	svc.RequestChildOfferingRepo = capture
	pinned := timezone.NewDate(2026, 8, 10)

	_, err := svc.ClassRoster(context.Background(), ClassRosterFilters{PhaseID: 55, SchoolClass: "1a", OfferingDate: &pinned})

	require.NoError(t, err)
	assert.Equal(t, pinned, capture.seenDate)
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

func TestClassDayDepartureRendersSingleDay(t *testing.T) {
	pickupAndBus := &userModels.Student{
		AllowedDepartureModes: userModels.AllowedDepartureModes{
			"fri": {userModels.DepartureBus, userModels.DeparturePickup},
		},
	}
	assert.Equal(t, "Bus, Abholung", classDayDeparture(pickupAndBus, "fri", nil))
	// A day without any allowed mode falls back to walking home alone.
	assert.Equal(t, "Geht alleine", classDayDeparture(pickupAndBus, "mon", nil))

	note := "mit Bruder Max"
	accompanied := &userModels.Student{
		AllowedDepartureModes: userModels.AllowedDepartureModes{
			"fri": {userModels.DepartureAccompanied},
		},
		DepartureCompanionNote: &note,
	}
	assert.Equal(t, "Mit anderem Kind (mit Bruder Max)", classDayDeparture(accompanied, "fri", nil))
}

func TestClassDayRequiresSchoolClass(t *testing.T) {
	svc := &reportService{}

	_, err := svc.ClassDay(context.Background(), "  ", timezone.NewDate(2026, 8, 5), 42, "lehrkraft")

	require.ErrorIs(t, err, ErrReportInvalidFilter)
}
