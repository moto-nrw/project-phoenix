package parent_test

// Integration tests for the parent-side care-schedule request paths
// (parent_care_schedule_service.go): CreateCareScheduleRequest and
// WithdrawCareScheduleRequest. These port the security-relevant scenarios that
// previously lived on the chat request path (CreateChildRequest /
// WithdrawChildRequest) onto the decoupled Stammdaten request flow (#1803):
//   - submitting a change request requires parent_portal.request.submit — NOT
//     the parent_portal.notes.write that plain chat needs,
//   - messaging and permanent-care request rights are independent,
//   - disabled field groups reject the whole manipulated request,
//   - withdraw stays available after a school disables request fields.
//
// Reuses parentSettingsStub from parent_settings_stub_test.go and the shared
// testpkg.RecordingBroadcaster.

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// buildCareScheduleService wires the parent service with the full care-schedule
// request stack (arrival/pickup read services + the schedule-domain request
// service), so the Stammdaten read view and request lifecycle run for real.
func buildCareScheduleService(t *testing.T, notesEnabled bool) (parentService.Service, *bun.DB, *repositories.Factory) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	return careScheduleServiceOn(t, db, repos, notesEnabled), db, repos
}

// careScheduleServiceOn wires the same parent service against an EXISTING
// db/repos, so a test can rebuild the service with a different messaging toggle
// while keeping the seeded guardian chain and persisted requests. Allocating a
// fresh SetupTestDB instead would point the rebuilt service at an empty schema.
func careScheduleServiceOn(t *testing.T, db *bun.DB, repos *repositories.Factory, notesEnabled bool) parentService.Service {
	t.Helper()
	return careScheduleServiceWithSettings(t, db, repos, map[string]bool{
		configModels.KeyParentSickNoteEnabled:           true,
		configModels.KeyParentNotesEnabled:              notesEnabled,
		configModels.KeyParentCareArrivalRequestEnabled: true,
		configModels.KeyParentCarePickupRequestEnabled:  true,
		configModels.KeyParentCareModeRequestEnabled:    true,
	})
}

func careScheduleServiceWithSettings(t *testing.T, db *bun.DB, repos *repositories.Factory, boolValues map[string]bool) parentService.Service {
	t.Helper()
	sf, err := services.NewFactory(repos, db, slog.Default())
	require.NoError(t, err)
	return parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StudentRepo:         repos.Student,
		StudentGuardianRepo: repos.StudentGuardian,
		GuardianProfileRepo: repos.GuardianProfile,
		PersonRepo:          repos.Person,
		Settings: parentSettingsStub{
			boolValues: boolValues,
			stringValues: map[string]string{
				configModels.KeyGuardianParentInviteMode: configModels.ParentInviteModeDisabled,
			},
		},
		Broadcaster:            testpkg.NewRecordingBroadcaster(),
		ArrivalSchedules:       sf.ArrivalSchedule,
		PickupSchedules:        sf.PickupSchedule,
		CareRequests:           sf.CareRequests,
		CareRequestRepo:        repos.CareScheduleChangeRequest,
		FamilyProtectionEvents: repos.FamilyProtection,
		ParentRequestShares:    repos.ParentRequestShare,
		StatusDayRepo:          repos.StudentStatusDay,
		MessageThreadRepo:      repos.ParentMessageThread,
		MessageRepo:            repos.ParentMessage,
		MessageReadRepo:        repos.ParentMessageRead,
		DB:                     db,
		Logger:                 slog.Default(),
	})
}

func carePayload() map[string]any {
	return map[string]any{"weekdays": []any{map[string]any{"weekday": 1, "pickup": "15:30"}}}
}

// TestCreateCareScheduleRequest_Happy persists a pending request and returns it
// on the refreshed Stammdaten view.
func TestCreateCareScheduleRequest_Happy(t *testing.T) {
	t.Parallel()

	svc, db, _ := buildCareScheduleService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	view, err := svc.CreateCareScheduleRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, carePayload())
	require.NoError(t, err)
	require.NotNil(t, view.PendingRequest, "the created request surfaces on the read view")
	assert.True(t, view.PendingRequest.SubmittedBySelf)
	require.NotEmpty(t, view.PendingRequest.Diff, "the pending card carries a current→requested diff")
}

func TestGetChildCareScheduleKeepsCareDayWithoutArrivalTime(t *testing.T) {
	t.Parallel()

	svc, db, _ := buildCareScheduleService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	staff := testpkg.CreateTestStaffForTenant(t, db, chain.TenantID, "Plan", "OhneZeit")
	testpkg.CreateTestArrivalSchedule(t, db, chain.StudentID, 1, staff.ID, "")

	view, err := svc.GetChildCareSchedule(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.Len(t, view.Weekdays, 5)
	assert.Equal(t, "scheduled", string(view.Weekdays[0].Status))
	assert.Empty(t, view.Weekdays[0].Arrival)
}

// TestCreateCareScheduleRequest_RequiresRequestSubmit is the core security
// separation: a change request overwrites the child's care schedule once staff
// approve, so it requires parent_portal.request.submit — NOT the
// parent_portal.notes.write that plain chat needs.
func TestCreateCareScheduleRequest_RequiresRequestSubmit(t *testing.T) {
	t.Parallel()

	svc, db, _ := buildCareScheduleService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	// Grant chat (notes.write) + visibility but explicitly NOT request.submit.
	_, err := db.ExecContext(testpkg.WithPackageTenantRuntime(context.Background()), `
		UPDATE users.students_guardians
		SET permissions = '{"parent_portal.access": true, "parent_portal.notes.write": true}'::jsonb
		WHERE tenant_id = ? AND student_id = ? AND guardian_profile_id = ?
	`, chain.TenantID, chain.StudentID, chain.GuardianProfileID)
	require.NoError(t, err)

	_, err = svc.CreateCareScheduleRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, carePayload())
	require.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied,
		"submitting a care-schedule change request requires request.submit, not just notes.write")
}

// TestCreateCareScheduleRequest_MessagingDisabledStillAllowed pins that messages
// and permanent-data requests are independent capabilities.
func TestCreateCareScheduleRequest_MessagingDisabledStillAllowed(t *testing.T) {
	t.Parallel()

	svc, db, _ := buildCareScheduleService(t, false) // messaging OFF
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	view, err := svc.CreateCareScheduleRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, carePayload())
	require.NoError(t, err)
	require.NotNil(t, view.PendingRequest)
}

func TestCreateCareScheduleRequest_RejectsArrivalWhenLegacySettingIsEnabled(t *testing.T) {
	t.Parallel()

	_, db, repos := buildCareScheduleService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	svc := careScheduleServiceWithSettings(t, db, repos, map[string]bool{
		configModels.KeyParentCareArrivalRequestEnabled: true,
		configModels.KeyParentCarePickupRequestEnabled:  true,
		configModels.KeyParentCareModeRequestEnabled:    true,
	})
	view, err := svc.GetChildCareSchedule(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.False(t, view.RequestCapabilities.Arrival)
	assert.True(t, view.RequestCapabilities.Pickup)
	assert.True(t, view.RequestCapabilities.DepartureMode)

	payload := map[string]any{"weekdays": []any{map[string]any{
		"weekday": 1, "arrival": "08:00", "pickup": "16:00",
	}}}
	_, err = svc.CreateCareScheduleRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, payload)
	require.ErrorIs(t, err, parentService.ErrCareRequestFieldDisabled)

	var count int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM schedule.care_schedule_change_requests
		WHERE tenant_id = ? AND student_id = ?
	`, chain.TenantID, chain.StudentID).Scan(testpkg.WithPackageTenantRuntime(context.Background()), &count))
	assert.Zero(t, count, "an arrival change must be rejected atomically")
}

func TestGetAndCreateCareScheduleRequest_AllFieldsDisabled(t *testing.T) {
	t.Parallel()

	_, db, repos := buildCareScheduleService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	svc := careScheduleServiceWithSettings(t, db, repos, map[string]bool{})
	view, err := svc.GetChildCareSchedule(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.False(t, view.CanRequest)
	assert.Equal(t, parentService.CareScheduleRequestCapabilities{}, view.RequestCapabilities)

	_, err = svc.CreateCareScheduleRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, carePayload())
	require.ErrorIs(t, err, parentService.ErrCareRequestFieldDisabled)
}

// TestWithdrawCareScheduleRequest_WorksWhenDisabled is the documented contract
// that withdraw (unlike create) deliberately skips the enabled-check: a parent
// must be able to wind down an outstanding request even after the school turns
// messaging OFF, instead of leaving it frozen open forever.
func TestWithdrawCareScheduleRequest_WorksWhenDisabled(t *testing.T) {
	t.Parallel()

	svc, db, repos := buildCareScheduleService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	created, err := svc.CreateCareScheduleRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, carePayload())
	require.NoError(t, err)
	reqID := created.PendingRequest.ID

	// Rebuild against the SAME db/repos with every permanent-care field disabled.
	disabled := careScheduleServiceWithSettings(t, db, repos, map[string]bool{})

	view, err := disabled.WithdrawCareScheduleRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, reqID)
	require.NoError(t, err, "withdraw must stay available after request fields are disabled")
	assert.Nil(t, view.PendingRequest, "the withdrawn request no longer appears on the read view")
}

// TestWithdrawCareScheduleRequest_WorksAfterSubmitRevoked is the review-finding
// regression: withdraw is gated ONLY on parent_portal.access, not
// parent_portal.request.submit. If the school revokes request.submit AFTER a
// guardian filed a request, the read view still exposes that request as
// submitted_by_self and renders a withdraw button, so the owning guardian must
// still be able to withdraw it (ownership is enforced inside WithdrawRequest).
// Gating withdraw on request.submit would strand the request behind an
// always-403 button.
func TestWithdrawCareScheduleRequest_WorksAfterSubmitRevoked(t *testing.T) {
	t.Parallel()

	svc, db, _ := buildCareScheduleService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	created, err := svc.CreateCareScheduleRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, carePayload())
	require.NoError(t, err)
	reqID := created.PendingRequest.ID

	// Revoke request.submit but keep parent_portal.access (mirrors a school
	// tightening permissions while a request is already open).
	_, err = db.ExecContext(testpkg.WithPackageTenantRuntime(context.Background()), `
		UPDATE users.students_guardians
		SET permissions = '{"parent_portal.access": true}'::jsonb
		WHERE tenant_id = ? AND student_id = ? AND guardian_profile_id = ?
	`, chain.TenantID, chain.StudentID, chain.GuardianProfileID)
	require.NoError(t, err)

	// The read view still surfaces the request as the caller's own (withdraw
	// button visible) even though CanRequest has dropped.
	read, err := svc.GetChildCareSchedule(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.NotNil(t, read.PendingRequest)
	assert.True(t, read.PendingRequest.SubmittedBySelf, "the open request is still shown as the caller's own")
	assert.False(t, read.CanRequest, "request.submit was revoked, so a NEW request is no longer offered")

	// Withdraw must succeed on portal access + ownership despite the missing
	// request.submit permission.
	view, err := svc.WithdrawCareScheduleRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, reqID)
	require.NoError(t, err, "the owning guardian withdraws on portal access + ownership, not request.submit")
	assert.Nil(t, view.PendingRequest, "the withdrawn request no longer appears on the read view")
}

// TestGetChildCareSchedule_ReadViewReflectsPendingRequest drives the parent
// read-view method (GetChildCareSchedule): it needs only parent_portal.access,
// so it stays available regardless of the request feature gates, and it
// surfaces an open request on the view once one exists.
func TestGetChildCareSchedule_ReadViewReflectsPendingRequest(t *testing.T) {
	t.Parallel()

	svc, db, _ := buildCareScheduleService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	// Before any request: the view loads and, with messaging on + request.submit,
	// invites the guardian to request a change.
	view, err := svc.GetChildCareSchedule(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.NotNil(t, view)
	assert.Nil(t, view.PendingRequest, "no request has been filed yet")
	assert.True(t, view.CanRequest, "messaging on + request.submit enables the request action")

	// After filing a request, the read view surfaces it with its diff.
	_, err = svc.CreateCareScheduleRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, carePayload())
	require.NoError(t, err)

	view, err = svc.GetChildCareSchedule(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	require.NotNil(t, view.PendingRequest, "the open request appears on the read view")
	assert.True(t, view.PendingRequest.SubmittedBySelf)
}

// TestGetChildCareSchedule_RequiresAccess proves the read view is gated on
// parent_portal.access: a guardian link without it cannot read the schedule.
func TestGetChildCareSchedule_RequiresAccess(t *testing.T) {
	t.Parallel()

	svc, db, _ := buildCareScheduleService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	// Strip every parent_portal permission, including access.
	_, err := db.ExecContext(testpkg.WithPackageTenantRuntime(context.Background()), `
		UPDATE users.students_guardians
		SET permissions = '{}'::jsonb
		WHERE tenant_id = ? AND student_id = ? AND guardian_profile_id = ?
	`, chain.TenantID, chain.StudentID, chain.GuardianProfileID)
	require.NoError(t, err)

	_, err = svc.GetChildCareSchedule(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.Error(t, err, "reading the care schedule requires parent_portal.access")
}

// TestGetChildCareSchedule_TodayAbsentReflectsStatusDay proves the parent-safe
// absence signal on the read view catches a staff-created class-trip day for
// today — exactly the row ListSickDays deliberately hides. Without it the
// "Heute → Abholung" tile would show a pickup time for a child the school has
// recorded as off.
func TestGetChildCareSchedule_TodayAbsentReflectsStatusDay(t *testing.T) {
	t.Parallel()

	svc, db, _ := buildCareScheduleService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	ctx := testpkg.WithPackageTenantRuntime(context.Background())

	// No status day yet: the tile has no absence to report.
	view, err := svc.GetChildCareSchedule(ctx, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.False(t, view.TodayAbsent, "no status day → not absent")

	// Staff record a class trip for today (source=planned) — the kind of row
	// ListSickDays hides from guardians.
	_, err = db.ExecContext(ctx, `
		INSERT INTO active.student_status_days
			(tenant_id, student_id, date, status, reported_at, source)
		VALUES (?, ?, ?, ?, now(), ?)
	`, chain.TenantID, chain.StudentID, timezone.TodayDate(),
		activeModels.StudentStatusDayClassTrip, activeModels.StudentStatusSourcePlanned)
	require.NoError(t, err)
	defer func() {
		_, _ = db.ExecContext(testpkg.WithPackageTenantRuntime(context.Background()),
			`DELETE FROM active.student_status_days WHERE student_id = ?`, chain.StudentID)
	}()

	// The read view now reports the absence...
	view, err = svc.GetChildCareSchedule(ctx, chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.True(t, view.TodayAbsent, "an active class-trip day today makes the child absent")

	// ...even though ListSickDays still hides the staff-created class-trip row,
	// which is exactly why the tile needs a separate parent-safe signal.
	days, err := svc.ListSickDays(ctx, chain.AccountID, chain.StudentID,
		timezone.TodayDate(), timezone.TodayDate())
	require.NoError(t, err)
	assert.Empty(t, days, "class-trip days stay hidden from the parent sick-day list")
}

// TestChildFeatures_ReflectsPermissionsAndOpenRequest drives the parent
// overview feature-flag resolver (ChildFeatures) and, through it,
// hasOpenChangeRequest. With messaging on and the default guardian permissions,
// the request/notes features are enabled; and once a care-schedule request is
// pending, HasOpenChangeRequest badges the Stammdaten entry.
func TestChildFeatures_ReflectsPermissionsAndOpenRequest(t *testing.T) {
	t.Parallel()

	svc, db, _ := buildCareScheduleService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	// No request yet: the badge is clear, and request/notes features resolve
	// enabled from the default guardian permissions + messaging on.
	flags, err := svc.ChildFeatures(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.False(t, flags.HasOpenChangeRequest, "no pending request → no badge")
	assert.True(t, flags.RequestSubmitEnabled, "default guardian holds request.submit with messaging on")
	assert.True(t, flags.NotesEnabled, "default guardian holds notes.write with messaging on")

	// File a care-schedule request; the open-request badge now lights up.
	_, err = svc.CreateCareScheduleRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, carePayload())
	require.NoError(t, err)

	flags, err = svc.ChildFeatures(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID)
	require.NoError(t, err)
	assert.True(t, flags.HasOpenChangeRequest, "a pending care request badges the Stammdaten entry")
}

func TestChildFeatures_HidesAnotherGuardiansRequestUntilNamedShare(t *testing.T) {
	t.Parallel()

	svc, db, _ := buildCareScheduleService(t, true)
	author := testpkg.CreateTestParentGuardianChain(t, db)
	recipient := testpkg.CreateTestParentGuardianChain(t, db)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	_, err := db.ExecContext(ctx, `
		UPDATE users.students_guardians SET student_id = ?
		WHERE guardian_profile_id = ?
	`, author.StudentID, recipient.GuardianProfileID)
	require.NoError(t, err)

	created, err := svc.CreateCareScheduleRequest(ctx, author.AccountID, author.StudentID, carePayload())
	require.NoError(t, err)
	require.NotNil(t, created.PendingRequest)
	flags, err := svc.ChildFeatures(ctx, recipient.AccountID, author.StudentID)
	require.NoError(t, err)
	assert.False(t, flags.HasOpenChangeRequest, "another guardian must not learn that a private request exists")

	sharing := svc.(parentService.RequestSharingService)
	_, err = sharing.SetRequestSharing(
		ctx, author.AccountID, author.StudentID, parentService.RequestShareCareSchedule,
		created.PendingRequest.ID, []int64{recipient.GuardianProfileID},
	)
	require.NoError(t, err)
	flags, err = svc.ChildFeatures(ctx, recipient.AccountID, author.StudentID)
	require.NoError(t, err)
	assert.True(t, flags.HasOpenChangeRequest, "a named recipient may see the shared open request")
}

// TestWithdrawCareScheduleRequest_NotFound covers the parent-side error mapping:
// withdrawing a request id the guardian does not own (or that does not exist)
// surfaces as the parent not-found sentinel, not a raw 500 — the id space must
// not be probeable from the parents portal.
func TestWithdrawCareScheduleRequest_NotFound(t *testing.T) {
	t.Parallel()

	svc, db, _ := buildCareScheduleService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	created, err := svc.CreateCareScheduleRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, carePayload())
	require.NoError(t, err)
	bogusID := created.PendingRequest.ID + 1_000_000

	_, err = svc.WithdrawCareScheduleRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, bogusID)
	require.ErrorIs(t, err, parentService.ErrCareRequestNotFound,
		"withdrawing an unknown request id maps to the parent not-found sentinel")
}
