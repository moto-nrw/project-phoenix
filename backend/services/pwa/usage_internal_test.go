package pwa

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/iot"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errUsageRepo = errors.New("usage repository failure")

type recordingUsageRepository struct {
	iot.PWAStandaloneUsageRepository
	recorded      []*iot.PWAStandaloneUsage
	recordErr     error
	deletedCutoff time.Time
	deleteCount   int
	deleteErr     error
}

func (r *recordingUsageRepository) RecordSeen(_ context.Context, usage *iot.PWAStandaloneUsage) error {
	if r.recordErr != nil {
		return r.recordErr
	}
	r.recorded = append(r.recorded, usage)
	return nil
}

func (r *recordingUsageRepository) DeleteLastSeenBefore(_ context.Context, cutoff time.Time) (int, error) {
	r.deletedCutoff = cutoff
	return r.deleteCount, r.deleteErr
}

type accountTenantStub struct {
	authModels.AccountTenantRepository
	mappings []authModels.AccountTenant
	err      error
}

func (r accountTenantStub) FindActiveGuardianByAccountID(context.Context, int64) ([]authModels.AccountTenant, error) {
	return r.mappings, r.err
}

type summariesStub struct {
	platformModels.OperatorSummariesRepository
	rows  []platformModels.SchoolPWAUsageRow
	err   error
	calls int
}

func (s *summariesStub) PWAUsage(context.Context, int64, time.Duration) ([]platformModels.SchoolPWAUsageRow, error) {
	s.calls++
	return s.rows, s.err
}

func retentionSettings(days int) *configtest.Mock {
	return &configtest.Mock{
		ResolveIntFn: func(_ context.Context, key string) (int, error) {
			if key != configModel.KeyGDPRPWAUsageRetentionDays {
				return 0, fmt.Errorf("unexpected settings key %q", key)
			}
			return days, nil
		},
	}
}

func TestUsageServiceReportStaff(t *testing.T) {
	t.Parallel()

	repo := &recordingUsageRepository{}
	service := NewUsageService(nil, repo, nil, nil, nil, nil)

	require.NoError(t, service.ReportStaff(context.Background(), 42))
	require.Len(t, repo.recorded, 1)
	assert.Equal(t, int64(42), repo.recorded[0].AccountID)
	assert.Equal(t, iot.PushPortalStaff, repo.recorded[0].Portal)

	require.Error(t, service.ReportStaff(context.Background(), 0), "invalid account must not reach the repository")
	require.Len(t, repo.recorded, 1)
}

func TestUsageServiceReportParent(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)

	t.Run("fans out one row per active guardian mapping", func(t *testing.T) {
		repo := &recordingUsageRepository{}
		mappings := accountTenantStub{mappings: []authModels.AccountTenant{{TenantID: 11}, {TenantID: 12}}}
		service := NewUsageService(db, repo, nil, mappings, nil, nil)

		require.NoError(t, service.ReportParent(context.Background(), 42))
		require.Len(t, repo.recorded, 2)
		assert.Equal(t, int64(11), repo.recorded[0].TenantID)
		assert.Equal(t, int64(12), repo.recorded[1].TenantID)
		for _, usage := range repo.recorded {
			assert.Equal(t, iot.PushPortalParent, usage.Portal)
			assert.Equal(t, int64(42), usage.AccountID)
		}
	})

	t.Run("zero mappings is a no-op, not an error", func(t *testing.T) {
		repo := &recordingUsageRepository{}
		service := NewUsageService(db, repo, nil, accountTenantStub{}, nil, nil)
		require.NoError(t, service.ReportParent(context.Background(), 42))
		assert.Empty(t, repo.recorded)
	})

	t.Run("forwards mapping and repository errors", func(t *testing.T) {
		service := NewUsageService(db, nil, nil, accountTenantStub{err: errUsageRepo}, nil, nil)
		err := service.ReportParent(context.Background(), 42)
		require.ErrorIs(t, err, errUsageRepo)
		assert.ErrorContains(t, err, "resolving guardian tenant mappings")

		repo := &recordingUsageRepository{recordErr: errUsageRepo}
		service = NewUsageService(db, repo, nil, accountTenantStub{mappings: []authModels.AccountTenant{{TenantID: 11}}}, nil, nil)
		err = service.ReportParent(context.Background(), 42)
		require.ErrorIs(t, err, errUsageRepo)
		assert.ErrorContains(t, err, "recording pwa usage for tenant 11")
	})
}

func TestUsageServiceCleanupExpiredUsage(t *testing.T) {
	t.Parallel()

	ctx := testpkg.Ctx(t)

	t.Run("deletes with the configured retention cutoff", func(t *testing.T) {
		repo := &recordingUsageRepository{deleteCount: 3}
		service := NewUsageService(nil, repo, nil, nil, retentionSettings(90), nil)

		result, err := service.CleanupExpiredUsage(ctx)
		require.NoError(t, err)
		assert.Equal(t, 3, result.RowsDeleted)
		assert.Equal(t, 90, result.RetentionDays)
		assert.WithinDuration(t, time.Now().AddDate(0, 0, -90), repo.deletedCutoff, time.Minute)
	})

	t.Run("fails closed without tenant, settings, or valid retention", func(t *testing.T) {
		service := NewUsageService(nil, &recordingUsageRepository{}, nil, nil, retentionSettings(90), nil)
		_, err := service.CleanupExpiredUsage(context.Background())
		require.ErrorContains(t, err, "no tenant in context")

		service = NewUsageService(nil, &recordingUsageRepository{}, nil, nil, nil, nil)
		_, err = service.CleanupExpiredUsage(ctx)
		require.ErrorContains(t, err, "settings service not configured")

		service = NewUsageService(nil, &recordingUsageRepository{}, nil, nil, retentionSettings(0), nil)
		_, err = service.CleanupExpiredUsage(ctx)
		require.ErrorContains(t, err, "retention must be positive")
	})

	t.Run("forwards delete errors", func(t *testing.T) {
		repo := &recordingUsageRepository{deleteErr: errUsageRepo}
		service := NewUsageService(nil, repo, nil, nil, retentionSettings(90), nil)
		_, err := service.CleanupExpiredUsage(ctx)
		require.ErrorIs(t, err, errUsageRepo)
	})
}

func TestUsageServiceSnapshotUsage(t *testing.T) {
	t.Parallel()

	rows := []platformModels.SchoolPWAUsageRow{{TenantID: testpkg.Tenant(t), Portal: "staff", StandaloneUsers: 2, EligibleUsers: 5}}
	summaries := &summariesStub{rows: rows}
	// nil db degrades WithAdminTxOrDirect to a direct call — exactly what a
	// unit test needs.
	service := NewUsageService(nil, nil, summaries, nil, nil, nil)

	got, err := service.SnapshotUsage()
	require.NoError(t, err)
	assert.Equal(t, rows, got)

	// Second call inside the TTL must serve the cache, not the repository.
	_, err = service.SnapshotUsage()
	require.NoError(t, err)
	assert.Equal(t, 1, summaries.calls)

	failing := &summariesStub{err: errUsageRepo}
	service = NewUsageService(nil, nil, failing, nil, nil, nil)
	_, err = service.SnapshotUsage()
	require.ErrorIs(t, err, errUsageRepo)
}
