package scheduler

import (
	"context"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type fakeOperatorInvitationCleaner struct{}

func (*fakeOperatorInvitationCleaner) CleanupExpiredOperatorInvitations(context.Context) (int, error) {
	return 0, nil
}

func minimalWorkerDependencies(t *testing.T) WorkerDependencies {
	t.Helper()
	runtime, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error {
			return fn(ctx, struct{}{})
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
	)
	require.NoError(t, err)
	return WorkerDependencies{
		Logger:                    slog.Default(),
		DB:                        new(bun.DB),
		SchoolRepo:                &testpkg.SchoolRepoMock{},
		TenantRuntime:             &runtime,
		Settings:                  &stubSettingsResolver{},
		AuthCleanup:               &fakeAuthCleanup{},
		InvitationCleanup:         &fakeInvitationCleaner{},
		EmailChangeCleanup:        &fakeEmailChangeCleaner{},
		OperatorInvitationCleanup: &fakeOperatorInvitationCleaner{},
	}
}

func TestNewWorkerRejectsMissingRuntimeDependencies(t *testing.T) {
	t.Parallel()

	_, err := NewWorker(WorkerDependencies{})

	require.ErrorContains(t, err, "logger")
}

func TestNewWorkerRejectsMissingJobs(t *testing.T) {
	t.Parallel()

	_, err := NewWorker(minimalWorkerDependencies(t))

	require.ErrorContains(t, err, "missing worker jobs")
}

func TestNewWorkerRejectsMissingTokenCleanupDependencies(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		remove func(*WorkerDependencies)
	}{
		{name: "auth cleanup", remove: func(deps *WorkerDependencies) { deps.AuthCleanup = nil }},
		{name: "invitation cleanup", remove: func(deps *WorkerDependencies) { deps.InvitationCleanup = nil }},
		{name: "email change cleanup", remove: func(deps *WorkerDependencies) { deps.EmailChangeCleanup = nil }},
		{name: "operator invitation cleanup", remove: func(deps *WorkerDependencies) { deps.OperatorInvitationCleanup = nil }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			deps := minimalWorkerDependencies(t)
			tt.remove(&deps)

			_, err := NewWorker(deps)

			require.ErrorContains(t, err, tt.name)
		})
	}
}

func TestNewWorkerRejectsTypedNilDependencies(t *testing.T) {
	t.Parallel()

	t.Run("core dependency", func(t *testing.T) {
		t.Parallel()
		deps := minimalWorkerDependencies(t)
		var schoolRepo *testpkg.SchoolRepoMock
		deps.SchoolRepo = schoolRepo

		_, err := NewWorker(deps)

		require.ErrorContains(t, err, "school repository")
	})

	t.Run("job dependency", func(t *testing.T) {
		t.Parallel()
		deps := minimalWorkerDependencies(t)
		var cleanup *mockCleanupService
		deps.ActiveCleanup = cleanup

		_, err := NewWorker(deps)

		require.ErrorContains(t, err, "visit-cleanup")
	})
}
