package enrollment_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
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
	group.SourceCareOfferingIDs = []int64{offeringID}
	group.SourceGradeLevels = gradeLevels
	group.CalendarPeriodID = &period.ID
	require.NoError(t, repositories.NewFactory(env.db).ActivityGroup.Update(testpkg.Ctx(t), group))
	createCareOfferingTemplateSchedule(t, env.db, group.ID, activitiesModels.WeekdayMonday, &period.ID)
	return group
}

func createSourceOffering(t *testing.T, env *decisionTestEnv, name string, activityGroupID *int64) *enrollmentModels.CareOffering {
	t.Helper()
	ctx := testpkg.Ctx(t)
	offering := &enrollmentModels.CareOffering{
		PhaseID:         env.sourcePhase.ID,
		ActivityGroupID: activityGroupID,
		Name:            uniqueSchemaName(name + "-" + t.Name()),
		DaysOfWeekMode:  enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:   []string{"mon"},
		PickupTimes:     carePickupTimes("mon"),
		IsActive:        true,
	}
	offering.SetTenantID(testpkg.Tenant(t))
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
	ctx := testpkg.Ctx(t)
	submitted, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
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
		Where(`"student_enrollment".tenant_id = ?`, testpkg.Tenant(t)).
		Where(`"student_enrollment".activity_group_id = ?`, templateID).
		Order("id ASC").
		Scan(testpkg.Ctx(t)))
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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
		testpkg.Ctx(t),
		scheduleService.OfferingRosterResyncInput{
			TemplateID:       template.ID,
			OfferingIDs:      []int64{offering.ID},
			GradeLevels:      []int{2},
			CalendarPeriodID: &period.ID,
			EffectiveFrom:    decisionTestToday,
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
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "SeedAlle", nil)
	submitAndApproveOfferingChild(t, env, offering.ID, "seed-all-1@example.com", "Ada", 1)
	submitAndApproveOfferingChild(t, env, offering.ID, "seed-all-2@example.com", "Ben", 4)

	template := createSourcedTemplate(t, env, "SeedAlleTermine", offering.ID, nil, period)
	input := scheduleService.OfferingRosterResyncInput{
		TemplateID:       template.ID,
		OfferingIDs:      []int64{offering.ID},
		CalendarPeriodID: &period.ID,
		EffectiveFrom:    timezone.TodayDate(),
	}
	resyncer := offeringResyncer(t, env)
	require.NoError(t, resyncer.ResyncTemplateOfferingRoster(testpkg.Ctx(t), input))
	require.Len(t, loadTemplateEnrollments(t, env, template.ID), 2, "empty filter admits every enrolled child")

	// Re-running with unchanged input must keep matching rows instead of
	// duplicating or churning them.
	require.NoError(t, resyncer.ResyncTemplateOfferingRoster(testpkg.Ctx(t), input))
	assert.Len(t, loadTemplateEnrollments(t, env, template.ID), 2)
}

func TestResyncTemplateOfferingRoster_FilterChangeAndSourceRemoval(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "SeedWechsel", nil)
	studentGrade1, _ := submitAndApproveOfferingChild(t, env, offering.ID, "switch-grade1@example.com", "Eins", 1)
	studentGrade2, _ := submitAndApproveOfferingChild(t, env, offering.ID, "switch-grade2@example.com", "Zwei", 2)

	template := createSourcedTemplate(t, env, "SeedWechselTermin", offering.ID, []int{1}, period)
	resyncer := offeringResyncer(t, env)
	ctx := testpkg.Ctx(t)

	require.NoError(t, resyncer.ResyncTemplateOfferingRoster(ctx, scheduleService.OfferingRosterResyncInput{
		TemplateID:       template.ID,
		OfferingIDs:      []int64{offering.ID},
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
		TemplateID:       template.ID,
		OfferingIDs:      []int64{offering.ID},
		GradeLevels:      []int{2},
		CalendarPeriodID: &period.ID,
		EffectiveFrom:    timezone.TodayDate(),
	}))
	rows = loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1)
	assert.Equal(t, studentGrade2, rows[0].StudentID)

	// Source removal clears the sourced roster.
	require.NoError(t, resyncer.ResyncTemplateOfferingRoster(ctx, scheduleService.OfferingRosterResyncInput{
		TemplateID:    template.ID,
		OfferingIDs:   nil,
		EffectiveFrom: timezone.TodayDate(),
	}))
	assert.Empty(t, loadTemplateEnrollments(t, env, template.ID))
}

func TestResyncTemplateOfferingRoster_SeedsFutureDatedLinkFromItsStart(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

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
			OfferingIDs:      []int64{offering.ID},
			GradeLevels:      []int{2},
			CalendarPeriodID: &period.ID,
			EffectiveFrom:    decisionTestToday,
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
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

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
			OfferingIDs:      []int64{offering.ID},
			GradeLevels:      []int{2},
			CalendarPeriodID: &period.ID,
			EffectiveFrom:    decisionTestToday,
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
	t.Parallel()

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
		testpkg.Ctx(t),
		scheduleService.OfferingRosterResyncInput{
			TemplateID:       legacyTemplate.ID,
			OfferingIDs:      []int64{otherOffering.ID},
			CalendarPeriodID: &period.ID,
			EffectiveFrom:    decisionTestToday,
		},
	))
	rows := loadTemplateEnrollments(t, env, legacyTemplate.ID)
	require.Len(t, rows, 1, "legacy-fed row survives sourcing from another offering")
	assert.Equal(t, studentID, rows[0].StudentID)
}

// The legacy protection follows the linked template's split lineage: the
// legacy fan-out writes rows on every overlapping series segment, so a
// successor's resync — e.g. the dropped-source cleanup after a split away
// from 'angebot' — must not treat carried legacy-fed rows as source-owned
// leftovers and delete them (#2147 review).
func TestResyncTemplateOfferingRoster_LegacyProtectionFollowsSplitLineage(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()

	period := offeringSourcePeriod(t, env)
	predecessor := createCareOfferingTemplateGroup(t, env.db, "LineageVorgänger")
	createCareOfferingTemplateSchedule(t, env.db, predecessor.ID, activitiesModels.WeekdayMonday, &period.ID)
	legacyOffering := createSourceOffering(t, env, "LineageFeed", &predecessor.ID)
	studentID, _ := submitAndApproveOfferingChild(t, env, legacyOffering.ID, "lineage-protect@example.com", "Lin", 2)
	predecessorRows := loadTemplateEnrollments(t, env, predecessor.ID)
	require.Len(t, predecessorRows, 1)

	// A successor segment of the same series with the carried legacy-fed row —
	// the shapes a split writes. The legacy link still points at the
	// predecessor, only the lineage connects it to the successor.
	repoFactory := repositories.NewFactory(env.db)
	successor := createCareOfferingTemplateGroup(t, env.db, "LineageNachfolger")
	successor.SeriesRootID = &predecessor.ID
	require.NoError(t, repoFactory.ActivityGroup.Update(testpkg.Ctx(t), successor))
	createCareOfferingTemplateSchedule(t, env.db, successor.ID, activitiesModels.WeekdayMonday, &period.ID)

	carried := predecessorRows[0]
	carriedRow := &activitiesModels.StudentEnrollment{
		StudentID:                carried.StudentID,
		ActivityGroupID:          successor.ID,
		ValidFrom:                carried.ValidFrom,
		ValidUntil:               carried.ValidUntil,
		CalendarPeriodID:         carried.CalendarPeriodID,
		EnrollmentRequestChildID: carried.EnrollmentRequestChildID,
		SelectedWeekdays:         carried.SelectedWeekdays,
	}
	carriedRow.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repoFactory.StudentEnrollment.Create(testpkg.Ctx(t), carriedRow))
	t.Cleanup(func() {
		_, _ = env.db.NewDelete().
			TableExpr("activities.student_enrollments").
			Where("id = ?", carriedRow.ID).
			Exec(context.Background())
	})

	// The dropped-source cleanup runs with OfferingID nil on the successor.
	require.NoError(t, offeringResyncer(t, env).ResyncTemplateOfferingRoster(
		testpkg.Ctx(t),
		scheduleService.OfferingRosterResyncInput{
			TemplateID:    successor.ID,
			EffectiveFrom: timezone.TodayDate(),
		},
	))

	rows := loadTemplateEnrollments(t, env, successor.ID)
	require.Len(t, rows, 1, "the legacy-fed carried row must survive the successor's cleanup")
	assert.Equal(t, studentID, rows[0].StudentID)
}

// A vanished offering id (row deleted without the detach flow running — the
// jsonb array carries no FK backstop) must not wedge the template: the resync
// drops the dangling id and degrades to the surviving sources, mirroring the
// old FK's graceful SET NULL. A hard reject would 400 every later edit of the
// template and make the tenant-wide resync skip it forever (review follow-up;
// replaces the earlier hard-reject behavior).
func TestResyncTemplateOfferingRoster_VanishedOfferingIsDropped(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "RestQuelle", nil)
	template := createSourcedTemplate(t, env, "RestTermin", offering.ID, nil, period)
	studentID, _ := submitAndApproveOfferingChild(t, env, offering.ID, "vanished-rest@example.com", "Vera", 2)

	missing := int64(999999999)
	require.NoError(t, offeringResyncer(t, env).ResyncTemplateOfferingRoster(ctx,
		scheduleService.OfferingRosterResyncInput{
			TemplateID:       template.ID,
			OfferingIDs:      []int64{missing, offering.ID},
			CalendarPeriodID: &period.ID,
			EffectiveFrom:    decisionTestToday,
		}))
	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1, "the surviving source must still seed its children")
	assert.Equal(t, studentID, rows[0].StudentID)

	// Only dangling ids left: the resync degrades to cleanup and retires the
	// sourced rows, the manual-roster end state the FK era reached via SET NULL.
	require.NoError(t, offeringResyncer(t, env).ResyncTemplateOfferingRoster(ctx,
		scheduleService.OfferingRosterResyncInput{
			TemplateID:       template.ID,
			OfferingIDs:      []int64{missing},
			CalendarPeriodID: &period.ID,
			EffectiveFrom:    decisionTestToday,
		}))
	assert.Empty(t, loadTemplateEnrollments(t, env, template.ID),
		"with no surviving source the resync must retire the sourced rows")
}

// The id list is bounded before any per-id lookup: the resync resolves ids
// individually, and the list arrives from query strings and the stored jsonb
// array, so an unbounded list would turn into thousands of sequential queries
// inside one tenant transaction.
func TestResyncTemplateOfferingRoster_CapsSourceCount(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()

	template := createCareOfferingTemplateGroup(t, env.db, "CapTermin")
	ids := make([]int64, scheduleService.MaxOfferingSourcesPerTemplate+1)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	err := offeringResyncer(t, env).ResyncTemplateOfferingRoster(
		testpkg.Ctx(t),
		scheduleService.OfferingRosterResyncInput{
			TemplateID:    template.ID,
			OfferingIDs:   ids,
			EffectiveFrom: timezone.TodayDate(),
		},
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, scheduleService.ErrOfferingSourceInvalid)
	assert.ErrorContains(t, err, "at most")
}

// ---- review follow-ups (#2147) --------------------------------------------

// A child who leaves the offering mid-phase and re-joins later holds two
// disjoint links. Merging them into one interval would plan the child during
// the gap, so the resync must seed one row per window.
func TestResyncTemplateOfferingRoster_GapBetweenLinksStaysUnplanned(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

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
			OfferingIDs:      []int64{offering.ID},
			GradeLevels:      []int{2},
			CalendarPeriodID: &period.ID,
			EffectiveFrom:    decisionTestToday,
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
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "SchrumpfQuelle", nil)
	studentID, childID := submitAndApproveOfferingChild(t, env, offering.ID, "shrink-link@example.com", "Kurz", 2)

	template := createSourcedTemplate(t, env, "SchrumpfTermin", offering.ID, []int{2}, period)
	input := scheduleService.OfferingRosterResyncInput{
		TemplateID:       template.ID,
		OfferingIDs:      []int64{offering.ID},
		GradeLevels:      []int{2},
		CalendarPeriodID: &period.ID,
		EffectiveFrom:    decisionTestToday,
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
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	offeringA := createSourceOffering(t, env, "QuelleAlt", nil)
	offeringB := createSourceOffering(t, env, "QuelleNeu", nil)
	studentID, childID := submitAndApproveOfferingChild(t, env, offeringA.ID, "switch-source@example.com", "Sway", 2)

	template := createSourcedTemplate(t, env, "QuellwechselTermin", offeringA.ID, []int{2}, period)
	resyncer := offeringResyncer(t, env)
	require.NoError(t, resyncer.ResyncTemplateOfferingRoster(ctx, scheduleService.OfferingRosterResyncInput{
		TemplateID:       template.ID,
		OfferingIDs:      []int64{offeringA.ID},
		GradeLevels:      []int{2},
		CalendarPeriodID: &period.ID,
		EffectiveFrom:    decisionTestToday,
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
		TemplateID:       template.ID,
		OfferingIDs:      []int64{offeringB.ID},
		GradeLevels:      []int{2},
		CalendarPeriodID: &period.ID,
		EffectiveFrom:    decisionTestToday,
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
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

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
	require.NoError(t, resyncAll.ResyncOfferingSourcedTemplates(ctx, decisionTestToday))

	assert.Empty(t, loadTemplateEnrollments(t, env, templateGrade2.ID),
		"the promoted child must leave the Jahrgang-2 Termin")
	rows := loadTemplateEnrollments(t, env, templateGrade3.ID)
	require.Len(t, rows, 1, "the promoted child must enter the Jahrgang-3 Termin")
	assert.Equal(t, studentID, rows[0].StudentID)
}

// A confirmed Änderungsanmeldung or admin data correction changes the class
// through SyncApprovedChildData. When that moves the child into another
// Jahrgang, the Jahrgang-filtered sourced Regeltermine must follow in the
// same flow, exactly like a direct school_class edit or a grade transition
// (#2147 review round 13).
func TestSyncApprovedChildData_ClassChangeResyncsSourcedTemplates(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "KorrekturQuelle", nil)
	templateGrade2 := createSourcedTemplate(t, env, "KorrekturJg2", offering.ID, []int{2}, period)
	templateGrade3 := createSourcedTemplate(t, env, "KorrekturJg3", offering.ID, []int{3}, period)

	studentID, childID := submitAndApproveOfferingChild(t, env, offering.ID, "korrektur@example.com", "Korri", 2)
	require.Len(t, loadTemplateEnrollments(t, env, templateGrade2.ID), 1)
	require.Empty(t, loadTemplateEnrollments(t, env, templateGrade3.ID))

	// The confirmed edit carries the new Jahrgang on the request child; the
	// sync re-derives school_class from it.
	child, err := env.repos.RequestChild.FindByID(ctx, childID)
	require.NoError(t, err)
	newGrade := int16(3)
	child.TargetGradeLevel = &newGrade
	require.NoError(t, env.repos.RequestChild.UpdateData(ctx, child))

	applier := changeRequestApplierForTest(t, env)
	_, err = applier.SyncApprovedChildData(ctx, enrollmentService.SyncApprovedChildDataInput{
		RequestID:      child.RequestID,
		ChildID:        childID,
		ActorAccountID: env.creatorID,
	})
	require.NoError(t, err)

	assert.Empty(t, loadTemplateEnrollments(t, env, templateGrade2.ID),
		"the corrected child must leave the Jahrgang-2 Termin")
	rows := loadTemplateEnrollments(t, env, templateGrade3.ID)
	require.Len(t, rows, 1, "the corrected child must enter the Jahrgang-3 Termin")
	assert.Equal(t, studentID, rows[0].StudentID)
}

// A re-enrollment approval (existing_students match) and the annual rollover
// share the attachApprovalToExistingStudent tail, which rewrites school_class.
// When that moves the child into another Jahrgang, the Jahrgang-filtered
// sourced Regeltermine must follow within the approval, exactly like a direct
// school_class edit or a confirmed Änderungsanmeldung (#2147 review round 17).
func TestDecide_ExistingStudentClassChangeResyncsSourcedTemplates(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "RenewalQuelle", nil)
	templateGrade2 := createSourcedTemplate(t, env, "RenewalJg2", offering.ID, []int{2}, period)
	templateGrade3 := createSourcedTemplate(t, env, "RenewalJg3", offering.ID, []int{3}, period)

	studentID, _ := submitAndApproveOfferingChild(t, env, offering.ID, "renewal-jg@example.com", "Runa", 2)
	require.Len(t, loadTemplateEnrollments(t, env, templateGrade2.ID), 1)
	require.Empty(t, loadTemplateEnrollments(t, env, templateGrade3.ID))

	// The renewal form carries the next Jahrgang and selects no offerings, so
	// only the approval's class-change resync can move the sourced rows.
	// Matching pins the child to the already-created student, the way an
	// existing_students phase does at submission time.
	grade := int16(3)
	submitted, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Erneuerung",
		GuardianEmail:     "renewal-jg-neu@example.com",
		ConsentFlags: map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		},
		Children: []enrollmentService.SubmitChild{{
			FirstName:        "Runa-Neu",
			LastName:         "Erneuerung",
			DateOfBirth:      timezone.NewDate(2018, 4, 15),
			TargetGradeLevel: &grade,
		}},
	})
	require.NoError(t, err)
	require.Len(t, submitted.Children, 1)
	matchChildToExistingStudent(t, env, submitted.Children[0].ID, studentID)

	_, err = env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  submitted.Request.ID,
		ChildID:    submitted.Children[0].ID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)

	assert.Empty(t, loadTemplateEnrollments(t, env, templateGrade2.ID),
		"the promoted child must leave the Jahrgang-2 Termin")
	rows := loadTemplateEnrollments(t, env, templateGrade3.ID)
	require.Len(t, rows, 1, "the promoted child must enter the Jahrgang-3 Termin")
	assert.Equal(t, studentID, rows[0].StudentID)
}

// Deleting an offering must degrade its sourced templates to plain rosters.
// Since the multi-source follow-up the source id array is jsonb and carries
// no FK, so there is no ON DELETE backstop anymore: the detach flow the
// care-offering/phase delete paths run BEFORE the row delete is the ONLY
// mechanism that rewrites the template's source columns — including the
// grade filter, whose CHECK forbids it without a source.
func TestOfferingDelete_DegradesSourcedTemplate(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "LoeschQuelle", nil)
	template := createSourcedTemplate(t, env, "LoeschTermin", offering.ID, []int{2}, period)

	detacher, ok := env.decision.(enrollmentService.CareOfferingSourcedTemplateResyncer)
	require.True(t, ok, "decision service must implement the sourced-template detach contract")
	require.NoError(t, detacher.DetachTemplatesSourcedFromOffering(ctx, offering.ID, timezone.TodayDate()))

	_, err := env.db.NewDelete().
		TableExpr("enrollment.care_offerings").
		Where("id = ?", offering.ID).
		Exec(ctx)
	require.NoError(t, err, "deleting an offering with a grade-filtered sourced template must not fail")

	degraded, err := repositories.NewFactory(env.db).ActivityGroup.FindByID(ctx, template.ID)
	require.NoError(t, err)
	assert.Empty(t, degraded.SourceCareOfferingIDs)
	assert.Empty(t, degraded.SourceGradeLevels)
}

// Migration 1.15.281 guard: the jsonb id array carries no FK and the
// orphaned-filter trigger of 1.15.266 is gone, so the CHECK constraint is the
// only DB-level line keeping "a grade filter never outlives its source" true
// for writes that bypass the service layer (generic Update, data migrations).
// NULL ids next to a non-NULL filter must be rejected: jsonb_typeof(NULL) is
// NULL, and without the explicit IS NOT NULL conjunct the CHECK would treat
// that branch as satisfied.
func TestOfferingSourceCheck_RejectsFilterWithoutSource(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "CheckQuelle", nil)
	template := createSourcedTemplate(t, env, "CheckTermin", offering.ID, []int{2}, period)

	_, err := env.db.NewUpdate().
		TableExpr("activities.groups").
		Set("source_care_offering_ids = NULL").
		Where("id = ?", template.ID).
		Exec(ctx)
	require.Error(t, err, "clearing the source while a grade filter remains must violate the CHECK")
	require.ErrorContains(t, err, "chk_activities_groups_offering_source")
}

// Multi-source follow-up: detaching ONE of two source offerings must keep the
// template sourced by the remainder — only the departing offering's children
// leave the roster.
func TestOfferingDetach_KeepsRemainingSources(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	offeringA := createSourceOffering(t, env, "DetachQuelleA", nil)
	offeringB := createSourceOffering(t, env, "DetachQuelleB", nil)
	template := createSourcedTemplate(t, env, "DetachTermin", offeringA.ID, nil, period)
	template.SourceCareOfferingIDs = []int64{offeringA.ID, offeringB.ID}
	require.NoError(t, repositories.NewFactory(env.db).ActivityGroup.Update(ctx, template))

	studentA, _ := submitAndApproveOfferingChild(t, env, offeringA.ID, "detach-a@example.com", "Alwa", 2)
	studentB, _ := submitAndApproveOfferingChild(t, env, offeringB.ID, "detach-b@example.com", "Bodo", 2)
	require.NoError(t, offeringResyncer(t, env).ResyncTemplateOfferingRoster(ctx,
		scheduleService.OfferingRosterResyncInput{
			TemplateID:       template.ID,
			OfferingIDs:      []int64{offeringA.ID, offeringB.ID},
			CalendarPeriodID: &period.ID,
			EffectiveFrom:    decisionTestToday,
		}))
	require.Len(t, loadTemplateEnrollments(t, env, template.ID), 2,
		"both offerings' children must be planned before the detach")

	detacher, ok := env.decision.(enrollmentService.CareOfferingSourcedTemplateResyncer)
	require.True(t, ok)
	require.NoError(t, detacher.DetachTemplatesSourcedFromOffering(ctx, offeringA.ID, decisionTestToday))

	kept, err := repositories.NewFactory(env.db).ActivityGroup.FindByID(ctx, template.ID)
	require.NoError(t, err)
	assert.Equal(t, []int64{offeringB.ID}, kept.SourceCareOfferingIDs,
		"the template must stay sourced by the remaining offering")

	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1, "only offering B's child may remain planned")
	assert.Equal(t, studentB, rows[0].StudentID)
	assert.NotEqual(t, studentA, rows[0].StudentID)
}

// A NEWLY submitted unknown source id must be rejected: without the
// single-source era's FK, accepting it would persist a dead source with a
// permanently empty roster (and the editor re-submits it on every save). A
// vanished id that is ALREADY stored on the template stays tolerated so
// edits of an orphaned template do not wedge.
func TestValidateTemplateOfferingSource_RejectsNewUnknownToleratesStored(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	offering := createSourceOffering(t, env, "PruefQuelle", nil)
	svc := enrollmentService.NewCareOfferingService(enrollmentService.CareOfferingServiceConfig{
		Repo:               env.repos.CareOffering,
		PhaseRepo:          env.repos.Phase,
		CalendarPeriodRepo: env.repos.CalendarPeriod,
	})
	validator, ok := svc.(enrollmentService.CareOfferingSeriesValidator)
	require.True(t, ok, "care offering service must implement the pre-write source validator")

	missing := int64(999999999)
	err := validator.ValidateTemplateOfferingSource(ctx, []int64{offering.ID, missing}, nil, nil)
	require.Error(t, err, "a new unknown id must not be accepted")
	require.ErrorIs(t, err, scheduleService.ErrOfferingSourceInvalid)
	require.ErrorContains(t, err, "not found")

	require.NoError(t, validator.ValidateTemplateOfferingSource(ctx, []int64{offering.ID, missing}, []int64{missing}, nil),
		"a stored dangling id must keep the template editable")
}

// A remaining source that has drifted invalid (here: the template re-pinned
// to a period the phase's service window cannot fit) must not strip the
// template's remaining sources on detach — the sibling is valid data, only
// its combination with the period pin failed. The detach keeps the source
// columns and skips the full union resync, but the DEPARTING offering's rows
// must still be retired: the delete behind the detach cascades their
// provenance away, after which no later resync could ever attribute them.
// Children the kept sibling also feeds stay planned.
func TestOfferingDetach_KeepsRemainingSourcesWhenSiblingDrifted(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	offeringA := createSourceOffering(t, env, "DriftQuelleA", nil)
	offeringB := createSourceOffering(t, env, "DriftQuelleB", nil)
	template := createSourcedTemplate(t, env, "DriftTermin", offeringA.ID, []int{2}, period)
	template.SourceCareOfferingIDs = []int64{offeringA.ID, offeringB.ID}
	require.NoError(t, repositories.NewFactory(env.db).ActivityGroup.Update(ctx, template))

	onlyA, _ := submitAndApproveOfferingChild(t, env, offeringA.ID, "drift-only-a@example.com", "Anke", 2)
	onlyB, _ := submitAndApproveOfferingChild(t, env, offeringB.ID, "drift-only-b@example.com", "Bern", 2)
	require.NoError(t, offeringResyncer(t, env).ResyncTemplateOfferingRoster(ctx,
		scheduleService.OfferingRosterResyncInput{
			TemplateID:       template.ID,
			OfferingIDs:      []int64{offeringA.ID, offeringB.ID},
			GradeLevels:      []int{2},
			CalendarPeriodID: &period.ID,
			EffectiveFrom:    decisionTestToday,
		}))
	require.Len(t, loadTemplateEnrollments(t, env, template.ID), 2,
		"both offerings' children must be planned before the drift")

	// Drift: re-pin the template to a period the phase window lies outside of,
	// so validating the remaining source fails with ErrOfferingSourceInvalid.
	shortPeriod := createCareOfferingTestPeriod(t, env.db, "drift-short",
		timezone.NewDate(2030, 1, 1),
		timezone.NewDate(2030, 1, 31))
	template.CalendarPeriodID = &shortPeriod.ID
	require.NoError(t, repositories.NewFactory(env.db).ActivityGroup.Update(ctx, template))

	detacher, ok := env.decision.(enrollmentService.CareOfferingSourcedTemplateResyncer)
	require.True(t, ok)
	require.NoError(t, detacher.DetachTemplatesSourcedFromOffering(ctx, offeringA.ID, decisionTestToday))

	kept, err := repositories.NewFactory(env.db).ActivityGroup.FindByID(ctx, template.ID)
	require.NoError(t, err)
	assert.Equal(t, []int64{offeringB.ID}, kept.SourceCareOfferingIDs,
		"a drifted sibling must not strip the remaining valid source")
	assert.Equal(t, []int{2}, kept.SourceGradeLevels,
		"the grade filter belongs to the kept source and must survive the detach")

	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1, "the departing offering's child must be retired before its provenance is deleted")
	assert.Equal(t, onlyB, rows[0].StudentID, "the kept sibling's child stays planned")
	assert.NotEqual(t, onlyA, rows[0].StudentID)
}

// Multi-source follow-up: a child enrolled in TWO of the template's source
// offerings must be planned exactly once — the union subtracts the second
// offering's overlapping weekday×date contribution instead of seeding a
// duplicate row.
func TestResync_UnionAcrossOfferings_PlansSharedChildOnce(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	offeringA := createSourceOffering(t, env, "UnionQuelleA", nil)
	offeringB := createSourceOffering(t, env, "UnionQuelleB", nil)
	template := createSourcedTemplate(t, env, "UnionTermin", offeringA.ID, nil, period)
	template.SourceCareOfferingIDs = []int64{offeringA.ID, offeringB.ID}
	require.NoError(t, repositories.NewFactory(env.db).ActivityGroup.Update(ctx, template))

	// One child holds BOTH offerings (same request), one child only offering B.
	grade := int16(2)
	submitted, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Union",
		GuardianEmail:     "union-both@example.com",
		ConsentFlags: map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		},
		Children: []enrollmentService.SubmitChild{{
			FirstName:        "Uma",
			LastName:         "Union",
			DateOfBirth:      timezone.NewDate(2018, 4, 15),
			TargetGradeLevel: &grade,
			OfferingIDs:      []int64{offeringA.ID, offeringB.ID},
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
	sharedStudent := *outcome.Child.CreatedStudentID
	t.Cleanup(func() {
		_, _ = env.db.NewDelete().
			TableExpr("activities.student_enrollments").
			Where("student_id = ?", sharedStudent).
			Exec(context.Background())
	})
	onlyB, _ := submitAndApproveOfferingChild(t, env, offeringB.ID, "union-b@example.com", "Bela", 2)

	require.NoError(t, offeringResyncer(t, env).ResyncTemplateOfferingRoster(ctx,
		scheduleService.OfferingRosterResyncInput{
			TemplateID:       template.ID,
			OfferingIDs:      []int64{offeringA.ID, offeringB.ID},
			CalendarPeriodID: &period.ID,
			EffectiveFrom:    timezone.TodayDate(),
		}))

	rows := loadTemplateEnrollments(t, env, template.ID)
	perStudent := make(map[int64]int, len(rows))
	for _, row := range rows {
		perStudent[row.StudentID]++
	}
	assert.Equal(t, 1, perStudent[sharedStudent],
		"a child in both source offerings must be planned exactly once")
	assert.Equal(t, 1, perStudent[onlyB])
	assert.Len(t, rows, 2)

	// Idempotence: a second resync with unchanged sources must not rewrite
	// or duplicate anything.
	require.NoError(t, offeringResyncer(t, env).ResyncTemplateOfferingRoster(ctx,
		scheduleService.OfferingRosterResyncInput{
			TemplateID:       template.ID,
			OfferingIDs:      []int64{offeringA.ID, offeringB.ID},
			CalendarPeriodID: &period.ID,
			EffectiveFrom:    timezone.TodayDate(),
		}))
	assert.Len(t, loadTemplateEnrollments(t, env, template.ID), 2,
		"the union resync must be idempotent")
}

// Multi-source follow-up: the fan-out on approval must plan a child into a
// multi-source template through the union resync (the per-link draft merge is
// skipped for those templates), so approving after the template exists still
// seeds the roster.
func TestDecide_FansOutToMultiSourceTemplate(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	offeringA := createSourceOffering(t, env, "FanoutQuelleA", nil)
	offeringB := createSourceOffering(t, env, "FanoutQuelleB", nil)
	template := createSourcedTemplate(t, env, "FanoutMultiTermin", offeringA.ID, nil, period)
	template.SourceCareOfferingIDs = []int64{offeringA.ID, offeringB.ID}
	require.NoError(t, repositories.NewFactory(env.db).ActivityGroup.Update(ctx, template))

	studentID, _ := submitAndApproveOfferingChild(t, env, offeringB.ID, "fanout-multi@example.com", "Mio", 2)

	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1, "the approval must seed the multi-source template via the union resync")
	assert.Equal(t, studentID, rows[0].StudentID)
}

// A child in BOTH the departing and a remaining offering must not keep the
// departing offering's exclusive weekday footprint when the drift fallback
// runs: the fallback reconciles the child against the remaining sources'
// link coverage (drift-tolerant), so A's Di–Fr disappear while B's Montag
// survives. Skipping shared children wholesale would leave the Di–Fr rows
// planned forever, with the regular resync permanently blocked by the drift.
func TestOfferingDetach_DriftedSiblingCapsExclusiveCoverage(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	offeringA := &enrollmentModels.CareOffering{
		PhaseID:        env.sourcePhase.ID,
		Name:           uniqueSchemaName("DriftBreitQuelleA-" + t.Name()),
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		IsActive:       true,
	}
	offeringA.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.CareOffering.Create(ctx, offeringA))
	t.Cleanup(func() {
		_, _ = env.db.NewDelete().
			TableExpr("enrollment.care_offerings").
			Where("id = ?", offeringA.ID).
			Exec(context.Background())
	})
	offeringB := createSourceOffering(t, env, "DriftBreitQuelleB", nil) // Montag only

	template := createSourcedTemplate(t, env, "DriftBreitTermin", offeringA.ID, nil, period)
	template.SourceCareOfferingIDs = []int64{offeringA.ID, offeringB.ID}
	require.NoError(t, repositories.NewFactory(env.db).ActivityGroup.Update(ctx, template))

	// One child holds BOTH offerings; the approval fan-out seeds the union
	// row shaped by A (first in the array): Mo–Fr.
	grade := int16(2)
	submitted, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Breit",
		GuardianEmail:     "drift-breit@example.com",
		ConsentFlags: map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		},
		Children: []enrollmentService.SubmitChild{{
			FirstName:        "Bea",
			LastName:         "Breit",
			DateOfBirth:      timezone.NewDate(2018, 4, 15),
			TargetGradeLevel: &grade,
			OfferingIDs:      []int64{offeringA.ID, offeringB.ID},
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
	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1)
	require.Equal(t, []int{1, 2, 3, 4, 5}, rows[0].SelectedWeekdays,
		"the union row must carry A's Mo–Fr before the detach")

	// Drift: B turns inactive behind the guard's back (direct write — the
	// service-level deactivation is rejected while B feeds templates).
	_, err = env.db.NewRaw(`UPDATE enrollment.care_offerings SET is_active = false WHERE id = ?`, offeringB.ID).Exec(ctx)
	require.NoError(t, err)

	detacher, ok := env.decision.(enrollmentService.CareOfferingSourcedTemplateResyncer)
	require.True(t, ok)
	require.NoError(t, detacher.DetachTemplatesSourcedFromOffering(ctx, offeringA.ID, decisionTestToday))

	kept, err := repositories.NewFactory(env.db).ActivityGroup.FindByID(ctx, template.ID)
	require.NoError(t, err)
	assert.Equal(t, []int64{offeringB.ID}, kept.SourceCareOfferingIDs)

	rows = loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1, "the shared child stays planned through B")
	assert.Equal(t, []int{1}, rows[0].SelectedWeekdays,
		"A's exclusive Di–Fr contribution must be retired with the detach")
}

// The multi-source fan-out must seed the child's rows from the phase's
// service start — like the single-source drafts — even when the phase is
// already running. With the old today-anchored boundary, an approval on day N
// of a running phase silently lost [phase start, N) for multi-source
// templates only.
func TestDecide_MultiSourceFanOutSeedsFromPhaseStart(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	// The phase started a week ago; the period is built around it so the
	// template pin stays valid regardless of the real date.
	phaseStart := timezone.NewDate(2026, 8, 24).AddDays(-7)
	setSourcePhaseServiceStartDate(t, env, phaseStart)
	period := createCareOfferingTestPeriod(t, env.db, "running-phase-fanout",
		phaseStart.AddDays(-5),
		env.sourcePhase.ServiceEndDate.AddDays(30))

	offeringA := createSourceOffering(t, env, "LaufendQuelleA", nil)
	offeringB := createSourceOffering(t, env, "LaufendQuelleB", nil)
	template := createSourcedTemplate(t, env, "LaufendMultiTermin", offeringA.ID, nil, period)
	template.SourceCareOfferingIDs = []int64{offeringA.ID, offeringB.ID}
	require.NoError(t, repositories.NewFactory(env.db).ActivityGroup.Update(ctx, template))

	submitAndApproveOfferingChild(t, env, offeringB.ID, "laufend-multi@example.com", "Lars", 2)

	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1)
	assert.Equal(t, phaseStart, rows[0].ValidFrom,
		"the union row must start at the phase's service start, not at the approval date")
}

// An UNDATED offering correction deletes the child's rows and rematerializes
// the whole phase window. Multi-source templates must come back from the
// phase start like the single-source drafts do — with the old today-anchored
// resync boundary, the correction permanently truncated the already-elapsed
// window, including the roster basis of recorded attendance.
func TestUpdateChildOfferings_UndatedCorrectionKeepsPhaseStartOnMultiSource(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	phaseStart := timezone.NewDate(2026, 8, 24).AddDays(-7)
	setSourcePhaseServiceStartDate(t, env, phaseStart)
	period := createCareOfferingTestPeriod(t, env.db, "running-phase-correction",
		phaseStart.AddDays(-5),
		env.sourcePhase.ServiceEndDate.AddDays(30))

	offeringA := createSourceOffering(t, env, "KorrekturQuelleA", nil)
	offeringB := createSourceOffering(t, env, "KorrekturQuelleB", nil)
	template := createSourcedTemplate(t, env, "KorrekturMultiTermin", offeringA.ID, nil, period)
	template.SourceCareOfferingIDs = []int64{offeringA.ID, offeringB.ID}
	require.NoError(t, repositories.NewFactory(env.db).ActivityGroup.Update(ctx, template))

	_, childID := submitAndApproveOfferingChild(t, env, offeringA.ID, "korrektur-multi@example.com", "Kim", 2)
	child, err := env.repos.RequestChild.FindByID(ctx, childID)
	require.NoError(t, err)
	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1)
	require.Equal(t, phaseStart, rows[0].ValidFrom)

	// Undated correction: the selection was wrong from the start, switch the
	// child from offering A to B (both feed the same template).
	_, err = env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID:      child.RequestID,
		ChildID:        childID,
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
		Reason:         "Tippfehler in der Anmeldung",
		Offerings: []enrollmentService.OfferingAdjustmentSelection{
			{OfferingID: offeringB.ID},
		},
	})
	require.NoError(t, err)

	rows = loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1, "the child still feeds the template through offering B")
	assert.Equal(t, phaseStart, rows[0].ValidFrom,
		"the rematerialized union row must cover the whole phase window again")
}

// The post-flip multi-source resync must honor the care-offerings feature
// gate: with the setting off, applyApproval writes no enrollment rows, so
// pre-existing offering links must not start materializing through the union
// resync either.
func TestDecide_MultiSourceResyncRespectsCareOfferingsDisabled(t *testing.T) {
	t.Parallel()

	disabled := false
	env, cleanup := setupDecisionTestWithSettings(t, stubActivationSettings{careOfferingsEnabled: &disabled})
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	offeringA := createSourceOffering(t, env, "GateQuelleA", nil)
	offeringB := createSourceOffering(t, env, "GateQuelleB", nil)
	template := createSourcedTemplate(t, env, "GateMultiTermin", offeringA.ID, nil, period)
	template.SourceCareOfferingIDs = []int64{offeringA.ID, offeringB.ID}
	require.NoError(t, repositories.NewFactory(env.db).ActivityGroup.Update(ctx, template))

	submitAndApproveOfferingChild(t, env, offeringB.ID, "gate-multi@example.com", "Gero", 2)

	assert.Empty(t, loadTemplateEnrollments(t, env, template.ID),
		"with care offerings disabled the approval must not materialize the multi-source union")
}

// #2147 review round 11: deleting a phase cascades its care offerings away
// (templates flip to manual via ON DELETE SET NULL) and erases the roster
// rows' provenance with the request children. The delete flow must retire
// the offering-derived enrollment rows and their materialized occurrence
// rows FIRST — left behind, they would keep materializing children while
// being invisible to every later resync.
func TestPhaseDelete_RetiresSourcedRosterRows(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "PhasenLoeschQuelle", nil)
	studentID, _ := submitAndApproveOfferingChild(t, env, offering.ID, "phase-delete@example.com", "Phia", 2)
	template := createSourcedTemplate(t, env, "PhasenLoeschTermin", offering.ID, nil, period)
	require.NoError(t, offeringResyncer(t, env).ResyncTemplateOfferingRoster(ctx,
		scheduleService.OfferingRosterResyncInput{
			TemplateID:       template.ID,
			OfferingIDs:      []int64{offering.ID},
			CalendarPeriodID: &period.ID,
			EffectiveFrom:    decisionTestToday,
		},
	))
	require.Len(t, loadTemplateEnrollments(t, env, template.ID), 1)

	occurrenceDate := firstInPhaseMondayOnOrAfter(env.sourcePhase.ServiceStartDate)
	require.NotNil(t, template.PlannedRoomID)
	planned := testpkg.CreateTestActivityInstance(t, env.db, occurrenceDate, *template.PlannedRoomID, testpkg.ActivityInstanceOpts{
		ActivityGroupID:  &template.ID,
		CalendarPeriodID: &period.ID,
	})
	registerSourcedInstanceCleanup(t, env, planned.ID)
	testpkg.CreateTestInstanceStudent(t, env.db, planned.ID, studentID, "expected")

	repoFactory := repositories.NewFactory(env.db)
	phaseSvc := enrollmentService.NewPhaseService(enrollmentService.PhaseServiceConfig{
		Repo:                   repoFactory.Phase,
		RequestRepo:            repoFactory.Request,
		RequestChildRepo:       repoFactory.RequestChild,
		CareOfferingRepo:       repoFactory.CareOffering,
		FormSchemaRepo:         repoFactory.FormSchema,
		LockTemplateRecurrence: func(context.Context) error { return nil },
		DB:                     env.db,
		Today:                  func() timezone.Date { return decisionTestToday },
	})
	binder, ok := phaseSvc.(enrollmentService.CareOfferingSourceResyncBinder)
	require.True(t, ok, "phase service must accept the sourced-template resyncer")
	sourcedResyncer, ok := env.decision.(enrollmentService.CareOfferingSourcedTemplateResyncer)
	require.True(t, ok, "decision service must implement the delete-side detach")
	binder.SetSourcedTemplateResyncer(sourcedResyncer)

	require.NoError(t, phaseSvc.Delete(ctx, env.sourcePhase.ID))

	assert.Empty(t, loadTemplateEnrollments(t, env, template.ID),
		"the not-yet-effective sourced rows must be retired with the phase")
	assert.Empty(t, loadSourcedInstanceStudents(t, env, planned.ID),
		"the child's still-planned occurrence row must be removed with the phase")
	degraded, err := repoFactory.ActivityGroup.FindByID(ctx, template.ID)
	require.NoError(t, err)
	assert.Empty(t, degraded.SourceCareOfferingIDs, "the cascade flips the now source-less template to a manual roster")
}

// createBoundedTemplateSchedule mirrors createCareOfferingTemplateSchedule but
// stamps a recurrence envelope onto the schedule row, standing in for one
// segment of a split series (predecessor capped at the split date, successor
// starting there).
func createBoundedTemplateSchedule(
	t *testing.T,
	env *decisionTestEnv,
	groupID int64,
	weekday int,
	periodID *int64,
	validFrom, validUntil *timezone.Date,
) {
	t.Helper()
	timeframe := testpkg.CreateTestTimeframeForTenant(t, env.db, testpkg.Tenant(t), "CareTemplate")
	schedule := &activitiesModels.Schedule{
		Weekday:          weekday,
		TimeframeID:      &timeframe.ID,
		ActivityGroupID:  groupID,
		WeekPattern:      0,
		CalendarPeriodID: periodID,
		ValidFrom:        validFrom,
		ValidUntil:       validUntil,
	}
	schedule.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, repositories.NewFactory(env.db).ActivitySchedule.Create(testpkg.Ctx(t), schedule))
}

// createSourcedTemplateSegment is createSourcedTemplate with an explicit
// schedule envelope — one side of a split series.
func createSourcedTemplateSegment(
	t *testing.T,
	env *decisionTestEnv,
	name string,
	offeringID int64,
	gradeLevels []int,
	period *scheduleModels.CalendarPeriod,
	validFrom, validUntil *timezone.Date,
) *activitiesModels.Group {
	t.Helper()
	group := createCareOfferingTemplateGroup(t, env.db, name)
	group.TargetGroupType = activitiesModels.TargetGroupTypeAngebot
	group.SourceCareOfferingIDs = []int64{offeringID}
	group.SourceGradeLevels = gradeLevels
	group.CalendarPeriodID = &period.ID
	require.NoError(t, repositories.NewFactory(env.db).ActivityGroup.Update(testpkg.Ctx(t), group))
	createBoundedTemplateSchedule(t, env, group.ID, activitiesModels.WeekdayMonday, &period.ID, validFrom, validUntil)
	return group
}

// FindTemplatesBySourceOffering returns both sides of a split. An approval
// must bound each segment's rows by that segment's schedule envelope — the
// capped predecessor must not receive coverage past its end, nor the
// successor coverage before its start (#2147 review).
func TestDecide_FanOutBoundsRowsToSegmentEnvelope(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "SplitQuelle", nil)
	splitDate := env.sourcePhase.ServiceStartDate.AddDays(60)
	predecessor := createSourcedTemplateSegment(t, env, "SplitVorher", offering.ID, []int{2}, period, nil, &splitDate)
	successor := createSourcedTemplateSegment(t, env, "SplitNachher", offering.ID, []int{2}, period, &splitDate, nil)

	studentID, _ := submitAndApproveOfferingChild(t, env, offering.ID, "split-fanout@example.com", "Spalt", 2)

	predecessorRows := loadTemplateEnrollments(t, env, predecessor.ID)
	require.Len(t, predecessorRows, 1)
	assert.Equal(t, studentID, predecessorRows[0].StudentID)
	assert.Equal(t, env.sourcePhase.ServiceStartDate, predecessorRows[0].ValidFrom)
	require.NotNil(t, predecessorRows[0].ValidUntil)
	assert.Equal(t, splitDate, *predecessorRows[0].ValidUntil,
		"the capped predecessor must not be planned past its segment end")

	successorRows := loadTemplateEnrollments(t, env, successor.ID)
	require.Len(t, successorRows, 1)
	assert.Equal(t, studentID, successorRows[0].StudentID)
	assert.Equal(t, splitDate, successorRows[0].ValidFrom,
		"the successor must not be planned before its segment start")
	require.NotNil(t, successorRows[0].ValidUntil)
	assert.Equal(t, env.sourcePhase.ServiceEndDate.AddDays(1), *successorRows[0].ValidUntil)
}

// The tenant-wide resync (grade transitions) also visits capped split
// predecessors. Its wanted windows must stop at the segment envelope, or a
// re-reconcile would extend the capped rows back out to the phase end.
func TestResyncTemplateOfferingRoster_BoundsWindowsToScheduleEnvelope(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "SegmentQuelle", nil)
	studentID, _ := submitAndApproveOfferingChild(t, env, offering.ID, "segment-bound@example.com", "Seg", 2)

	splitDate := env.sourcePhase.ServiceStartDate.AddDays(60)
	template := createSourcedTemplateSegment(t, env, "SegmentTermin", offering.ID, []int{2}, period, nil, &splitDate)
	input := scheduleService.OfferingRosterResyncInput{
		TemplateID:       template.ID,
		OfferingIDs:      []int64{offering.ID},
		GradeLevels:      []int{2},
		CalendarPeriodID: &period.ID,
		EffectiveFrom:    timezone.TodayDate(),
	}
	resyncer := offeringResyncer(t, env)
	require.NoError(t, resyncer.ResyncTemplateOfferingRoster(ctx, input))

	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1)
	assert.Equal(t, studentID, rows[0].StudentID)
	require.NotNil(t, rows[0].ValidUntil)
	assert.Equal(t, splitDate, *rows[0].ValidUntil,
		"the seeded row must stop at the segment's schedule valid_until")

	// Re-running must keep the capped row instead of extending it.
	require.NoError(t, resyncer.ResyncTemplateOfferingRoster(ctx, input))
	rows = loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].ValidUntil)
	assert.Equal(t, splitDate, *rows[0].ValidUntil)
}

// The materializer never revisits an existing instance, so the resync itself
// must propagate roster changes onto already-materialized future occurrences:
// children leaving the filter disappear, children entering it appear, and
// rows carrying an observation or a hand decision survive (#2147 review).
func TestResyncTemplateOfferingRoster_ReconcilesMaterializedInstances(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "InstanzQuelle", nil)
	studentGrade1, _ := submitAndApproveOfferingChild(t, env, offering.ID, "instanz-grade1@example.com", "Insa", 1)
	studentGrade2, _ := submitAndApproveOfferingChild(t, env, offering.ID, "instanz-grade2@example.com", "Ivo", 2)

	template := createSourcedTemplate(t, env, "InstanzTermin", offering.ID, []int{1}, period)
	input := scheduleService.OfferingRosterResyncInput{
		TemplateID:       template.ID,
		OfferingIDs:      []int64{offering.ID},
		GradeLevels:      []int{1},
		CalendarPeriodID: &period.ID,
		EffectiveFrom:    timezone.TodayDate(),
	}
	resyncer := offeringResyncer(t, env)
	require.NoError(t, resyncer.ResyncTemplateOfferingRoster(ctx, input))

	// Stand in for the materializer: one planned occurrence on the first
	// in-phase Monday, already carrying the grade-1 child, plus a second one
	// where a human decided the child's slot by hand.
	occurrenceDate := env.sourcePhase.ServiceStartDate
	for occurrenceDate.Weekday() != time.Monday {
		occurrenceDate = occurrenceDate.AddDays(1)
	}
	require.NotNil(t, template.PlannedRoomID)
	planned := testpkg.CreateTestActivityInstance(t, env.db, occurrenceDate, *template.PlannedRoomID, testpkg.ActivityInstanceOpts{
		ActivityGroupID:  &template.ID,
		CalendarPeriodID: &period.ID,
	})
	decided := testpkg.CreateTestActivityInstance(t, env.db, occurrenceDate.AddDays(7), *template.PlannedRoomID, testpkg.ActivityInstanceOpts{
		ActivityGroupID:  &template.ID,
		CalendarPeriodID: &period.ID,
	})
	t.Cleanup(func() {
		_, _ = env.db.NewDelete().
			TableExpr("schedule.instance_students").
			Where("instance_id IN (?, ?)", planned.ID, decided.ID).
			Exec(context.Background())
	})
	testpkg.CreateTestInstanceStudent(t, env.db, planned.ID, studentGrade1, "expected")
	manualAt := time.Now()
	testpkg.CreateTestInstanceStudent(t, env.db, decided.ID, studentGrade1, "absent", testpkg.InstanceStudentOpts{
		ManualStatusAt: &manualAt,
	})

	// Filter switch 1 → 2: the enrollment rows change AND the materialized
	// occurrences must follow.
	input.GradeLevels = []int{2}
	require.NoError(t, resyncer.ResyncTemplateOfferingRoster(ctx, input))

	loadInstanceStudents := func(instanceID int64) []scheduleModels.InstanceStudent {
		var rows []scheduleModels.InstanceStudent
		require.NoError(t, env.db.NewSelect().
			Model(&rows).
			ModelTableExpr(`schedule.instance_students AS "instance_student"`).
			Where(`"instance_student".instance_id = ?`, instanceID).
			Order("student_id ASC").
			Scan(ctx))
		return rows
	}

	plannedRows := loadInstanceStudents(planned.ID)
	require.Len(t, plannedRows, 1, "the occurrence must hold exactly the child matching the new filter")
	assert.Equal(t, studentGrade2, plannedRows[0].StudentID,
		"the grade-2 child must be added to the already-materialized occurrence")
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, plannedRows[0].Status)

	decidedRows := loadInstanceStudents(decided.ID)
	require.Len(t, decidedRows, 2, "a hand-decided row survives the resync next to the added child")
	assert.Equal(t, studentGrade1, decidedRows[0].StudentID,
		"a row a human decided by hand must never be removed by the resync")
	assert.Equal(t, studentGrade2, decidedRows[1].StudentID)
}

// #2147 review round 11: a resync that rewrites a still-covered child's
// enrollment row (here: the child's link is capped mid-phase) must not
// resurrect an occurrence row staff removed by hand — only newly gained
// coverage may create instance rows.
func TestResyncTemplateOfferingRoster_PreservesManualOccurrenceRemoval(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "HandEntfernt", nil)
	_, childID := submitAndApproveOfferingChild(t, env, offering.ID, "hand-removed@example.com", "Hanna", 2)
	template := createSourcedTemplate(t, env, "HandEntferntTermin", offering.ID, []int{2}, period)
	input := scheduleService.OfferingRosterResyncInput{
		TemplateID:       template.ID,
		OfferingIDs:      []int64{offering.ID},
		GradeLevels:      []int{2},
		CalendarPeriodID: &period.ID,
		EffectiveFrom:    timezone.TodayDate(),
	}
	resyncer := offeringResyncer(t, env)
	require.NoError(t, resyncer.ResyncTemplateOfferingRoster(ctx, input))

	// One materialized occurrence the child is deliberately NOT on: staff
	// removed the child's row by hand after materialization.
	occurrenceDate := firstInPhaseMondayOnOrAfter(env.sourcePhase.ServiceStartDate)
	require.NotNil(t, template.PlannedRoomID)
	planned := testpkg.CreateTestActivityInstance(t, env.db, occurrenceDate, *template.PlannedRoomID, testpkg.ActivityInstanceOpts{
		ActivityGroupID:  &template.ID,
		CalendarPeriodID: &period.ID,
	})
	registerSourcedInstanceCleanup(t, env, planned.ID)

	// The child leaves the offering four weeks later: the resync resizes the
	// enrollment row while the occurrence date stays covered.
	switchDate := occurrenceDate.AddDays(28)
	require.NoError(t, env.repos.RequestChildOffering.ScheduleReplacementForRequestChild(ctx, childID, switchDate, nil))
	require.NoError(t, resyncer.ResyncTemplateOfferingRoster(ctx, input))

	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].ValidUntil)
	require.Equal(t, switchDate, *rows[0].ValidUntil,
		"sanity: the resync must have touched the child's enrollment row")
	assert.Empty(t, loadSourcedInstanceStudents(t, env, planned.ID),
		"a hand-removed occurrence row must not be recreated while the child's coverage there is unchanged")
}

// ---- review follow-ups, round 4 (#2147) -----------------------------------

// loadSourcedInstanceStudents reads one occurrence's attendance rows ordered
// by student id.
func loadSourcedInstanceStudents(t *testing.T, env *decisionTestEnv, instanceID int64) []scheduleModels.InstanceStudent {
	t.Helper()
	var rows []scheduleModels.InstanceStudent
	require.NoError(t, env.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`schedule.instance_students AS "instance_student"`).
		Where(`"instance_student".instance_id = ?`, instanceID).
		Order("student_id ASC").
		Scan(testpkg.Ctx(t)))
	return rows
}

// firstInPhaseMondayOnOrAfter returns the first Monday on or after the date.
func firstInPhaseMondayOnOrAfter(date timezone.Date) timezone.Date {
	for date.Weekday() != time.Monday {
		date = date.AddDays(1)
	}
	return date
}

// registerSourcedInstanceCleanup drops an occurrence with its attendance rows.
func registerSourcedInstanceCleanup(t *testing.T, env *decisionTestEnv, instanceIDs ...int64) {
	t.Helper()
	t.Cleanup(func() {
		for _, instanceID := range instanceIDs {
			_, _ = env.db.NewDelete().
				TableExpr("schedule.instance_students").
				Where("instance_id = ?", instanceID).
				Exec(context.Background())
		}
	})
}

// The approval fan-out must honour the offering link's own validity window: a
// child whose link ends mid-phase must not stay planned for the rest of the
// phase (#2147 review, round 4).
func TestDecide_FanOutCapsRowAtLinkEnd(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "LinkFenster", nil)
	template := createSourcedTemplate(t, env, "LinkFensterTermin", offering.ID, []int{2}, period)

	grade := int16(2)
	submitted, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Fenster",
		GuardianEmail:     "link-window@example.com",
		ConsentFlags: map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		},
		Children: []enrollmentService.SubmitChild{{
			FirstName:        "Fenja",
			LastName:         "Fenster",
			DateOfBirth:      timezone.NewDate(2018, 4, 15),
			TargetGradeLevel: &grade,
			OfferingIDs:      []int64{offering.ID},
		}},
	})
	require.NoError(t, err)
	require.Len(t, submitted.Children, 1)
	childID := submitted.Children[0].ID

	// The child leaves the offering mid-phase BEFORE the request is decided.
	switchDate := env.sourcePhase.ServiceStartDate.AddDays(60)
	require.NoError(t, env.repos.RequestChildOffering.ScheduleReplacementForRequestChild(ctx, childID, switchDate, nil))

	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  submitted.Request.ID,
		ChildID:    childID,
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

	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1)
	assert.Equal(t, env.sourcePhase.ServiceStartDate, rows[0].ValidFrom)
	require.NotNil(t, rows[0].ValidUntil)
	assert.Equal(t, switchDate, *rows[0].ValidUntil,
		"the approval fan-out must not plan the child past the offering link's end")
}

// An approval AFTER materialization must add the child to the template's
// already-materialized future occurrences itself — the materializer is
// insert-only and never revisits an existing instance (#2147 review, round 4).
func TestDecide_ApprovalReconcilesMaterializedOccurrences(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "InstanzGenehmigung", nil)
	template := createSourcedTemplate(t, env, "InstanzGenehmigungTermin", offering.ID, nil, period)

	// The occurrence exists BEFORE any child is approved.
	occurrenceDate := firstInPhaseMondayOnOrAfter(env.sourcePhase.ServiceStartDate)
	require.NotNil(t, template.PlannedRoomID)
	planned := testpkg.CreateTestActivityInstance(t, env.db, occurrenceDate, *template.PlannedRoomID, testpkg.ActivityInstanceOpts{
		ActivityGroupID:  &template.ID,
		CalendarPeriodID: &period.ID,
	})
	registerSourcedInstanceCleanup(t, env, planned.ID)

	studentID, _ := submitAndApproveOfferingChild(t, env, offering.ID, "instanz-genehmigung@example.com", "Insa", 2)

	rows := loadSourcedInstanceStudents(t, env, planned.ID)
	require.Len(t, rows, 1, "the approval must add the child to the pre-existing occurrence")
	assert.Equal(t, studentID, rows[0].StudentID)
	assert.Equal(t, scheduleModels.AttendanceStatusExpected, rows[0].Status)
}

// A dated offering switch must propagate onto already-materialized future
// occurrences: the child leaves the sourced template at the switch date, so a
// still-planned attendance row after that date has to disappear (#2147
// review, round 4).
func TestUpdateChildOfferings_DatedSwitchReconcilesMaterializedOccurrences(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "InstanzWechselQuelle", nil)
	otherOffering := createSourceOffering(t, env, "InstanzWechselZiel", nil)
	template := createSourcedTemplate(t, env, "InstanzWechselTermin", offering.ID, []int{2}, period)

	studentID, childID := submitAndApproveOfferingChild(t, env, offering.ID, "instanz-wechsel@example.com", "Iwa", 2)
	child, err := env.repos.RequestChild.FindByID(ctx, childID)
	require.NoError(t, err)

	switchDate := env.sourcePhase.ServiceStartDate.AddDays(60)
	occurrenceDate := firstInPhaseMondayOnOrAfter(switchDate)
	require.NotNil(t, template.PlannedRoomID)
	planned := testpkg.CreateTestActivityInstance(t, env.db, occurrenceDate, *template.PlannedRoomID, testpkg.ActivityInstanceOpts{
		ActivityGroupID:  &template.ID,
		CalendarPeriodID: &period.ID,
	})
	registerSourcedInstanceCleanup(t, env, planned.ID)
	testpkg.CreateTestInstanceStudent(t, env.db, planned.ID, studentID, "expected")

	_, err = env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID:      child.RequestID,
		ChildID:        childID,
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
		Reason:         "Wechsel zum Halbjahr",
		EffectiveFrom:  &switchDate,
		Offerings: []enrollmentService.OfferingAdjustmentSelection{
			{OfferingID: otherOffering.ID},
		},
	})
	require.NoError(t, err)

	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].ValidUntil)
	require.Equal(t, switchDate, *rows[0].ValidUntil)
	assert.Empty(t, loadSourcedInstanceStudents(t, env, planned.ID),
		"the switched-away child must leave the already-materialized occurrence after the switch date")
}

// A dated adjustment that KEEPS the offering must not extend the retained row
// past the sourced segment's envelope: a capped split predecessor would
// otherwise regain coverage past the split and overlap its successor (#2147
// review, round 4).
func TestUpdateChildOfferings_DatedKeepRespectsSegmentEnd(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "SegmentBehaltQuelle", nil)
	splitDate := env.sourcePhase.ServiceStartDate.AddDays(90)
	template := createSourcedTemplateSegment(t, env, "SegmentBehaltTermin", offering.ID, []int{2}, period, nil, &splitDate)

	_, childID := submitAndApproveOfferingChild(t, env, offering.ID, "segment-behalt@example.com", "Seba", 2)
	child, err := env.repos.RequestChild.FindByID(ctx, childID)
	require.NoError(t, err)

	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].ValidUntil)
	require.Equal(t, splitDate, *rows[0].ValidUntil)

	// The switch keeps the same offering with unchanged days, so the existing
	// row is retained rather than capped and re-seeded.
	switchDate := env.sourcePhase.ServiceStartDate.AddDays(30)
	_, err = env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID:      child.RequestID,
		ChildID:        childID,
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
		Reason:         "Korrektur ohne Angebotswechsel",
		EffectiveFrom:  &switchDate,
		Offerings: []enrollmentService.OfferingAdjustmentSelection{
			{OfferingID: offering.ID},
		},
	})
	require.NoError(t, err)

	rows = loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].ValidUntil)
	assert.Equal(t, splitDate, *rows[0].ValidUntil,
		"the retained row must keep the segment end — not be extended to the phase end")
}

// A child fed into a template by the legacy CareOffering.ActivityGroupID link
// AND holding the template's source offering keeps the legacy-shaped row
// untouched, while the source contributes exactly its NON-overlapping
// weekdays as an own row — weekday-level protection, not child-level (#2147
// review, round 5). Removing the source reconciles the source-shaped row away
// by provenance instead of letting it survive behind the legacy protection.
func TestResyncTemplateOfferingRoster_LegacyChildGainsNonOverlappingSourceDays(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	legacyTemplate := createCareOfferingTemplateGroup(t, env.db, "LegacyMisch")
	createCareOfferingTemplateSchedule(t, env.db, legacyTemplate.ID, activitiesModels.WeekdayMonday, &period.ID)
	createCareOfferingTemplateSchedule(t, env.db, legacyTemplate.ID, activitiesModels.WeekdayTuesday, &period.ID)
	legacyOffering := createSourceOffering(t, env, "LegacyMischFeed", &legacyTemplate.ID)
	sourceOffering := createSourceOffering(t, env, "LegacyMischQuelle", nil)
	sourceOffering.AvailableDays = []string{"tue"}
	sourceOffering.PickupTimes = carePickupTimes("tue")
	require.NoError(t, env.repos.CareOffering.Update(ctx, sourceOffering))

	// The child holds BOTH offerings.
	grade := int16(2)
	submitted, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  "Misch",
		GuardianEmail:     "legacy-misch@example.com",
		ConsentFlags: map[string]any{
			"agb": true, "data_processing": true, "email_contact": true, "photo": true,
		},
		Children: []enrollmentService.SubmitChild{{
			FirstName:        "Mila",
			LastName:         "Misch",
			DateOfBirth:      timezone.NewDate(2018, 4, 15),
			TargetGradeLevel: &grade,
			OfferingIDs:      []int64{legacyOffering.ID, sourceOffering.ID},
		}},
	})
	require.NoError(t, err)
	require.Len(t, submitted.Children, 1)
	childID := submitted.Children[0].ID
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  submitted.Request.ID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	studentID := *outcome.Child.CreatedStudentID
	t.Cleanup(func() {
		_, _ = env.db.NewDelete().
			TableExpr("activities.student_enrollments").
			Where("student_id = ?", studentID).
			Exec(context.Background())
	})

	rows := loadTemplateEnrollments(t, env, legacyTemplate.ID)
	require.Len(t, rows, 1, "the legacy link owns the child's row — no second, source-fed row")
	assert.Equal(t, []int{1}, rows[0].SelectedWeekdays,
		"the row carries the legacy offering's days, not a merge with the source days")

	// Sourcing the same template from the second offering seeds the child's
	// Tuesday contribution — the legacy Monday must not suppress it — while
	// the legacy Monday row stays untouched.
	resyncer := offeringResyncer(t, env)
	require.NoError(t, resyncer.ResyncTemplateOfferingRoster(ctx, scheduleService.OfferingRosterResyncInput{
		TemplateID:       legacyTemplate.ID,
		OfferingIDs:      []int64{sourceOffering.ID},
		CalendarPeriodID: &period.ID,
		EffectiveFrom:    timezone.TodayDate(),
	}))
	rows = loadTemplateEnrollments(t, env, legacyTemplate.ID)
	require.Len(t, rows, 2, "the source contributes its non-overlapping weekday next to the legacy row")
	assert.Equal(t, []int{1}, rows[0].SelectedWeekdays,
		"the legacy-shaped row survives the source resync untouched")
	assert.Equal(t, []int{2}, rows[1].SelectedWeekdays,
		"the source-seeded row carries only the source offering's weekday")
	assert.Equal(t, studentID, rows[1].StudentID)

	// Removing the source must reconcile the source-shaped row away — it must
	// not hide behind the legacy protection, which covers Monday only.
	require.NoError(t, resyncer.ResyncTemplateOfferingRoster(ctx, scheduleService.OfferingRosterResyncInput{
		TemplateID:    legacyTemplate.ID,
		OfferingIDs:   nil,
		EffectiveFrom: timezone.TodayDate(),
	}))
	rows = loadTemplateEnrollments(t, env, legacyTemplate.ID)
	require.Len(t, rows, 1, "the source-shaped row must be reconciled away")
	assert.Equal(t, []int{1}, rows[0].SelectedWeekdays,
		"the legacy-shaped row survives the source removal untouched")
}

// A legacy link that only starts mid-phase must not suppress the source's
// rows before it begins: the legacy protection is bounded by each link's
// validity window, not just its weekday set (#2147 review, round 5).
func TestResyncTemplateOfferingRoster_FutureLegacyLinkDoesNotSuppressEarlierSourceRows(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	sourceOffering := createSourceOffering(t, env, "FruehQuelle", nil)
	studentID, childID := submitAndApproveOfferingChild(t, env, sourceOffering.ID, "future-legacy@example.com", "Frueh", 2)

	template := createSourcedTemplate(t, env, "FruehTermin", sourceOffering.ID, []int{2}, period)
	legacyOffering := createSourceOffering(t, env, "SpaetLegacy", &template.ID)

	// The child joins the legacy offering mid-phase and keeps the source
	// offering; before that date only the source plans the child.
	legacyStart := env.sourcePhase.ServiceStartDate.AddDays(60)
	require.NoError(t, env.repos.RequestChildOffering.ScheduleReplacementForRequestChild(
		ctx,
		childID,
		legacyStart,
		[]*enrollmentModels.RequestChildOffering{
			{CareOfferingID: sourceOffering.ID},
			{CareOfferingID: legacyOffering.ID},
		},
	))

	require.NoError(t, offeringResyncer(t, env).ResyncTemplateOfferingRoster(ctx,
		scheduleService.OfferingRosterResyncInput{
			TemplateID:       template.ID,
			OfferingIDs:      []int64{sourceOffering.ID},
			GradeLevels:      []int{2},
			CalendarPeriodID: &period.ID,
			EffectiveFrom:    decisionTestToday,
		},
	))

	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1, "the source plans the child until the legacy link begins")
	assert.Equal(t, studentID, rows[0].StudentID)
	assert.Equal(t, []int{1}, rows[0].SelectedWeekdays)
	assert.Equal(t, env.sourcePhase.ServiceStartDate, rows[0].ValidFrom)
	require.NotNil(t, rows[0].ValidUntil)
	assert.Equal(t, legacyStart, *rows[0].ValidUntil,
		"the source row stops where the legacy feed takes over — and is not suppressed before it")
}

// Editing a care offering must immediately re-reconcile every template
// sourcing it: changed available days reshape the wanted roster, and without
// the update-time resync the sourced rows would keep the pre-edit weekdays
// until an unrelated template save (#2147 review, round 5).
func TestCareOfferingUpdate_ResyncsSourcedTemplates(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "TagQuelle", nil)
	studentID, _ := submitAndApproveOfferingChild(t, env, offering.ID, "offering-update@example.com", "Tag", 2)
	template := createSourcedTemplate(t, env, "TagTermin", offering.ID, []int{2}, period)

	require.NoError(t, offeringResyncer(t, env).ResyncTemplateOfferingRoster(ctx,
		scheduleService.OfferingRosterResyncInput{
			TemplateID:       template.ID,
			OfferingIDs:      []int64{offering.ID},
			GradeLevels:      []int{2},
			CalendarPeriodID: &period.ID,
			EffectiveFrom:    timezone.TodayDate(),
		},
	))
	rows := loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1)
	assert.Equal(t, []int{1}, rows[0].SelectedWeekdays)

	svc := enrollmentService.NewCareOfferingService(enrollmentService.CareOfferingServiceConfig{
		Repo:                     env.repos.CareOffering,
		RequestChildOfferingRepo: env.repos.RequestChildOffering,
		ActivityGroupRepo:        env.repos.ActivityGroup,
		ActivityScheduleRepo:     env.repos.ActivitySchedule,
		CalendarPeriodRepo:       env.repos.CalendarPeriod,
		TimeframeRepo:            env.repos.Timeframe,
		ActivityExceptionRepo:    env.repos.ActivityException,
		PhaseRepo:                env.repos.Phase,
	})
	binder, ok := svc.(enrollmentService.CareOfferingSourceResyncBinder)
	require.True(t, ok, "care offering service must accept the sourced-template resyncer")
	sourcedResyncer, ok := env.decision.(enrollmentService.CareOfferingSourcedTemplateResyncer)
	require.True(t, ok, "decision service must implement the offering-update resync")
	binder.SetSourcedTemplateResyncer(sourcedResyncer)
	bindTestPickupResyncer(t, svc)

	offering.AvailableDays = []string{"tue"}
	offering.PickupTimes = carePickupTimes("tue")
	require.NoError(t, svc.Update(ctx, offering))

	rows = loadTemplateEnrollments(t, env, template.ID)
	require.Len(t, rows, 1, "the pre-edit Monday row is replaced, not kept alongside")
	assert.Equal(t, studentID, rows[0].StudentID)
	assert.Equal(t, []int{2}, rows[0].SelectedWeekdays,
		"the sourced row follows the offering's new weekday immediately after the update")
}

// ---- drifted-invalid sources: skip vs reject ------------------------------

// TestResyncSourcedTemplates_InvalidSourceSkipVsReject pins the #2147 round-7
// review contract: the tenant-wide resync (grade transitions) still skips a
// template whose source has drifted incompatible — one broken template must
// not block a whole transition — while the offering-scoped resync backing
// phase/offering edits surfaces ErrOfferingSourceInvalid so the caller
// rejects the edit instead of committing it with stranded sourced rows.
func TestResyncSourcedTemplates_InvalidSourceSkipVsReject(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	// The phase's service window (2026-09-01 .. 2027-07-31) reaches past this
	// period's end, so the template's source fails the period-fit validation.
	shortPeriod := createCareOfferingTestPeriod(t, env.db, "drift-short",
		timezone.NewDate(2026, 8, 1),
		timezone.NewDate(2026, 12, 31))
	offering := createSourceOffering(t, env, "DriftQuelle", nil)
	createSourcedTemplate(t, env, "DriftTermin", offering.ID, nil, shortPeriod)

	resyncAll, ok := env.decision.(interface {
		ResyncOfferingSourcedTemplates(ctx context.Context, effectiveFrom timezone.Date) error
	})
	require.True(t, ok, "decision service must implement the tenant-wide resync")
	require.NoError(t, resyncAll.ResyncOfferingSourcedTemplates(ctx, timezone.TodayDate()),
		"the tenant-wide resync must skip a drifted-invalid template with a warning")

	scoped, ok := env.decision.(enrollmentService.CareOfferingSourcedTemplateResyncer)
	require.True(t, ok, "decision service must implement the offering-scoped resync")
	err := scoped.ResyncTemplatesSourcedFromOffering(ctx, offering.ID, timezone.TodayDate())
	require.ErrorIs(t, err, scheduleService.ErrOfferingSourceInvalid,
		"the offering-scoped resync must surface the incompatibility so phase/offering edits are rejected")
}

// TestCareOfferingUpdate_RejectsEditThatInvalidatesSourcedTemplate: moving an
// offering to a phase whose service window lies outside a sourced template's
// planning period must fail the update as ErrCareOfferingInvalid (HTTP 400)
// and request rollback of the ambient tenant transaction — instead of
// committing an offering whose sourced rosters and materialized occurrences
// every later resync would only skip (#2147 review round 7).
func TestCareOfferingUpdate_RejectsEditThatInvalidatesSourcedTemplate(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := tenant.WithRollbackMarker(testpkg.Ctx(t))

	period := offeringSourcePeriod(t, env) // 2026-08-01 .. 2027-08-31
	offering := createSourceOffering(t, env, "PhasenWechsel", nil)
	createSourcedTemplate(t, env, "PhasenTermin", offering.ID, nil, period)

	latePhase := &enrollmentModels.Phase{
		Name:             uniqueSchemaName("late-phase-" + t.Name()),
		Kind:             enrollmentModels.PhaseKindSchoolYear,
		ServiceStartDate: timezone.NewDate(2027, 9, 1),
		ServiceEndDate:   timezone.NewDate(2028, 7, 31),
		IsActive:         true,
		CareOverflowMode: enrollmentModels.PhaseCareOverflowWaitlist,
	}
	latePhase.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.Phase.Create(ctx, latePhase))
	t.Cleanup(func() {
		_, _ = env.db.NewDelete().
			TableExpr("enrollment.phases").
			Where("id = ?", latePhase.ID).
			Exec(context.Background())
	})

	svc := enrollmentService.NewCareOfferingService(enrollmentService.CareOfferingServiceConfig{
		Repo:                     env.repos.CareOffering,
		RequestChildOfferingRepo: env.repos.RequestChildOffering,
		ActivityGroupRepo:        env.repos.ActivityGroup,
		ActivityScheduleRepo:     env.repos.ActivitySchedule,
		CalendarPeriodRepo:       env.repos.CalendarPeriod,
		TimeframeRepo:            env.repos.Timeframe,
		ActivityExceptionRepo:    env.repos.ActivityException,
		PhaseRepo:                env.repos.Phase,
	})
	binder, ok := svc.(enrollmentService.CareOfferingSourceResyncBinder)
	require.True(t, ok, "care offering service must accept the sourced-template resyncer")
	sourcedResyncer, ok := env.decision.(enrollmentService.CareOfferingSourcedTemplateResyncer)
	require.True(t, ok, "decision service must implement the offering-update resync")
	binder.SetSourcedTemplateResyncer(sourcedResyncer)
	bindTestPickupResyncer(t, svc)

	offering.PhaseID = latePhase.ID
	err := svc.Update(ctx, offering)
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingInvalid,
		"an edit invalidating a sourced template must be rejected as client-correctable")
	require.ErrorIs(t, err, scheduleService.ErrOfferingSourceInvalid)
	assert.True(t, tenant.RollbackRequested(ctx),
		"the rejected update must discard the already-written offering row via the ambient-transaction rollback marker")
}

// #2147 review round 9: the selector's counts must be scoped to the selected
// planning period — a link that ends before a future period begins would
// otherwise inflate the preview for that period.
func TestListOfferingSourceOptions_CountsScopedToSelectedPeriod(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()

	// Move the phase into the future so a period starting after today can
	// still contain it, keeping the test independent of the real date.
	phaseStart := timezone.NewDate(2026, 8, 24).AddDays(60)
	setSourcePhaseServiceStartDate(t, env, phaseStart)
	futurePeriod := createCareOfferingTestPeriod(t, env.db, "offering-source-future",
		phaseStart.AddDays(-5),
		env.sourcePhase.ServiceEndDate.AddDays(30))

	offering := createSourceOffering(t, env, "ZaehlerPeriode", nil)
	_, childID := submitAndApproveOfferingChild(t, env, offering.ID, "count-period@example.com", "Paula", 2)

	// The child leaves the offering before the future period begins: the link
	// still counts today, but contributes nothing to that period. valid_from
	// opens so the interval stays non-empty (the approval stamped the phase
	// start, which lies after the cap).
	ctx := testpkg.Ctx(t)
	_, err := env.db.NewRaw(
		`UPDATE enrollment.request_child_offerings SET valid_from = NULL, valid_until = ? WHERE request_child_id = ? AND care_offering_id = ?`,
		phaseStart.AddDays(-20), childID, offering.ID,
	).Exec(ctx)
	require.NoError(t, err)

	lister, ok := env.decision.(enrollmentService.OfferingSourceOptionLister)
	require.True(t, ok, "decision service must implement the editor's option lister")

	findOption := func(options []enrollmentService.OfferingSourceOption) *enrollmentService.OfferingSourceOption {
		for i := range options {
			if options[i].ID == offering.ID {
				return &options[i]
			}
		}
		return nil
	}

	unscoped, err := lister.ListOfferingSourceOptions(ctx, nil)
	require.NoError(t, err)
	current := findOption(unscoped)
	require.NotNil(t, current, "the offering must be listed without a period filter")
	assert.Equal(t, 1, current.TotalCount, "the link is still active today")
	assert.Equal(t, phaseStart, current.PhaseServiceStart,
		"editor needs the phase service start so it can warn about empty early occurrences")

	scoped, err := lister.ListOfferingSourceOptions(ctx, &futurePeriod.ID)
	require.NoError(t, err)
	future := findOption(scoped)
	require.NotNil(t, future, "the offering's phase fits the future period")
	assert.Equal(t, 0, future.TotalCount,
		"a link ending before the period begins must not count for that period")
	assert.Equal(t, phaseStart, future.PhaseServiceStart)
}

// #2147 review round 9: sourced rows are phase-bounded (non-null valid_until)
// by design; the template list aggregates and the weekday roster read must
// still surface them as long as they cover today or later.
func TestSourcedRosterRows_AppearInTemplateListReads(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()

	period := offeringSourcePeriod(t, env)
	offering := createSourceOffering(t, env, "ListRead", nil)
	studentID, _ := submitAndApproveOfferingChild(t, env, offering.ID, "list-read@example.com", "Lia", 2)

	template := createSourcedTemplate(t, env, "ListReadJg2", offering.ID, []int{2}, period)
	require.NoError(t, offeringResyncer(t, env).ResyncTemplateOfferingRoster(
		testpkg.Ctx(t),
		scheduleService.OfferingRosterResyncInput{
			TemplateID:       template.ID,
			OfferingIDs:      []int64{offering.ID},
			GradeLevels:      []int{2},
			CalendarPeriodID: &period.ID,
			EffectiveFrom:    timezone.TodayDate(),
		},
	))

	ctx := testpkg.Ctx(t)
	repo := repositories.NewFactory(env.db).ActivityGroup

	rows, err := repo.ListTemplateRows(ctx, &template.ID)
	require.NoError(t, err)
	require.NotEmpty(t, rows, "the sourced template must appear in the list read")
	assert.Equal(t, 1, rows[0].EnrollmentCount, "the phase-bounded sourced row must count")
	assert.Contains(t, rows[0].StudentIDs, studentID)

	roster, err := repo.ListTemplateWeekdayRoster(ctx, &template.ID, &period.ID)
	require.NoError(t, err)
	foundProtected := false
	for _, row := range roster {
		if row.Kind == activitiesModels.TemplateWeekdayRosterKindProtectedStudent && row.PersonID == studentID {
			foundProtected = true
		}
	}
	assert.True(t, foundProtected,
		"the sourced child must surface as a protected weekday assignment")
}

func TestExplainEmptyOfferingRoster_UsesServiceStartAndStaysNeutralAfterward(t *testing.T) {
	t.Parallel()

	sourceID := time.Now().UnixNano()
	serviceStart := timezone.NewDate(2026, 8, 13)
	options := []enrollmentService.OfferingSourceOption{{
		ID:                sourceID,
		PhaseName:         "Schuljahr 2026/27",
		PhaseServiceStart: serviceStart,
	}}

	early := enrollmentService.ExplainEmptyOfferingRoster(
		options, []int64{sourceID}, timezone.NewDate(2026, 8, 10),
	)
	require.NotNil(t, early)
	assert.Equal(t, enrollmentService.EmptyOfferingRosterBeforeServiceStart, early.Kind)
	assert.Equal(t, serviceStart, early.ServiceStartDate)

	started := enrollmentService.ExplainEmptyOfferingRoster(
		options, []int64{sourceID}, serviceStart,
	)
	require.NotNil(t, started)
	assert.Equal(t, enrollmentService.EmptyOfferingRosterSourceEmpty, started.Kind)

	assert.Nil(t, enrollmentService.ExplainEmptyOfferingRoster(options, nil, serviceStart))
}
