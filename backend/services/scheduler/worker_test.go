package scheduler

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type fakeOperatorInvitationCleaner struct{}

func (*fakeOperatorInvitationCleaner) DeleteExpiredOperatorInvitations(context.Context) (int, error) {
	return 0, nil
}

type fakeFeedbackCleaner struct{ err error }

func (f *fakeFeedbackCleaner) DeleteExpired(context.Context) (int, error) { return 0, f.err }

func minimalWorkerDependencies(t *testing.T) WorkerDependencies {
	t.Helper()
	return WorkerDependencies{
		Logger:                    slog.Default(),
		SchoolRepo:                dbTenantDirectory{},
		TenantRuntime:             unitTenantRuntime(),
		Settings:                  &stubSettingsResolver{},
		AuthCleanup:               &fakeAuthCleanup{},
		InvitationCleanup:         &fakeInvitationCleaner{},
		EmailChangeCleanup:        &fakeEmailChangeCleaner{},
		OperatorInvitationCleanup: &fakeOperatorInvitationCleaner{},
		FeedbackCleaner:           &fakeFeedbackCleaner{},
		Lease: WorkerLease{
			Store: newMemoryLeaseStore(), Name: "worker", Holder: "test-worker",
			TTL: 30 * time.Second, RenewEvery: 10 * time.Second, RetryEvery: 5 * time.Second, FenceMargin: 5 * time.Second,
		},
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
		var schoolRepo *erroringSchoolRepo
		deps.SchoolRepo = schoolRepo

		_, err := NewWorker(deps)

		require.ErrorContains(t, err, "tenant directory")
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
