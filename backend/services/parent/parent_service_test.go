package parent_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	modelsBase "github.com/moto-nrw/project-phoenix/models/base"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// stubEnrollmentRequestRepo captures inputs to the repo and replays
// canned outputs. The parent service test only needs to verify (a) the
// repo gets called with the right account_id, (b) error propagation,
// and (c) the admin-tx wrapping wires through to the repo (verified by
// the call happening at all).
type stubEnrollmentRequestRepo struct {
	gotAccountID int64

	listResult []*parentModels.EnrollmentRequestSummary
	listErr    error

	backfillResult int
	backfillErr    error

	gotBackfillAccountID int64
	gotBackfillEmail     string
}

func (s *stubEnrollmentRequestRepo) ListByAccount(_ context.Context, accountID int64) ([]*parentModels.EnrollmentRequestSummary, error) {
	s.gotAccountID = accountID
	return s.listResult, s.listErr
}

func (s *stubEnrollmentRequestRepo) BackfillGuardianAccountID(_ context.Context, accountID int64, email string) (int, error) {
	s.gotBackfillAccountID = accountID
	s.gotBackfillEmail = email
	return s.backfillResult, s.backfillErr
}

// buildService wires the parent service with stubs for the two other
// repos (ChildRepository, EnrollablePhaseRepository) — they're not
// exercised by these tests but the service struct requires non-nil
// fields nowhere; the constructor only logs the missing repo at call
// time, so leaving them nil here is fine.
func buildParentService(t *testing.T, repo *stubEnrollmentRequestRepo) parentService.Service {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	return parentService.NewService(parentService.ServiceConfig{
		EnrollmentRequestRepo: repo,
		DB:                    db,
		Logger:                slog.Default(),
	})
}

func TestService_ListEnrollmentsForAccount_PassesAccountIDThrough(t *testing.T) {
	repo := &stubEnrollmentRequestRepo{
		listResult: []*parentModels.EnrollmentRequestSummary{
			{RequestID: 42, TenantID: 7, SubmittedAt: time.Now()},
		},
	}
	svc := buildParentService(t, repo)

	result, err := svc.ListEnrollmentsForAccount(context.Background(), 1234)
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, int64(42), result[0].RequestID)
	assert.Equal(t, int64(1234), repo.gotAccountID,
		"service must forward account_id to the repo unmodified")
}

func TestService_ListEnrollmentsForAccount_RejectsZeroAccount(t *testing.T) {
	repo := &stubEnrollmentRequestRepo{}
	svc := buildParentService(t, repo)

	_, err := svc.ListEnrollmentsForAccount(context.Background(), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account_id must be positive")
	assert.Equal(t, int64(0), repo.gotAccountID, "repo must not be called for invalid input")
}

func TestService_ListEnrollmentsForAccount_RejectsNegativeAccount(t *testing.T) {
	repo := &stubEnrollmentRequestRepo{}
	svc := buildParentService(t, repo)

	_, err := svc.ListEnrollmentsForAccount(context.Background(), -5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account_id must be positive")
}

func TestService_ListEnrollmentsForAccount_PropagatesRepoError(t *testing.T) {
	want := errors.New("synthetic repo failure")
	repo := &stubEnrollmentRequestRepo{listErr: want}
	svc := buildParentService(t, repo)

	_, err := svc.ListEnrollmentsForAccount(context.Background(), 1)
	require.Error(t, err)
	assert.ErrorIs(t, err, want, "service must wrap, not swallow, repo errors")
}

func TestService_ListEnrollmentsForAccount_NilRepoReturnsError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { require.NoError(t, db.Close()) }()
	svc := parentService.NewService(parentService.ServiceConfig{
		EnrollmentRequestRepo: nil,
		DB:                    db,
		Logger:                slog.Default(),
	})

	_, err := svc.ListEnrollmentsForAccount(context.Background(), 1)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "enrollment request repo not wired")
}

func TestService_ListEnrollmentsForAccount_EmptyResultPropagates(t *testing.T) {
	repo := &stubEnrollmentRequestRepo{
		listResult: []*parentModels.EnrollmentRequestSummary{},
	}
	svc := buildParentService(t, repo)

	result, err := svc.ListEnrollmentsForAccount(context.Background(), 1)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

// Regression: status_reason is admin-internal. When the owning phase has
// show_status_reason_to_parent=false, the dashboard payload must not carry
// the stored reason — even though the repo returns it.
func TestService_ListEnrollmentsForAccount_RedactsReasonWhenPhaseDisablesIt(t *testing.T) {
	reason := "intern: Kapazität voll"
	repo := &stubEnrollmentRequestRepo{
		listResult: []*parentModels.EnrollmentRequestSummary{
			{
				RequestID:                42,
				TenantID:                 7,
				ShowStatusReasonToParent: false,
				Children: []parentModels.EnrollmentRequestChildSummary{
					{ChildID: 100, FirstName: "Lina", Status: "rejected", StatusReason: &reason},
				},
			},
		},
	}
	svc := buildParentService(t, repo)

	result, err := svc.ListEnrollmentsForAccount(context.Background(), 1234)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Len(t, result[0].Children, 1)
	assert.Nil(t, result[0].Children[0].StatusReason,
		"status reason must be stripped when the phase disables parent-facing reasons")
}

// Counterpart: phase opted in → the reason is preserved on the dashboard.
func TestService_ListEnrollmentsForAccount_KeepsReasonWhenPhaseEnablesIt(t *testing.T) {
	reason := "Leider kein Platz"
	repo := &stubEnrollmentRequestRepo{
		listResult: []*parentModels.EnrollmentRequestSummary{
			{
				RequestID:                43,
				TenantID:                 7,
				ShowStatusReasonToParent: true,
				Children: []parentModels.EnrollmentRequestChildSummary{
					{ChildID: 101, FirstName: "Tom", Status: "rejected", StatusReason: &reason},
				},
			},
		},
	}
	svc := buildParentService(t, repo)

	result, err := svc.ListEnrollmentsForAccount(context.Background(), 1234)
	require.NoError(t, err)
	require.Len(t, result, 1)
	require.Len(t, result[0].Children, 1)
	require.NotNil(t, result[0].Children[0].StatusReason)
	assert.Equal(t, reason, *result[0].Children[0].StatusReason)
}

// --- portal locale profile -----------------------------------------------

// stubGuardianProfileRepo implements userModels.GuardianProfileRepository.
// Only FindByAccountID and UpdatePortalLocaleByAccountID carry behaviour; the
// rest satisfy the interface and are never exercised by the profile tests.
type stubGuardianProfileRepo struct {
	findResult *userModels.GuardianProfile
	findErr    error

	updateErr           error
	gotUpdateAccountID  int64
	gotUpdateLocale     string
	updateCallCount     int
	gotFindAccountIDArg int64
}

func (s *stubGuardianProfileRepo) FindByAccountID(_ context.Context, accountID int64) (*userModels.GuardianProfile, error) {
	s.gotFindAccountIDArg = accountID
	return s.findResult, s.findErr
}

func (s *stubGuardianProfileRepo) UpdatePortalLocaleByAccountID(_ context.Context, accountID int64, locale string) error {
	s.updateCallCount++
	s.gotUpdateAccountID = accountID
	s.gotUpdateLocale = locale
	return s.updateErr
}

func (s *stubGuardianProfileRepo) Create(context.Context, *userModels.GuardianProfile) error {
	return nil
}
func (s *stubGuardianProfileRepo) FindByID(context.Context, int64) (*userModels.GuardianProfile, error) {
	return nil, nil
}
func (s *stubGuardianProfileRepo) LockByIDForUpdate(context.Context, int64) error { return nil }
func (s *stubGuardianProfileRepo) FindByEmail(context.Context, string) (*userModels.GuardianProfile, error) {
	return nil, nil
}
func (s *stubGuardianProfileRepo) FindWithoutAccount(context.Context) ([]*userModels.GuardianProfile, error) {
	return nil, nil
}
func (s *stubGuardianProfileRepo) FindInvitable(context.Context) ([]*userModels.GuardianProfile, error) {
	return nil, nil
}
func (s *stubGuardianProfileRepo) ListWithOptions(context.Context, *modelsBase.QueryOptions) ([]*userModels.GuardianProfile, error) {
	return nil, nil
}
func (s *stubGuardianProfileRepo) SearchByText(context.Context, string, int) ([]*userModels.GuardianProfile, error) {
	return nil, nil
}
func (s *stubGuardianProfileRepo) FindByIDs(context.Context, []int64) (map[int64]*userModels.GuardianProfile, error) {
	return nil, nil
}
func (s *stubGuardianProfileRepo) FindActivePortalProfilesByIDs(context.Context, []int64) (map[int64]*userModels.GuardianProfile, error) {
	return nil, nil
}
func (s *stubGuardianProfileRepo) Count(context.Context) (int, error) { return 0, nil }
func (s *stubGuardianProfileRepo) Update(context.Context, *userModels.GuardianProfile) error {
	return nil
}
func (s *stubGuardianProfileRepo) Delete(context.Context, int64) error             { return nil }
func (s *stubGuardianProfileRepo) LinkAccount(context.Context, int64, int64) error { return nil }
func (s *stubGuardianProfileRepo) UnlinkAccount(context.Context, int64) error      { return nil }
func (s *stubGuardianProfileRepo) GetStudentCount(context.Context, int64) (int, error) {
	return 0, nil
}
func (s *stubGuardianProfileRepo) LoadProfileWithChildren(context.Context, int64) (*userModels.GuardianProfileWithChildren, error) {
	return nil, nil
}

func buildParentServiceWithGuardian(t *testing.T, repo *stubGuardianProfileRepo) parentService.Service {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	return parentService.NewService(parentService.ServiceConfig{
		GuardianProfileRepo: repo,
		DB:                  db,
		Logger:              slog.Default(),
	})
}

func localePtr(s string) *string { return &s }

func TestService_GetProfile_ExplicitLocale(t *testing.T) {
	repo := &stubGuardianProfileRepo{
		findResult: &userModels.GuardianProfile{
			FirstName:    "Karin",
			LastName:     "Klein",
			PortalLocale: localePtr("en"),
		},
	}
	svc := buildParentServiceWithGuardian(t, repo)

	profile, err := svc.GetProfile(context.Background(), 1234)
	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.True(t, profile.Explicit, "a stored portal_locale must be reported as an explicit choice")
	assert.Equal(t, "en", profile.Locale)
	assert.Equal(t, "Karin", profile.FirstName)
	assert.Equal(t, "Klein", profile.LastName)
	assert.Equal(t, int64(1234), repo.gotFindAccountIDArg, "account_id must reach the repo unmodified")
}

func TestService_GetProfile_NormalizesRegionSubtag(t *testing.T) {
	repo := &stubGuardianProfileRepo{
		findResult: &userModels.GuardianProfile{PortalLocale: localePtr("en-US")},
	}
	svc := buildParentServiceWithGuardian(t, repo)

	profile, err := svc.GetProfile(context.Background(), 1234)
	require.NoError(t, err)
	assert.True(t, profile.Explicit)
	assert.Equal(t, "en", profile.Locale, "region subtag must be stripped to the base locale")
}

func TestService_GetProfile_NullLocaleIsNotExplicit(t *testing.T) {
	repo := &stubGuardianProfileRepo{
		findResult: &userModels.GuardianProfile{PortalLocale: nil},
	}
	svc := buildParentServiceWithGuardian(t, repo)

	profile, err := svc.GetProfile(context.Background(), 1234)
	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.False(t, profile.Explicit,
		"NULL portal_locale means the parent never chose — caller keeps the anonymous locale")
	assert.Equal(t, "de", profile.Locale)
}

func TestService_GetProfile_EmptyLocaleIsNotExplicit(t *testing.T) {
	repo := &stubGuardianProfileRepo{
		findResult: &userModels.GuardianProfile{PortalLocale: localePtr("")},
	}
	svc := buildParentServiceWithGuardian(t, repo)

	profile, err := svc.GetProfile(context.Background(), 1234)
	require.NoError(t, err)
	assert.False(t, profile.Explicit, "an empty stored value is treated as unset, not as a choice")
	assert.Equal(t, "de", profile.Locale)
}

func TestService_GetProfile_MissingProfileIsBenign(t *testing.T) {
	repo := &stubGuardianProfileRepo{
		findErr: userModels.ErrGuardianProfileNotFound,
	}
	svc := buildParentServiceWithGuardian(t, repo)

	profile, err := svc.GetProfile(context.Background(), 1234)
	require.NoError(t, err, "a parent without a guardian-profile row is benign, not an error")
	require.NotNil(t, profile)
	assert.False(t, profile.Explicit)
	assert.Equal(t, "de", profile.Locale)
}

func TestService_GetProfile_PropagatesRealRepoError(t *testing.T) {
	want := errors.New("synthetic db failure")
	repo := &stubGuardianProfileRepo{findErr: want}
	svc := buildParentServiceWithGuardian(t, repo)

	_, err := svc.GetProfile(context.Background(), 1234)
	require.Error(t, err)
	assert.ErrorIs(t, err, want, "a genuine repo fault must surface, not be masked as an unset preference")
}

func TestService_GetProfile_RejectsNonPositiveAccount(t *testing.T) {
	repo := &stubGuardianProfileRepo{}
	svc := buildParentServiceWithGuardian(t, repo)

	_, err := svc.GetProfile(context.Background(), 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account_id must be positive")
	assert.Equal(t, int64(0), repo.gotFindAccountIDArg, "repo must not be called for invalid input")
}

func TestService_GetProfile_NilRepoReturnsError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	svc := parentService.NewService(parentService.ServiceConfig{
		GuardianProfileRepo: nil,
		DB:                  db,
		Logger:              slog.Default(),
	})

	_, err := svc.GetProfile(context.Background(), 1234)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "guardian profile repo not wired")
}

func TestService_UpdatePortalLocale_NormalizesAndPersists(t *testing.T) {
	repo := &stubGuardianProfileRepo{}
	svc := buildParentServiceWithGuardian(t, repo)

	profile, err := svc.UpdatePortalLocale(context.Background(), 1234, "en-US")
	require.NoError(t, err)
	require.NotNil(t, profile)
	assert.True(t, profile.Explicit, "a write is always an explicit choice")
	assert.Equal(t, "en", profile.Locale)
	assert.Equal(t, 1, repo.updateCallCount)
	assert.Equal(t, int64(1234), repo.gotUpdateAccountID)
	assert.Equal(t, "en", repo.gotUpdateLocale, "the normalized locale, not the raw region tag, must be persisted")
}

func TestService_UpdatePortalLocale_RejectsNonPositiveAccount(t *testing.T) {
	repo := &stubGuardianProfileRepo{}
	svc := buildParentServiceWithGuardian(t, repo)

	_, err := svc.UpdatePortalLocale(context.Background(), -1, "en")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "account_id must be positive")
	assert.Equal(t, 0, repo.updateCallCount, "repo must not be called for invalid input")
}

func TestService_UpdatePortalLocale_PropagatesRepoError(t *testing.T) {
	want := errors.New("synthetic write failure")
	repo := &stubGuardianProfileRepo{updateErr: want}
	svc := buildParentServiceWithGuardian(t, repo)

	_, err := svc.UpdatePortalLocale(context.Background(), 1234, "en")
	require.Error(t, err)
	assert.ErrorIs(t, err, want)
}

func TestService_UpdatePortalLocale_NilRepoReturnsError(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()
	svc := parentService.NewService(parentService.ServiceConfig{
		GuardianProfileRepo: nil,
		DB:                  db,
		Logger:              slog.Default(),
	})

	_, err := svc.UpdatePortalLocale(context.Background(), 1234, "en")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "guardian profile repo not wired")
}

func TestService_UpdatePortalLocale_RejectsUnsupportedLocale(t *testing.T) {
	repo := &stubGuardianProfileRepo{}
	svc := buildParentServiceWithGuardian(t, repo)

	_, err := svc.UpdatePortalLocale(context.Background(), 1234, "xx")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported locale")
	assert.Equal(t, 0, repo.updateCallCount,
		"an unknown locale must be rejected before any write, never coerced to the default")
}

func TestService_UpdatePortalLocale_SurfacesMissingProfile(t *testing.T) {
	// The repo returns ErrGuardianProfileNotFound when the UPDATE matched zero
	// rows (account has no guardian_profiles row). The service must surface it,
	// not report the locale as saved — a preference that never persisted cannot
	// masquerade as success.
	repo := &stubGuardianProfileRepo{updateErr: userModels.ErrGuardianProfileNotFound}
	svc := buildParentServiceWithGuardian(t, repo)

	_, err := svc.UpdatePortalLocale(context.Background(), 1234, "en")
	require.Error(t, err)
	assert.ErrorIs(t, err, userModels.ErrGuardianProfileNotFound)
}
