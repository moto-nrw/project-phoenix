package parent_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
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
	defer db.Close()
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
