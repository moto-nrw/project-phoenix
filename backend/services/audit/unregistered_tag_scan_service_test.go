package audit

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/strutil"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/require"
)

type fakeUnregisteredTagScanRepo struct {
	created       *auditModels.UnregisteredTagScan
	createErr     error
	listFilter    auditModels.UnregisteredTagScanFilter
	listResult    []*auditModels.UnregisteredTagScan
	listErr       error
	resolveID     int64
	resolveOpID   int64
	resolveNote   *string
	resolveResult *auditModels.UnregisteredTagScan
	resolveErr    error
	deleteCutoff  time.Time
	deleteResult  int
	deleteErr     error
}

func (r *fakeUnregisteredTagScanRepo) Create(_ context.Context, scan *auditModels.UnregisteredTagScan) error {
	r.created = scan
	return r.createErr
}

func (r *fakeUnregisteredTagScanRepo) FindByID(_ context.Context, _ int64) (*auditModels.UnregisteredTagScan, error) {
	return nil, nil
}

func (r *fakeUnregisteredTagScanRepo) ListForOperator(_ context.Context, filter auditModels.UnregisteredTagScanFilter) ([]*auditModels.UnregisteredTagScan, error) {
	r.listFilter = filter
	return r.listResult, r.listErr
}

func (r *fakeUnregisteredTagScanRepo) Resolve(_ context.Context, id int64, operatorID int64, note *string) (*auditModels.UnregisteredTagScan, error) {
	r.resolveID = id
	r.resolveOpID = operatorID
	r.resolveNote = note
	return r.resolveResult, r.resolveErr
}

func (r *fakeUnregisteredTagScanRepo) DeleteOlderThan(_ context.Context, cutoff time.Time) (int, error) {
	r.deleteCutoff = cutoff
	return r.deleteResult, r.deleteErr
}

func TestUnregisteredTagScanRecordTrimsAndStampsTenant(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	repo := &fakeUnregisteredTagScanRepo{}
	service := NewUnregisteredTagScanService(repo, nil)

	err := service.Record(context.Background(), "ABC123", nil)

	require.ErrorContains(t, err, "tenant context is required")
	require.Nil(t, repo.created)
}

func TestUnregisteredTagScanRecordRequiresTagUID(t *testing.T) {
	t.Parallel()

	repo := &fakeUnregisteredTagScanRepo{}
	service := NewUnregisteredTagScanService(repo, nil)
	ctx := tenant.WithTenantID(context.Background(), 99)

	err := service.Record(ctx, "   ", nil)

	require.ErrorContains(t, err, "tag UID is required")
	require.Nil(t, repo.created)
}

func TestUnregisteredTagScanRecordPropagatesRepositoryError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("insert failed")
	repo := &fakeUnregisteredTagScanRepo{createErr: wantErr}
	service := NewUnregisteredTagScanService(repo, nil)
	ctx := tenant.WithTenantID(context.Background(), 99)

	err := service.Record(ctx, "ABC123", nil)

	require.ErrorIs(t, err, wantErr)
	require.NotNil(t, repo.created)
}

func TestUnregisteredTagScanListForOperatorPassesFilter(t *testing.T) {
	t.Parallel()

	schoolID := int64(10)
	orgID := schoolID / 2
	want := []*auditModels.UnregisteredTagScan{{TagUID: "ABC123"}}
	repo := &fakeUnregisteredTagScanRepo{listResult: want}
	service := NewUnregisteredTagScanService(repo, nil)

	got, err := service.ListForOperator(context.Background(), auditModels.UnregisteredTagScanFilter{
		SchoolID:       &schoolID,
		OrganizationID: &orgID,
		UnresolvedOnly: true,
	})

	require.NoError(t, err)
	require.Equal(t, want, got)
	require.Equal(t, &schoolID, repo.listFilter.SchoolID)
	require.Equal(t, &orgID, repo.listFilter.OrganizationID)
	require.True(t, repo.listFilter.UnresolvedOnly)
}

func TestUnregisteredTagScanListForOperatorPropagatesRepositoryError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("list failed")
	repo := &fakeUnregisteredTagScanRepo{listErr: wantErr}
	service := NewUnregisteredTagScanService(repo, nil)

	got, err := service.ListForOperator(context.Background(), auditModels.UnregisteredTagScanFilter{})

	require.ErrorIs(t, err, wantErr)
	require.Nil(t, got)
}

func TestUnregisteredTagScanResolveValidatesIDs(t *testing.T) {
	t.Parallel()

	service := NewUnregisteredTagScanService(&fakeUnregisteredTagScanRepo{}, nil)

	_, err := service.Resolve(context.Background(), 0, 15, nil)
	require.ErrorContains(t, err, "scan ID is required")

	_, err = service.Resolve(context.Background(), 300, 0, nil)
	require.ErrorContains(t, err, "operator ID is required")
}

func TestUnregisteredTagScanResolveNormalizesNoteAndReturnsScan(t *testing.T) {
	t.Parallel()

	scan := &auditModels.UnregisteredTagScan{TagUID: "ABC123"}
	repo := &fakeUnregisteredTagScanRepo{resolveResult: scan}
	service := NewUnregisteredTagScanService(repo, nil)
	note := "  replacement issued  "

	got, err := service.Resolve(context.Background(), 300, 15, &note)

	require.NoError(t, err)
	require.Equal(t, scan, got)
	require.Equal(t, int64(300), repo.resolveID)
	require.Equal(t, int64(15), repo.resolveOpID)
	require.NotNil(t, repo.resolveNote)
	require.Equal(t, "replacement issued", *repo.resolveNote)
}

func TestUnregisteredTagScanResolvePropagatesRepositoryError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("resolve failed")
	repo := &fakeUnregisteredTagScanRepo{resolveErr: wantErr}
	service := NewUnregisteredTagScanService(repo, nil)

	got, err := service.Resolve(context.Background(), 300, 15, nil)

	require.ErrorIs(t, err, wantErr)
	require.Nil(t, got)
}

func TestUnregisteredTagScanDeleteOlderThanUsesDefaultRetention(t *testing.T) {
	t.Parallel()

	repo := &fakeUnregisteredTagScanRepo{deleteResult: 3}
	service := NewUnregisteredTagScanService(repo, nil)
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
	service := NewUnregisteredTagScanService(repo, nil)
	before := time.Now().AddDate(0, 0, -7)

	deleted, err := service.DeleteOlderThan(context.Background(), 7)

	after := time.Now().AddDate(0, 0, -7)
	require.ErrorIs(t, err, wantErr)
	require.Zero(t, deleted)
	require.False(t, repo.deleteCutoff.Before(before))
	require.False(t, repo.deleteCutoff.After(after))
}

func TestNormalizeNote(t *testing.T) {
	t.Parallel()

	note := "  assigned to replacement card  "

	require.Nil(t, strutil.TrimPtrToNil(nil))
	require.Nil(t, strutil.TrimPtrToNil(pointerToString("   ")))
	require.Equal(t, "assigned to replacement card", *strutil.TrimPtrToNil(&note))
}

func pointerToString(value string) *string {
	return &value
}
