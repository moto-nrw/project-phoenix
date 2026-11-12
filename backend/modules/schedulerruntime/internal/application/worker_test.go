package application

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/schedulerruntime/ports"

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

type fakeTraegerDocumentReadRetention struct{}

type fakeSEPADebitDownloadRetention struct{}

func (fakeSEPADebitDownloadRetention) DeleteExpiredSEPADebitDownloads(context.Context) (int, error) {
	return 0, nil
}

func (fakeTraegerDocumentReadRetention) DeleteExpiredTraegerDocumentReads(context.Context) (int, error) {
	return 0, nil
}

type fakeBelegRetention struct{}

func (fakeBelegRetention) ApplyBelegRetention(context.Context) (int, error) { return 0, nil }

type fakeAccountRetention struct{}

func (fakeAccountRetention) EraseUnusedAccounts(context.Context) (int, error) { return 0, nil }

func (fakeAccountRetention) CleanupErasedAccountAvatars(context.Context) (int, error) { return 0, nil }

func (fakeAccountRetention) DeleteUnusedCaterers(context.Context) (int, error) { return 0, nil }

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
		TraegerDocumentReadRetention: fakeTraegerDocumentReadRetention{},
		BelegRetention:               fakeBelegRetention{},
		SEPADebitDownloadRetention:   fakeSEPADebitDownloadRetention{},
		FeedbackCleaner:              &fakeFeedbackCleaner{},
		RetentionLog:                 func(context.Context, string, ports.RetentionOutcome) error { return nil },
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
		{name: "retention log", remove: func(deps *ports.WorkerDependencies) { deps.RetentionLog = nil }},
		{name: "auth cleanup", remove: func(deps *ports.WorkerDependencies) { deps.AuthCleanup = nil }},
		{name: "invitation cleanup", remove: func(deps *ports.WorkerDependencies) { deps.InvitationCleanup = nil }},
		{name: "email change cleanup", remove: func(deps *ports.WorkerDependencies) { deps.EmailChangeCleanup = nil }},
		{name: "operator invitation cleanup", remove: func(deps *ports.WorkerDependencies) { deps.OperatorInvitationCleanup = nil }},
		{name: "account retention", remove: func(deps *ports.WorkerDependencies) { deps.AccountRetention = nil }},
		{name: "holiday registration retention", remove: func(deps *ports.WorkerDependencies) { deps.HolidayRegistrationRetention = nil }},
		{name: "Träger document read retention", remove: func(deps *ports.WorkerDependencies) { deps.TraegerDocumentReadRetention = nil }},
		{name: "beleg retention", remove: func(deps *ports.WorkerDependencies) { deps.BelegRetention = nil }},
		{name: "SEPA debit download retention", remove: func(deps *ports.WorkerDependencies) { deps.SEPADebitDownloadRetention = nil }},
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
	t        *testing.T
	err      error
	erased   bool
	cleaned  bool
	caterers bool
}

func (r *transactionalAccountRetention) EraseUnusedAccounts(ctx context.Context) (int, error) {
	r.t.Helper()
	require.Equal(r.t, true, ctx.Value(cleanupTransactionKey{}), "anonymization uses the scheduler transaction")
	r.erased = true
	return 0, nil
}

func (r *transactionalAccountRetention) DeleteUnusedCaterers(ctx context.Context) (int, error) {
	r.t.Helper()
	require.Equal(r.t, true, ctx.Value(cleanupTransactionKey{}), "the caterer deletion uses the scheduler transaction")
	require.True(r.t, r.erased, "caterers go after the account erasure, which decides whether their accounts are still needed")
	r.caterers = true
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
	require.True(t, retention.caterers, "a file failure does not stop the caterer deletion")
}

// transactionalTraegerDocumentReadRetention records whether the hourly
// cleanup handed it the scheduler's administrative transaction.
type transactionalTraegerDocumentReadRetention struct{ inAdmin []bool }

func (r *transactionalTraegerDocumentReadRetention) DeleteExpiredTraegerDocumentReads(ctx context.Context) (int, error) {
	r.inAdmin = append(r.inAdmin, ctx.Value(cleanupTransactionKey{}) == true)
	return 0, nil
}

// The Träger document reads expire across every Träger, which only the
// administrative transaction may do (#4692): the job runs inside the
// scheduler's WithinAdmin, never with an own transaction.
func TestTraegerDocumentReadRetentionRunsInTheAdministrativeTransaction(t *testing.T) {
	t.Parallel()
	deps := minimalWorkerDependencies(t)
	deps.TenantRuntime = markingCleanupRuntime{unitTenantRuntime()}
	retention := &transactionalTraegerDocumentReadRetention{}
	deps.TraegerDocumentReadRetention = retention

	require.NoError(t, newScheduler(deps).runCleanupJobs(t.Context()))

	require.Equal(t, []bool{true}, retention.inAdmin, "the job runs once, inside the administrative transaction")
}

// transactionalSEPADebitDownloadRetention records whether the hourly
// cleanup handed it the scheduler's administrative transaction.
type transactionalSEPADebitDownloadRetention struct{ inAdmin []bool }

func (r *transactionalSEPADebitDownloadRetention) DeleteExpiredSEPADebitDownloads(ctx context.Context) (int, error) {
	r.inAdmin = append(r.inAdmin, ctx.Value(cleanupTransactionKey{}) == true)
	return 0, nil
}

// The unsubmitted PAIN.008 downloads expire across every Träger (ADR 0187),
// which only the administrative transaction may do.
func TestSEPADebitDownloadRetentionRunsInTheAdministrativeTransaction(t *testing.T) {
	t.Parallel()
	deps := minimalWorkerDependencies(t)
	deps.TenantRuntime = markingCleanupRuntime{unitTenantRuntime()}
	retention := &transactionalSEPADebitDownloadRetention{}
	deps.SEPADebitDownloadRetention = retention

	require.NoError(t, newScheduler(deps).runCleanupJobs(t.Context()))

	require.Equal(t, []bool{true}, retention.inAdmin, "the job runs once, inside the administrative transaction")
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
