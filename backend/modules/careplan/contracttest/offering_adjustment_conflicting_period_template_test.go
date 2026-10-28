package contracttest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"

	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Regeltermine whose schedules resolve to different planning periods
// (#5153): the template-save resync plans their sourced children under the
// template's own period; the decision and adjustment flows must plan them
// the same way instead of skipping the template, otherwise a correction
// deletes rows it never re-creates.

// createConflictingPeriodSourcedTemplate creates a live template that
// sources the offering and pins templatePeriodID (nil for none). Its Monday
// schedule pins its own period, its Wednesday schedule none, so the two
// schedules resolve to different planning periods.
func createConflictingPeriodSourcedTemplate(t *testing.T, env *decisionTestEnv, name string, offeringID int64, templatePeriodID *int64) *timetable.Group {
	t.Helper()
	schedulePeriod := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, env.db), testpkg.Ctx(t), testpkg.Tenant(t),
		uniqueSchemaName(name+"-schedule-period-"+t.Name()),
		calendar.NewDate(2026, 8, 1), calendar.NewDate(2027, 8, 31))
	group := createCareOfferingTemplateGroup(t, env.db, name)
	group.TargetGroupType = timetable.TargetGroupTypeOffering
	group.SourceCareOfferingIDs = []int64{offeringID}
	group.CalendarPeriodID = templatePeriodID
	require.NoError(t, timetableCompose.NativeGroupRecords(testTimetableRepositories(env.db).Timetable).Update(testpkg.Ctx(t), group))
	createCareOfferingTemplateSchedule(t, env.db, group.ID, timetable.WeekdayMonday, &schedulePeriod.ID)
	createCareOfferingTemplateSchedule(t, env.db, group.ID, timetable.WeekdayWednesday, nil)
	return group
}

func TestDecide_PlansChildOnSourcedTemplateWithConflictingSchedulePeriods(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	source := createSourceOffering(t, env, "FreigabeGemischteZeitraeume", nil)
	template := createConflictingPeriodSourcedTemplate(t, env, "FreigabeTerminGemischt", source.ID, nil)

	studentID, childID := submitAndApproveOfferingChild(t, env, source.ID, "freigabe-gemischt@example.com", "Gina", 2)

	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1, "approval must plan the child like the template-save resync does")
	assert.Equal(t, studentID, rows[0].StudentID)
	require.NotNil(t, rows[0].EnrollmentRequestChildID)
	assert.Equal(t, childID, *rows[0].EnrollmentRequestChildID)
	assert.Nil(t, rows[0].CalendarPeriodID, "the template pins no period, so neither does the row")
}

func TestUpdateChildOfferings_UndatedCorrectionKeepsTemplateWithConflictingSchedulePeriods(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	ctx := testpkg.Ctx(t)
	source := createAdjustmentCareOfferingWith(t, env, "Mittagessen", nil)
	other := createAdjustmentCareOfferingWith(t, env, "Spaetbetreuung", nil)
	templatePeriod := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) }, testpkg.NewDatabase(t, env.db), ctx, testpkg.Tenant(t),
		uniqueSchemaName("template-period-"+t.Name()),
		calendar.NewDate(2026, 8, 1), calendar.NewDate(2027, 8, 31))
	template := createConflictingPeriodSourcedTemplate(t, env, "MittagessenGemischt", source.ID, &templatePeriod.ID)

	requestID, childID, studentID := submitApprovedAdjustmentChild(t, env, "korrektur-gemischt@example.com", "Gemischt",
		[]*offeringContract.CareOfferingRecord{source, other})
	require.NoError(t, offeringResyncer(t, env).ResyncTemplateOfferingRoster(ctx, timetable.OfferingRosterResyncInput{
		TemplateID:       template.ID,
		OfferingIDs:      []int64{source.ID},
		CalendarPeriodID: &templatePeriod.ID,
		EffectiveFrom:    offeringResyncToday,
	}))
	before := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, before, 1, "the template-save resync plans the child on the template")
	require.NotNil(t, before[0].CalendarPeriodID)
	require.Equal(t, templatePeriod.ID, *before[0].CalendarPeriodID)

	replacement := createAdjustmentCareOfferingWith(t, env, "Fruehbetreuung", nil)
	// Correct only the other offering; the template's source stays booked.
	_, err := testpkg.TenantQuery1(t, ctx, env.db, env.decision.UpdateChildOfferings, enrollmentTest.UpdateChildOfferingsInput{
		RequestID:      requestID,
		ChildID:        childID,
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
		Reason:         "Falsches Angebot erfasst",
		Offerings: []enrollmentTest.OfferingAdjustmentSelection{
			{OfferingID: source.ID, SelectedDays: []string{"mon"}},
			{OfferingID: replacement.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1, "the correction must not drop the child from a template whose schedules disagree on the period")
	assert.Equal(t, studentID, rows[0].StudentID)
	require.NotNil(t, rows[0].EnrollmentRequestChildID)
	assert.Equal(t, childID, *rows[0].EnrollmentRequestChildID)
	require.NotNil(t, rows[0].CalendarPeriodID)
	assert.Equal(t, templatePeriod.ID, *rows[0].CalendarPeriodID, "the row keeps the period the resync planned it under")
}
