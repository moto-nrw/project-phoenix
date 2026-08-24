package enrollment

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	baseModels "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
)

func TestClassDayWeekdayKey(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "mon", classDayWeekdayKey(timezone.NewDate(2026, 8, 3)))
	assert.Equal(t, "tue", classDayWeekdayKey(timezone.NewDate(2026, 8, 4)))
	assert.Equal(t, "wed", classDayWeekdayKey(timezone.NewDate(2026, 8, 5)))
	assert.Equal(t, "thu", classDayWeekdayKey(timezone.NewDate(2026, 8, 6)))
	assert.Equal(t, "fri", classDayWeekdayKey(timezone.NewDate(2026, 8, 7)))
	assert.Equal(t, "", classDayWeekdayKey(timezone.NewDate(2026, 8, 8)))
	assert.Equal(t, "", classDayWeekdayKey(timezone.NewDate(2026, 8, 9)))
}

func TestBuildClassDayReportProjection(t *testing.T) {
	t.Parallel()

	rows := []ClassRosterRow{
		{
			StudentID:  1,
			FirstName:  "Mila",
			LastName:   "Anders",
			Registered: true,
			OfferingsByDay: map[string][]string{
				"wed": {"Ganztag"},
			},
			PickupByDay:    map[string]string{"wed": "15:00"},
			ArrivalByDay:   map[string]string{"wed": "07:30"},
			DepartureByDay: map[string]string{"wed": "wird abgeholt"},
		},
		{
			StudentID:      2,
			FirstName:      "Finn",
			LastName:       "Becker",
			Registered:     false,
			DepartureByDay: map[string]string{"mon": "geht alleine"},
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

	report := buildClassDayReport("1a", timezone.NewDate(2026, 8, 5), "Schuljahr 2026/27", rows, classDayFacts{
		statuses:   statuses,
		departures: map[int64]string{1: "Abholung"},
		pickups:    map[int64]string{1: "16:00"},
	})

	require.Len(t, report.Rows, 3)
	assert.Equal(t, "wed", report.Weekday)
	assert.True(t, report.SchoolDay)
	assert.Equal(t, "Schuljahr 2026/27", report.PhaseName)

	assert.True(t, report.Rows[0].StaysToday)
	assert.Equal(t, []string{"Ganztag"}, report.Rows[0].Offerings)
	// The effective projected pickup time beats the form answer.
	assert.Equal(t, "16:00", report.Rows[0].Pickup)
	assert.Equal(t, "07:30", report.Rows[0].Arrival)
	// The day-specific departure is the ONLY source on a school day.
	assert.Equal(t, "Abholung", report.Rows[0].Departure)
	// The roster's per-day map must NOT leak onto the day sheet: it is never
	// empty and floors at "geht alleine", so a child without any per-day plan
	// renders as explicit "Keine Angabe" instead.
	assert.Equal(t, "Keine Angabe", report.Rows[1].Departure)
	assert.Equal(t, "Keine Angabe", report.Rows[2].Departure)

	assert.False(t, report.Rows[1].StaysToday)
	assert.Empty(t, report.Rows[1].Offerings)

	// A reported sick day wins over the enrollment: the student is absent,
	// not staying, even though the weekday has an offering.
	assert.False(t, report.Rows[2].StaysToday)
	assert.Equal(t, activeModels.StudentStatusDaySick, report.Rows[2].Status)

	assert.Equal(t, ClassDayTotals{Students: 3, Staying: 1, Leaving: 1, Absent: 1}, report.Totals)
}

func TestBuildClassDayReportWeekend(t *testing.T) {
	t.Parallel()

	rows := []ClassRosterRow{{
		StudentID:      1,
		Registered:     true,
		OfferingsByDay: map[string][]string{"mon": {"Ganztag"}},
		DepartureByDay: map[string]string{"mon": "fährt Bus", "tue": "wird abgeholt"},
	}}

	report := buildClassDayReport("1a", timezone.NewDate(2026, 8, 8), "", rows, newClassDayFacts())

	assert.False(t, report.SchoolDay)
	assert.Equal(t, "", report.Weekday)
	require.Len(t, report.Rows, 1)
	assert.False(t, report.Rows[0].StaysToday)
	// Kein Schultag: keine Übergabe, also auch keine Abgangs-Aussage — die
	// Tageswerte des Rosters dürfen nicht ins Einzeltag-Feld durchsickern
	// (auch nicht für Nicht-UI-Consumer des Endpoints).
	assert.Equal(t, "", report.Rows[0].Departure)
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

func TestClassDayPhasesReturnsEveryActiveCoveringPhase(t *testing.T) {
	t.Parallel()

	svc := &reportService{ReportServiceConfig: ReportServiceConfig{PhaseRepo: &fakeClassDayPhaseRepo{phases: []*enrollmentModels.Phase{
		{Model: baseModels.Model{ID: 1}, Name: "Altjahr", ServiceStartDate: timezone.NewDate(2025, 8, 1), ServiceEndDate: timezone.NewDate(2026, 7, 31)},
		{Model: baseModels.Model{ID: 2}, Name: "Schuljahr", IsActive: true, ServiceStartDate: timezone.NewDate(2026, 8, 1), ServiceEndDate: timezone.NewDate(2027, 7, 31)},
		{Model: baseModels.Model{ID: 3}, Name: "Ferien", IsActive: true, ServiceStartDate: timezone.NewDate(2026, 8, 3), ServiceEndDate: timezone.NewDate(2026, 8, 14)},
		{Model: baseModels.Model{ID: 4}, Name: "InaktivAlt", ServiceStartDate: timezone.NewDate(2026, 8, 1), ServiceEndDate: timezone.NewDate(2027, 7, 31)},
	}}}}

	phases, err := svc.classDayPhases(context.Background(), timezone.NewDate(2026, 8, 5))

	require.NoError(t, err)
	// EVERY covering ACTIVE phase, not one "best": a child enrolled only in
	// the Schuljahr phase must not vanish because a Ferien phase also covers
	// the date. Latest start first. The deactivated phase whose window still
	// covers the date is excluded — !IsActive is a hard reject on every
	// enrollment path, and a stale approval must not put a child into
	// "Bleiben in der Betreuung".
	require.Len(t, phases, 2)
	assert.Equal(t, []int64{3, 2}, []int64{phases[0].id, phases[1].id})
}

func TestClassDayPhasesNoneCovering(t *testing.T) {
	t.Parallel()

	svc := &reportService{ReportServiceConfig: ReportServiceConfig{PhaseRepo: &fakeClassDayPhaseRepo{phases: []*enrollmentModels.Phase{
		{Model: baseModels.Model{ID: 1}, ServiceStartDate: timezone.NewDate(2025, 8, 1), ServiceEndDate: timezone.NewDate(2026, 7, 31)},
	}}}}

	phases, err := svc.classDayPhases(context.Background(), timezone.NewDate(2026, 8, 5))

	require.NoError(t, err)
	assert.Empty(t, phases)
}

func TestMergeClassDayRostersUnionsRegistrations(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	svc := &reportService{ReportServiceConfig: ReportServiceConfig{PhaseRepo: &fakeClassDayPhaseRepo{}}}

	_, err := svc.ClassDay(context.Background(), "1a", timezone.NewDate(2026, 8, 5), 42, "lehrkraft")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "not configured")
}

func TestClassDayWithoutPhaseListsFullClass(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	svc := &reportService{ReportServiceConfig: ReportServiceConfig{CareDaySvc: &fakeCareDayService{statuses: map[int64]scheduleService.CareDayStatus{
		1: scheduleService.CareDayCancelled,
		2: scheduleService.CareDayScheduled,
		3: scheduleService.CareDayNotScheduled,
	}}}}

	facts := newClassDayFacts()
	cancelled, err := svc.classDayCancellations(context.Background(), []int64{1, 2, 3}, timezone.NewDate(2026, 8, 5), &facts)

	require.NoError(t, err)
	assert.Equal(t, map[int64]bool{1: true}, cancelled)
	// "An dem Tag nicht gebucht" is not an absence, but the child must not
	// be listed as staying either — surfaced separately.
	assert.Equal(t, map[int64]bool{3: true}, facts.notScheduled)
}

func TestClassDayCancellationUsesArrivalExceptionReportTime(t *testing.T) {
	t.Parallel()

	reportedAt := time.Date(2026, 8, 5, 7, 24, 0, 0, time.UTC)
	svc := &reportService{ReportServiceConfig: ReportServiceConfig{
		ArrivalScheduleSvc: &fakeArrivalScheduleService{byStudent: map[int64]*scheduleService.EffectiveArrivalTime{
			1: {IsException: true, ChangedAt: &reportedAt},
		}},
		CareDaySvc: &fakeCareDayService{statuses: map[int64]scheduleService.CareDayStatus{
			1: scheduleService.CareDayCancelled,
		}},
	}}
	facts := newClassDayFacts()

	require.NoError(t, svc.classDayEffectiveTimes(context.Background(), []int64{1}, timezone.NewDate(2026, 8, 5), &facts))
	cancelled, err := svc.classDayCancellations(context.Background(), []int64{1}, timezone.NewDate(2026, 8, 5), &facts)
	require.NoError(t, err)
	for studentID := range cancelled {
		facts.statuses[studentID] = StudentStatusDayCancelled
		if stamp, ok := facts.arrivalCancelledAt[studentID]; ok {
			facts.statusReportedAt[studentID] = stamp
		}
	}

	report := buildClassDayReport("1a", timezone.NewDate(2026, 8, 5), "Schuljahr", []ClassRosterRow{{
		StudentID: 1, Registered: true, OfferingsByDay: map[string][]string{"wed": {"Ganztag"}},
	}}, facts)
	require.NotNil(t, report.Rows[0].ReportedAt)
	assert.Equal(t, reportedAt, *report.Rows[0].ReportedAt)
}

func TestBuildClassDayReportNotScheduledOverridesOffering(t *testing.T) {
	t.Parallel()

	rows := []ClassRosterRow{{
		StudentID:      1,
		Registered:     true,
		OfferingsByDay: map[string][]string{"wed": {"Ganztag"}},
	}}

	report := buildClassDayReport("1a", timezone.NewDate(2026, 8, 5), "Schuljahr", rows, classDayFacts{notScheduled: map[int64]bool{1: true}})

	require.Len(t, report.Rows, 1)
	// The approved offering still lists Wednesday, but the parents struck
	// the weekday from the care plan: the child goes home, not "bleibt in
	// der Betreuung" without a pickup time.
	assert.False(t, report.Rows[0].StaysToday)
	assert.Equal(t, "", report.Rows[0].Status)
	assert.Equal(t, ClassDayTotals{Students: 1, Leaving: 1}, report.Totals)
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
	t.Parallel()

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
	t.Parallel()

	svc := &reportService{ReportServiceConfig: ReportServiceConfig{StudentStatusDayRepo: &fakeClassDayStatusRepo{entries: []*activeModels.StudentStatusDay{
		{StudentID: 1, Status: activeModels.StudentStatusDayExcused},
		{StudentID: 1, Status: activeModels.StudentStatusDaySick},
		{StudentID: 2, Status: activeModels.StudentStatusDayClassTrip},
		{StudentID: 2, Status: activeModels.StudentStatusDayExcused},
	}}}}

	statuses, _, err := svc.classDayStatuses(context.Background(), []int64{1, 2}, timezone.NewDate(2026, 8, 5))

	require.NoError(t, err)
	assert.Equal(t, activeModels.StudentStatusDaySick, statuses[1])
	assert.Equal(t, activeModels.StudentStatusDayClassTrip, statuses[2])
}

func TestClassDayStatusUnknownValueStillCounts(t *testing.T) {
	t.Parallel()

	// The status set is an extension point: a value the precedence map does
	// not know yet still means "reported for the day" and must survive —
	// silently dropping it would put a reported-absent child under "bleibt
	// in der Betreuung". Known statuses keep precedence over unknown ones.
	svc := &reportService{ReportServiceConfig: ReportServiceConfig{StudentStatusDayRepo: &fakeClassDayStatusRepo{entries: []*activeModels.StudentStatusDay{
		{StudentID: 1, Status: "quarantine"},
		{StudentID: 2, Status: "quarantine"},
		{StudentID: 2, Status: activeModels.StudentStatusDayExcused},
	}}}}

	statuses, _, err := svc.classDayStatuses(context.Background(), []int64{1, 2}, timezone.NewDate(2026, 8, 5))

	require.NoError(t, err)
	assert.Equal(t, "quarantine", statuses[1])
	assert.Equal(t, activeModels.StudentStatusDayExcused, statuses[2])
}

func TestClassDayDepartureRendersSingleDay(t *testing.T) {
	t.Parallel()

	pickupAndBus := &userModels.Student{
		AllowedDepartureModes: userModels.AllowedDepartureModes{
			"fri": {userModels.DepartureBus, userModels.DeparturePickup},
		},
	}
	assert.Equal(t, "Bus, Abholung", classDayDeparture(pickupAndBus, "fri", nil, nil))
	// A day without any allowed mode is UNKNOWN — the empty string lets the
	// report fall back to the form answer / "Keine Angabe", never to a
	// fabricated "Geht alleine".
	assert.Equal(t, "", classDayDeparture(pickupAndBus, "mon", nil, nil))

	// The parent-written note is NOT rendered here: it is free text that
	// routinely carries a guardian's name and phone number, which this view
	// promises never to show.
	note := "Abholung durch Nachbarin Frau Meier, 0171-1234567"
	accompanied := &userModels.Student{
		AllowedDepartureModes: userModels.AllowedDepartureModes{
			"fri": {userModels.DepartureAccompanied},
		},
		DepartureCompanionNote: &note,
	}
	assert.Equal(t, "Mit anderem Kind", classDayDeparture(accompanied, "fri", nil, nil))
}

func TestClassDayDepartureNamesOnlyCompanionsOnTheSheet(t *testing.T) {
	t.Parallel()

	// A Laufgemeinschaft may pair children of different classes, and the
	// class-day sheet is scoped to one class: a companion from 4c must not
	// surface on the 1a sheet, a classmate already listed there may.
	const classmateID, foreignID = int64(11), int64(99)
	accompanied := &userModels.Student{
		AllowedDepartureModes: userModels.AllowedDepartureModes{
			"fri": {userModels.DepartureAccompanied},
		},
	}
	onSheet := map[int64]bool{classmateID: true}

	classmate := []userModels.CompanionLink{
		{CompanionStudentID: classmateID, FirstName: "Tom", LastName: "Berger", Weekdays: []string{"fri"}},
	}
	foreign := []userModels.CompanionLink{
		{CompanionStudentID: foreignID, FirstName: "Max", LastName: "Mustermann", Weekdays: []string{"fri"}},
	}

	// The weekday suffix is FormatCompanionLinks' doing: a link that does not
	// cover all five days names them, so a Friday-only arrangement cannot be
	// misread as a standing one.
	assert.Equal(t, "Mit anderem Kind (Tom Berger (Fr))", classDayDeparture(accompanied, "fri", classmate, onSheet))
	assert.Equal(t, "Mit anderem Kind", classDayDeparture(accompanied, "fri", foreign, onSheet))
	// Mixed links keep the classmate and drop the outsider rather than
	// falling back to naming nobody.
	assert.Equal(t, "Mit anderem Kind (Tom Berger (Fr))",
		classDayDeparture(accompanied, "fri", append(append([]userModels.CompanionLink{}, foreign...), classmate...), onSheet))
}

func TestClassDayReportRendersUnknownForUnplannedDay(t *testing.T) {
	t.Parallel()

	// Real classDayDeparture output, not a hand-built impossible state: the
	// roster row carries the never-empty per-day map the real builder
	// produces, the day map carries what classDayDeparture actually returns
	// for a day the plan does not cover.
	monOnly := &userModels.Student{
		AllowedDepartureModes: userModels.AllowedDepartureModes{
			"mon": {userModels.DepartureBus},
		},
	}
	noPlan := &userModels.Student{}
	rows := []ClassRosterRow{
		{StudentID: 1, Registered: true, DepartureByDay: map[string]string{"mon": "fährt Bus"}},
		{StudentID: 2, Registered: true, DepartureByDay: map[string]string{"wed": "geht alleine"}},
	}
	departures := map[int64]string{
		1: classDayDeparture(monOnly, "wed", nil, nil),
		2: classDayDeparture(noPlan, "wed", nil, nil),
	}

	report := buildClassDayReport("1a", timezone.NewDate(2026, 8, 5), "Schuljahr", rows, classDayFacts{departures: departures})

	require.Len(t, report.Rows, 2)
	// A Wednesday sheet must neither print a Monday-only roster value nor
	// fabricate "Geht alleine" for a child without any plan.
	assert.Equal(t, "Keine Angabe", report.Rows[0].Departure)
	assert.Equal(t, "Keine Angabe", report.Rows[1].Departure)
}

// fakeClassDayAccessLogRepo captures audit rows and answers ExistsSince from
// the captured rows, mirroring the real dedupe semantics.
type fakeClassDayAccessLogRepo struct {
	entries []*auditModels.DataAccessLog
}

func (f *fakeClassDayAccessLogRepo) Create(_ context.Context, entry *auditModels.DataAccessLog) error {
	f.entries = append(f.entries, entry)
	return nil
}

func (f *fakeClassDayAccessLogRepo) ExistsSince(_ context.Context, actorAccountID int64, resourceType string, metadata map[string]string, _ time.Time) (bool, error) {
	for _, entry := range f.entries {
		if entry.ActorAccountID != actorAccountID || entry.ResourceType != resourceType {
			continue
		}
		match := true
		for key, value := range metadata {
			if fmt.Sprint(entry.Metadata[key]) != value {
				match = false
				break
			}
		}
		if match {
			return true, nil
		}
	}
	return false, nil
}

func TestRecordClassDayViewAuditDedupesPerActorClassAndDate(t *testing.T) {
	t.Parallel()

	repo := &fakeClassDayAccessLogRepo{}
	svc := &reportService{ReportServiceConfig: ReportServiceConfig{DataAccessLogRepo: repo}}
	report := &ClassDayReport{SchoolClass: "1a", Date: timezone.NewDate(2026, 8, 5)}

	// The view revalidates itself (interval + focus): identical repeat reads
	// must collapse into ONE evidential row.
	require.NoError(t, svc.recordClassDayViewAudit(context.Background(), report, 42, "lehrkraft"))
	require.NoError(t, svc.recordClassDayViewAudit(context.Background(), report, 42, "lehrkraft"))
	assert.Len(t, repo.entries, 1)

	// A different class, date, or actor is a distinct access and writes again.
	otherClass := &ClassDayReport{SchoolClass: "1b", Date: timezone.NewDate(2026, 8, 5)}
	require.NoError(t, svc.recordClassDayViewAudit(context.Background(), otherClass, 42, "lehrkraft"))
	otherDate := &ClassDayReport{SchoolClass: "1a", Date: timezone.NewDate(2026, 8, 6)}
	require.NoError(t, svc.recordClassDayViewAudit(context.Background(), otherDate, 42, "lehrkraft"))
	require.NoError(t, svc.recordClassDayViewAudit(context.Background(), report, 43, "lehrkraft"))
	assert.Len(t, repo.entries, 4)
}

func TestClassDayRequiresSchoolClass(t *testing.T) {
	t.Parallel()

	svc := &reportService{}

	_, err := svc.ClassDay(context.Background(), "  ", timezone.NewDate(2026, 8, 5), 42, "lehrkraft")

	require.ErrorIs(t, err, ErrReportInvalidFilter)
}

// clockTime builds a wall-clock TIME value the way the driver scans one.
func clockTime(hour, minute int) *time.Time {
	value := time.Date(2000, 1, 1, hour, minute, 0, 0, time.UTC)
	return &value
}

// TestApplyClassDayPickupDeviation pins what counts as "heute anders als
// sonst" for a Lehrkraft (#2294). The distinction matters in the product: a
// block that also announces unchanged times trains the reader to skip it.
func TestApplyClassDayPickupDeviation(t *testing.T) {
	t.Parallel()

	recorded := time.Date(2026, 8, 24, 9, 12, 0, 0, time.UTC)

	tests := []struct {
		name        string
		entry       *scheduleService.EffectivePickupTime
		wantPickup  string
		wantChanged bool
		wantRegular string
		wantStamp   bool
	}{
		{
			name:       "plan time without exception is no deviation",
			entry:      &scheduleService.EffectivePickupTime{PickupTime: clockTime(15, 0), RegularPickupTime: clockTime(15, 0)},
			wantPickup: "15:00",
		},
		{
			name: "earlier pickup names the regular time it replaces",
			entry: &scheduleService.EffectivePickupTime{
				PickupTime: clockTime(12, 15), RegularPickupTime: clockTime(15, 0),
				IsException: true, ChangedAt: &recorded,
			},
			wantPickup: "12:15", wantChanged: true, wantRegular: "15:00", wantStamp: true,
		},
		{
			name: "later pickup is a deviation too",
			entry: &scheduleService.EffectivePickupTime{
				PickupTime: clockTime(16, 30), RegularPickupTime: clockTime(15, 0),
				IsException: true, ChangedAt: &recorded,
			},
			wantPickup: "16:30", wantChanged: true, wantRegular: "15:00", wantStamp: true,
		},
		{
			// A parent re-entering the time the plan already holds must not
			// reach the Lehrkraft as a change.
			name: "exception repeating the plan time is not a deviation",
			entry: &scheduleService.EffectivePickupTime{
				PickupTime: clockTime(15, 0), RegularPickupTime: clockTime(15, 0),
				IsException: true, ChangedAt: &recorded,
			},
			wantPickup: "15:00", wantStamp: true,
		},
		{
			// The child is not normally in care that weekday; there is no
			// "sonst" to name, but the time itself is news.
			name: "pickup on a day without a plan time is a deviation without a regular time",
			entry: &scheduleService.EffectivePickupTime{
				PickupTime: clockTime(14, 0), IsException: true, ChangedAt: &recorded,
			},
			wantPickup: "14:00", wantChanged: true, wantStamp: true,
		},
		{
			// "Kommt heute nicht" travels as a status, never as a changed
			// pickup time — otherwise the row claims a pickup that is not
			// happening.
			name: "timeless exception records only when it became known",
			entry: &scheduleService.EffectivePickupTime{
				RegularPickupTime: clockTime(15, 0), IsException: true, ChangedAt: &recorded,
			},
			wantStamp: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			facts := newClassDayFacts()

			applyClassDayPickup(&facts, 7, tc.entry)

			assert.Equal(t, tc.wantPickup, facts.pickups[7])
			assert.Equal(t, tc.wantChanged, facts.pickupChanged[7])
			assert.Equal(t, tc.wantRegular, facts.pickupRegular[7])
			_, hasStamp := facts.pickupChangedAt[7]
			assert.Equal(t, tc.wantStamp, hasStamp)
		})
	}
}

// TestBuildClassDayReportCarriesPickupDeviation checks the projection onto
// the row a Lehrkraft reads.
func TestBuildClassDayReportCarriesPickupDeviation(t *testing.T) {
	t.Parallel()

	recorded := time.Date(2026, 8, 5, 8, 40, 0, 0, time.UTC)
	rows := []ClassRosterRow{
		{StudentID: 1, LastName: "Adam", Registered: true, OfferingsByDay: map[string][]string{"wed": {"Ganztag"}}},
		{StudentID: 2, LastName: "Bosch", Registered: true, OfferingsByDay: map[string][]string{"wed": {"Ganztag"}}},
	}

	report := buildClassDayReport("1a", timezone.NewDate(2026, 8, 5), "Schuljahr", rows, classDayFacts{
		pickups:         map[int64]string{1: "12:15", 2: "15:00"},
		pickupChanged:   map[int64]bool{1: true},
		pickupRegular:   map[int64]string{1: "15:00"},
		pickupChangedAt: map[int64]time.Time{1: recorded},
	})

	require.Len(t, report.Rows, 2)
	assert.True(t, report.Rows[0].PickupChanged)
	assert.Equal(t, "12:15", report.Rows[0].Pickup)
	assert.Equal(t, "15:00", report.Rows[0].PickupRegular)
	require.NotNil(t, report.Rows[0].ReportedAt)
	assert.Equal(t, recorded, *report.Rows[0].ReportedAt)
	// The unchanged child stays clean: no deviation, nothing to date.
	assert.False(t, report.Rows[1].PickupChanged)
	assert.Empty(t, report.Rows[1].PickupRegular)
	assert.Nil(t, report.Rows[1].ReportedAt)
	// A deviating pickup time does not take the child out of care.
	assert.True(t, report.Rows[0].StaysToday)
}

// TestBuildClassDayReportReportedAtFollowsStatus pins the precedence: the
// badge the reader sees is the status, so the timestamp next to it must date
// the status and not an unrelated pickup edit.
func TestBuildClassDayReportReportedAtFollowsStatus(t *testing.T) {
	t.Parallel()

	statusStamp := time.Date(2026, 8, 5, 7, 5, 0, 0, time.UTC)
	pickupStamp := time.Date(2026, 8, 4, 16, 0, 0, 0, time.UTC)
	rows := []ClassRosterRow{{StudentID: 1, Registered: true, OfferingsByDay: map[string][]string{"wed": {"Ganztag"}}}}

	report := buildClassDayReport("1a", timezone.NewDate(2026, 8, 5), "Schuljahr", rows, classDayFacts{
		statuses:         map[int64]string{1: activeModels.StudentStatusDaySick},
		statusReportedAt: map[int64]time.Time{1: statusStamp},
		pickupChanged:    map[int64]bool{1: true},
		pickupChangedAt:  map[int64]time.Time{1: pickupStamp},
	})

	require.Len(t, report.Rows, 1)
	require.NotNil(t, report.Rows[0].ReportedAt)
	assert.Equal(t, statusStamp, *report.Rows[0].ReportedAt)
}

// TestBuildClassDayReportListEntryHasNoDeviation guards the #2382 boundary:
// a child without any OGS record has no plan to deviate from, so no stray
// facts keyed on student id 0 may attach to it.
func TestBuildClassDayReportListEntryHasNoDeviation(t *testing.T) {
	t.Parallel()

	rows := []ClassRosterRow{{ListEntry: true, ListEntryID: 42, LastName: "Cole"}}

	report := buildClassDayReport("1a", timezone.NewDate(2026, 8, 5), "Schuljahr", rows, classDayFacts{
		pickupChanged: map[int64]bool{0: true},
		pickupRegular: map[int64]string{0: "15:00"},
	})

	require.Len(t, report.Rows, 1)
	assert.False(t, report.Rows[0].PickupChanged)
	assert.Empty(t, report.Rows[0].PickupRegular)
}

// TestClassDayStatusesReportTime keeps the stamp attached to the winning
// status: sick outranks excused, and so does its report time.
func TestClassDayStatusesReportTime(t *testing.T) {
	t.Parallel()

	sickAt := time.Date(2026, 8, 5, 11, 24, 0, 0, time.UTC)
	excusedAt := time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)
	svc := &reportService{ReportServiceConfig: ReportServiceConfig{StudentStatusDayRepo: &fakeClassDayStatusRepo{entries: []*activeModels.StudentStatusDay{
		{StudentID: 1, Status: activeModels.StudentStatusDayExcused, ReportedAt: excusedAt},
		{StudentID: 1, Status: activeModels.StudentStatusDaySick, ReportedAt: sickAt},
	}}}}

	statuses, stamps, err := svc.classDayStatuses(context.Background(), []int64{1}, timezone.NewDate(2026, 8, 5))

	require.NoError(t, err)
	assert.Equal(t, activeModels.StudentStatusDaySick, statuses[1])
	assert.Equal(t, sickAt, stamps[1])
}
