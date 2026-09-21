package care_test

// Integration tests for the parent-side care-schedule request paths
// (parent_care_schedule_service.go): CreateCareScheduleRequest and
// —These port the security-relevant scenarios that
// previously lived on the chat request path (CreateChildRequest /
// WithdrawChildRequest) onto the decoupled Stammdaten request flow (#1803):
//   - submitting a change request requires parent_portal.request.submit — NOT
//     the parent_portal.notes.write that plain chat needs,
//   - messaging and permanent-care request rights are independent,
//   - disabled field groups reject the whole manipulated request,
//   - withdraw stays available after a school disables request fields.
//
// Uses the package-local parentSettingsStub (settings_stub_test.go).

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/services"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/care"
)

// buildCareScheduleService wires the parent service with the full care-schedule
// request stack (arrival/pickup read services + the schedule-domain request
// service), so the Stammdaten read view and request lifecycle run for real.
func buildCareScheduleService(t *testing.T, notesEnabled bool) (*care.Service, *bun.DB, *repositories.Factory) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))
	return careScheduleServiceOn(t, db, repos, notesEnabled), db, repos
}

// careScheduleServiceOn wires the same parent service against an EXISTING
// db/repos, so a test can rebuild the service with a different messaging toggle
// while keeping the seeded guardian chain and persisted requests. Allocating a
// fresh SetupTestDB instead would point the rebuilt service at an empty schema.
func careScheduleServiceOn(t *testing.T, db *bun.DB, repos *repositories.Factory, notesEnabled bool) *care.Service {
	t.Helper()
	return careScheduleServiceWithSettings(t, db, repos, map[string]bool{
		configModels.KeyParentSickNoteEnabled:           true,
		configModels.KeyParentNotesEnabled:              notesEnabled,
		configModels.KeyParentCareArrivalRequestEnabled: true,
		configModels.KeyParentCarePickupRequestEnabled:  true,
		configModels.KeyParentCareModeRequestEnabled:    true,
	})
}

func careScheduleServiceWithSettings(t *testing.T, db *bun.DB, repos *repositories.Factory, boolValues map[string]bool, requestService ...*carerequests.Service) *care.Service {
	t.Helper()
	sf, err := services.NewFactoryForTests(repos, db, slog.Default(), func() time.Time {
		return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	})
	require.NoError(t, err)
	if len(requestService) > 0 {
		*requestService[0] = sf.CareRequests
	}
	return care.New(care.Config{
		RequestSharing: unconfiguredRequestSharer{},
		ChildRepo:      repos.ParentChild,
		StudentRepo:    repos.Student,
		Settings: parentSettingsStub{
			boolValues: boolValues,
			stringValues: map[string]string{
				configModels.KeyGuardianParentInviteMode: configModels.ParentInviteModeDisabled,
			},
		},
		ArrivalSchedules: sf.ArrivalSchedule,
		PickupSchedules:  sf.PickupSchedule,
		CareRequests:     sf.CareRequests,
		StatusDayRepo:    repos.StudentStatusDay,
		DB:               db,
		Logger:           slog.Default(),
		Now: func() time.Time {
			return time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
		},
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
	require.ErrorIs(t, err, care.ErrGuardianPermissionDenied,
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
	require.ErrorIs(t, err, care.ErrCareRequestFieldDisabled)

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
	assert.Equal(t, care.CareScheduleRequestCapabilities{}, view.RequestCapabilities)

	_, err = svc.CreateCareScheduleRequest(testpkg.WithPackageTenantRuntime(context.Background()), chain.AccountID, chain.StudentID, carePayload())
	require.ErrorIs(t, err, care.ErrCareRequestFieldDisabled)
}

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
