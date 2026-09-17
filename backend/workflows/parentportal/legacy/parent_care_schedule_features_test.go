package legacy_test

// Parent-service scenarios that combine the relocated care-schedule request
// path (workflows/parentportal/care, #3227) with the overview feature
// flags, the sick-day list and request sharing, which stay in this package.

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/communication/communicationtest"
	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal/legacy"
)

// buildCareScheduleService wires the full parent service with the care-schedule
// request stack and messaging on, so the feature flags see real requests.
func buildCareScheduleService(t *testing.T) (parentService.Service, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	sf, err := services.NewFactoryForTests(repos, db, slog.Default(), func() time.Time {
		return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	})
	require.NoError(t, err)
	return parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StudentRepo:         repos.Student,
		StudentGuardianRepo: repos.StudentGuardian,
		GuardianProfileRepo: repos.GuardianProfile,
		PersonRepo:          repos.Person,
		Settings: parentSettingsStub{
			boolValues: map[string]bool{
				configModels.KeyParentSickNoteEnabled:           true,
				configModels.KeyParentNotesEnabled:              true,
				configModels.KeyParentCareArrivalRequestEnabled: true,
				configModels.KeyParentCarePickupRequestEnabled:  true,
				configModels.KeyParentCareModeRequestEnabled:    true,
			},
			stringValues: map[string]string{
				configModels.KeyGuardianParentInviteMode: configModels.ParentInviteModeDisabled,
			},
		},
		MealPlan:               availableMealPlan(false),
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
		Conversations:          communicationtest.NewParentConversationCore(repos.ParentMessageThread, repos.ParentMessage, repos.ParentMessageRead, testpkg.NewRecordingBroadcaster(), slog.Default()),
		DB:                     db,
		Logger:                 slog.Default(),
		Now: func() time.Time {
			return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
		},
	}), db
}

func carePayload() map[string]any {
	return map[string]any{"weekdays": []any{map[string]any{"weekday": 1, "pickup": "15:30"}}}
}

// TestGetChildCareSchedule_TodayAbsentReflectsStatusDay proves the parent-safe
// absence signal on the read view catches a staff-created class-trip day for
// today — exactly the row ListSickDays deliberately hides. Without it the
// "Heute → Abholung" tile would show a pickup time for a child the school has
// recorded as off.
func TestGetChildCareSchedule_TodayAbsentReflectsStatusDay(t *testing.T) {
	t.Parallel()

	svc, db := buildCareScheduleService(t)
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
	`, chain.TenantID, chain.StudentID, timezone.NewDate(2026, 8, 24),
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
		timezone.NewDate(2026, 8, 24), timezone.NewDate(2026, 8, 24))
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

	svc, db := buildCareScheduleService(t)
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

	svc, db := buildCareScheduleService(t)
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
