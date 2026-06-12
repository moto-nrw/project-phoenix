package audit

import (
	"context"
	"testing"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/require"
)

type fakeUnregisteredTagScanRepo struct {
	created *auditModels.UnregisteredTagScan
}

func (r *fakeUnregisteredTagScanRepo) Create(_ context.Context, scan *auditModels.UnregisteredTagScan) error {
	r.created = scan
	return nil
}

func (r *fakeUnregisteredTagScanRepo) FindByID(_ context.Context, _ int64) (*auditModels.UnregisteredTagScan, error) {
	return nil, nil
}

func (r *fakeUnregisteredTagScanRepo) ListForOperator(_ context.Context, _ auditModels.UnregisteredTagScanFilter) ([]*auditModels.UnregisteredTagScan, error) {
	return nil, nil
}

func (r *fakeUnregisteredTagScanRepo) Resolve(_ context.Context, _ int64, _ int64, _ *string) (*auditModels.UnregisteredTagScan, error) {
	return nil, nil
}

func (r *fakeUnregisteredTagScanRepo) DeleteOlderThan(_ context.Context, _ time.Time) (int, error) {
	return 0, nil
}

func TestUnregisteredTagScanRecordTrimsAndStampsTenant(t *testing.T) {
	repo := &fakeUnregisteredTagScanRepo{}
	service := NewUnregisteredTagScanService(repo, nil)
	deviceID := int64(42)
	ctx := tenant.WithTenantID(context.Background(), 99)

	err := service.Record(ctx, "  ABC123  ", &deviceID)

	require.NoError(t, err)
	require.NotNil(t, repo.created)
	require.Equal(t, "ABC123", repo.created.TagUID)
	require.Equal(t, int64(99), repo.created.TenantID)
	require.Equal(t, &deviceID, repo.created.DeviceID)
	require.False(t, repo.created.ScannedAt.IsZero())
}

func TestUnregisteredTagScanRecordRequiresTenant(t *testing.T) {
	repo := &fakeUnregisteredTagScanRepo{}
	service := NewUnregisteredTagScanService(repo, nil)

	err := service.Record(context.Background(), "ABC123", nil)

	require.ErrorContains(t, err, "tenant context is required")
	require.Nil(t, repo.created)
}

func TestNormalizeNote(t *testing.T) {
	note := "  assigned to replacement card  "

	require.Nil(t, normalizeNote(nil))
	require.Nil(t, normalizeNote(pointerToString("   ")))
	require.Equal(t, "assigned to replacement card", *normalizeNote(&note))
}

func pointerToString(value string) *string {
	return &value
}
