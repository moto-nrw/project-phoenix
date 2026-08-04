package enrollment_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Offering-source integration tests (#2137): one Betreuungsangebot feeding
// several grade-filtered Regeltermine, via both write paths — the decision
// fan-out at approval time and the template-save resync.

// createSourcedTemplate creates a live template that declares the offering as
// its roster source, with one Monday schedule pinned to the period.
func createSourcedTemplate(
	t *testing.T,
	env *decisionTestEnv,
	name string,
	offeringID int64,
	gradeLevels []int,
	period *scheduleModels.CalendarPeriod,
) *activitiesModels.Group {
	t.Helper()
	group := createCareOfferingTemplateGroup(t, env.db, name)
	group.TargetGroupType = activitiesModels.TargetGroupTypeAngebot
	group.SourceCareOfferingID = &offeringID
	group.SourceGradeLevels = gradeLevels
	group.CalendarPeriodID = &period.ID
	require.NoError(t, repositories.NewFactory(env.db).ActivityGroup.Update(testpkg.TenantContext(1), group))
	createCareOfferingTemplateSchedule(t, env.db, group.ID, activitiesModels.WeekdayMonday, &period.ID)
	return group
}

func createSourceOffering(t *testing.T, env *decisionTestEnv, name string, activityGroupID *int64) *enrollmentModels.CareOffering {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	offering := &enrollmentModels.CareOffering{
		PhaseID:         env.sourcePhase.ID,
		ActivityGroupID: activityGroupID,
		Name:            uniqueSchemaName(name + "-" + t.Name()),
		DaysOfWeekMode:  enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:   []string{"mon"},
		IsActive:        true,
	}
	offering.SetTenantID(1)
	require.NoError(t, env.repos.CareOffering.Create(ctx, offering))
	t.Cleanup(func() {
		_, _ = env.db.NewDelete().
			TableExpr("enrollment.care_offerings").
			Where("id = ?", offering.ID).
			Exec(context.Background())
	})
	return offering
}

// submitAndApproveOfferingChild submits one child with the offering selected
// and approves it, returning the created student id and request child id.
func submitAndApproveOfferingChild(
	t *testing.T,
	env *decisionTestEnv,
	offeringID int64,
	guardianEmail, firstName string,
	grade int16,
) (studentID, requestChildID int64) {
	t.Helper()
	ctx := testpkg.TenantContext(1)
	submitted, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          1,
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Quelle",
		GuardianEmail:     guardianEmail,
		ConsentFlags: map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		},
		Children: []enrollmentService.SubmitChild{{
			FirstName:        firstName,
			LastName:         "Quelle",
			DateOfBirth:      timezone.NewDate(2018, 4, 15),
			TargetGradeLevel: &grade,
			OfferingIDs:      []int64{offeringID},
		}},
	})
	require.NoError(t, err)
	require.Len(t, submitted.Children, 1)
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  submitted.Request.ID,
		ChildID:    submitted.Children[0].ID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	t.Cleanup(func() {
		_, _ = env.db.NewDelete().
			TableExpr("activities.student_enrollments").
			Where("student_id = ?", *outcome.Child.CreatedStudentID).
			Exec(context.Background())
	})
	return *outcome.Child.CreatedStudentID, submitted.Children[0].ID
}

func loadTemplateEnrollments(t *testing.T, env *decisionTestEnv, templateID int64) []activitiesModels.StudentEnrollment {
	t.Helper()
	var rows []activitiesModels.StudentEnrollment
	require.NoError(t, env.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`activities.student_enrollments AS "student_enrollment"`).
		Where(`"student_enrollment".tenant_id = ?`, 1).
		Where(`"student_enrollment".activity_group_id = ?`, templateID).
		Order("id ASC").
		Scan(testpkg.TenantContext(1)))
	return rows
}

func offeringSourcePeriod(t *testing.T, env *decisionTestEnv) *scheduleModels.CalendarPeriod {
	t.Helper()
	return createCareOfferingTestPeriod(t, env.db, "offering-source",
		timezone.NewDate(2026, 8, 1),
		timezone.NewDate(2027, 8, 31))
}

func offeringResyncer(t *testing.T, env *decisionTestEnv) enrollmentService.OfferingRosterResyncer {
	t.Helper()
	resyncer, ok := env.decision.(enrollmentService.OfferingRosterResyncer)
	require.True(t, ok, "decision service must implement the offering roster resync contract")
	return resyncer
}

// ---- decision fan-out ----------------------------------------------------

func TestDecide_FansOutToGradeFilteredSourcedTemplates(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "Nachmittag", nil)
	templateGrade2 := createSourcedTemplate(t, env, "Jahrgang2", offering.ID, []int{2}, period)
	templateGrade3 := createSourcedTemplate(t, env, "Jahrgang3", offering.ID, []int{3}, period)

	studentID, requestChildID := submitAndApproveOfferingChild(t, env, offering.ID, "fanout-grade2@example.com", "Zoe", 2)

	rows2 := loadTemplateEnrollments(t, env, templateGrade2.ID)
	require.Len(t, rows2, 1, "grade-2 child must land on the grade-2 Termin")
	assert.Equal(t, studentID, rows2[0].StudentID)
	require.NotNil(t, rows2[0].EnrollmentRequestChildID)
	assert.Equal(t, requestChildID, *rows2[0].EnrollmentRequestChildID)
	assert.Equal(t, []int{1}, rows2[0].SelectedWeekdays, "fixed offering days constrain the row")
	require.NotNil(t, rows2[0].CalendarPeriodID)
	assert.Equal(t, period.ID, *rows2[0].CalendarPeriodID)

	assert.Empty(t, loadTemplateEnrollments(t, env, templateGrade3.ID),
		"grade-2 child must not land on the grade-3 Termin")
}

func TestDecide_OverlappingSourcedTemplatesBothReceiveTheChild(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "Ueberlapp", nil)
	templateBroad := createSourcedTemplate(t, env, "Jg1bis2", offering.ID, []int{1, 2}, period)
	templateNarrow := createSourcedTemplate(t, env, "Jg2", offering.ID, []int{2}, period)

	studentID, _ := submitAndApproveOfferingChild(t, env, offering.ID, "fanout-overlap@example.com", "Ova", 2)

	broad := loadTemplateEnrollments(t, env, templateBroad.ID)
	narrow := loadTemplateEnrollments(t, env, templateNarrow.ID)
	require.Len(t, broad, 1, "overlapping filters plan the child into both Termine (the editor warns)")
	require.Len(t, narrow, 1)
	assert.Equal(t, studentID, broad[0].StudentID)
	assert.Equal(t, studentID, narrow[0].StudentID)
}

func TestDecide_SourcedFanOutCoexistsWithLegacyLink(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()

	period := offeringSourcePeriod(t, env)
	legacyTemplate := createCareOfferingTemplateGroup(t, env.db, "Legacy")
	createCareOfferingTemplateSchedule(t, env.db, legacyTemplate.ID, activitiesModels.WeekdayMonday, &period.ID)
	offering := createSourceOffering(t, env, "LegacyPlusSourced", &legacyTemplate.ID)
	sourcedTemplate := createSourcedTemplate(t, env, "SourcedJg2", offering.ID, []int{2}, period)

	studentID, _ := submitAndApproveOfferingChild(t, env, offering.ID, "fanout-legacy@example.com", "Lea", 2)

	legacyRows := loadTemplateEnrollments(t, env, legacyTemplate.ID)
	sourcedRows := loadTemplateEnrollments(t, env, sourcedTemplate.ID)
	require.Len(t, legacyRows, 1, "the legacy ActivityGroupID feed keeps working")
	require.Len(t, sourcedRows, 1, "the sourced template receives the child too")
	assert.Equal(t, studentID, legacyRows[0].StudentID)
	assert.Equal(t, studentID, sourcedRows[0].StudentID)
}

// ---- template-save resync ------------------------------------------------

func TestResyncTemplateOfferingRoster_SeedsExistingApprovedChildren(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "SeedBestand", nil)

	// Approved BEFORE any sourced template exists: without a legacy link the
	// approval creates no enrollment rows at all.
	studentGrade2, childGrade2 := submitAndApproveOfferingChild(t, env, offering.ID, "seed-grade2@example.com", "Sena", 2)
	studentGrade3, _ := submitAndApproveOfferingChild(t, env, offering.ID, "seed-grade3@example.com", "Drei", 3)

	template := createSourcedTemplate(t, env, "SeedJg2", offering.ID, []int{2}, period)
	require.NoError(t, offeringResyncer(t, env).ResyncTemplateOfferingRoster(
		testpkg.TenantContext(1),
		scheduleService.OfferingRosterResyncInput{
			TemplateID:       template.ID,
			OfferingID:       &offering.ID,
			GradeLevels:      []int{2},
			CalendarPeriodID: &period.ID,
			EffectiveFrom:    timezone.TodayDate(),
		},
	))

	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1, "only the grade-2 child is seeded")
	assert.Equal(t, studentGrade2, rows[0].StudentID)
	require.NotNil(t, rows[0].EnrollmentRequestChildID)
	assert.Equal(t, childGrade2, *rows[0].EnrollmentRequestChildID)
	assert.Equal(t, []int{1}, rows[0].SelectedWeekdays)
	require.NotNil(t, rows[0].ValidUntil, "seeded rows are bounded by the phase window")
	assert.NotEqual(t, studentGrade3, rows[0].StudentID)
}

func TestResyncTemplateOfferingRoster_EmptyFilterSeedsAllAndIsIdempotent(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "SeedAlle", nil)
	submitAndApproveOfferingChild(t, env, offering.ID, "seed-all-1@example.com", "Ada", 1)
	submitAndApproveOfferingChild(t, env, offering.ID, "seed-all-2@example.com", "Ben", 4)

	template := createSourcedTemplate(t, env, "SeedAlleTermine", offering.ID, nil, period)
	input := scheduleService.OfferingRosterResyncInput{
		TemplateID:       template.ID,
		OfferingID:       &offering.ID,
		CalendarPeriodID: &period.ID,
		EffectiveFrom:    timezone.TodayDate(),
	}
	resyncer := offeringResyncer(t, env)
	require.NoError(t, resyncer.ResyncTemplateOfferingRoster(testpkg.TenantContext(1), input))
	require.Len(t, loadTemplateEnrollments(t, env, template.ID), 2, "empty filter admits every enrolled child")

	// Re-running with unchanged input must keep matching rows instead of
	// duplicating or churning them.
	require.NoError(t, resyncer.ResyncTemplateOfferingRoster(testpkg.TenantContext(1), input))
	assert.Len(t, loadTemplateEnrollments(t, env, template.ID), 2)
}

func TestResyncTemplateOfferingRoster_FilterChangeAndSourceRemoval(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "SeedWechsel", nil)
	studentGrade1, _ := submitAndApproveOfferingChild(t, env, offering.ID, "switch-grade1@example.com", "Eins", 1)
	studentGrade2, _ := submitAndApproveOfferingChild(t, env, offering.ID, "switch-grade2@example.com", "Zwei", 2)

	template := createSourcedTemplate(t, env, "SeedWechselTermin", offering.ID, []int{1}, period)
	resyncer := offeringResyncer(t, env)
	ctx := testpkg.TenantContext(1)

	require.NoError(t, resyncer.ResyncTemplateOfferingRoster(ctx, scheduleService.OfferingRosterResyncInput{
		TemplateID:       template.ID,
		OfferingID:       &offering.ID,
		GradeLevels:      []int{1},
		CalendarPeriodID: &period.ID,
		EffectiveFrom:    timezone.TodayDate(),
	}))
	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1)
	assert.Equal(t, studentGrade1, rows[0].StudentID)

	// Filter change 1 → 2: the grade-1 row disappears, the grade-2 child is
	// seeded (the rows have future validity, so removal deletes them).
	require.NoError(t, resyncer.ResyncTemplateOfferingRoster(ctx, scheduleService.OfferingRosterResyncInput{
		TemplateID:         template.ID,
		PreviousOfferingID: &offering.ID,
		OfferingID:         &offering.ID,
		GradeLevels:        []int{2},
		CalendarPeriodID:   &period.ID,
		EffectiveFrom:      timezone.TodayDate(),
	}))
	rows = loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1)
	assert.Equal(t, studentGrade2, rows[0].StudentID)

	// Source removal clears the sourced roster.
	require.NoError(t, resyncer.ResyncTemplateOfferingRoster(ctx, scheduleService.OfferingRosterResyncInput{
		TemplateID:         template.ID,
		PreviousOfferingID: &offering.ID,
		OfferingID:         nil,
		EffectiveFrom:      timezone.TodayDate(),
	}))
	assert.Empty(t, loadTemplateEnrollments(t, env, template.ID))
}

func TestResyncTemplateOfferingRoster_SeedsFutureDatedLinkFromItsStart(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "SpaeterWechsel", nil)
	studentID, childID := submitAndApproveOfferingChild(t, env, offering.ID, "future-link@example.com", "Fee", 2)

	// The child only holds the offering from a future switch date onward.
	require.NoError(t, env.repos.RequestChildOffering.ReplaceForRequestChild(ctx, childID, nil))
	switchDate := env.sourcePhase.ServiceStartDate.AddDays(30)
	require.NoError(t, env.repos.RequestChildOffering.ScheduleReplacementForRequestChild(
		ctx,
		childID,
		switchDate,
		[]*enrollmentModels.RequestChildOffering{{CareOfferingID: offering.ID}},
	))

	template := createSourcedTemplate(t, env, "SpaeterWechselTermin", offering.ID, []int{2}, period)
	require.NoError(t, offeringResyncer(t, env).ResyncTemplateOfferingRoster(ctx,
		scheduleService.OfferingRosterResyncInput{
			TemplateID:       template.ID,
			OfferingID:       &offering.ID,
			GradeLevels:      []int{2},
			CalendarPeriodID: &period.ID,
			EffectiveFrom:    timezone.TodayDate(),
		},
	))

	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1)
	assert.Equal(t, studentID, rows[0].StudentID)
	assert.Equal(t, switchDate, rows[0].ValidFrom,
		"a future-dated offering link must not plan the child before the switch date")
	require.NotNil(t, rows[0].ValidUntil)
	assert.Equal(t, env.sourcePhase.ServiceEndDate.AddDays(1), *rows[0].ValidUntil)
}

func TestResyncTemplateOfferingRoster_CapsRowAtLinkEnd(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "EndetFrueher", nil)
	studentID, childID := submitAndApproveOfferingChild(t, env, offering.ID, "ending-link@example.com", "Ende", 2)

	// The child leaves the offering mid-phase: the link is closed at the
	// switch date and nothing follows it.
	switchDate := env.sourcePhase.ServiceStartDate.AddDays(60)
	require.NoError(t, env.repos.RequestChildOffering.ScheduleReplacementForRequestChild(ctx, childID, switchDate, nil))

	template := createSourcedTemplate(t, env, "EndetFrueherTermin", offering.ID, []int{2}, period)
	require.NoError(t, offeringResyncer(t, env).ResyncTemplateOfferingRoster(ctx,
		scheduleService.OfferingRosterResyncInput{
			TemplateID:       template.ID,
			OfferingID:       &offering.ID,
			GradeLevels:      []int{2},
			CalendarPeriodID: &period.ID,
			EffectiveFrom:    timezone.TodayDate(),
		},
	))

	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1)
	assert.Equal(t, studentID, rows[0].StudentID)
	assert.Equal(t, env.sourcePhase.ServiceStartDate, rows[0].ValidFrom)
	require.NotNil(t, rows[0].ValidUntil)
	assert.Equal(t, switchDate, *rows[0].ValidUntil,
		"a link ending mid-phase must not plan the child for the rest of the phase")
}

func TestResyncTemplateOfferingRoster_ProtectsLegacyLinkedRows(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()

	period := offeringSourcePeriod(t, env)
	legacyTemplate := createCareOfferingTemplateGroup(t, env.db, "LegacyProtect")
	createCareOfferingTemplateSchedule(t, env.db, legacyTemplate.ID, activitiesModels.WeekdayMonday, &period.ID)
	legacyOffering := createSourceOffering(t, env, "LegacyFeed", &legacyTemplate.ID)
	studentID, _ := submitAndApproveOfferingChild(t, env, legacyOffering.ID, "legacy-protect@example.com", "Leg", 2)
	require.Len(t, loadTemplateEnrollments(t, env, legacyTemplate.ID), 1)

	// Sourcing the SAME template from a different offering must not clear the
	// legacy offering's rows.
	otherOffering := createSourceOffering(t, env, "AndereQuelle", nil)
	require.NoError(t, offeringResyncer(t, env).ResyncTemplateOfferingRoster(
		testpkg.TenantContext(1),
		scheduleService.OfferingRosterResyncInput{
			TemplateID:       legacyTemplate.ID,
			OfferingID:       &otherOffering.ID,
			CalendarPeriodID: &period.ID,
			EffectiveFrom:    timezone.TodayDate(),
		},
	))
	rows := loadTemplateEnrollments(t, env, legacyTemplate.ID)
	require.Len(t, rows, 1, "legacy-fed row survives sourcing from another offering")
	assert.Equal(t, studentID, rows[0].StudentID)
}

func TestResyncTemplateOfferingRoster_UnknownOfferingIsInvalid(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()

	period := offeringSourcePeriod(t, env)
	template := createCareOfferingTemplateGroup(t, env.db, "InvalidQuelle")
	createCareOfferingTemplateSchedule(t, env.db, template.ID, activitiesModels.WeekdayMonday, &period.ID)

	missing := int64(999999999)
	err := offeringResyncer(t, env).ResyncTemplateOfferingRoster(
		testpkg.TenantContext(1),
		scheduleService.OfferingRosterResyncInput{
			TemplateID:    template.ID,
			OfferingID:    &missing,
			EffectiveFrom: timezone.TodayDate(),
		},
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, scheduleService.ErrOfferingSourceInvalid)
}

// ---- review follow-ups (#2147) --------------------------------------------

// A child who leaves the offering mid-phase and re-joins later holds two
// disjoint links. Merging them into one interval would plan the child during
// the gap, so the resync must seed one row per window.
func TestResyncTemplateOfferingRoster_GapBetweenLinksStaysUnplanned(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "LueckeQuelle", nil)
	studentID, childID := submitAndApproveOfferingChild(t, env, offering.ID, "gap-link@example.com", "Gap", 2)

	leaveDate := env.sourcePhase.ServiceStartDate.AddDays(30)
	rejoinDate := env.sourcePhase.ServiceStartDate.AddDays(90)
	require.NoError(t, env.repos.RequestChildOffering.ScheduleReplacementForRequestChild(ctx, childID, leaveDate, nil))
	require.NoError(t, env.repos.RequestChildOffering.ScheduleReplacementForRequestChild(
		ctx,
		childID,
		rejoinDate,
		[]*enrollmentModels.RequestChildOffering{{CareOfferingID: offering.ID}},
	))

	template := createSourcedTemplate(t, env, "LueckeTermin", offering.ID, []int{2}, period)
	require.NoError(t, offeringResyncer(t, env).ResyncTemplateOfferingRoster(ctx,
		scheduleService.OfferingRosterResyncInput{
			TemplateID:       template.ID,
			OfferingID:       &offering.ID,
			GradeLevels:      []int{2},
			CalendarPeriodID: &period.ID,
			EffectiveFrom:    timezone.TodayDate(),
		},
	))

	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 2, "two disjoint links yield two rows — one merged row would plan the gap")
	assert.Equal(t, studentID, rows[0].StudentID)
	assert.Equal(t, env.sourcePhase.ServiceStartDate, rows[0].ValidFrom)
	require.NotNil(t, rows[0].ValidUntil)
	assert.Equal(t, leaveDate, *rows[0].ValidUntil)
	assert.Equal(t, studentID, rows[1].StudentID)
	assert.Equal(t, rejoinDate, rows[1].ValidFrom)
	require.NotNil(t, rows[1].ValidUntil)
	assert.Equal(t, env.sourcePhase.ServiceEndDate.AddDays(1), *rows[1].ValidUntil)
}

// An existing row must SHRINK when the child's offering window contracted
// after the row was seeded — only ever extending would keep planning the
// child past the link's end.
func TestResyncTemplateOfferingRoster_ShrinksRetainedRowToLinkEnd(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "SchrumpfQuelle", nil)
	studentID, childID := submitAndApproveOfferingChild(t, env, offering.ID, "shrink-link@example.com", "Kurz", 2)

	template := createSourcedTemplate(t, env, "SchrumpfTermin", offering.ID, []int{2}, period)
	input := scheduleService.OfferingRosterResyncInput{
		TemplateID:       template.ID,
		OfferingID:       &offering.ID,
		GradeLevels:      []int{2},
		CalendarPeriodID: &period.ID,
		EffectiveFrom:    timezone.TodayDate(),
	}
	resyncer := offeringResyncer(t, env)
	require.NoError(t, resyncer.ResyncTemplateOfferingRoster(ctx, input))
	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].ValidUntil)
	require.Equal(t, env.sourcePhase.ServiceEndDate.AddDays(1), *rows[0].ValidUntil)

	// The child leaves the offering mid-phase AFTER the row was seeded.
	capDate := env.sourcePhase.ServiceStartDate.AddDays(45)
	require.NoError(t, env.repos.RequestChildOffering.ScheduleReplacementForRequestChild(ctx, childID, capDate, nil))

	require.NoError(t, resyncer.ResyncTemplateOfferingRoster(ctx, input))
	rows = loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1)
	assert.Equal(t, studentID, rows[0].StudentID)
	require.NotNil(t, rows[0].ValidUntil)
	assert.Equal(t, capDate, *rows[0].ValidUntil,
		"the retained row must shrink to the contracted offering window")
}

// Retargeting a template onto another offering must respect WHEN the child
// holds the new offering: a row inherited from the old source may not keep a
// start before the new link's ValidFrom.
func TestResyncTemplateOfferingRoster_SourceSwitchRespectsNewLinkStart(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	period := offeringSourcePeriod(t, env)
	offeringA := createSourceOffering(t, env, "QuelleAlt", nil)
	offeringB := createSourceOffering(t, env, "QuelleNeu", nil)
	studentID, childID := submitAndApproveOfferingChild(t, env, offeringA.ID, "switch-source@example.com", "Sway", 2)

	template := createSourcedTemplate(t, env, "QuellwechselTermin", offeringA.ID, []int{2}, period)
	resyncer := offeringResyncer(t, env)
	require.NoError(t, resyncer.ResyncTemplateOfferingRoster(ctx, scheduleService.OfferingRosterResyncInput{
		TemplateID:       template.ID,
		OfferingID:       &offeringA.ID,
		GradeLevels:      []int{2},
		CalendarPeriodID: &period.ID,
		EffectiveFrom:    timezone.TodayDate(),
	}))
	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1)
	require.Equal(t, env.sourcePhase.ServiceStartDate, rows[0].ValidFrom)

	// The child switches to offering B mid-phase; the template is retargeted
	// from A to B in the same breath.
	switchDate := env.sourcePhase.ServiceStartDate.AddDays(45)
	require.NoError(t, env.repos.RequestChildOffering.ScheduleReplacementForRequestChild(
		ctx,
		childID,
		switchDate,
		[]*enrollmentModels.RequestChildOffering{{CareOfferingID: offeringB.ID}},
	))
	require.NoError(t, resyncer.ResyncTemplateOfferingRoster(ctx, scheduleService.OfferingRosterResyncInput{
		TemplateID:         template.ID,
		PreviousOfferingID: &offeringA.ID,
		OfferingID:         &offeringB.ID,
		GradeLevels:        []int{2},
		CalendarPeriodID:   &period.ID,
		EffectiveFrom:      timezone.TodayDate(),
	}))

	rows = loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1)
	assert.Equal(t, studentID, rows[0].StudentID)
	assert.Equal(t, switchDate, rows[0].ValidFrom,
		"the row must not start before the child holds the NEW offering")
	require.NotNil(t, rows[0].ValidUntil)
	assert.Equal(t, env.sourcePhase.ServiceEndDate.AddDays(1), *rows[0].ValidUntil)
}

// A grade transition rewrites school classes; the tenant-wide resync must
// move children between Jahrgang-filtered Termine accordingly.
func TestResyncOfferingSourcedTemplates_FollowsClassChange(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "VersetzungsQuelle", nil)
	templateGrade2 := createSourcedTemplate(t, env, "VersetzungJg2", offering.ID, []int{2}, period)
	templateGrade3 := createSourcedTemplate(t, env, "VersetzungJg3", offering.ID, []int{3}, period)

	studentID, _ := submitAndApproveOfferingChild(t, env, offering.ID, "promotion@example.com", "Promo", 2)
	require.Len(t, loadTemplateEnrollments(t, env, templateGrade2.ID), 1)
	require.Empty(t, loadTemplateEnrollments(t, env, templateGrade3.ID))

	// Promotion: the child moves from Jahrgang 2 into Jahrgang 3.
	_, err := env.db.NewUpdate().
		TableExpr("users.students").
		Set("school_class = ?", "3a").
		Where("id = ?", studentID).
		Exec(ctx)
	require.NoError(t, err)

	resyncAll, ok := env.decision.(interface {
		ResyncOfferingSourcedTemplates(ctx context.Context, effectiveFrom timezone.Date) error
	})
	require.True(t, ok, "decision service must implement the tenant-wide offering resync")
	require.NoError(t, resyncAll.ResyncOfferingSourcedTemplates(ctx, timezone.TodayDate()))

	assert.Empty(t, loadTemplateEnrollments(t, env, templateGrade2.ID),
		"the promoted child must leave the Jahrgang-2 Termin")
	rows := loadTemplateEnrollments(t, env, templateGrade3.ID)
	require.Len(t, rows, 1, "the promoted child must enter the Jahrgang-3 Termin")
	assert.Equal(t, studentID, rows[0].StudentID)
}

// Deleting an offering must degrade its sourced templates to plain rosters:
// the FK nulls the source and the DB trigger clears the grade filter, so the
// CHECK constraint cannot fail the delete.
func TestOfferingDelete_DegradesSourcedTemplate(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "LoeschQuelle", nil)
	template := createSourcedTemplate(t, env, "LoeschTermin", offering.ID, []int{2}, period)

	_, err := env.db.NewDelete().
		TableExpr("enrollment.care_offerings").
		Where("id = ?", offering.ID).
		Exec(ctx)
	require.NoError(t, err, "deleting an offering with a grade-filtered sourced template must not fail")

	degraded, err := repositories.NewFactory(env.db).ActivityGroup.FindByID(ctx, template.ID)
	require.NoError(t, err)
	assert.Nil(t, degraded.SourceCareOfferingID)
	assert.Empty(t, degraded.SourceGradeLevels)
}
