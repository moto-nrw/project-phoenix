package contracttest_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"

	enrollmentTest "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"
	offeringContract "github.com/moto-nrw/project-phoenix/modules/enrollment/offeringcontract"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Regeltermine without a planning period (#5114): neither the template nor
// its schedules carry a calendar_period_id. The template-save resync plans
// such a template's sourced children; the decision and adjustment flows
// must plan them the same way instead of skipping the template — otherwise a
// correction deletes rows it never re-creates.

// createUnperiodizedSourcedTemplate creates a live Monday template that
// sources the offering and has no planning period on the template or its
// schedule.
func createUnperiodizedSourcedTemplate(t *testing.T, env *decisionTestEnv, name string, offeringID int64) *timetable.Group {
	t.Helper()
	group := createCareOfferingTemplateGroup(t, env.db, name)
	group.TargetGroupType = timetable.TargetGroupTypeOffering
	group.SourceCareOfferingIDs = []int64{offeringID}
	require.NoError(t, timetableCompose.NativeGroupRecords(testTimetableRepositories(env.db).Timetable).Update(testpkg.Ctx(t), group))
	createCareOfferingTemplateSchedule(t, env.db, group.ID, timetable.WeekdayMonday, nil)
	return group
}

// unperiodizedTemplateFixture is one approved child holding the template's
// source offering plus one other offering, with its row on the template
// planned by the template-save resync.
type unperiodizedTemplateFixture struct {
	env       *decisionTestEnv
	template  *timetable.Group
	source    *offeringContract.CareOfferingRecord
	other     *offeringContract.CareOfferingRecord
	requestID int64
	childID   int64
	studentID int64
}

func setupUnperiodizedTemplateFixture(t *testing.T, email string) unperiodizedTemplateFixture {
	t.Helper()
	env := setupDecisionTest(t)
	source := createAdjustmentCareOfferingWith(t, env, "Mittagessen", nil)
	other := createAdjustmentCareOfferingWith(t, env, "Spaetbetreuung", nil)
	template := createUnperiodizedSourcedTemplate(t, env, "MittagessenOhneZeitraum", source.ID)

	requestID, childID, studentID := submitApprovedAdjustmentChild(t, env, email, "OhneZeitraum",
		[]*offeringContract.CareOfferingRecord{source, other})
	require.NoError(t, offeringResyncer(t, env).ResyncTemplateOfferingRoster(testpkg.Ctx(t), timetable.OfferingRosterResyncInput{
		TemplateID:    template.ID,
		OfferingIDs:   []int64{source.ID},
		EffectiveFrom: offeringResyncToday,
	}))
	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1, "the template-save resync plans the child on the template without a period")
	require.Equal(t, studentID, rows[0].StudentID)

	return unperiodizedTemplateFixture{
		env: env, template: template, source: source, other: other,
		requestID: requestID, childID: childID, studentID: studentID,
	}
}

func TestDecide_PlansChildOnSourcedTemplateWithoutPeriod(t *testing.T) {
	t.Parallel()

	env := setupDecisionTest(t)
	source := createSourceOffering(t, env, "FreigabeOhneZeitraum", nil)
	template := createUnperiodizedSourcedTemplate(t, env, "FreigabeTerminOhneZeitraum", source.ID)

	studentID, childID := submitAndApproveOfferingChild(t, env, source.ID, "freigabe-ohne-zeitraum@example.com", "Fio", 2)

	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1, "approval must plan the child like the template-save resync does")
	assert.Equal(t, studentID, rows[0].StudentID)
	require.NotNil(t, rows[0].EnrollmentRequestChildID)
	assert.Equal(t, childID, *rows[0].EnrollmentRequestChildID)
	assert.Nil(t, rows[0].CalendarPeriodID)
	assert.Equal(t, []int{1}, rows[0].SelectedWeekdays)
}

func TestUpdateChildOfferings_UndatedCorrectionKeepsTemplateWithoutPeriod(t *testing.T) {
	t.Parallel()

	f := setupUnperiodizedTemplateFixture(t, "korrektur-ohne-zeitraum@example.com")
	ctx := testpkg.Ctx(t)
	replacement := createAdjustmentCareOfferingWith(t, f.env, "Fruehbetreuung", nil)

	// Correct only the other offering; the template's source stays booked.
	_, err := testpkg.TenantQuery1(t, ctx, f.env.db, f.env.decision.UpdateChildOfferings, enrollmentTest.UpdateChildOfferingsInput{
		RequestID:      f.requestID,
		ChildID:        f.childID,
		ActorAccountID: f.env.creatorID,
		ActorRole:      "admin",
		Reason:         "Falsches Angebot erfasst",
		Offerings: []enrollmentTest.OfferingAdjustmentSelection{
			{OfferingID: f.source.ID, SelectedDays: []string{"mon"}},
			{OfferingID: replacement.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	rows := loadTemplateEnrollments(t, f.env, f.template.ID)
	require.Len(t, rows, 1, "the correction must not drop the child from a template it cannot resolve a period for")
	assert.Equal(t, f.studentID, rows[0].StudentID)
	require.NotNil(t, rows[0].EnrollmentRequestChildID)
	assert.Equal(t, f.childID, *rows[0].EnrollmentRequestChildID)
	assert.Equal(t, string(f.env.sourcePhase.ServiceStartDate), rows[0].ValidFrom)
}

func TestUpdateChildOfferings_DatedSwitchKeepsTemplateWithoutPeriod(t *testing.T) {
	t.Parallel()

	f := setupUnperiodizedTemplateFixture(t, "wechsel-ohne-zeitraum@example.com")
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
