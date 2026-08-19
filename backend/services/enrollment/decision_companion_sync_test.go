package enrollment_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Companion regressions for the admin-correction sync of an APPROVED child
// (#1694). Both cover the same blind spot: SyncApprovedChildData drives the
// shared student write path, which reconciles the "läuft mit" links against the
// plan it stores — so the sync can be REFUSED by a link it never mentions, and
// it can CHANGE rows on another child's card without saying so.
//
//   - the refusal must reach the caller even outside the replacement path,
//     or an admin correction reports success for a plan that never landed
//   - student_companions_changed must fire on the writes that actually trimmed
//     a link and stay silent on the ones that did not, because the event makes
//     every open companion editor in the school discard or block its draft

// newCompanionSyncApplier wires the decision service exactly like
// newDecisionServiceForTest but with a recording broadcaster, so the
// after-commit plan announcements become observable. Returns the
// change-request applier surface the admin correction goes through.
func newCompanionSyncApplier(
	t *testing.T,
	env *decisionTestEnv,
	bc *testpkg.RecordingBroadcaster,
) enrollmentService.ChangeRequestDecisionApplier {
	t.Helper()
	repoFactory := repositories.NewFactory(env.db)
	svc := enrollmentService.NewDecisionService(enrollmentService.DecisionServiceConfig{
		RequestRepo:              repoFactory.Request,
		RequestChildRepo:         repoFactory.RequestChild,
		RequestGuardianRepo:      repoFactory.RequestGuardian,
		RequestChildOfferingRepo: repoFactory.RequestChildOffering,
		CareOfferingRepo:         repoFactory.CareOffering,
		PhaseRepo:                repoFactory.Phase,
		FormSchemaRepo:           repoFactory.FormSchema,
		OfferingAdjustmentRepo:   repoFactory.EnrollmentOfferingAdjustment,
		PersonRepo:               repoFactory.Person,
		StaffRepo:                repoFactory.Staff,
		StudentRepo:              repoFactory.Student,
		StudentGuardianRepo:      repoFactory.StudentGuardian,
		GuardianProfileRepo:      repoFactory.GuardianProfile,
		GuardianPhoneRepo:        repoFactory.GuardianPhoneNumber,
		PickupScheduleRepo:       repoFactory.StudentPickupSchedule,
		ArrivalScheduleRepo:      repoFactory.StudentArrivalSchedule,
		StudentEnrollmentRepo:    repoFactory.StudentEnrollment,
		ActivityGroupRepo:        repoFactory.ActivityGroup,
		ActivityScheduleRepo:     repoFactory.ActivitySchedule,
		CalendarPeriodRepo:       repoFactory.CalendarPeriod,
		TimeframeRepo:            repoFactory.Timeframe,
		ActivityExceptionRepo:    repoFactory.ActivityException,
		AccountRepo:              repoFactory.Account,
		AccountTenantRepo:        repoFactory.AccountTenant,
		AccountRoleRepo:          repoFactory.AccountRole,
		RoleRepo:                 repoFactory.Role,
		OutboxEnqueuer:           env.outbox,
		Broadcaster:              bc,
		FrontendURL:              "http://localhost:3000",
		ParentsURL:               "http://parents.localhost:3000",
		Logger:                   slog.Default(),
	})
	applier, ok := svc.(enrollmentService.ChangeRequestDecisionApplier)
	require.True(t, ok, "decision service must implement the change-request applier contract")
	return applier
}

// publishCompanionModesSchema pins the phase to a form whose single field
// targets the student's allowed departure modes — the payload that makes the
// student write reconcile the "läuft mit" links.
func publishCompanionModesSchema(t *testing.T, env *decisionTestEnv, name string) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	schemaSvc := enrollmentService.NewFormSchemaService(enrollmentService.FormSchemaServiceConfig{
		Repo:   env.repos.FormSchema,
		Logger: slog.Default(),
	})
	schema, err := schemaSvc.CreateSchema(ctx, name, []enrollmentModels.FormField{{
		Key:         "allowed_modes",
		Label:       "Erlaubte Heimwege",
		Type:        enrollmentModels.FormFieldWeekdayMultiMode,
		Target:      enrollmentModels.TargetStudentAllowedDepartureModes,
		AppliesToCh: true,
		SortOrder:   0,
	}}, env.creatorID)
	require.NoError(t, err)
	env.sourcePhase.FormSchemaID = &schema.ID
	require.NoError(t, env.repos.Phase.Update(ctx, env.sourcePhase))
}

// approveAccompaniedTuesdayChild submits and approves one child whose plan
// allows "Anderes Kind" on Tuesday (with the coupled "mit wem" note, so the
// plan is valid on its own) and returns the request/child ids plus the created
// student id.
func approveAccompaniedTuesdayChild(
	t *testing.T,
	env *decisionTestEnv,
	guardianEmail, first, last string,
) (requestID, childID, studentID int64) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	requestID, childID = submitOneChildWithCustomData(t, env, guardianEmail, first, last, map[string]any{
		"allowed_modes": map[string]any{"tue": []any{"accompanied"}},
		enrollmentModels.TargetStudentDepartureCompanionNote: "Nachbarskind",
	})
	outcome, err := env.decision.Decide(ctx, enrollmentService.DecideInput{
		RequestID:  requestID,
		ChildID:    childID,
		Status:     enrollmentService.DecisionApproved,
		ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)
	require.NotNil(t, outcome.Child.CreatedStudentID)
	student, err := env.repos.Student.FindByID(ctx, *outcome.Child.CreatedStudentID)
	require.NoError(t, err)
	require.True(t, usersModels.AccompaniedWeekdays(student.AllowedDepartureModes, student.DepartureDays)[usersModels.PickupDayTuesday],
		"the approval must leave the child accompanied on Tuesday, or the link below has no basis")
	return requestID, childID, student.ID
}

// linkCompanionPartnerOnTuesday gives studentID a Laufgemeinschaft partner on
// Tuesday and returns the partner's id. withOwnNote decides whether the partner
// keeps its own free-text "mit wem" detail: with a note, dropping the link is
// allowed; without one the link is the partner's only cover, so dropping it
// strands them and the whole write is refused.
func linkCompanionPartnerOnTuesday(t *testing.T, env *decisionTestEnv, studentID int64, withOwnNote bool) int64 {
	t.Helper()
	ctx := testpkg.Ctx(t)

	partner := testpkg.CreateTestStudent(t, env.db, "Companion", "SyncPartner", "1a")
	t.Cleanup(func() { testpkg.CleanupActivityFixtures(t, env.db, partner.ID) })
	t.Cleanup(func() {
		_, err := env.db.NewDelete().
			TableExpr("users.student_companions").
			Where("student_low_id IN (?, ?) OR student_high_id IN (?, ?)",
				studentID, partner.ID, studentID, partner.ID).
			Exec(context.Background())
		if err != nil {
			t.Logf("Warning: failed to cleanup users.student_companions: %v", err)
		}
	})

	// Give the partner an accompanied Tuesday. A note is required to save that
	// plan before any link exists; it is cleared again below when the test wants
	// the link to be the partner's only cover.
	stored, err := env.repos.Student.FindByID(ctx, partner.ID)
	require.NoError(t, err)
	stored.AllowedDepartureModes = usersModels.AllowedDepartureModes{
		usersModels.PickupDayTuesday: []usersModels.DepartureMode{usersModels.DepartureAccompanied},
	}
	stored.DepartureDays = nil
	stored.BusDays = nil
	stored.PickupDays = nil
	note := "Nachbarskind"
	stored.DepartureCompanionNote = &note
	require.NoError(t, env.repos.Student.Update(ctx, stored))

	edge, err := usersModels.NewStudentCompanion(studentID, partner.ID, 2)
	require.NoError(t, err)
	require.NoError(t, env.repos.StudentCompanion.ReplaceForStudent(ctx, studentID,
		[]*usersModels.StudentCompanion{edge}))

	if !withOwnNote {
		// The link now covers the accompanied Tuesday, so the note can go — and
		// with it the partner's only alternative detail.
		stored, err = env.repos.Student.FindByID(ctx, partner.ID)
		require.NoError(t, err)
		stored.DepartureCompanionNote = nil
		require.NoError(t, env.repos.Student.Update(ctx, stored))
	}
	return partner.ID
}

// narrowChildToBusMonday rewrites the approved child's submitted answer to a
// plan that no longer allows "Anderes Kind" on Tuesday — the admin correction
// whose student write must trim the Tuesday link.
func narrowChildToBusMonday(t *testing.T, env *decisionTestEnv, childID int64) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	child, err := env.repos.RequestChild.FindByID(ctx, childID)
	require.NoError(t, err)
	child.CustomData = map[string]any{
		"allowed_modes": map[string]any{"mon": []any{"bus"}},
	}
	require.NoError(t, env.repos.RequestChild.UpdateData(ctx, child))
}

// syncApprovedChild runs the admin correction inside a tenant transaction, the
// way the handler does — the plan broadcasts are registered after-commit, so
// they only materialize when the transaction actually commits.
func syncApprovedChild(t *testing.T, env *decisionTestEnv, applier enrollmentService.ChangeRequestDecisionApplier, requestID, childID int64) error {
	t.Helper()
	return tenant.WithTenantTx(testpkg.Ctx(t), env.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		_, err := applier.SyncApprovedChildData(txCtx, enrollmentService.SyncApprovedChildDataInput{
			RequestID:           requestID,
			ChildID:             childID,
			ActorAccountID:      env.creatorID,
			ReplaceTargetedData: false,
		})
		return err
	})
}

// TestDecisionService_SyncApprovedChildData_CompanionRefusalSurfacesWithoutReplace
// pins that a REFUSED departure sync fails the whole correction even on the
// non-replacement path. Targeted-field errors are deliberately best-effort
// there — but the companion sentinels are not field noise: they mean the plan
// was rejected because storing it would leave the linked child with an
// accompanied weekday and no "mit wem" detail at all. Swallowing them let the
// admin correction report success for a change the database refused, leaving
// the UI showing a plan nobody stored.
func TestDecisionService_SyncApprovedChildData_CompanionRefusalSurfacesWithoutReplace(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := testpkg.Ctx(t)
	publishCompanionModesSchema(t, env, "Testformular Companion Refusal")

	reqID, childID, studentID := approveAccompaniedTuesdayChild(t, env,
		"companion-refusal@example.com", "Refus", "Companion")
	// The partner keeps no note of their own, so this link is their only cover.
	partnerID := linkCompanionPartnerOnTuesday(t, env, studentID, false)

	narrowChildToBusMonday(t, env, childID)

	bc := testpkg.NewRecordingBroadcaster()
	applier := newCompanionSyncApplier(t, env, bc)
	err := syncApprovedChild(t, env, applier, reqID, childID)

	require.Error(t, err, "a refused departure sync must not report success")
	assert.ErrorIs(t, err, usersModels.ErrCompanionWouldLoseDeparture,
		"the refusal has to stay reachable so the handler can answer with the actionable 4xx")

	// Nothing may have landed: the plan is unchanged and the link still exists.
	student, ferr := env.repos.Student.FindByID(ctx, studentID)
	require.NoError(t, ferr)
	assert.True(t, usersModels.AccompaniedWeekdays(student.AllowedDepartureModes, student.DepartureDays)[usersModels.PickupDayTuesday],
		"the refused correction must roll back, leaving the accompanied Tuesday in place")
	edges, eerr := env.repos.StudentCompanion.ListForStudent(ctx, studentID)
	require.NoError(t, eerr)
	require.Len(t, edges, 1, "the refused correction must not drop the link")
	far, ok := edges[0].Other(studentID)
	require.True(t, ok)
	assert.Equal(t, partnerID, far)

	assert.False(t, bc.HasEventType(realtime.EventStudentCompanionsChanged),
		"a rolled-back sync announces nothing")
}

// TestDecisionService_SyncApprovedChildData_CompanionEventOnlyOnEffectiveTrim
// pins WHO the correction wakes: student_companions_changed makes every open
// "läuft mit" editor in the school mark itself stale and refuse to save, so it
// may only fire when the sync actually reconciled a link. Carrying a departure
// plan is not that question — writing the same modes back trims nothing, and
// announcing it anyway costs unrelated staff their unsaved companion edits.
func TestDecisionService_SyncApprovedChildData_CompanionEventOnlyOnEffectiveTrim(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	publishCompanionModesSchema(t, env, "Testformular Companion Event")

	bc := testpkg.NewRecordingBroadcaster()
	applier := newCompanionSyncApplier(t, env, bc)

	t.Run("a departure sync that trims no link stays silent", func(t *testing.T) {
		reqID, childID, _ := approveAccompaniedTuesdayChild(t, env,
			"companion-silent@example.com", "Silent", "Companion")

		bc.Reset()
		require.NoError(t, syncApprovedChild(t, env, applier, reqID, childID))

		assert.False(t, bc.HasEventType(realtime.EventStudentCompanionsChanged),
			"a departure sync that trims no link changed no Laufgemeinschaft to announce")
	})

	t.Run("a departure sync that trims a link announces it", func(t *testing.T) {
		reqID, childID, studentID := approveAccompaniedTuesdayChild(t, env,
			"companion-announce@example.com", "Announce", "Companion")
		// The partner keeps their own note, so dropping the link is allowed.
		linkCompanionPartnerOnTuesday(t, env, studentID, true)
		narrowChildToBusMonday(t, env, childID)

		bc.Reset()
		require.NoError(t, syncApprovedChild(t, env, applier, reqID, childID))

		assert.True(t, bc.HasEventType(realtime.EventStudentCompanionsChanged),
			"Tuesday no longer allows 'Anderes Kind', so the link is gone from the partner's card too")

		edges, err := env.repos.StudentCompanion.ListForStudent(testpkg.Ctx(t), studentID)
		require.NoError(t, err)
		assert.Empty(t, edges, "the announced sync must actually have trimmed the link")
	})
}
