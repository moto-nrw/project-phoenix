// Issue #3165: a Regeltermin can take its children from an offering AND staff
// each weekday differently. Schule am Berg runs its Randstunde with a different
// supervisor per weekday; switching the Zielgruppe to "Angebot" must keep that
// staffing instead of flattening it.
package httpintegration_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// sourcedWeekdayStaffFixture is a Mo/Di/Mi Randstunde: one supervisor per
// weekday, an offering booked Monday and Tuesday, one child in class 1c and
// one child outside the class filter.
type sourcedWeekdayStaffFixture struct {
	s                            *scenarioSetup
	roster                       *weekdayRosterScenario
	monday, tuesday, wednesday   calendar.Date
	offering                     *testpkg.CareOffering
	offeringID                   int64
	staffMon, staffTue, staffWed int64
	childInFilter                int64
	childOutsideFilter           int64
}

func makeSourcedWeekdayStaffFixture(t *testing.T, monday calendar.Date) *sourcedWeekdayStaffFixture {
	t.Helper()
	s := makeScenario(t, activitiesModels.WeekdayMonday, monday)
	offering := createSourceCareOfferingOnDays(t, s,
		calendar.Date(s.period.StartDate), calendar.Date(s.period.EndDate), []string{"mon", "tue"})
	linkApprovedChildToOffering(t, s, offering, s.students[0], "1c")
	linkApprovedChildToOffering(t, s, offering, s.students[1], "2b")

	suffix := time.Now().UnixNano()
	return &sourcedWeekdayStaffFixture{
		s:                  s,
		roster:             &weekdayRosterScenario{db: s.db, ctx: s.ctx, tenantID: s.tenantID},
		monday:             monday,
		tuesday:            monday.AddDays(1),
		wednesday:          monday.AddDays(2),
		offering:           offering,
		offeringID:         offering.ID,
		staffMon:           testpkg.CreateTestStaffForTenant(t, s.db, s.tenantID, "Anna", fmt.Sprintf("Montag-%d", suffix)).ID,
		staffTue:           testpkg.CreateTestStaffForTenant(t, s.db, s.tenantID, "Bea", fmt.Sprintf("Dienstag-%d", suffix)).ID,
		staffWed:           testpkg.CreateTestStaffForTenant(t, s.db, s.tenantID, "Cem", fmt.Sprintf("Mittwoch-%d", suffix)).ID,
		childInFilter:      s.students[0],
		childOutsideFilter: s.students[1],
	}
}

func (f *sourcedWeekdayStaffFixture) weekdays() []int {
	return []int{activitiesModels.WeekdayMonday, activitiesModels.WeekdayTuesday, activitiesModels.WeekdayWednesday}
}

// staffOnlyAssignments is the payload the editor sends for a sourced series:
// per-weekday staff, no per-weekday children.
func (f *sourcedWeekdayStaffFixture) staffOnlyAssignments() []timetable.WeekdayRosterAssignment {
	return []timetable.WeekdayRosterAssignment{
		{Weekday: activitiesModels.WeekdayMonday, StaffIDs: []int64{f.staffMon}, PrimaryStaffID: &f.staffMon},
		{Weekday: activitiesModels.WeekdayTuesday, StaffIDs: []int64{f.staffTue}, PrimaryStaffID: &f.staffTue},
		{Weekday: activitiesModels.WeekdayWednesday, StaffIDs: []int64{f.staffWed}, PrimaryStaffID: &f.staffWed},
	}
}

// assertWeekOfTemplate materializes Mo–Mi starting at monday and checks the
// people on each occurrence. wantKids are the sourced children booked on
// Monday and Tuesday; the offering is not booked on Wednesday.
func (f *sourcedWeekdayStaffFixture) assertWeekOfTemplate(t *testing.T, templateID int64, monday calendar.Date, wantKids []int64) {
	t.Helper()
	_, err := f.s.factory.Materialization.MaterializeForTenant(
		f.s.ctx, monday, monday.AddDays(2), timetable.MaterializationSourceManual,
	)
	require.NoError(t, err)

	mon := f.roster.singleInstance(t, templateID, monday)
	tue := f.roster.singleInstance(t, templateID, monday.AddDays(1))
	wed := f.roster.singleInstance(t, templateID, monday.AddDays(2))

	assert.Equal(t, []int64{f.staffMon}, f.roster.instanceStaffIDs(t, mon), "Monday keeps Monday's supervisor only")
	assert.Equal(t, []int64{f.staffTue}, f.roster.instanceStaffIDs(t, tue), "Tuesday keeps Tuesday's supervisor only")
	assert.Equal(t, []int64{f.staffWed}, f.roster.instanceStaffIDs(t, wed), "Wednesday keeps Wednesday's supervisor only")
	assert.True(t, f.roster.instanceHasPrimary(t, mon, f.staffMon))
	assert.True(t, f.roster.instanceHasPrimary(t, wed, f.staffWed))

	sortedKids := slices.Sorted(slices.Values(wantKids))
	assert.Equal(t, sortedKids, f.roster.instanceStudentIDs(t, mon),
		"class-1c children are booked on Monday; the 2b child stays outside the filter")
	assert.Equal(t, sortedKids, f.roster.instanceStudentIDs(t, tue))
	assert.Empty(t, f.roster.instanceStudentIDs(t, wed),
		"the offering is not booked on Wednesday, so the sourced roster has no child there")
}

func TestTemplateOfferingSource_CreateWithWeekdayStaffMaterializesEachDaysSupervisor(t *testing.T) {
	t.Parallel()

	f := makeSourcedWeekdayStaffFixture(t, futureMonday(1))
	defer f.s.runCleanup(t)

	result, err := f.s.factory.TimetableData.Templates.CreateTemplate(f.s.ctx, timetable.CreateTemplateCommand{
		Name:                  fmt.Sprintf("Randstunde-%d", time.Now().UnixNano()),
		Type:                  activitiesModels.GroupTypeCare,
		Weekdays:              f.weekdays(),
		StartTime:             clockTime(13, 0),
		EndTime:               clockTime(14, 0),
		RoomID:                f.s.roomID,
		CategoryID:            f.s.categoryID,
		MaxParticipants:       20,
		CalendarPeriodID:      &f.s.period.ID,
		TargetGroupType:       activitiesModels.TargetGroupTypeAngebot,
		SourceCareOfferingIDs: []int64{f.offeringID},
		SourceSchoolClasses:   []string{"1c"},
		WeekdayAssignments:    f.staffOnlyAssignments(),
		RosterValidFrom:       f.monday.AddDays(-30),
		GradeLevelMax:         testpkg.MaxSchoolGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, f.s, result.TemplateID, result.TimeframeID)

	f.assertWeekOfTemplate(t, result.TemplateID, f.monday, []int64{f.childInFilter})
}

// The Schule-am-Berg migration: a manual Randstunde with per-weekday staff is
// switched to Zielgruppe "Angebot". The staffing survives, the manual child
// list is replaced by the source.
func TestTemplateOfferingSource_UpdateToSourceKeepsWeekdayStaff(t *testing.T) {
	t.Parallel()

	f := makeSourcedWeekdayStaffFixture(t, futureMonday(1))
	defer f.s.runCleanup(t)

	name := fmt.Sprintf("Randstunde-%d", time.Now().UnixNano())
	manualAssignments := f.staffOnlyAssignments()
	for i := range manualAssignments {
		manualAssignments[i].StudentIDs = []int64{f.childOutsideFilter}
	}
	result, err := f.s.factory.TimetableData.Templates.CreateTemplate(f.s.ctx, timetable.CreateTemplateCommand{
		Name:               name,
		Type:               activitiesModels.GroupTypeCare,
		Weekdays:           f.weekdays(),
		StartTime:          clockTime(13, 0),
		EndTime:            clockTime(14, 0),
		RoomID:             f.s.roomID,
		CategoryID:         f.s.categoryID,
		MaxParticipants:    20,
		CalendarPeriodID:   &f.s.period.ID,
		TargetGroupType:    activitiesModels.TargetGroupTypeNone,
		WeekdayAssignments: manualAssignments,
		RosterValidFrom:    f.monday.AddDays(-30),
		GradeLevelMax:      testpkg.MaxSchoolGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, f.s, result.TemplateID, result.TimeframeID)

	require.NoError(t, f.s.factory.TimetableData.Templates.UpdateTemplate(f.s.ctx, timetable.UpdateTemplateCommand{
		TemplateID: result.TemplateID,
		Fields: timetable.TemplateFields{
			Name:                  name,
			Type:                  activitiesModels.GroupTypeCare,
			CategoryID:            f.s.categoryID,
			RoomID:                f.s.roomID,
			MaxParticipants:       20,
			CalendarPeriodID:      &f.s.period.ID,
			TargetGroupType:       activitiesModels.TargetGroupTypeAngebot,
			SourceCareOfferingIDs: []int64{f.offeringID},
			SourceSchoolClasses:   []string{"1c"},
		},
		Weekdays:           f.weekdays(),
		TimeframeID:        result.TimeframeID,
		CalendarPeriodID:   &f.s.period.ID,
		StudentIDs:         []int64{},
		WeekdayAssignments: f.staffOnlyAssignments(),
		RosterValidFrom:    f.monday.AddDays(-30),
		GradeLevelMax:      testpkg.MaxSchoolGradeLevel,
	}))

	for _, row := range f.roster.openSupervisorRows(t, result.TemplateID) {
		require.NotNil(t, row.Weekday, "the staffing must stay weekday-scoped, not collapse to a shared list")
	}
	f.assertWeekOfTemplate(t, result.TemplateID, f.monday, []int64{f.childInFilter})
}

// "Ab jetzt dauerhaft" on a sourced series with per-weekday staff: the
// successor keeps each weekday's supervisor instead of a merged list.
func TestTemplateOfferingSource_SplitKeepsWeekdayStaff(t *testing.T) {
	t.Parallel()

	f := makeSourcedWeekdayStaffFixture(t, futureMonday(1))
	defer f.s.runCleanup(t)

	name := fmt.Sprintf("Randstunde-%d", time.Now().UnixNano())
	result, err := f.s.factory.TimetableData.Templates.CreateTemplate(f.s.ctx, timetable.CreateTemplateCommand{
		Name:                  name,
		Type:                  activitiesModels.GroupTypeCare,
		Weekdays:              f.weekdays(),
		StartTime:             clockTime(13, 0),
		EndTime:               clockTime(14, 0),
		RoomID:                f.s.roomID,
		CategoryID:            f.s.categoryID,
		MaxParticipants:       20,
		CalendarPeriodID:      &f.s.period.ID,
		TargetGroupType:       activitiesModels.TargetGroupTypeAngebot,
		SourceCareOfferingIDs: []int64{f.offeringID},
		SourceSchoolClasses:   []string{"1c"},
		WeekdayAssignments:    f.staffOnlyAssignments(),
		RosterValidFrom:       f.monday.AddDays(-30),
		GradeLevelMax:         testpkg.MaxSchoolGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, f.s, result.TemplateID, result.TimeframeID)

	effective := f.monday.AddDays(7)
	split, err := f.s.factory.TimetableData.Templates.SplitTemplate(f.s.ctx, timetable.SplitTemplateCommand{
		TemplateID:       result.TemplateID,
		EffectiveDate:    effective,
		Name:             name,
		Type:             activitiesModels.GroupTypeCare,
		Weekdays:         f.weekdays(),
		StartTime:        clockTime(13, 30),
		EndTime:          clockTime(14, 30),
		RoomID:           f.s.roomID,
		CategoryID:       f.s.categoryID,
		CalendarPeriodID: &f.s.period.ID,
		TargetGroupType:  activitiesModels.TargetGroupTypeAngebot,
		// The editor's payload for a sourced series: explicit empty shared
		// lists, staff per weekday, children from the source.
		StudentIDs:         []int64{},
		StaffIDs:           []int64{},
		WeekdayAssignments: f.staffOnlyAssignments(),
		GradeLevelMax:      testpkg.MaxSchoolGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, f.s, split.NewTemplateID)

	successor := loadTemplateGroup(t, f.s, split.NewTemplateID)
	assert.Equal(t, []int64{f.offeringID}, successor.SourceCareOfferingIDs)
	f.assertWeekOfTemplate(t, split.NewTemplateID, effective, []int64{f.childInFilter})
}

// offeringRosterResyncer is the enrollment side's resync entry point, the
// one an approval or an offering change fans out to.
type offeringRosterResyncer interface {
	ResyncTemplateOfferingRoster(context.Context, timetable.OfferingRosterResyncInput) error
}

// timetableRosterResync hands the Timetable owner's resync input to Care
// Plan's booking materialization field by field, like the server's roster
// resync hook (#3560).
type timetableRosterResync struct{ rosters careplan.SourcedRosters }

func (r timetableRosterResync) ResyncTemplateOfferingRoster(ctx context.Context, in timetable.OfferingRosterResyncInput) error {
	return r.rosters.ResyncTemplateOfferingRoster(ctx, careplan.OfferingRosterResync{
		TemplateID: in.TemplateID, OfferingIDs: in.OfferingIDs, GradeLevels: in.GradeLevels,
		SchoolClasses: in.SchoolClasses, CalendarPeriodID: in.CalendarPeriodID, EffectiveFrom: in.EffectiveFrom,
		ScopeRequestChildIDs: in.ScopeRequestChildIDs, TolerateDriftedSources: in.TolerateDriftedSources,
	})
}

// A child booked into the offering after the series was set up joins the
// sourced roster on its booked weekdays; the per-weekday staffing stays.
func TestTemplateOfferingSource_NewlyBookedChildJoinsWithoutTouchingWeekdayStaff(t *testing.T) {
	t.Parallel()

	f := makeSourcedWeekdayStaffFixture(t, futureMonday(1))
	defer f.s.runCleanup(t)

	result, err := f.s.factory.TimetableData.Templates.CreateTemplate(f.s.ctx, timetable.CreateTemplateCommand{
		Name:                  fmt.Sprintf("Randstunde-%d", time.Now().UnixNano()),
		Type:                  activitiesModels.GroupTypeCare,
		Weekdays:              f.weekdays(),
		StartTime:             clockTime(13, 0),
		EndTime:               clockTime(14, 0),
		RoomID:                f.s.roomID,
		CategoryID:            f.s.categoryID,
		MaxParticipants:       20,
		CalendarPeriodID:      &f.s.period.ID,
		TargetGroupType:       activitiesModels.TargetGroupTypeAngebot,
		SourceCareOfferingIDs: []int64{f.offeringID},
		SourceSchoolClasses:   []string{"1c"},
		WeekdayAssignments:    f.staffOnlyAssignments(),
		RosterValidFrom:       f.monday.AddDays(-30),
		GradeLevelMax:         testpkg.MaxSchoolGradeLevel,
	})
	require.NoError(t, err)
	registerSourcedTemplateCleanup(t, f.s, result.TemplateID, result.TimeframeID)

	newChild := f.s.students[2]
	stored := loadTemplateGroup(t, f.s, result.TemplateID)
	linkApprovedChildToOffering(t, f.s, f.offering, newChild, "1c")

	require.NotNil(t, f.s.factory.EnrollmentCareOffering, "Care Plan's booking materialization owns the offering roster resync")
	var resyncer offeringRosterResyncer = timetableRosterResync{rosters: f.s.factory.EnrollmentCareOffering}
	require.NoError(t, testpkg.WithTenantTx(t, f.s.ctx, f.s.db, f.s.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		return resyncer.ResyncTemplateOfferingRoster(txCtx, timetable.OfferingRosterResyncInput{
			TemplateID:       result.TemplateID,
			OfferingIDs:      stored.SourceCareOfferingIDs,
			SchoolClasses:    stored.SourceSchoolClasses,
			CalendarPeriodID: &f.s.period.ID,
			EffectiveFrom:    f.monday,
		})
	}))

	for _, row := range f.roster.openSupervisorRows(t, result.TemplateID) {
		require.NotNil(t, row.Weekday, "a roster resync must not collapse the per-weekday staffing")
	}
	f.assertWeekOfTemplate(t, result.TemplateID, f.monday, []int64{f.childInFilter, newChild})
}
