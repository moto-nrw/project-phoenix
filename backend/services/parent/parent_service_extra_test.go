package parent_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

// stubChildRepo records the account_id and replays a canned result. We
// only verify the service forwards the id, propagates the result, and
// surfaces errors — the admin-tx wrap is exercised by the fact that
// these tests need a real *bun.DB to compile.
type stubChildRepo struct {
	gotAccountID int64
	result       []*parentModels.ChildSummary
	err          error
}

func (s *stubChildRepo) ListByAccount(_ context.Context, accountID int64) ([]*parentModels.ChildSummary, error) {
	s.gotAccountID = accountID
	return s.result, s.err
}

// FindForAccount satisfies the ChildRepository interface. Returns the
// first canned child whose StudentID matches, or nil when none — mirrors
// the real repo's "not linked" contract.
func (s *stubChildRepo) FindForAccount(_ context.Context, accountID, studentID int64) (*parentModels.ChildSummary, error) {
	s.gotAccountID = accountID
	if s.err != nil {
		return nil, s.err
	}
	for _, c := range s.result {
		if c.StudentID == studentID {
			return c, nil
		}
	}
	return nil, nil
}

type stubEnrollableRepo struct {
	gotAccountID int64
	result       []*parentModels.EnrollablePhase
	err          error
}

type stubEnrollmentSettings struct {
	values map[int64]bool
	err    error
}

func (s stubEnrollmentSettings) EnrollmentEnabledForTenants(_ context.Context, tenantIDs []int64) (map[int64]bool, error) {
	if s.err != nil {
		return nil, s.err
	}
	result := make(map[int64]bool, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		result[tenantID] = s.values == nil || s.values[tenantID]
	}
	return result, nil
}

func (s *stubEnrollableRepo) ListEnrollable(_ context.Context, accountID int64) ([]*parentModels.EnrollablePhase, error) {
	s.gotAccountID = accountID
	return s.result, s.err
}

func (s *stubEnrollableRepo) GuardianSubmitStatus(_ context.Context, _, _ int64) (*parentModels.GuardianSubmitStatus, error) {
	return &parentModels.GuardianSubmitStatus{}, nil
}

func newSvcWithChild(t *testing.T, child *stubChildRepo) parentService.Service {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	return parentService.NewService(parentService.ServiceConfig{
		ChildRepo: child,
		DB:        db,
		Logger:    slog.Default(),
	})
}

func newSvcWithEnrollable(t *testing.T, enroll *stubEnrollableRepo) parentService.Service {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	return parentService.NewService(parentService.ServiceConfig{
		EnrollablePhaseRepo: enroll,
		EnrollmentSettings:  stubEnrollmentSettings{},
		DB:                  db,
		Logger:              slog.Default(),
	})
}

func fakeAdminContext(t *testing.T) context.Context {
	t.Helper()
	runtime, err := tenant.NewUnitOfWork(
		func(ctx context.Context, _ int64, fn func(context.Context, any) error) error {
			return fn(ctx, bun.Tx{})
		},
		func(ctx context.Context, fn func(context.Context, any) error) error {
			return fn(ctx, bun.Tx{})
		},
		func(context.Context, tenant.SavepointAction) error { return nil },
		func(error) bool { return false },
	)
	require.NoError(t, err)
	return tenant.WithUnitOfWork(context.Background(), runtime)
}

func TestService_ListEnrollableForAccount_FiltersDisabledTenantsThroughTypedSettings(t *testing.T) {
	t.Parallel()

	repo := &stubEnrollableRepo{result: []*parentModels.EnrollablePhase{
		{SchoolID: 11, PhaseID: 101},
		{SchoolID: 22, PhaseID: 202},
	}}
	svc := parentService.NewService(parentService.ServiceConfig{
		EnrollablePhaseRepo: repo,
		EnrollmentSettings:  stubEnrollmentSettings{values: map[int64]bool{11: true, 22: false}},
		Logger:              slog.Default(),
	})

	got, err := svc.ListEnrollableForAccount(fakeAdminContext(t), 999)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, int64(11), got[0].SchoolID)
}

func TestService_ListEnrollableForAccount_PropagatesTypedSettingsFailure(t *testing.T) {
	t.Parallel()

	want := errors.New("settings unavailable")
	svc := parentService.NewService(parentService.ServiceConfig{
		EnrollablePhaseRepo: &stubEnrollableRepo{result: []*parentModels.EnrollablePhase{{SchoolID: 11}}},
		EnrollmentSettings:  stubEnrollmentSettings{err: want},
		Logger:              slog.Default(),
	})

	_, err := svc.ListEnrollableForAccount(fakeAdminContext(t), 999)
	assert.ErrorIs(t, err, want)
}

// --- ListChildrenForAccount ----------------------------------------------

func TestService_ListChildrenForAccount_PassesAccountIDThrough(t *testing.T) {
	t.Parallel()

	repo := &stubChildRepo{
		result: []*parentModels.ChildSummary{
			{StudentID: 4321, FirstName: "Lara", LastName: "Beispiel"},
		},
	}
	svc := newSvcWithChild(t, repo)

	got, err := svc.ListChildrenForAccount(testpkg.WithPackageTenantRuntime(context.Background()), 4321)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "Lara", got[0].FirstName)
	assert.Equal(t, int64(4321), repo.gotAccountID)
}

func TestService_ListChildrenForAccount_RejectsZeroAccount(t *testing.T) {
	t.Parallel()

	repo := &stubChildRepo{}
	svc := newSvcWithChild(t, repo)

	_, err := svc.ListChildrenForAccount(testpkg.WithPackageTenantRuntime(context.Background()), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account_id must be positive")
	assert.Equal(t, int64(0), repo.gotAccountID, "repo must not be called for invalid input")
}

func TestService_ListChildrenForAccount_RejectsNegativeAccount(t *testing.T) {
	t.Parallel()

	repo := &stubChildRepo{}
	svc := newSvcWithChild(t, repo)

	_, err := svc.ListChildrenForAccount(testpkg.WithPackageTenantRuntime(context.Background()), -42)
	require.Error(t, err)
}

func TestService_ListChildrenForAccount_PropagatesRepoError(t *testing.T) {
	t.Parallel()

	want := errors.New("synthetic child repo failure")
	repo := &stubChildRepo{err: want}
	svc := newSvcWithChild(t, repo)

	_, err := svc.ListChildrenForAccount(testpkg.WithPackageTenantRuntime(context.Background()), 1)
	require.Error(t, err)
	assert.ErrorIs(t, err, want, "service must wrap, not swallow, repo errors")
}

func TestService_ListChildrenForAccount_EmptyResultPropagates(t *testing.T) {
	t.Parallel()

	repo := &stubChildRepo{result: []*parentModels.ChildSummary{}}
	svc := newSvcWithChild(t, repo)

	got, err := svc.ListChildrenForAccount(testpkg.WithPackageTenantRuntime(context.Background()), 1)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

// --- ListEnrollableForAccount --------------------------------------------

func TestService_ListEnrollableForAccount_PassesAccountIDThrough(t *testing.T) {
	t.Parallel()

	repo := &stubEnrollableRepo{
		result: []*parentModels.EnrollablePhase{
			{SchoolID: 5005, PhaseID: 7007, PhaseName: "Schuljahr 2026/27", AlreadyLinked: true},
		},
	}
	svc := newSvcWithEnrollable(t, repo)

	got, err := svc.ListEnrollableForAccount(testpkg.WithPackageTenantRuntime(context.Background()), 999)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, int64(7007), got[0].PhaseID)
	assert.True(t, got[0].AlreadyLinked)
	assert.Equal(t, int64(999), repo.gotAccountID)
}

func TestService_ListEnrollableForAccount_RejectsZeroAccount(t *testing.T) {
	t.Parallel()

	repo := &stubEnrollableRepo{}
	svc := newSvcWithEnrollable(t, repo)

	_, err := svc.ListEnrollableForAccount(testpkg.WithPackageTenantRuntime(context.Background()), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account_id must be positive")
	assert.Equal(t, int64(0), repo.gotAccountID)
}

func TestService_ListEnrollableForAccount_RejectsNegativeAccount(t *testing.T) {
	t.Parallel()

	repo := &stubEnrollableRepo{}
	svc := newSvcWithEnrollable(t, repo)

	_, err := svc.ListEnrollableForAccount(testpkg.WithPackageTenantRuntime(context.Background()), -1)
	require.Error(t, err)
}

func TestService_ListEnrollableForAccount_PropagatesRepoError(t *testing.T) {
	t.Parallel()

	want := errors.New("synthetic enrollable repo failure")
	repo := &stubEnrollableRepo{err: want}
	svc := newSvcWithEnrollable(t, repo)

	_, err := svc.ListEnrollableForAccount(testpkg.WithPackageTenantRuntime(context.Background()), 1)
	require.Error(t, err)
	assert.ErrorIs(t, err, want)
}

func TestService_ListEnrollableForAccount_EmptyResultPropagates(t *testing.T) {
	t.Parallel()

	repo := &stubEnrollableRepo{result: []*parentModels.EnrollablePhase{}}
	svc := newSvcWithEnrollable(t, repo)

	got, err := svc.ListEnrollableForAccount(testpkg.WithPackageTenantRuntime(context.Background()), 1)
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

// --- NewService logger fallback ------------------------------------------

func TestNewService_NilLoggerFallsBackToDefault(t *testing.T) {
	t.Parallel()

	// Constructor must not panic when the caller forgets to pass a
	// logger — the parent service is constructed at app boot, and a nil
	// pointer dereference there would mean no server. The fallback is
	// slog.Default(), which is non-nil by definition.
	db := testpkg.SetupTestDB(t)

	svc := parentService.NewService(parentService.ServiceConfig{
		ChildRepo: &stubChildRepo{},
		DB:        db,
		Logger:    nil,
	})
	require.NotNil(t, svc)

	// Round-trip a call so the nil-logger path actually executes.
	_, err := svc.ListChildrenForAccount(testpkg.WithPackageTenantRuntime(context.Background()), 1)
	require.NoError(t, err)
}
