package audit

import (
	"context"
	"errors"
	"testing"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/stretchr/testify/require"
)

type fakeUnregisteredTagScanRepo struct {
	created      *auditModels.UnregisteredTagScan
	createErr    error
	deleteCutoff time.Time
	deleteResult int
	deleteErr    error
}

func (r *fakeUnregisteredTagScanRepo) Create(_ context.Context, scan *auditModels.UnregisteredTagScan) error {
	r.created = scan
	return r.createErr
}

func (r *fakeUnregisteredTagScanRepo) FindByID(_ context.Context, _ int64) (*auditModels.UnregisteredTagScan, error) {
	return nil, nil
}

func (r *fakeUnregisteredTagScanRepo) DeleteOlderThan(_ context.Context, cutoff time.Time) (int, error) {
	r.deleteCutoff = cutoff
	return r.deleteResult, r.deleteErr
}

type testTenantKey struct{}

func newUnregisteredTagScanService(t *testing.T, repo *fakeUnregisteredTagScanRepo) UnregisteredTagScanService {
	t.Helper()
	service, err := NewUnregisteredTagScanService(repo, UnregisteredTagScanRuntime{
		TenantID: func(ctx context.Context) int64 {
			id, _ := ctx.Value(testTenantKey{}).(int64)
			return id
		},
	})
	require.NoError(t, err)
	return service
}

func withTestTenant(id int64) context.Context {
	return context.WithValue(context.Background(), testTenantKey{}, id)
}

func TestUnregisteredTagScanRecordTrimsAndStampsTenant(t *testing.T) {
	t.Parallel()

	repo := &fakeUnregisteredTagScanRepo{}
	service := newUnregisteredTagScanService(t, repo)
	deviceID := int64(42)
	ctx := withTestTenant(99)

	err := service.Record(ctx, "  ABC123  ", &deviceID)

	require.NoError(t, err)
	require.NotNil(t, repo.created)
	require.Equal(t, "ABC123", repo.created.TagUID)
	require.Equal(t, int64(99), repo.created.TenantID)
	require.Equal(t, &deviceID, repo.created.DeviceID)
	require.False(t, repo.created.ScannedAt.IsZero())
}

func TestUnregisteredTagScanRecordRequiresTenant(t *testing.T) {
	t.Parallel()

	repo := &fakeUnregisteredTagScanRepo{}
	service := newUnregisteredTagScanService(t, repo)

	err := service.Record(context.Background(), "ABC123", nil)

	require.ErrorContains(t, err, "tenant context is required")
	require.Nil(t, repo.created)
}

func TestUnregisteredTagScanRecordRequiresTagUID(t *testing.T) {
	t.Parallel()

	repo := &fakeUnregisteredTagScanRepo{}
	service := newUnregisteredTagScanService(t, repo)
	ctx := withTestTenant(99)

	err := service.Record(ctx, "   ", nil)

	require.ErrorContains(t, err, "tag UID is required")
	require.Nil(t, repo.created)
}

func TestUnregisteredTagScanRecordPropagatesRepositoryError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("insert failed")
	repo := &fakeUnregisteredTagScanRepo{createErr: wantErr}
	service := newUnregisteredTagScanService(t, repo)
	ctx := withTestTenant(99)

	err := service.Record(ctx, "ABC123", nil)

	require.ErrorIs(t, err, wantErr)
	require.NotNil(t, repo.created)
}

func TestUnregisteredTagScanDeleteOlderThanUsesDefaultRetention(t *testing.T) {
	t.Parallel()

	repo := &fakeUnregisteredTagScanRepo{deleteResult: 3}
	service := newUnregisteredTagScanService(t, repo)
	before := time.Now().AddDate(0, 0, -UnregisteredTagScanRetentionDays)

	deleted, err := service.DeleteOlderThan(context.Background(), 0)

	after := time.Now().AddDate(0, 0, -UnregisteredTagScanRetentionDays)
	require.NoError(t, err)
	require.Equal(t, 3, deleted)
	require.False(t, repo.deleteCutoff.Before(before))
	require.False(t, repo.deleteCutoff.After(after))
}

func TestUnregisteredTagScanDeleteOlderThanUsesCustomDaysAndPropagatesError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("delete failed")
	repo := &fakeUnregisteredTagScanRepo{deleteErr: wantErr}
	service := newUnregisteredTagScanService(t, repo)
	before := time.Now().AddDate(0, 0, -7)

	deleted, err := service.DeleteOlderThan(context.Background(), 7)

	after := time.Now().AddDate(0, 0, -7)
	require.ErrorIs(t, err, wantErr)
	require.Zero(t, deleted)
	require.False(t, repo.deleteCutoff.Before(before))
	require.False(t, repo.deleteCutoff.After(after))
}
