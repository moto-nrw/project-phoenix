package contracttest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api"
	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Regeltermine whose schedules name different planning periods (#5153):
// the schedules do not resolve one calendar_period_id, so the decision and
// adjustment flows cannot take the period from them. The template resync
// plans such a template without a period, as its instances materialize
// under different ones (sourcedRowPeriodID, #5176); the flows must plan it
// the same way instead of skipping it — otherwise a correction deletes rows
// it never re-creates.

// createMixedPeriodSourcedTemplate creates a live template that sources the
// offering, carries templatePeriodID as its own pin, and has a Monday
// schedule in one period and a Wednesday schedule in another.
func createMixedPeriodSourcedTemplate(
	t *testing.T,
	env *decisionTestEnv,
	name string,
	offeringID int64,
	templatePeriodID *int64,
) *timetable.Group {
	t.Helper()
	monday := createMixedPeriodCalendarPeriod(t, env, name+"-mo")
	wednesday := createMixedPeriodCalendarPeriod(t, env, name+"-mi")
	group := createCareOfferingTemplateGroup(t, env.db, name)
	group.TargetGroupType = timetable.TargetGroupTypeOffering
	group.SourceCareOfferingIDs = []int64{offeringID}
	if templatePeriodID != nil {
		group.CalendarPeriodID = templatePeriodID
	}
	require.NoError(t, timetableCompose.NativeGroupRecords(testTimetableRepositories(env.db).Timetable).Update(testpkg.Ctx(t), group))
	createCareOfferingTemplateSchedule(t, env.db, group.ID, timetable.WeekdayMonday, &monday)
	createCareOfferingTemplateSchedule(t, env.db, group.ID, timetable.WeekdayWednesday, &wednesday)
	return group
}

// createMixedPeriodCalendarPeriod creates a period that contains the env's
// enrollment phase (2026-09-01 to 2027-07-31).
func createMixedPeriodCalendarPeriod(t *testing.T, env *decisionTestEnv, name string) int64 {
	t.Helper()
	period := api.CreateCareOfferingCalendarPeriod(func(err error) { require.NoError(t, err) },
		testpkg.NewDatabase(t, env.db), testpkg.Ctx(t), testpkg.Tenant(t),
		uniqueSchemaName(name+"-"+t.Name()), calendar.NewDate(2026, 8, 1), calendar.NewDate(2027, 8, 31))
	return period.ID
}

// mixedPeriodTemplateFixture is one approved child holding the template's
// source offering plus one other offering, with its row on the
// mixed-period template planned by the template resync without a period.
type mixedPeriodTemplateFixture struct {
	env       *decisionTestEnv
	template  *timetable.Group
	source    *offeringContract.CareOfferingRecord
	requestID int64
	childID   int64
}

func setupMixedPeriodTemplateFixture(t *testing.T, email string) mixedPeriodTemplateFixture {
	t.Helper()
	env := setupDecisionTest(t)
	source := createAdjustmentCareOfferingWith(t, env, "Mittagessen", nil)
	other := createAdjustmentCareOfferingWith(t, env, "Spaetbetreuung", nil)
	pinID := createMixedPeriodCalendarPeriod(t, env, "pin")
	template := createMixedPeriodSourcedTemplate(t, env, "MittagessenGemischt", source.ID, &pinID)

	requestID, childID, studentID := submitApprovedAdjustmentChild(t, env, email, "Gemischt",
		[]*offeringContract.CareOfferingRecord{source, other})
	require.NoError(t, offeringResyncer(t, env).ResyncTemplateOfferingRoster(testpkg.Ctx(t), timetable.OfferingRosterResyncInput{
		TemplateID:       template.ID,
		OfferingIDs:      []int64{source.ID},
		CalendarPeriodID: &pinID,
		EffectiveFrom:    offeringResyncToday,
	}))
	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1, "the template resync plans the child on the template")
	require.Equal(t, studentID, rows[0].StudentID)
	require.Nil(t, rows[0].CalendarPeriodID, "the schedules name different periods, so the row carries none (#5176)")

	return mixedPeriodTemplateFixture{
		env: env, template: template, source: source,
		requestID: requestID, childID: childID,
	}
}

func TestDecide_PlansChildOnMixedPeriodTemplateWithTemplatePin(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	source := createSourceOffering(t, env, "FreigabeGemischt", nil)
	pinID := createMixedPeriodCalendarPeriod(t, env, "pin")
	template := createMixedPeriodSourcedTemplate(t, env, "FreigabeTerminGemischt", source.ID, &pinID)

	studentID, childID := submitAndApproveOfferingChild(t, env, source.ID, "freigabe-gemischt@example.com", "Gina", 2)

	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1, "approval must plan the child like the template resync does")
	assert.Equal(t, studentID, rows[0].StudentID)
	require.NotNil(t, rows[0].EnrollmentRequestChildID)
	assert.Equal(t, childID, *rows[0].EnrollmentRequestChildID)
	assert.Nil(t, rows[0].CalendarPeriodID, "the row carries no period, as the resync writes it (#5176)")
	assert.Equal(t, []int{1}, rows[0].SelectedWeekdays)
}

func TestUpdateChildOfferings_DatedSwitchKeepsMixedPeriodTemplate(t *testing.T) {
	t.Parallel()

	f := setupMixedPeriodTemplateFixture(t, "wechsel-gemischt@example.com")
	ctx := testpkg.Ctx(t)
	replacement := createAdjustmentCareOfferingWith(t, f.env, "Fruehbetreuung", nil)
	before := loadTemplateEnrollments(t, f.env, f.template.ID)[0]
	switchDate := calendar.Date(f.env.sourcePhase.ServiceStartDate).AddDays(60)

	_, err := testpkg.TenantQuery1(t, ctx, f.env.db, f.env.decision.UpdateChildOfferings, enrollmentTest.UpdateChildOfferingsInput{
		RequestID:      f.requestID,
		ChildID:        f.childID,
		ActorAccountID: f.env.creatorID,
		ActorRole:      "admin",
		Reason:         "Wechsel zum Halbjahr",
		EffectiveFrom:  &switchDate,
		Offerings: []enrollmentTest.OfferingAdjustmentSelection{
			{OfferingID: f.source.ID, SelectedDays: []string{"mon"}},
			{OfferingID: replacement.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	rows := loadTemplateEnrollments(t, f.env, f.template.ID)
	require.Len(t, rows, 1, "the unchanged source offering keeps its row instead of a cap and a gap")
	assert.Equal(t, before.ID, rows[0].ID)
	assert.Equal(t, before.ValidFrom, rows[0].ValidFrom)
	assert.Equal(t, before.ValidUntil, rows[0].ValidUntil, "the row must not be capped at the switch date")
}
