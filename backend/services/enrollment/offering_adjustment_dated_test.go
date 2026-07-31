package enrollment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Dated offering adjustments (issue #1665). A parent-requested switch takes
// effect on a date instead of rewriting the whole care period, so the groups
// the child actually attended before the switch stay on the record.

// submitApprovedAdjustmentChild submits and approves a one-child request for
// the given offerings and returns the request, child and created student.
func submitApprovedAdjustmentChild(
	t *testing.T,
	env *decisionTestEnv,
	email, lastName string,
	offerings []*enrollmentModels.CareOffering,
) (requestID, childID, studentID int64) {
	t.Helper()
	ctx := testpkg.TenantContext(1)

	offeringIDs := make([]int64, 0, len(offerings))
	offeringDays := make([]enrollmentService.SubmitOfferingDays, 0, len(offerings))
	for _, offering := range offerings {
		offeringIDs = append(offeringIDs, offering.ID)
		offeringDays = append(offeringDays, enrollmentService.SubmitOfferingDays{
			OfferingID:   offering.ID,
			SelectedDays: []string{"mon"},
		})
	}

	submitted, err := env.requestSvc.Submit(ctx, enrollmentService.SubmitRequest{
		TenantID:          1,
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Eltern",
		GuardianLastName:  lastName,
		GuardianEmail:     email,
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           true,
		},
		Children: []enrollmentService.SubmitChild{
			{
				FirstName:        "Kind",
				LastName:         lastName,
				DateOfBirth:      timezone.NewDate(2018, 4, 15),
				TargetGradeLevel: testpkg.Int16Ptr(2),
				OfferingIDs:      offeringIDs,
				OfferingDays:     offeringDays,
			},
		},
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

	return submitted.Request.ID, submitted.Children[0].ID, *outcome.Child.CreatedStudentID
}

func rowsByGroupForAdjustmentTest(
	rows []activitiesModels.StudentEnrollment,
) map[int64]activitiesModels.StudentEnrollment {
	byGroup := make(map[int64]activitiesModels.StudentEnrollment, len(rows))
	for _, row := range rows {
		byGroup[row.ActivityGroupID] = row
	}
	return byGroup
}

func TestDecisionService_UpdateChildOfferings_DatedSwitchCapsOldAndStartsNewGroup(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	oldGroup := testpkg.CreateTestActivityGroup(t, env.db, "DatedSwitchOld")
	newGroup := testpkg.CreateTestActivityGroup(t, env.db, "DatedSwitchNew")
	defer testpkg.CleanupActivityFixtures(t, env.db,
		oldGroup.ID, oldGroup.CategoryID, *oldGroup.CreatedBy,
		newGroup.ID, newGroup.CategoryID, *newGroup.CreatedBy,
	)
	oldOffering := createAdjustmentCareOfferingWith(t, env, "Frühbetreuung", func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &oldGroup.ID
		o.SortOrder = 101
	})
	newOffering := createAdjustmentCareOfferingWith(t, env, "Spätbetreuung", func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &newGroup.ID
		o.SortOrder = 102
	})

	requestID, childID, studentID := submitApprovedAdjustmentChild(
		t, env, "dated-switch@example.com", "DatedSwitch",
		[]*enrollmentModels.CareOffering{oldOffering},
	)

	rows := listStudentEnrollmentRowsForDecisionTest(t, env, studentID)
	require.Len(t, rows, 1)
	require.Equal(t, oldGroup.ID, rows[0].ActivityGroupID)
	originalValidUntil := rows[0].ValidUntil
	require.NotNil(t, originalValidUntil)

	// Mid-phase switch date: the phase starts in the future, so this is both
	// after the materialized valid_from and not in the past.
	switchDate := env.sourcePhase.ServiceStartDate.AddDays(150)

	_, err := env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID:      requestID,
		ChildID:        childID,
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
		Reason:         "Wechsel zum Halbjahr",
		EffectiveFrom:  &switchDate,
		Offerings: []enrollmentService.OfferingAdjustmentSelection{
			{OfferingID: newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	rows = listStudentEnrollmentRowsForDecisionTest(t, env, studentID)
	require.Len(t, rows, 2, "the attended group must be kept as history, not deleted")
	byGroup := rowsByGroupForAdjustmentTest(rows)

	oldRow, ok := byGroup[oldGroup.ID]
	require.True(t, ok, "row of the group attended before the switch must survive")
	assert.Equal(t, env.sourcePhase.ServiceStartDate, oldRow.ValidFrom)
	require.NotNil(t, oldRow.ValidUntil)
	assert.Equal(t, switchDate, *oldRow.ValidUntil, "old row ends exclusively on the switch date")

	newRow, ok := byGroup[newGroup.ID]
	require.True(t, ok, "new selection must be materialized")
	assert.Equal(t, switchDate, newRow.ValidFrom, "new row starts on the switch date")
	require.NotNil(t, newRow.ValidUntil)
	assert.Equal(t, *originalValidUntil, *newRow.ValidUntil, "new row runs to the end of the care period")
	require.NotNil(t, newRow.EnrollmentRequestChildID)
	assert.Equal(t, childID, *newRow.EnrollmentRequestChildID)

	currentLinks, err := env.repos.RequestChildOffering.ListByRequestChildIDAtDate(ctx, childID, timezone.TodayDate())
	require.NoError(t, err)
	require.Len(t, currentLinks, 1)
	assert.Equal(t, oldOffering.ID, currentLinks[0].CareOfferingID,
		"the parent-facing booking remains the old offering before the switch")
	futureLinks, err := env.repos.RequestChildOffering.ListByRequestChildIDAtDate(ctx, childID, switchDate)
	require.NoError(t, err)
	require.Len(t, futureLinks, 1)
	assert.Equal(t, newOffering.ID, futureLinks[0].CareOfferingID)

	oldTaken, err := env.repos.RequestChildOffering.CountActiveByCareOfferingOnDate(ctx, oldOffering.ID, switchDate)
	require.NoError(t, err)
	assert.Zero(t, oldTaken)
	newTaken, err := env.repos.RequestChildOffering.CountActiveByCareOfferingOnDate(ctx, newOffering.ID, switchDate)
	require.NoError(t, err)
	assert.Equal(t, 1, newTaken)
}

func TestDecisionService_UpdateChildOfferings_DatedSwitchKeepsUnchangedOffering(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	keptGroup := testpkg.CreateTestActivityGroup(t, env.db, "DatedKeptGroup")
	addedGroup := testpkg.CreateTestActivityGroup(t, env.db, "DatedAddedGroup")
	defer testpkg.CleanupActivityFixtures(t, env.db,
		keptGroup.ID, keptGroup.CategoryID, *keptGroup.CreatedBy,
		addedGroup.ID, addedGroup.CategoryID, *addedGroup.CreatedBy,
	)
	keptOffering := createAdjustmentCareOfferingWith(t, env, "Bleibt", func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &keptGroup.ID
		o.SortOrder = 111
	})
	addedOffering := createAdjustmentCareOfferingWith(t, env, "Kommt dazu", func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &addedGroup.ID
		o.SortOrder = 112
	})

	requestID, childID, studentID := submitApprovedAdjustmentChild(
		t, env, "dated-keep@example.com", "DatedKeep",
		[]*enrollmentModels.CareOffering{keptOffering},
	)

	rows := listStudentEnrollmentRowsForDecisionTest(t, env, studentID)
	require.Len(t, rows, 1)
	keptRowID := rows[0].ID

	switchDate := env.sourcePhase.ServiceStartDate.AddDays(150)

	_, err := env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID:      requestID,
		ChildID:        childID,
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
		Reason:         "Zusatzangebot ab Februar",
		EffectiveFrom:  &switchDate,
		Offerings: []enrollmentService.OfferingAdjustmentSelection{
			{OfferingID: keptOffering.ID, SelectedDays: []string{"mon"}},
			{OfferingID: addedOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	rows = listStudentEnrollmentRowsForDecisionTest(t, env, studentID)
	require.Len(t, rows, 2, "an unchanged offering must not be split into two adjacent rows")
	byGroup := rowsByGroupForAdjustmentTest(rows)

	keptRow, ok := byGroup[keptGroup.ID]
	require.True(t, ok)
	assert.Equal(t, keptRowID, keptRow.ID, "unchanged offering keeps its original row")
	assert.Equal(t, env.sourcePhase.ServiceStartDate, keptRow.ValidFrom)

	addedRow, ok := byGroup[addedGroup.ID]
	require.True(t, ok)
	assert.Equal(t, switchDate, addedRow.ValidFrom)
}

func TestDecisionService_UpdateChildOfferings_CurrentCorrectionPreservesScheduledSwitch(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	oldGroup := testpkg.CreateTestActivityGroup(t, env.db, "CorrectionOld")
	correctedGroup := testpkg.CreateTestActivityGroup(t, env.db, "CorrectionNow")
	scheduledGroup := testpkg.CreateTestActivityGroup(t, env.db, "CorrectionLater")
	defer testpkg.CleanupActivityFixtures(t, env.db,
		oldGroup.ID, oldGroup.CategoryID, *oldGroup.CreatedBy,
		correctedGroup.ID, correctedGroup.CategoryID, *correctedGroup.CreatedBy,
		scheduledGroup.ID, scheduledGroup.CategoryID, *scheduledGroup.CreatedBy,
	)
	oldOffering := createAdjustmentCareOfferingWith(t, env, "Bisher", func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &oldGroup.ID
		o.SortOrder = 141
	})
	correctedOffering := createAdjustmentCareOfferingWith(t, env, "Korrektur", func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &correctedGroup.ID
		o.SortOrder = 142
	})
	scheduledOffering := createAdjustmentCareOfferingWith(t, env, "Geplant", func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &scheduledGroup.ID
		o.SortOrder = 143
	})

	requestID, childID, studentID := submitApprovedAdjustmentChild(
		t, env, "current-correction@example.com", "CurrentCorrection", []*enrollmentModels.CareOffering{oldOffering},
	)
	switchDate := env.sourcePhase.ServiceStartDate.AddDays(150)
	_, err := env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID: requestID, ChildID: childID, ActorAccountID: env.creatorID, ActorRole: "admin",
		Reason: "Geplante Änderung", EffectiveFrom: &switchDate,
		Offerings: []enrollmentService.OfferingAdjustmentSelection{{OfferingID: scheduledOffering.ID, SelectedDays: []string{"mon"}}},
	})
	require.NoError(t, err)

	_, err = env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID: requestID, ChildID: childID, ActorAccountID: env.creatorID, ActorRole: "admin",
		Reason:    "Korrektur für heute",
		Offerings: []enrollmentService.OfferingAdjustmentSelection{{OfferingID: correctedOffering.ID, SelectedDays: []string{"mon"}}},
	})
	require.NoError(t, err)

	futureLinks, err := env.repos.RequestChildOffering.ListByRequestChildIDAtDate(ctx, childID, switchDate)
	require.NoError(t, err)
	require.Len(t, futureLinks, 1)
	assert.Equal(t, scheduledOffering.ID, futureLinks[0].CareOfferingID)

	rows := rowsByGroupForAdjustmentTest(listStudentEnrollmentRowsForDecisionTest(t, env, studentID))
	assert.Equal(t, switchDate, rows[scheduledGroup.ID].ValidFrom)
}

func TestDecisionService_UpdateChildOfferings_DatedSwitchExtendsRetainedOfferingPastSupersededSwitch(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	keptGroup := testpkg.CreateTestActivityGroup(t, env.db, "DatedExtendedKeptGroup")
	firstAddedGroup := testpkg.CreateTestActivityGroup(t, env.db, "DatedExtendedFirstGroup")
	secondAddedGroup := testpkg.CreateTestActivityGroup(t, env.db, "DatedExtendedSecondGroup")
	defer testpkg.CleanupActivityFixtures(t, env.db,
		keptGroup.ID, keptGroup.CategoryID, *keptGroup.CreatedBy,
		firstAddedGroup.ID, firstAddedGroup.CategoryID, *firstAddedGroup.CreatedBy,
		secondAddedGroup.ID, secondAddedGroup.CategoryID, *secondAddedGroup.CreatedBy,
	)
	keptOffering := createAdjustmentCareOfferingWith(t, env, "Bleibt durchgehend", func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &keptGroup.ID
		o.SortOrder = 114
	})
	firstAddedOffering := createAdjustmentCareOfferingWith(t, env, "Erste Planung", func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &firstAddedGroup.ID
		o.SortOrder = 115
	})
	secondAddedOffering := createAdjustmentCareOfferingWith(t, env, "Neue Planung", func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &secondAddedGroup.ID
		o.SortOrder = 116
	})

	requestID, childID, studentID := submitApprovedAdjustmentChild(
		t, env, "dated-extend@example.com", "DatedExtend",
		[]*enrollmentModels.CareOffering{keptOffering},
	)
	firstSwitch := env.sourcePhase.ServiceStartDate.AddDays(150)
	_, err := env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID: requestID, ChildID: childID, ActorAccountID: env.creatorID, ActorRole: "admin",
		Reason: "Erste geplante Änderung", EffectiveFrom: &firstSwitch,
		Offerings: []enrollmentService.OfferingAdjustmentSelection{
			{OfferingID: keptOffering.ID, SelectedDays: []string{"mon"}},
			{OfferingID: firstAddedOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	secondSwitch := env.sourcePhase.ServiceStartDate.AddDays(80)
	_, err = env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID: requestID, ChildID: childID, ActorAccountID: env.creatorID, ActorRole: "admin",
		Reason: "Erste Planung ersetzt", EffectiveFrom: &secondSwitch,
		Offerings: []enrollmentService.OfferingAdjustmentSelection{
			{OfferingID: keptOffering.ID, SelectedDays: []string{"mon"}},
			{OfferingID: secondAddedOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	rows := rowsByGroupForAdjustmentTest(listStudentEnrollmentRowsForDecisionTest(t, env, studentID))
	kept, ok := rows[keptGroup.ID]
	require.True(t, ok)
	require.NotNil(t, kept.ValidUntil)
	assert.Equal(t, env.sourcePhase.ServiceEndDate.AddDays(1), *kept.ValidUntil,
		"the retained offering must not end at the superseded switch date")
	_, firstStillPlanned := rows[firstAddedGroup.ID]
	assert.False(t, firstStillPlanned)
	assert.Equal(t, secondSwitch, rows[secondAddedGroup.ID].ValidFrom)
}

func TestDecisionService_UpdateChildOfferings_DatedSwitchBeforePhaseStartDropsUnstartedRow(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	oldGroup := testpkg.CreateTestActivityGroup(t, env.db, "DatedUnstartedOld")
	newGroup := testpkg.CreateTestActivityGroup(t, env.db, "DatedUnstartedNew")
	defer testpkg.CleanupActivityFixtures(t, env.db,
		oldGroup.ID, oldGroup.CategoryID, *oldGroup.CreatedBy,
		newGroup.ID, newGroup.CategoryID, *newGroup.CreatedBy,
	)
	oldOffering := createAdjustmentCareOfferingWith(t, env, "Vorher gewählt", func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &oldGroup.ID
		o.SortOrder = 121
	})
	newOffering := createAdjustmentCareOfferingWith(t, env, "Stattdessen", func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &newGroup.ID
		o.SortOrder = 122
	})

	requestID, childID, studentID := submitApprovedAdjustmentChild(
		t, env, "dated-unstarted@example.com", "DatedUnstarted",
		[]*enrollmentModels.CareOffering{oldOffering},
	)

	// A switch that lands before the care period even starts: nothing was
	// attended yet, so the superseded row has no interval worth keeping.
	switchDate := timezone.TodayDate()
	require.True(t, switchDate.Before(env.sourcePhase.ServiceStartDate),
		"fixture phase must still lie in the future for this case")

	_, err := env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID:      requestID,
		ChildID:        childID,
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
		Reason:         "Umgewählt vor Betreuungsbeginn",
		EffectiveFrom:  &switchDate,
		Offerings: []enrollmentService.OfferingAdjustmentSelection{
			{OfferingID: newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	rows := listStudentEnrollmentRowsForDecisionTest(t, env, studentID)
	require.Len(t, rows, 1, "a row that never took effect is dropped, not capped")
	assert.Equal(t, newGroup.ID, rows[0].ActivityGroupID)
	assert.Equal(t, env.sourcePhase.ServiceStartDate, rows[0].ValidFrom,
		"an effective date before the care period must not widen the window")
}

func TestDecisionService_UpdateChildOfferings_RejectsEffectiveFromOutsideWindow(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.TenantContext(1)

	group := testpkg.CreateTestActivityGroup(t, env.db, "DatedRejectGroup")
	defer testpkg.CleanupActivityFixtures(t, env.db, group.ID, group.CategoryID, *group.CreatedBy)
	offering := createAdjustmentCareOfferingWith(t, env, "Egal", func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &group.ID
		o.SortOrder = 131
	})

	requestID, childID, _ := submitApprovedAdjustmentChild(
		t, env, "dated-reject@example.com", "DatedReject",
		[]*enrollmentModels.CareOffering{offering},
	)

	cases := map[string]timezone.Date{
		"past":            timezone.TodayDate().AddDays(-1),
		"after care ends": env.sourcePhase.ServiceEndDate.AddDays(1),
	}
	for name, switchDate := range cases {
		t.Run(name, func(t *testing.T) {
			effectiveFrom := switchDate
			_, err := env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
				RequestID:      requestID,
				ChildID:        childID,
				ActorAccountID: env.creatorID,
				ActorRole:      "admin",
				Reason:         "Ungültiger Stichtag",
				EffectiveFrom:  &effectiveFrom,
				Offerings: []enrollmentService.OfferingAdjustmentSelection{
					{OfferingID: offering.ID, SelectedDays: []string{"mon"}},
				},
			})
			require.ErrorIs(t, err, enrollmentService.ErrOfferingAdjustmentInvalid)
		})
	}
}
