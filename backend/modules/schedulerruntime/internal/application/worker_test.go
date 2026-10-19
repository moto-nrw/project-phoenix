package application

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"

	"github.com/stretchr/testify/require"
)

type fakeOperatorInvitationCleaner struct{ fakeSignInDataCleaner }

func (*fakeOperatorInvitationCleaner) DeleteExpiredOperatorInvitations(context.Context) (int, error) {
	return 0, nil
}

type fakeOperatorRetention struct{}

func (fakeOperatorRetention) DeleteExpiredOperatorAuditLog(context.Context) (int, error) {
	return 0, nil
}
func (fakeOperatorRetention) DeleteDeactivatedOperators(context.Context) (int, error) { return 0, nil }

type fakeAccountRetention struct{}

func (fakeAccountRetention) EraseUnusedAccounts(context.Context) (int, error) { return 0, nil }

type fakeFeedbackCleaner struct{ err error }

func (f *fakeFeedbackCleaner) DeleteExpired(context.Context) (int, error) { return 0, f.err }

func minimalWorkerDependencies(t *testing.T) ports.WorkerDependencies {
	t.Helper()
	return ports.WorkerDependencies{
		SettleExpiredTransfers:    noExpiredTransfers,
		Logger:                    slog.Default(),
		SchoolRepo:                dbTenantDirectory{},
		TenantRuntime:             unitTenantRuntime(),
		Settings:                  &stubSettingsResolver{},
		AuthCleanup:               &fakeAuthCleanup{},
		InvitationCleanup:         &fakeInvitationCleaner{},
		EmailChangeCleanup:        &fakeEmailChangeCleaner{},
		OperatorInvitationCleanup: &fakeOperatorInvitationCleaner{},
		OperatorRetention:         ports.OperatorRetention{Ledger: fakeOperatorRetention{}, Accounts: fakeOperatorRetention{}},
		AccountRetention:          fakeAccountRetention{},
		FeedbackCleaner:           &fakeFeedbackCleaner{},
		CleanupRuns:               newFakeCleanupRuns(),
		Lease: ports.WorkerLease{
			Store: newMemoryLeaseStore(), Name: "worker", Holder: "test-worker",
			TTL: 30 * time.Second, RenewEvery: 10 * time.Second, RetryEvery: 5 * time.Second, FenceMargin: 5 * time.Second,
		},
	}
}

func TestNewWorkerRejectsMissingRuntimeDependencies(t *testing.T) {
	t.Parallel()

	_, err := NewWorker(ports.WorkerDependencies{})

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
		remove func(*ports.WorkerDependencies)
	}{
		{name: "auth cleanup", remove: func(deps *ports.WorkerDependencies) { deps.AuthCleanup = nil }},
		{name: "invitation cleanup", remove: func(deps *ports.WorkerDependencies) { deps.InvitationCleanup = nil }},
		{name: "email change cleanup", remove: func(deps *ports.WorkerDependencies) { deps.EmailChangeCleanup = nil }},
		{name: "operator invitation cleanup", remove: func(deps *ports.WorkerDependencies) { deps.OperatorInvitationCleanup = nil }},
		{name: "account retention", remove: func(deps *ports.WorkerDependencies) { deps.AccountRetention = nil }},
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

// The hourly cleanup erases the accounts no school needs (#4043).
func TestCleanupJobsEraseUnusedAccounts(t *testing.T) {
	t.Parallel()

	jobs := newScheduler(minimalWorkerDependencies(t)).cleanupJobs

	require.NotEmpty(t, jobs)
	descriptions := make([]string, 0, len(jobs))
	for _, job := range jobs {
		descriptions = append(descriptions, job.Description)
	}
	require.Contains(t, descriptions, "Unused account erasure")
}
