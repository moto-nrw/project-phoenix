package parent_test

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	absenceSvc "github.com/moto-nrw/project-phoenix/services/absence"
	parentService "github.com/moto-nrw/project-phoenix/services/parent"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

// stubExcused is an ExcusedAbsenceRequestService whose three service-facing
// methods return configurable errors, so the parent write service's error
// mapping (which is otherwise unreachable because the parent path validates
// input upstream) can be exercised.
type stubExcused struct {
	absenceSvc.ExcusedAbsenceRequestService
	createErr   error
	withdrawErr error
	listErr     error
}

func (s stubExcused) CreateRequest(_ context.Context, _, _ int64, _ []timezone.Date, _ string) (*activeModels.ExcusedAbsenceRequest, error) {
	return nil, s.createErr
}

func (s stubExcused) WithdrawRequest(_ context.Context, _, _, _ int64) (*activeModels.ExcusedAbsenceRequest, error) {
	return nil, s.withdrawErr
}

func (s stubExcused) ListForStudent(_ context.Context, _ int64, _ time.Time) ([]*activeModels.ExcusedAbsenceRequest, error) {
	return nil, s.listErr
}

// buildParentServiceWithExcused wires a parent service (approval gate on) with a
// caller-supplied excused-request service (real, stub, or nil).
func buildParentServiceWithExcused(t *testing.T, excused absenceSvc.ExcusedAbsenceRequestService) (parentService.Service, *bun.DB) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	repos := repositories.NewFactory(db)
	bc := testpkg.NewRecordingBroadcaster()
	return parentService.NewService(parentService.ServiceConfig{
		ChildRepo:       repos.ParentChild,
		StatusDayRepo:   repos.StudentStatusDay,
		StudentRepo:     repos.Student,
		Settings:        excusedApprovalSettings{requiresApproval: true},
		Broadcaster:     bc,
		ExcusedRequests: excused,
		DB:              db,
		Logger:          slog.Default(),
	}), db
}

// TestSubmitExcusedRequest_MapsServiceErrors verifies the parent write service
// translates the excused-request service's validation sentinels onto its own
// stable parent sentinels (so the handler renders consistent status codes).
func TestSubmitExcusedRequest_MapsServiceErrors(t *testing.T) {
	t.Parallel()

	day := timezone.TodayDate().AddDays(3)
	cases := []struct {
		name string
		in   error
		want error
	}{
		{"no dates", absenceSvc.ErrExcusedRequestNoDates, parentService.ErrNoDates},
		{"empty note", absenceSvc.ErrExcusedRequestEmptyNote, parentService.ErrEmptyNote},
		{"note too long", absenceSvc.ErrExcusedRequestNoteTooLong, parentService.ErrNoteTooLong},
		{"overlap", absenceSvc.ErrExcusedRequestOverlap, parentService.ErrExcusedRequestOverlap},
		{"partial absence conflict", absenceSvc.ErrExcusedRequestStatusConflict, parentService.ErrCareExceptionConflict},
		{"other error is wrapped", errors.New("boom"), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, db := buildParentServiceWithExcused(t, stubExcused{createErr: tc.in})
			chain := testpkg.CreateTestParentGuardianChain(t, db)

			_, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
				[]timezone.Date{day}, "Familienfeier", activeModels.StudentStatusDayExcused)
			require.Error(t, err)
			if tc.want != nil {
				assert.ErrorIs(t, err, tc.want)
			} else {
				// The default branch wraps the raw error rather than mapping it.
				assert.ErrorContains(t, err, "boom")
			}
		})
	}
}

// TestSubmitExcused_NoServiceConfigured verifies the guard when the excused
// service was not wired (gate on but ExcusedRequests nil).
func TestSubmitExcused_NoServiceConfigured(t *testing.T) {
	t.Parallel()

	svc, db := buildParentServiceWithExcused(t, nil)
	chain := testpkg.CreateTestParentGuardianChain(t, db)

	_, err := svc.SubmitSickNote(context.Background(), chain.AccountID, chain.StudentID,
		[]timezone.Date{timezone.TodayDate().AddDays(3)}, "Familienfeier", activeModels.StudentStatusDayExcused)
	require.Error(t, err)
	assert.ErrorContains(t, err, "not configured")
}

// TestListExcusedRequests_Errors covers the ownership guard, the nil-service
// short-circuit and the service-error wrap.
func TestListExcusedRequests_Errors(t *testing.T) {
	t.Parallel()

	t.Run("foreign child rejected", func(t *testing.T) {
		svc, db := buildParentServiceWithExcused(t, stubExcused{})
		chain := testpkg.CreateTestParentGuardianChain(t, db)

		_, err := svc.ListExcusedRequests(context.Background(), chain.AccountID, chain.StudentID+999999)
		require.Error(t, err, "a student the account does not guard must be refused")
	})

	t.Run("nil service returns empty", func(t *testing.T) {
		svc, db := buildParentServiceWithExcused(t, nil)
		chain := testpkg.CreateTestParentGuardianChain(t, db)

		out, err := svc.ListExcusedRequests(context.Background(), chain.AccountID, chain.StudentID)
		require.NoError(t, err)
		assert.Empty(t, out)
	})

	t.Run("service error is wrapped", func(t *testing.T) {
		svc, db := buildParentServiceWithExcused(t, stubExcused{listErr: errors.New("db down")})
		chain := testpkg.CreateTestParentGuardianChain(t, db)

		_, err := svc.ListExcusedRequests(context.Background(), chain.AccountID, chain.StudentID)
		require.Error(t, err)
		assert.ErrorContains(t, err, "db down")
	})
}

// TestWithdrawExcusedRequest_Errors covers the ownership guard, the nil-service
// path and the not-found / not-pending / wrapped mappings.
func TestWithdrawExcusedRequest_Errors(t *testing.T) {
	t.Parallel()

	t.Run("foreign child rejected", func(t *testing.T) {
		svc, db := buildParentServiceWithExcused(t, stubExcused{})
		chain := testpkg.CreateTestParentGuardianChain(t, db)

		_, err := svc.WithdrawExcusedRequest(context.Background(), chain.AccountID, chain.StudentID+999999, 1)
		require.Error(t, err)
	})

	t.Run("nil service returns not found", func(t *testing.T) {
		svc, db := buildParentServiceWithExcused(t, nil)
		chain := testpkg.CreateTestParentGuardianChain(t, db)

		_, err := svc.WithdrawExcusedRequest(context.Background(), chain.AccountID, chain.StudentID, 1)
		assert.ErrorIs(t, err, parentService.ErrExcusedRequestNotFound)
	})

	cases := []struct {
		name string
		in   error
		want error
	}{
		{"not found", activeModels.ErrExcusedRequestNotFound, parentService.ErrExcusedRequestNotFound},
		{"not pending", activeModels.ErrExcusedRequestNotPending, parentService.ErrExcusedRequestNotPending},
		{"other error wrapped", errors.New("kaboom"), nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, db := buildParentServiceWithExcused(t, stubExcused{withdrawErr: tc.in})
			chain := testpkg.CreateTestParentGuardianChain(t, db)

			_, err := svc.WithdrawExcusedRequest(context.Background(), chain.AccountID, chain.StudentID, 1)
			require.Error(t, err)
			if tc.want != nil {
				assert.ErrorIs(t, err, tc.want)
			} else {
				assert.ErrorContains(t, err, "kaboom")
			}
		})
	}
}
