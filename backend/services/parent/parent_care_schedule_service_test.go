package parent_test

// Integration tests for the parent-side care-schedule request paths
// (parent_care_schedule_service.go): CreateCareScheduleRequest and
// WithdrawCareScheduleRequest. These port the security-relevant scenarios that
// previously lived on the chat request path (CreateChildRequest /
// WithdrawChildRequest) onto the decoupled Stammdaten request flow (#1803):
//   - submitting a change request requires parent_portal.request.submit — NOT
//     the parent_portal.notes.write that plain chat needs,
//   - creating needs messaging enabled, but withdraw stays available after the
//     school disables it so outstanding requests can be wound down.
//
// Reuses stubSettings / captureBroadcaster from parent_write_service_test.go.

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
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
	t.Cleanup(func() { _ = db.Close() })
	repos := repositories.NewFactory(db)
	sf, err := services.NewFactory(repos, db, slog.Default())
	require.NoError(t, err)
	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo:           repos.ParentChild,
		StudentRepo:         repos.Student,
		GuardianProfileRepo: repos.GuardianProfile,
		PersonRepo:          repos.Person,
		Settings:            stubSettings{sickEnabled: true, notesEnabled: notesEnabled},
		Broadcaster:         &captureBroadcaster{},
		ArrivalSchedules:    sf.ArrivalSchedule,
		PickupSchedules:     sf.PickupSchedule,
		CareRequests:        sf.CareRequests,
		MessageThreadRepo:   repos.ParentMessageThread,
		MessageRepo:         repos.ParentMessage,
		MessageReadRepo:     repos.ParentMessageRead,
		DB:                  db,
		Logger:              slog.Default(),
	})
	return svc, db, repos
}

func carePayload() map[string]any {
	return map[string]any{"weekdays": []any{map[string]any{"weekday": 1, "arrival": "08:00"}}}
}

// TestCreateCareScheduleRequest_Happy persists a pending request and returns it
// on the refreshed Stammdaten view.
func TestCreateCareScheduleRequest_Happy(t *testing.T) {
	svc, db, _ := buildCareScheduleService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	view, err := svc.CreateCareScheduleRequest(context.Background(), chain.AccountID, chain.StudentID, carePayload())
	require.NoError(t, err)
	require.NotNil(t, view.PendingRequest, "the created request surfaces on the read view")
	assert.True(t, view.PendingRequest.SubmittedBySelf)
	require.NotEmpty(t, view.PendingRequest.Diff, "the pending card carries a current→requested diff")
}

// TestCreateCareScheduleRequest_RequiresRequestSubmit is the core security
// separation: a change request overwrites the child's care schedule once staff
// approve, so it requires parent_portal.request.submit — NOT the
// parent_portal.notes.write that plain chat needs.
func TestCreateCareScheduleRequest_RequiresRequestSubmit(t *testing.T) {
	svc, db, _ := buildCareScheduleService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	// Grant chat (notes.write) + visibility but explicitly NOT request.submit.
	_, err := db.ExecContext(context.Background(), `
		UPDATE users.students_guardians
		SET permissions = '{"parent_portal.access": true, "parent_portal.notes.write": true}'::jsonb
		WHERE tenant_id = ? AND student_id = ? AND guardian_profile_id = ?
	`, chain.TenantID, chain.StudentID, chain.GuardianProfileID)
	require.NoError(t, err)

	_, err = svc.CreateCareScheduleRequest(context.Background(), chain.AccountID, chain.StudentID, carePayload())
	require.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied,
		"submitting a care-schedule change request requires request.submit, not just notes.write")
}

// TestCreateCareScheduleRequest_DisabledRefused: creating a request needs
// messaging enabled (the chat pill is its feedback channel), so with the feature
// off it is ErrNotesDisabled.
func TestCreateCareScheduleRequest_DisabledRefused(t *testing.T) {
	svc, db, _ := buildCareScheduleService(t, false) // messaging OFF
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	_, err := svc.CreateCareScheduleRequest(context.Background(), chain.AccountID, chain.StudentID, carePayload())
	require.ErrorIs(t, err, parentService.ErrNotesDisabled)
}

// TestWithdrawCareScheduleRequest_WorksWhenDisabled is the documented contract
// that withdraw (unlike create) deliberately skips the enabled-check: a parent
// must be able to wind down an outstanding request even after the school turns
// messaging OFF, instead of leaving it frozen open forever.
func TestWithdrawCareScheduleRequest_WorksWhenDisabled(t *testing.T) {
	svc, db, _ := buildCareScheduleService(t, true)
	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	created, err := svc.CreateCareScheduleRequest(context.Background(), chain.AccountID, chain.StudentID, carePayload())
	require.NoError(t, err)
	reqID := created.PendingRequest.ID

	// Rebuild the service with messaging OFF, same DB.
	disabled, _, _ := buildCareScheduleService(t, false)

	view, err := disabled.WithdrawCareScheduleRequest(context.Background(), chain.AccountID, chain.StudentID, reqID)
	require.NoError(t, err, "withdraw must stay available even with messaging disabled")
	assert.Nil(t, view.PendingRequest, "the withdrawn request no longer appears on the read view")
}
