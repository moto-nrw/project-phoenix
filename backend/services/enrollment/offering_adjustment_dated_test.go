package enrollment_test

import (
	"strconv"
	"testing"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	phaseFixture "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"

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
	ctx := testpkg.Ctx(t)

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
		TenantID:          testpkg.Tenant(t),
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
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	oldGroup := testpkg.CreateTestActivityGroup(t, env.db, "DatedSwitchOld")
	newGroup := testpkg.CreateTestActivityGroup(t, env.db, "DatedSwitchNew")
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
	switchDate := timezone.Date(env.sourcePhase.ServiceStartDate).AddDays(150)

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
	assert.Equal(t, activitiesModels.Date(env.sourcePhase.ServiceStartDate), oldRow.ValidFrom)
	require.NotNil(t, oldRow.ValidUntil)
	assert.Equal(t, activitiesModels.Date(switchDate), *oldRow.ValidUntil, "old row ends exclusively on the switch date")

	newRow, ok := byGroup[newGroup.ID]
	require.True(t, ok, "new selection must be materialized")
	assert.Equal(t, activitiesModels.Date(switchDate), newRow.ValidFrom, "new row starts on the switch date")
	require.NotNil(t, newRow.ValidUntil)
	assert.Equal(t, activitiesModels.Date(*originalValidUntil), *newRow.ValidUntil, "new row runs to the end of the care period")
	require.NotNil(t, newRow.EnrollmentRequestChildID)
	assert.Equal(t, childID, *newRow.EnrollmentRequestChildID)

	currentLinks, err := env.repos.Enrollment().RequestChildOfferingsAtDate(ctx, childID, capability.Date(decisionTestToday))
	require.NoError(t, err)
	require.Len(t, currentLinks, 1)
	assert.Equal(t, oldOffering.ID, currentLinks[0].CareOfferingID,
		"the parent-facing booking remains the old offering before the switch")
	futureLinks, err := env.repos.Enrollment().RequestChildOfferingsAtDate(ctx, childID, capability.Date(switchDate))
	require.NoError(t, err)
	require.Len(t, futureLinks, 1)
	assert.Equal(t, newOffering.ID, futureLinks[0].CareOfferingID)

	oldTaken, err := env.repos.Enrollment().OfferingCapacityPeak(ctx, oldOffering.ID, nil, capability.Date(switchDate), capability.Date(switchDate.AddDays(1)))
	require.NoError(t, err)
	assert.Zero(t, oldTaken)
	newTaken, err := env.repos.Enrollment().OfferingCapacityPeak(ctx, newOffering.ID, nil, capability.Date(switchDate), capability.Date(switchDate.AddDays(1)))
	require.NoError(t, err)
	assert.Equal(t, 1, newTaken)
}

func TestDecisionService_UpdateChildOfferings_DatedSwitchKeepsUnchangedOffering(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	keptGroup := testpkg.CreateTestActivityGroup(t, env.db, "DatedKeptGroup")
	addedGroup := testpkg.CreateTestActivityGroup(t, env.db, "DatedAddedGroup")
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

	switchDate := timezone.Date(env.sourcePhase.ServiceStartDate).AddDays(150)

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
	assert.Equal(t, activitiesModels.Date(env.sourcePhase.ServiceStartDate), keptRow.ValidFrom)

	addedRow, ok := byGroup[addedGroup.ID]
	require.True(t, ok)
	assert.Equal(t, activitiesModels.Date(switchDate), addedRow.ValidFrom)
}

func TestDecisionService_UpdateChildOfferings_CurrentCorrectionPreservesScheduledSwitch(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	oldGroup := testpkg.CreateTestActivityGroup(t, env.db, "CorrectionOld")
	correctedGroup := testpkg.CreateTestActivityGroup(t, env.db, "CorrectionNow")
	scheduledGroup := testpkg.CreateTestActivityGroup(t, env.db, "CorrectionLater")
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
	switchDate := timezone.Date(env.sourcePhase.ServiceStartDate).AddDays(150)
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

	futureLinks, err := env.repos.Enrollment().RequestChildOfferingsAtDate(ctx, childID, capability.Date(switchDate))
	require.NoError(t, err)
	require.Len(t, futureLinks, 1)
	assert.Equal(t, scheduledOffering.ID, futureLinks[0].CareOfferingID)

	rows := rowsByGroupForAdjustmentTest(listStudentEnrollmentRowsForDecisionTest(t, env, studentID))
	assert.Equal(t, activitiesModels.Date(switchDate), rows[scheduledGroup.ID].ValidFrom)
}

func TestDecisionService_UpdateChildOfferings_DatedSwitchExtendsRetainedOfferingPastSupersededSwitch(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	keptGroup := testpkg.CreateTestActivityGroup(t, env.db, "DatedExtendedKeptGroup")
	firstAddedGroup := testpkg.CreateTestActivityGroup(t, env.db, "DatedExtendedFirstGroup")
	secondAddedGroup := testpkg.CreateTestActivityGroup(t, env.db, "DatedExtendedSecondGroup")
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
	firstSwitch := timezone.Date(env.sourcePhase.ServiceStartDate).AddDays(150)
	_, err := env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID: requestID, ChildID: childID, ActorAccountID: env.creatorID, ActorRole: "admin",
		Reason: "Erste geplante Änderung", EffectiveFrom: &firstSwitch,
		Offerings: []enrollmentService.OfferingAdjustmentSelection{
			{OfferingID: keptOffering.ID, SelectedDays: []string{"mon"}},
			{OfferingID: firstAddedOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	secondSwitch := timezone.Date(env.sourcePhase.ServiceStartDate).AddDays(80)
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
	assert.Equal(t, activitiesModels.Date(timezone.Date(env.sourcePhase.ServiceEndDate).AddDays(1)), *kept.ValidUntil,
		"the retained offering must not end at the superseded switch date")
	_, firstStillPlanned := rows[firstAddedGroup.ID]
	assert.False(t, firstStillPlanned)
	assert.Equal(t, activitiesModels.Date(secondSwitch), rows[secondAddedGroup.ID].ValidFrom)
}

func TestDecisionService_UpdateChildOfferings_DatedSwitchBeforePhaseStartDropsUnstartedRow(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	oldGroup := testpkg.CreateTestActivityGroup(t, env.db, "DatedUnstartedOld")
	newGroup := testpkg.CreateTestActivityGroup(t, env.db, "DatedUnstartedNew")
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
	switchDate := timezone.NewDate(2026, 8, 24)
	require.True(t, switchDate.Before(timezone.Date(env.sourcePhase.ServiceStartDate)),
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
	assert.Equal(t, activitiesModels.Date(env.sourcePhase.ServiceStartDate), rows[0].ValidFrom,
		"an effective date before the care period must not widen the window")
}

func TestDecisionService_UpdateChildOfferings_RejectsEffectiveFromOutsideWindow(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)

	group := testpkg.CreateTestActivityGroup(t, env.db, "DatedRejectGroup")
	offering := createAdjustmentCareOfferingWith(t, env, "Egal", func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &group.ID
		o.SortOrder = 131
	})

	requestID, childID, _ := submitApprovedAdjustmentChild(
		t, env, "dated-reject@example.com", "DatedReject",
		[]*enrollmentModels.CareOffering{offering},
	)

	cases := map[string]timezone.Date{
		"past":            timezone.NewDate(2026, 8, 24).AddDays(-1),
		"after care ends": timezone.Date(env.sourcePhase.ServiceEndDate).AddDays(1),
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

// startRunningCarePeriodForTest moves the fixture phase so that today lies
// inside its care period. The dated paths below only differ from the
// undated ones once a switch date can be reached, which a phase starting in
// the future never allows.
func startRunningCarePeriodForTest(t *testing.T, env *decisionTestEnv) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	env.sourcePhase.ServiceStartDate = phaseFixture.Date(decisionTestToday.AddDays(-60))
	env.sourcePhase.ServiceEndDate = phaseFixture.Date(decisionTestToday.AddDays(240))
	require.NoError(t, env.repos.Enrollment().UpdatePhase(ctx, enrollmentService.OwnerPhaseForTest(env.sourcePhase)))
}

// A dated switch splits the child's offering links into intervals. Everything
// that reads them as "the current booking" has to read them at a date -
// otherwise the reopened form offers the superseded selection back and
// approving an unrelated correction writes it over the switch.
func TestChangeRequestService_ApproveKeepsAppliedOfferingSwitch(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	startRunningCarePeriodForTest(t, env)

	oldGroup := testpkg.CreateTestActivityGroup(t, env.db, "ChangeRequestOld")
	newGroup := testpkg.CreateTestActivityGroup(t, env.db, "ChangeRequestNew")
	oldOffering := createAdjustmentCareOfferingWith(t, env, "Bisheriges Angebot", func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &oldGroup.ID
		o.SortOrder = 151
	})
	newOffering := createAdjustmentCareOfferingWith(t, env, "Neues Angebot", func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &newGroup.ID
		o.SortOrder = 152
	})

	requestID, childID, studentID := submitApprovedAdjustmentChild(
		t, env, "change-request-switch@example.com", "SwitchKeeper",
		[]*enrollmentModels.CareOffering{oldOffering},
	)

	switchDate := decisionTestToday
	_, err := env.decision.UpdateChildOfferings(ctx, enrollmentService.UpdateChildOfferingsInput{
		RequestID:      requestID,
		ChildID:        childID,
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
		Reason:         "Wechsel ab heute",
		EffectiveFrom:  &switchDate,
		Offerings: []enrollmentService.OfferingAdjustmentSelection{
			{OfferingID: newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	requestRow, err := enrollmentService.ReadOwnerRequestForTest(ctx, env.repos.Enrollment(), requestID)
	require.NoError(t, err)

	// The reopened form and the change request below predate the takeover
	// (ADR 0003); after the takeover both paths are closed for this child.
	restoreTakeover := liftTakeoverStamp(t, env, requestID)
	draft, err := env.requestSvc.GetEditDraft(ctx, requestRow.StatusToken)
	require.NoError(t, err)
	draftLinks := draft.OfferingsByChild[childID]
	require.Len(t, draftLinks, 1, "the reopened form must show the booking in force, not the interval history")
	assert.Equal(t, newOffering.ID, draftLinks[0].CareOfferingID)

	proposed := enrollmentService.SubmitRequest{
		TenantID:          testpkg.Tenant(t),
		PhaseID:           env.sourcePhase.ID,
		GuardianFirstName: "Elternteil",
		GuardianLastName:  "SwitchKeeper",
		GuardianEmail:     "change-request-switch@example.com",
		ConsentFlags: map[string]any{
			"agb":             true,
			"data_processing": true,
			"email_contact":   true,
			"photo":           true,
		},
		Children: []enrollmentService.SubmitChild{
			{
				ID:               childID,
				FirstName:        "Kind",
				LastName:         "SwitchKeeper",
				DateOfBirth:      timezone.NewDate(2018, 4, 15),
				TargetGradeLevel: testpkg.Int16Ptr(2),
				OfferingIDs:      []int64{draftLinks[0].CareOfferingID},
				OfferingDays: []enrollmentService.SubmitOfferingDays{
					{OfferingID: draftLinks[0].CareOfferingID, SelectedDays: []string{"mon"}},
				},
			},
		},
	}

	changeRequests := newChangeRequestServiceWithDecisionForTest(t, env)
	created, err := changeRequests.Create(ctx, requestRow.StatusToken, enrollmentService.CreateChangeRequestInput{
		Submission: proposed,
		ParentNote: "Bitte den Vornamen des Elternteils korrigieren.",
	})
	require.NoError(t, err)
	restoreTakeover()

	// The base the staff diff is rendered against is today's booking. Pinned to
	// the service start it would report an offering change the parent never
	// proposed - and miss a real one.
	// The base the staff diff is rendered against, and that the approval
	// compares against for conflicts, is today's booking. Pinned to the service
	// start it would describe the superseded one.
	baseChildren, ok := created.ChangeRequest.BaseSnapshot["children"].([]any)
	require.True(t, ok)
	require.Len(t, baseChildren, 1)
	baseChild, ok := baseChildren[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []any{strconv.FormatInt(newOffering.ID, 10)}, baseChild["offering_ids"])

	_, err = changeRequests.Approve(ctx, created.ChangeRequest.ID, enrollmentService.ReviewChangeRequestInput{
		Note:           "Freigegeben.",
		ActorAccountID: env.creatorID,
		ActorRole:      "admin",
	})
	require.NoError(t, err)

	currentLinks, err := env.repos.Enrollment().RequestChildOfferingsAtDate(ctx, childID, capability.Date(decisionTestToday))
	require.NoError(t, err)
	require.Len(t, currentLinks, 1)
	assert.Equal(t, newOffering.ID, currentLinks[0].CareOfferingID,
		"approving an unrelated correction must not restore the superseded booking")

	// An undated correction stays retroactive by design, so it rewrites the
	// group enrollment over the whole care period. What it must not do is
	// rewrite it to the group the child left.
	rows := listStudentEnrollmentRowsForDecisionTest(t, env, studentID)
	byGroup := rowsByGroupForAdjustmentTest(rows)
	_, enrolledInNew := byGroup[newGroup.ID]
	assert.True(t, enrolledInNew, "the switched-to group must stay enrolled")
	_, enrolledInOld := byGroup[oldGroup.ID]
	assert.False(t, enrolledInOld, "the group the child left must not be re-enrolled")
}
