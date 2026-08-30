package enrollment_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// This file pins the #2249 contract: a follow-up phase (Anschlussphase)
// carries the FULL care booking effective at the end of the source phase
// — the offering catalog is cloned into the target phase, booking rows
// are remapped onto the clones, weekday selections travel verbatim, and
// historical validity intervals are reset to the new service period.

// sourceOffering creates one care offering on the rollover source phase
// via the repository (no service-side template validation, matching how
// production rows exist before a rollover runs).
func sourceOffering(t *testing.T, env *rolloverTestEnv, offering *enrollmentModels.CareOffering) *enrollmentModels.CareOffering {
	t.Helper()
	offering.PhaseID = env.sourcePhase.ID
	offering.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.CareOffering.Create(testpkg.Ctx(t), offering))
	return offering
}

func linkChildOffering(t *testing.T, env *rolloverTestEnv, link *enrollmentModels.RequestChildOffering) *enrollmentModels.RequestChildOffering {
	t.Helper()
	link.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.RequestChildOffering.Create(testpkg.Ctx(t), link))
	return link
}

func TestRolloverService_CreatePhaseFromSource_ClonesCatalogAndRemapsBookings(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	// Seed the child before the exactly_one group exists — the Submit
	// path validates selection rules against the live catalog.
	child := seedApprovedChild(t, env, env.sourcePhase.ID,
		"Carla", "Carry", "carla.carry@example.com",
		"Kim", "Carry", int16(2))

	capacity := 20
	price := 4200
	kurz := sourceOffering(t, env, &enrollmentModels.CareOffering{
		Name:           "Betreuung kurz",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
		AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		IsActive:       true,
		Capacity:       &capacity,
		PriceCents:     &price,
		SelectionGroup: "kernzeit",
		SelectionRule:  enrollmentModels.SelectionRuleExactlyOne,
	})
	lang := sourceOffering(t, env, &enrollmentModels.CareOffering{
		Name:           "Betreuung lang",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
		AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		IsActive:       true,
		SelectionGroup: "kernzeit",
		SelectionRule:  enrollmentModels.SelectionRuleExactlyOne,
	})
	mittag := sourceOffering(t, env, &enrollmentModels.CareOffering{
		Name:           "Mittagessen",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		IncludesLunch:  true,
		IsActive:       true,
		IsRequired:     true,
	})
	frueh := sourceOffering(t, env, &enrollmentModels.CareOffering{
		Name:           "Frühbetreuung",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
		AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		IsActive:       false,
	})
	oldTimestamp := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	_, err := env.db.NewUpdate().
		TableExpr("enrollment.care_offerings").
		Set("created_at = ?", oldTimestamp).
		Set("updated_at = ?", oldTimestamp).
		Where("id = ?", kurz.ID).
		Exec(ctx)
	require.NoError(t, err)
	// Frühbetreuung is auto-added whenever Betreuung kurz is selected.
	require.NoError(t, env.repos.CareOffering.ReplaceAutoAddTriggers(ctx, frueh.ID, []int64{kurz.ID}))

	// A replaced historical booking (kurz, first months only) plus the
	// booking effective at the end of the source phase (kurz with a
	// mid-phase weekday change, and the required lunch).
	historyEnd := timezone.NewDate(2027, 1, 1)
	notes := "Oma holt ab"
	linkChildOffering(t, env, &enrollmentModels.RequestChildOffering{
		RequestChildID: child.ID,
		CareOfferingID: kurz.ID,
		SelectedDays:   []string{"mon", "wed"},
		ValidUntil:     &historyEnd,
	})
	linkChildOffering(t, env, &enrollmentModels.RequestChildOffering{
		RequestChildID:        child.ID,
		CareOfferingID:        kurz.ID,
		SelectedDays:          []string{"tue", "thu"},
		ManualSelectedDays:    []string{"tue"},
		AutomaticSelectedDays: []string{"thu"},
		Notes:                 &notes,
		ValidFrom:             &historyEnd,
	})
	linkChildOffering(t, env, &enrollmentModels.RequestChildOffering{
		RequestChildID: child.ID,
		CareOfferingID: mittag.ID,
	})

	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx,
		validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true))
	require.NoError(t, err)
	require.Equal(t, 4, result.ClonedOfferingCount)

	// The target phase owns a full clone of the catalog.
	clones, err := env.repos.CareOffering.ListByPhase(ctx, result.Phase.ID)
	require.NoError(t, err)
	require.Len(t, clones, 4)
	cloneByName := make(map[string]*enrollmentModels.CareOffering, len(clones))
	sourceIDs := map[int64]bool{kurz.ID: true, lang.ID: true, mittag.ID: true, frueh.ID: true}
	for _, clone := range clones {
		cloneByName[clone.Name] = clone
		assert.False(t, sourceIDs[clone.ID], "clone %q must be a new row", clone.Name)
		assert.Equal(t, result.Phase.ID, clone.PhaseID)
	}
	kurzClone := cloneByName["Betreuung kurz"]
	require.NotNil(t, kurzClone)
	assert.True(t, kurzClone.CreatedAt.After(oldTimestamp), "clone must receive a fresh creation timestamp")
	assert.True(t, kurzClone.UpdatedAt.After(oldTimestamp), "clone must receive a fresh update timestamp")
	assert.Equal(t, enrollmentModels.DaysOfWeekModeParentChoice, kurzClone.DaysOfWeekMode)
	assert.Equal(t, []string{"mon", "tue", "wed", "thu", "fri"}, kurzClone.AvailableDays)
	require.NotNil(t, kurzClone.Capacity)
	assert.Equal(t, capacity, *kurzClone.Capacity)
	require.NotNil(t, kurzClone.PriceCents)
	assert.Equal(t, price, *kurzClone.PriceCents)
	assert.Equal(t, "kernzeit", kurzClone.SelectionGroup)
	assert.Equal(t, enrollmentModels.SelectionRuleExactlyOne, kurzClone.SelectionRule)
	langClone := cloneByName["Betreuung lang"]
	require.NotNil(t, langClone)
	assert.Equal(t, enrollmentModels.SelectionRuleExactlyOne, langClone.SelectionRule)
	mittagClone := cloneByName["Mittagessen"]
	require.NotNil(t, mittagClone)
	assert.True(t, mittagClone.IsRequired)
	assert.True(t, mittagClone.IncludesLunch)
	fruehClone := cloneByName["Frühbetreuung"]
	require.NotNil(t, fruehClone)
	assert.False(t, fruehClone.IsActive, "inactive offerings clone as inactive")
	assert.Equal(t, []int64{kurzClone.ID}, fruehClone.AutoAddTriggerOfferingIDs,
		"auto-add triggers must be remapped onto the cloned catalog")

	// The rolled child's booking is the selection effective at the END of
	// the source phase, remapped onto the clones, days carried, validity
	// reset to the new service period.
	rolled, err := env.repos.RequestChild.ListByPhaseAndStatuses(
		ctx, result.Phase.ID, []string{enrollmentModels.ChildStatusAutoRenewed})
	require.NoError(t, err)
	require.Len(t, rolled, 1)
	copied, err := env.repos.RequestChildOffering.ListByRequestChildID(ctx, rolled[0].ID)
	require.NoError(t, err)
	require.Len(t, copied, 2, "only the effective booking is carried, not history rows")
	copiedByOffering := make(map[int64]*enrollmentModels.RequestChildOffering, len(copied))
	for _, row := range copied {
		copiedByOffering[row.CareOfferingID] = row
	}
	kurzCopy := copiedByOffering[kurzClone.ID]
	require.NotNil(t, kurzCopy, "booking must reference the target-phase clone")
	assert.Equal(t, []string{"tue", "thu"}, kurzCopy.SelectedDays)
	assert.Equal(t, []string{"tue"}, kurzCopy.ManualSelectedDays)
	assert.Equal(t, []string{"thu"}, kurzCopy.AutomaticSelectedDays)
	require.NotNil(t, kurzCopy.Notes)
	assert.Equal(t, notes, *kurzCopy.Notes)
	// Historical source-phase intervals are NOT carried: the repository
	// pins the copy to the target phase's service window (end exclusive).
	require.NotNil(t, kurzCopy.ValidFrom)
	assert.Equal(t, result.Phase.ServiceStartDate, *kurzCopy.ValidFrom,
		"historical validity must not carry into the target phase")
	require.NotNil(t, kurzCopy.ValidUntil)
	assert.Equal(t, result.Phase.ServiceEndDate.AddDays(1), *kurzCopy.ValidUntil)
	mittagCopy := copiedByOffering[mittagClone.ID]
	require.NotNil(t, mittagCopy)
	assert.Nil(t, mittagCopy.SelectedDays)

	// Source data is untouched: same offering rows, same booking history.
	sourceOfferings, err := env.repos.CareOffering.ListByPhase(ctx, env.sourcePhase.ID)
	require.NoError(t, err)
	assert.Len(t, sourceOfferings, 4)
	sourceHistory, err := env.repos.RequestChildOffering.ListHistoryByRequestChildID(ctx, child.ID)
	require.NoError(t, err)
	require.Len(t, sourceHistory, 3)
	for _, row := range sourceHistory {
		assert.True(t, sourceIDs[row.CareOfferingID], "source bookings keep source offerings")
	}
}

func TestRolloverService_CreatePhaseFromSource_ClonesLegacyCrossPhaseBooking(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	offering := sourceOffering(t, env, &enrollmentModels.CareOffering{
		Name:           "Legacy-Phasenreferenz",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		IsActive:       true,
	})
	child := seedApprovedChild(t, env, env.sourcePhase.ID,
		"Lena", "Legacy", "lena.legacy@example.com",
		"Luis", "Legacy", int16(2))
	linkChildOffering(t, env, &enrollmentModels.RequestChildOffering{
		RequestChildID: child.ID,
		CareOfferingID: offering.ID,
	})

	legacyPhase := *env.sourcePhase
	legacyPhase.ID = 0
	legacyPhase.Name = "legacy-owner-" + t.Name()
	legacyPhase.RolloverSourcePhaseID = nil
	require.NoError(t, env.repos.Phase.Create(ctx, &legacyPhase))
	_, err := env.db.NewUpdate().TableExpr("enrollment.care_offerings").
		Set("phase_id = ?", legacyPhase.ID).
		Where("id = ?", offering.ID).
		Exec(ctx)
	require.NoError(t, err)
	defer func() {
		_, _ = env.db.NewUpdate().TableExpr("enrollment.care_offerings").
			Set("phase_id = ?", env.sourcePhase.ID).
			Where("id = ?", offering.ID).
			Exec(context.Background())
		_, _ = env.db.NewDelete().TableExpr("enrollment.phases").Where("id = ?", legacyPhase.ID).Exec(context.Background())
	}()

	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx,
		validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true))
	require.NoError(t, err)
	require.Equal(t, 1, result.ClonedOfferingCount)

	clones, err := env.repos.CareOffering.ListByPhase(ctx, result.Phase.ID)
	require.NoError(t, err)
	require.Len(t, clones, 1)
	rolled, err := env.repos.RequestChild.ListByPhaseAndStatuses(ctx, result.Phase.ID, []string{enrollmentModels.ChildStatusAutoRenewed})
	require.NoError(t, err)
	require.Len(t, rolled, 1)
	bookings, err := env.repos.RequestChildOffering.ListByRequestChildID(ctx, rolled[0].ID)
	require.NoError(t, err)
	require.Len(t, bookings, 1)
	assert.Equal(t, clones[0].ID, bookings[0].CareOfferingID)
}

func TestRolloverService_CreatePhaseFromSource_RejectsConflictingLegacyGroupRules(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	for _, rule := range []string{enrollmentModels.SelectionRuleExactlyOne, enrollmentModels.SelectionRuleAtLeastOne} {
		sourceOffering(t, env, &enrollmentModels.CareOffering{
			Name:           "Legacy-Regel " + rule,
			DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
			AvailableDays:  []string{"mon"},
			IsActive:       true,
			SelectionGroup: "umfang",
			SelectionRule:  rule,
		})
	}

	err := testpkg.WithTenantTx(t, context.Background(), env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		_, createErr := env.rolloverSvc.CreatePhaseFromSource(txCtx,
			validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true))
		return createErr
	})
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingGroupRuleConflict)
	exists, findErr := env.repos.Phase.ExistsByRolloverSourcePhaseID(ctx, env.sourcePhase.ID)
	require.NoError(t, findErr)
	assert.False(t, exists)
}

func TestRolloverService_CreatePhaseFromSource_FailsAtomicallyOnInvalidSourceOffering(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	offering := sourceOffering(t, env, &enrollmentModels.CareOffering{
		Name:           "Legacy ohne Tage",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		IsActive:       true,
	})
	// Simulate a legacy row saved before #1885 made available_days
	// mandatory: cloning it into the target phase must fail validation.
	_, updErr := env.db.NewUpdate().
		TableExpr("enrollment.care_offerings").
		Set("available_days = '[]'::jsonb").
		Where("id = ?", offering.ID).
		Exec(ctx)
	require.NoError(t, updErr)

	child := seedApprovedChild(t, env, env.sourcePhase.ID,
		"Frank", "Fail", "frank.fail@example.com",
		"Fritz", "Fail", int16(2))
	linkChildOffering(t, env, &enrollmentModels.RequestChildOffering{
		RequestChildID: child.ID,
		CareOfferingID: offering.ID,
	})

	// Production wraps the rollover in the handler's tenant transaction —
	// that is where the atomicity contract comes from.
	err := testpkg.WithTenantTx(t, context.Background(), env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		_, createErr := env.rolloverSvc.CreatePhaseFromSource(txCtx,
			validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true))
		return createErr
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "rollover: clone offering catalog")

	// Atomic: no follow-up phase, no rolled rows, no parent emails.
	exists, err := env.repos.Phase.ExistsByRolloverSourcePhaseID(ctx, env.sourcePhase.ID)
	require.NoError(t, err)
	assert.False(t, exists, "failed rollover must not leave the follow-up phase behind")
	assert.Empty(t, env.outbox.ByKind("enrollment_rollover_opt_out"),
		"failed rollover must not enqueue parent emails")
	assert.Empty(t, env.outbox.ByKind("enrollment_rollover_opt_in"))
}

func TestRolloverService_CreatePhaseFromSource_ValidatesInactiveSelectedTemplateOffering(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	period := createCareOfferingTestPeriod(t, env.db, "rollover-inactive-selected",
		timezone.NewDate(2026, 8, 1), timezone.NewDate(2028, 8, 31))
	group := createCareOfferingTemplateGroup(t, env.db, "rollover-inactive-selected")
	schedule := &activitiesModels.Schedule{
		Weekday: activitiesModels.WeekdayMonday, ActivityGroupID: group.ID,
		CalendarPeriodID: &period.ID,
	}
	schedule.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.ActivitySchedule.Create(ctx, schedule))

	offering := sourceOffering(t, env, &enrollmentModels.CareOffering{
		Name:            "Inaktive Auswahl ohne Zeitfenster",
		ActivityGroupID: &group.ID,
		DaysOfWeekMode:  enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:   []string{"mon"},
		IsActive:        false,
	})
	child := seedApprovedChild(t, env, env.sourcePhase.ID,
		"Ines", "Inaktiv", "ines.inaktiv@example.com",
		"Ida", "Inaktiv", int16(2))
	linkChildOffering(t, env, &enrollmentModels.RequestChildOffering{
		RequestChildID: child.ID,
		CareOfferingID: offering.ID,
	})

	err := testpkg.WithTenantTx(t, context.Background(), env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		_, createErr := env.rolloverSvc.CreatePhaseFromSource(txCtx,
			validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true))
		return createErr
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "no complete timeframe")
}

func TestRolloverService_CreatePhaseFromSource_FailsWhenBookingHasNoClone(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	offering := sourceOffering(t, env, &enrollmentModels.CareOffering{
		Name:           "Ohne Cloner",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		IsActive:       true,
	})
	child := seedApprovedChild(t, env, env.sourcePhase.ID,
		"Nora", "NoClone", "nora.noclone@example.com",
		"Nils", "NoClone", int16(2))
	linkChildOffering(t, env, &enrollmentModels.RequestChildOffering{
		RequestChildID: child.ID,
		CareOfferingID: offering.ID,
	})

	// A mis-wired service (nil cloner) must fail the rollover instead of
	// silently persisting a source-phase offering reference.
	svcNoCloner := enrollmentService.NewRolloverService(enrollmentService.RolloverServiceConfig{
		PhaseRepo:                env.repos.Phase,
		RequestRepo:              env.repos.Request,
		RequestChildRepo:         env.repos.RequestChild,
		RequestChildOfferingRepo: env.repos.RequestChildOffering,
		OutboxEnqueuer:           env.outbox,
		Settings:                 env.settings,
		ParentsURL:               "http://parents.localhost:3000",
		DB:                       env.db,
	})
	err := testpkg.WithTenantTx(t, context.Background(), env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		_, createErr := svcNoCloner.CreatePhaseFromSource(txCtx,
			validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true))
		return createErr
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "has no clone in the target phase")

	exists, err := env.repos.Phase.ExistsByRolloverSourcePhaseID(ctx, env.sourcePhase.ID)
	require.NoError(t, err)
	assert.False(t, exists)
}

// The parent-status page must be able to reopen the rolled request: before
// #2249 the carried booking pointed at a source-phase offering, which the
// edit-draft loader rejects with ErrEditNotAllowed.
func TestRolloverService_CreatePhaseFromSource_RolledRequestIsEditableInParentStatus(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	child := seedApprovedChild(t, env, env.sourcePhase.ID,
		"Elena", "Edit", "elena.edit@example.com",
		"Emil", "Edit", int16(2))
	offering := sourceOffering(t, env, &enrollmentModels.CareOffering{
		Name:           "Editierbar",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeParentChoice,
		AvailableDays:  []string{"mon", "tue", "wed", "thu", "fri"},
		IsActive:       true,
	})
	linkChildOffering(t, env, &enrollmentModels.RequestChildOffering{
		RequestChildID: child.ID,
		CareOfferingID: offering.ID,
		SelectedDays:   []string{"tue"},
	})

	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx,
		validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true))
	require.NoError(t, err)

	rolled, err := env.repos.RequestChild.ListByPhaseAndStatuses(
		ctx, result.Phase.ID, []string{enrollmentModels.ChildStatusAutoRenewed})
	require.NoError(t, err)
	require.Len(t, rolled, 1)
	newRequest, err := env.repos.Request.FindByID(ctx, rolled[0].RequestID)
	require.NoError(t, err)

	draft, err := env.requestSvc.GetEditDraft(ctx, newRequest.StatusToken)
	require.NoError(t, err,
		"the rolled request must open in the parent status page without a phase-foreign rejection")
	require.NotNil(t, draft)
	links := draft.OfferingsByChild[rolled[0].ID]
	require.Len(t, links, 1)
	assert.NotEqual(t, offering.ID, links[0].CareOfferingID)
	assert.Equal(t, []string{"tue"}, links[0].SelectedDays)
	offeredIDs := make(map[int64]bool, len(draft.OpenOfferings))
	for _, open := range draft.OpenOfferings {
		offeredIDs[open.ID] = true
	}
	assert.True(t, offeredIDs[links[0].CareOfferingID],
		"the carried clone must be part of the target phase's offered catalog")
}

// Re-running the rollover must not duplicate target requests, bookings, or
// cloned offerings — the second execution fails up front and atomically.
func TestRolloverService_CreatePhaseFromSource_RepeatedExecutionCreatesNoDuplicates(t *testing.T) {
	t.Parallel()

	env, cleanup := setupRolloverTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	child := seedApprovedChild(t, env, env.sourcePhase.ID,
		"Rita", "Repeat", "rita.repeat@example.com",
		"Rio", "Repeat", int16(2))
	offering := sourceOffering(t, env, &enrollmentModels.CareOffering{
		Name:           "Wiederholung",
		DaysOfWeekMode: enrollmentModels.DaysOfWeekModeFixed,
		AvailableDays:  []string{"mon"},
		IsActive:       true,
	})
	linkChildOffering(t, env, &enrollmentModels.RequestChildOffering{
		RequestChildID: child.ID,
		CareOfferingID: offering.ID,
	})

	first, err := env.rolloverSvc.CreatePhaseFromSource(ctx,
		validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true))
	require.NoError(t, err)

	secondReq := validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true)
	secondReq.Name = "rollover-target-second-run"
	_, err = env.rolloverSvc.CreatePhaseFromSource(ctx, secondReq)
	require.ErrorIs(t, err, enrollmentService.ErrRolloverSourceAlreadyRolled)

	rolled, err := env.repos.RequestChild.ListByPhaseAndStatuses(
		ctx, first.Phase.ID, []string{enrollmentModels.ChildStatusAutoRenewed})
	require.NoError(t, err)
	require.Len(t, rolled, 1, "repeated execution must not duplicate target enrollments")
	links, err := env.repos.RequestChildOffering.ListByRequestChildID(ctx, rolled[0].ID)
	require.NoError(t, err)
	assert.Len(t, links, 1, "repeated execution must not duplicate bookings")
	clones, err := env.repos.CareOffering.ListByPhase(ctx, first.Phase.ID)
	require.NoError(t, err)
	assert.Len(t, clones, 1, "repeated execution must not duplicate cloned offerings")
}

// TestRolloverService_AutoApprove_MaterializesCarriedDaysIntoLinkedTemplate is
// the #2249 acceptance test: a parent-choice offering with several selected
// weekdays and a linked timetable template travels from rollover through
// Freigabe, and the approval materializes the carried weekdays into the
// template roster.
func TestRolloverService_AutoApprove_MaterializesCarriedDaysIntoLinkedTemplate(t *testing.T) {
	t.Parallel()

	env, cleanup := setupAutoApproveIntegrationEnv(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	// Template + period spanning source AND target phase (the half-year
	// rollover case: one planning period, two enrollment phases).
	group := createCareOfferingTemplateGroup(t, env.db, "rollover-carryover")
	period := createCareOfferingTestPeriod(
		t, env.db, "rollover-carryover",
		timezone.NewDate(2026, 8, 1),
		timezone.NewDate(2028, 8, 31),
	)
	createCareOfferingTemplateSchedule(t, env.db, group.ID, activitiesModels.WeekdayTuesday, &period.ID)
	createCareOfferingTemplateSchedule(t, env.db, group.ID, activitiesModels.WeekdayThursday, &period.ID)

	offering := &enrollmentModels.CareOffering{
		PhaseID:         env.sourcePhase.ID,
		ActivityGroupID: &group.ID,
		Name:            "Wochentagswahl mit Vorlage",
		DaysOfWeekMode:  enrollmentModels.DaysOfWeekModeParentChoice,
		AvailableDays:   []string{"tue", "thu"},
		IsActive:        true,
	}
	offering.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.CareOffering.Create(ctx, offering))

	source, existing := seedApprovedChildWithStudent(
		t, env,
		"Vera", "Vorlage", "vera.vorlage@example.com",
		"Vito", "Vorlage",
		int16(1),
	)
	link := &enrollmentModels.RequestChildOffering{
		RequestChildID:     source.ID,
		CareOfferingID:     offering.ID,
		SelectedDays:       []string{"tue", "thu"},
		ManualSelectedDays: []string{"tue", "thu"},
	}
	link.SetTenantID(testpkg.Tenant(t))
	require.NoError(t, env.repos.RequestChildOffering.Create(ctx, link))

	req := validRolloverRequest(env, enrollmentModels.PhaseRolloverModeOptOut, true)
	req.RolloverAutoApprove = true
	req.RolloverDeadline = time.Now().Add(-time.Hour)
	req.Name = "carryover-materialization-target"
	result, err := env.rolloverSvc.CreatePhaseFromSource(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 1, result.ClonedOfferingCount)

	// The clone keeps the timetable-template link — the series covers the
	// target phase, so approval materialization keeps working (#2249).
	clones, err := env.repos.CareOffering.ListByPhase(ctx, result.Phase.ID)
	require.NoError(t, err)
	require.Len(t, clones, 1)
	require.NotNil(t, clones[0].ActivityGroupID)
	assert.Equal(t, group.ID, *clones[0].ActivityGroupID)

	rolled, err := env.repos.RequestChild.ListByPhaseAndStatuses(
		ctx, result.Phase.ID, []string{enrollmentModels.ChildStatusAutoRenewed})
	require.NoError(t, err)
	require.Len(t, rolled, 1)
	rolledChildID := rolled[0].ID

	t.Cleanup(func() {
		_, _ = env.db.NewDelete().
			TableExpr("activities.student_enrollments").
			Where("student_id = ?", existing.ID).
			Exec(context.Background())
	})

	err = testpkg.WithTenantTx(t, context.Background(), env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		summary, workerErr := env.rolloverSvc.RunDeadlineWorker(txCtx, time.Now())
		if workerErr != nil {
			return workerErr
		}
		require.Equal(t, 1, summary.AutoRenewedToApproved)
		require.Equal(t, 0, summary.AutoApproveErrors)
		return nil
	})
	require.NoError(t, err)

	// Freigabe materialized the carried weekdays into the linked template.
	var rows []activitiesModels.StudentEnrollment
	require.NoError(t, env.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`activities.student_enrollments AS "student_enrollment"`).
		Where(`"student_enrollment".tenant_id = ?`, testpkg.Tenant(t)).
		Where(`"student_enrollment".student_id = ?`, existing.ID).
		Where(`"student_enrollment".activity_group_id = ?`, group.ID).
		Scan(ctx))
	require.Len(t, rows, 1)
	require.NotNil(t, rows[0].EnrollmentRequestChildID)
	assert.Equal(t, rolledChildID, *rows[0].EnrollmentRequestChildID)
	assert.Equal(t, []int{activitiesModels.WeekdayTuesday, activitiesModels.WeekdayThursday}, rows[0].SelectedWeekdays,
		"the carried weekday selection must reach the template roster")
}
