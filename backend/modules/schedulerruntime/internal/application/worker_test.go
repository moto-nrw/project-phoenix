package application

import (
	"context"
	"errors"
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

type fakeHolidayRegistrationRetention struct{}

func (fakeHolidayRegistrationRetention) DeleteExpiredHolidayRegistrations(context.Context) (int, error) {
	return 0, nil
}

type fakeAccountRetention struct{}

func (fakeAccountRetention) EraseUnusedAccounts(context.Context) (int, error) { return 0, nil }

func (fakeAccountRetention) CleanupErasedAccountAvatars(context.Context) (int, error) { return 0, nil }

type fakeFeedbackCleaner struct{ err error }

func (f *fakeFeedbackCleaner) DeleteExpired(context.Context) (int, error) { return 0, f.err }

func minimalWorkerDependencies(t *testing.T) ports.WorkerDependencies {
	t.Helper()
	return ports.WorkerDependencies{
		SettleExpiredTransfers:       noExpiredTransfers,
		Logger:                       slog.Default(),
		SchoolRepo:                   dbTenantDirectory{},
		TenantRuntime:                unitTenantRuntime(),
		Settings:                     &stubSettingsResolver{},
		AuthCleanup:                  &fakeAuthCleanup{},
		InvitationCleanup:            &fakeInvitationCleaner{},
		EmailChangeCleanup:           &fakeEmailChangeCleaner{},
		OperatorInvitationCleanup:    &fakeOperatorInvitationCleaner{},
		OperatorRetention:            ports.OperatorRetention{Ledger: fakeOperatorRetention{}, Accounts: fakeOperatorRetention{}},
		AccountRetention:             fakeAccountRetention{},
		HolidayRegistrationRetention: fakeHolidayRegistrationRetention{},
		FeedbackCleaner:              &fakeFeedbackCleaner{},
		CleanupRuns:                  newFakeCleanupRuns(),
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
		{name: "holiday registration retention", remove: func(deps *ports.WorkerDependencies) { deps.HolidayRegistrationRetention = nil }},
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
	require.Contains(t, descriptions, "Erased account avatar cleanup")
}

type cleanupTransactionKey struct{}

type markingCleanupRuntime struct{ ports.TenantRuntime }

func (r markingCleanupRuntime) WithinAdmin(ctx context.Context, fn func(context.Context) error) error {
	return fn(context.WithValue(ctx, cleanupTransactionKey{}, true))
}

type transactionalAccountRetention struct {
	t       *testing.T
	err     error
	cleaned bool
}

func (r *transactionalAccountRetention) EraseUnusedAccounts(ctx context.Context) (int, error) {
	r.t.Helper()
	require.Equal(r.t, true, ctx.Value(cleanupTransactionKey{}), "anonymization uses the scheduler transaction")
	return 0, nil
}

func (r *transactionalAccountRetention) CleanupErasedAccountAvatars(ctx context.Context) (int, error) {
	r.t.Helper()
	require.Nil(r.t, ctx.Value(cleanupTransactionKey{}), "file cleanup must own its commit so a partial error cannot roll it back")
	r.cleaned = true
	return 1, r.err
}

func TestAccountAvatarJobOwnsItsTransactionAndReportsPartialFailure(t *testing.T) {
	t.Parallel()
	deps := minimalWorkerDependencies(t)
	deps.TenantRuntime = markingCleanupRuntime{unitTenantRuntime()}
	partial := errors.New("one file remains for retry")
	retention := &transactionalAccountRetention{t: t, err: partial}
	deps.AccountRetention = retention
	err := newScheduler(deps).runCleanupJobs(t.Context())
	require.ErrorIs(t, err, partial)
	require.True(t, retention.cleaned)
}

func TestNewWorkerRejectsMissingSettings(t *testing.T) {
	t.Parallel()
	deps := minimalWorkerDependencies(t)
	deps.Settings = nil
	_, err := NewWorker(deps)
	require.ErrorContains(t, err, "worker dependency settings is required")
	var typedNil *stubSettingsResolver
	deps.Settings = typedNil
	_, err = NewWorker(deps)
	require.ErrorContains(t, err, "worker dependency settings is required")
}
