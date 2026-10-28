package contracttest_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Regeltermine whose schedules all share one period Q that differs from the
// template's own period P, or whose template has no period (#5176). Every
// resync plans a sourced template under the template's period
// (templateResync → sourcedTemplateDraft), NULL without one. Approval and
// both corrections must write the same period; otherwise each switch between
// a correction and a resync deletes or caps an unchanged row and re-creates it.

// createDivergingPeriodSourcedTemplate creates a live template that sources
// the offering, pins templatePeriodID (nil for none), and has a Monday and a
// Wednesday schedule that both pin schedulePeriodID.
func createDivergingPeriodSourcedTemplate(
	t *testing.T,
	env *decisionTestEnv,
	name string,
	offeringID int64,
	templatePeriodID *int64,
	schedulePeriodID int64,
) *timetable.Group {
	t.Helper()
	group := createCareOfferingTemplateGroup(t, env.db, name)
	group.TargetGroupType = timetable.TargetGroupTypeOffering
	group.SourceCareOfferingIDs = []int64{offeringID}
	group.CalendarPeriodID = templatePeriodID
	require.NoError(t, timetableCompose.NativeGroupRecords(testTimetableRepositories(env.db).Timetable).Update(testpkg.Ctx(t), group))
	createCareOfferingTemplateSchedule(t, env.db, group.ID, timetable.WeekdayMonday, &schedulePeriodID)
	createCareOfferingTemplateSchedule(t, env.db, group.ID, timetable.WeekdayWednesday, &schedulePeriodID)
	return group
}

// divergingPeriodFixture is one approved child holding the template's source
// offering plus another offering on a template whose period diverges from
// its schedules' shared period.
type divergingPeriodFixture struct {
	env          *decisionTestEnv
	template     *timetable.Group
	source       *offeringContract.CareOfferingRecord
	requestID    int64
	childID      int64
	wantPeriodID *int64
}

func setupDivergingPeriodFixture(t *testing.T, email string, withTemplatePeriod bool) divergingPeriodFixture {
	t.Helper()
	env := setupDecisionTest(t)
	source := createAdjustmentCareOfferingWith(t, env, "Mittagessen", nil)
	other := createAdjustmentCareOfferingWith(t, env, "Spaetbetreuung", nil)
	schedulePeriodID := createMixedPeriodCalendarPeriod(t, env, "zeitplan")
	var templatePeriodID *int64
	if withTemplatePeriod {
		id := createMixedPeriodCalendarPeriod(t, env, "termin")
		require.NotEqual(t, schedulePeriodID, id)
		templatePeriodID = &id
	}
	template := createDivergingPeriodSourcedTemplate(t, env, "MittagessenAbweichend", source.ID, templatePeriodID, schedulePeriodID)

	requestID, childID, studentID := submitApprovedAdjustmentChild(t, env, email, "Abweichend",
		[]*offeringContract.CareOfferingRecord{source, other})
	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1, "approval plans the child on the template")
	require.Equal(t, studentID, rows[0].StudentID)

	return divergingPeriodFixture{
		env: env, template: template, source: source,
		requestID: requestID, childID: childID, wantPeriodID: templatePeriodID,
	}
}

// requireSingleRow loads the template's only row and checks it carries the
// template's period, not the schedules' one.
func (f divergingPeriodFixture) requireSingleRow(t *testing.T, step string) timetable.StudentEnrollment {
	t.Helper()
	rows := loadTemplateEnrollments(t, f.env, f.template.ID)
	require.Len(t, rows, 1, "%s: the child keeps exactly one row on the template", step)
	if f.wantPeriodID == nil {
		assert.Nil(t, rows[0].CalendarPeriodID, "%s: a template without a period plans a row without one", step)
	} else if assert.NotNil(t, rows[0].CalendarPeriodID, "%s: the row carries the template period", step) {
		assert.Equal(t, *f.wantPeriodID, *rows[0].CalendarPeriodID, "%s: the template period wins over the schedules' period", step)
	}
	assert.Equal(t, []int{1}, rows[0].SelectedWeekdays, "%s", step)
	return rows[0]
}

// requireUnchanged asserts that a step neither replaced, re-dated nor
// re-planned the row.
func requireUnchanged(t *testing.T, step string, before, after timetable.StudentEnrollment) {
	t.Helper()
	assert.Equal(t, before.ID, after.ID, "%s: the unchanged row must not be replaced", step)
	assert.Equal(t, before.CalendarPeriodID, after.CalendarPeriodID, "%s", step)
	assert.Equal(t, before.SelectedWeekdays, after.SelectedWeekdays, "%s", step)
	assert.Equal(t, before.ValidFrom, after.ValidFrom, "%s", step)
	assert.Equal(t, before.ValidUntil, after.ValidUntil, "%s: the row must not be capped", step)
}

func (f divergingPeriodFixture) resyncTenant(t *testing.T) {
	t.Helper()
	resyncAll, ok := f.env.bookings.(interface {
		ResyncOfferingSourcedTemplates(ctx context.Context, effectiveFrom calendar.Date) error
	})
	require.True(t, ok, "decision service must implement the tenant-wide resync")
	require.NoError(t, resyncAll.ResyncOfferingSourcedTemplates(testpkg.Ctx(t), offeringResyncToday))
}

func (f divergingPeriodFixture) resyncTemplateSave(t *testing.T, effectiveFrom calendar.Date) {
	t.Helper()
	require.NoError(t, offeringResyncer(t, f.env).ResyncTemplateOfferingRoster(testpkg.Ctx(t), timetable.OfferingRosterResyncInput{
		TemplateID:       f.template.ID,
		OfferingIDs:      []int64{f.source.ID},
		CalendarPeriodID: f.template.CalendarPeriodID,
		EffectiveFrom:    effectiveFrom,
	}))
}

// correct replaces the child's second offering; the template's source stays
// booked on Monday. A nil effectiveFrom is the undated correction.
func (f divergingPeriodFixture) correct(t *testing.T, name string, effectiveFrom *calendar.Date) {
	t.Helper()
	replacement := createAdjustmentCareOfferingWith(t, f.env, name, nil)
	_, err := testpkg.TenantQuery1(t, testpkg.Ctx(t), f.env.db, f.env.decision.UpdateChildOfferings, enrollmentTest.UpdateChildOfferingsInput{
		RequestID:      f.requestID,
		ChildID:        f.childID,
		ActorAccountID: f.env.creatorID,
		ActorRole:      "admin",
		Reason:         "Anderes Angebot",
		EffectiveFrom:  effectiveFrom,
		Offerings: []enrollmentTest.OfferingAdjustmentSelection{
			{OfferingID: f.source.ID, SelectedDays: []string{"mon"}},
			{OfferingID: replacement.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)
}

// runDivergingPeriodCycle alternates approval, both corrections and every
// resync entry and requires the row to stay on the template period
// throughout, with resyncs leaving it untouched.
func runDivergingPeriodCycle(t *testing.T, email string, withTemplatePeriod bool) {
	t.Helper()
	f := setupDivergingPeriodFixture(t, email, withTemplatePeriod)
	phaseStart := calendar.Date(f.env.sourcePhase.ServiceStartDate)

	approved := f.requireSingleRow(t, "approval")

	f.resyncTenant(t)
	requireUnchanged(t, "tenant-wide resync after approval", approved, f.requireSingleRow(t, "tenant-wide resync after approval"))
	f.resyncTenant(t)
	requireUnchanged(t, "repeated tenant-wide resync", approved, f.requireSingleRow(t, "repeated tenant-wide resync"))

	// The undated correction rebuilds the child's rows; it must rebuild them
	// on the period the resync uses.
	f.correct(t, "Fruehbetreuung", nil)
	corrected := f.requireSingleRow(t, "undated correction")

	f.resyncTemplateSave(t, phaseStart)
	requireUnchanged(t, "template-save resync after undated correction", corrected, f.requireSingleRow(t, "template-save resync after undated correction"))
	// A resync taking effect mid-phase must not cap the running row and
	// re-seed it on another period.
	f.resyncTemplateSave(t, phaseStart.AddDays(30))
	requireUnchanged(t, "mid-phase template-save resync", corrected, f.requireSingleRow(t, "mid-phase template-save resync"))

	switchDate := phaseStart.AddDays(60)
	f.correct(t, "Ferienbetreuung", &switchDate)
	requireUnchanged(t, "dated correction", corrected, f.requireSingleRow(t, "dated correction"))

	f.resyncTenant(t)
	requireUnchanged(t, "tenant-wide resync after dated correction", corrected, f.requireSingleRow(t, "tenant-wide resync after dated correction"))
}

func TestSourcedTemplatePeriod_TemplatePeriodWinsOverSchedulePeriodAcrossCorrectionsAndResyncs(t *testing.T) {
	t.Parallel()
	runDivergingPeriodCycle(t, "zeitraum-abweichend@example.com", true)
}

func TestSourcedTemplatePeriod_TemplateWithoutPeriodPlansNullAcrossCorrectionsAndResyncs(t *testing.T) {
	t.Parallel()
	runDivergingPeriodCycle(t, "zeitraum-ohne-termin@example.com", false)
}
