package audit

import (
	"context"
	"errors"
	"testing"
	"time"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
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

type testTenantKey struct{}

type fakeAuditCommand struct{ repo *fakeUnregisteredTagScanRepo }

func (c fakeAuditCommand) Append(_ context.Context, event any) error {
	c.repo.created = event.(*auditModels.UnregisteredTagScan)
	return c.repo.createErr
}

type fakeOrganizationQuery struct {
	listByIDsFn func(context.Context, []int64) ([]organizationtenancy.Organization, error)
}

func (q *fakeOrganizationQuery) ListOrganizationsByID(ctx context.Context, ids []int64) ([]organizationtenancy.Organization, error) {
	if q.listByIDsFn != nil {
		return q.listByIDsFn(ctx, ids)
	}
	organizations := make([]organizationtenancy.Organization, 0, len(ids))
	for _, id := range ids {
		organizations = append(organizations, organizationtenancy.Organization{ID: id})
	}
	return organizations, nil
}

func newUnregisteredTagScanService(t *testing.T, repo *fakeUnregisteredTagScanRepo) UnregisteredTagScanService {
	return newUnregisteredTagScanServiceWithOrganizations(t, repo, &fakeOrganizationQuery{})
}

func newUnregisteredTagScanServiceWithOrganizations(t *testing.T, repo *fakeUnregisteredTagScanRepo, organizations OrganizationNameQuery) UnregisteredTagScanService {
	t.Helper()
	service, err := NewUnregisteredTagScanService(repo, fakeAuditCommand{repo: repo}, organizations, UnregisteredTagScanRuntime{
		TenantID: func(ctx context.Context) int64 {
			id, _ := ctx.Value(testTenantKey{}).(int64)
			return id
		},
		WithinAdmin: func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) },
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

func TestUnregisteredTagScanListForOperatorPassesFilter(t *testing.T) {
	t.Parallel()

	schoolID := int64(10)
	orgID := schoolID / 2
	want := []*auditModels.UnregisteredTagScan{{TagUID: "ABC123", OrganizationID: orgID}}
	repo := &fakeUnregisteredTagScanRepo{listResult: want}
	service := newUnregisteredTagScanServiceWithOrganizations(t, repo, &fakeOrganizationQuery{
		listByIDsFn: func(_ context.Context, ids []int64) ([]organizationtenancy.Organization, error) {
			require.Equal(t, []int64{orgID}, ids)
			return []organizationtenancy.Organization{{ID: orgID, Name: "Organization"}}, nil
		},
	})

	got, err := service.ListForOperator(context.Background(), auditModels.UnregisteredTagScanFilter{
		SchoolID:       &schoolID,
		OrganizationID: &orgID,
		UnresolvedOnly: true,
	})

	require.NoError(t, err)
	require.Equal(t, want, got)
	require.Equal(t, "Organization", got[0].OrganizationName)
	require.Equal(t, &schoolID, repo.listFilter.SchoolID)
	require.Equal(t, &orgID, repo.listFilter.OrganizationID)
	require.True(t, repo.listFilter.UnresolvedOnly)
}

func TestUnregisteredTagScanListForOperatorPropagatesRepositoryError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("list failed")
	repo := &fakeUnregisteredTagScanRepo{listErr: wantErr}
	service := newUnregisteredTagScanService(t, repo)

	got, err := service.ListForOperator(context.Background(), auditModels.UnregisteredTagScanFilter{})

	require.ErrorIs(t, err, wantErr)
	require.Nil(t, got)
}

func TestUnregisteredTagScanListForOperatorPropagatesOrganizationQueryError(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("organization query failed")
	repo := &fakeUnregisteredTagScanRepo{listResult: []*auditModels.UnregisteredTagScan{{OrganizationID: 42}}}
	service := newUnregisteredTagScanServiceWithOrganizations(t, repo, &fakeOrganizationQuery{
		listByIDsFn: func(context.Context, []int64) ([]organizationtenancy.Organization, error) {
			return nil, wantErr
		},
	})

	got, err := service.ListForOperator(context.Background(), auditModels.UnregisteredTagScanFilter{})

	require.ErrorIs(t, err, wantErr)
	require.Nil(t, got)
}

func TestUnregisteredTagScanResolveValidatesIDs(t *testing.T) {
	t.Parallel()

	service := newUnregisteredTagScanService(t, &fakeUnregisteredTagScanRepo{})

	_, err := service.Resolve(context.Background(), 0, 15, nil)
	require.ErrorContains(t, err, "scan ID is required")

	_, err = service.Resolve(context.Background(), 300, 0, nil)
	require.ErrorContains(t, err, "operator ID is required")
}

func TestUnregisteredTagScanResolveNormalizesNoteAndReturnsScan(t *testing.T) {
	t.Parallel()

	scan := &auditModels.UnregisteredTagScan{TagUID: "ABC123"}
	repo := &fakeUnregisteredTagScanRepo{resolveResult: scan}
	service := newUnregisteredTagScanService(t, repo)
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
	service := newUnregisteredTagScanService(t, repo)

	got, err := service.Resolve(context.Background(), 300, 15, nil)

	require.ErrorIs(t, err, wantErr)
	require.Nil(t, got)
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

func TestNormalizeNote(t *testing.T) {
	t.Parallel()

	note := "  assigned to replacement card  "

	require.Nil(t, trimPtrToNil(nil))
	require.Nil(t, trimPtrToNil(pointerToString("   ")))
	require.Equal(t, "assigned to replacement card", *trimPtrToNil(&note))
}

func pointerToString(value string) *string {
	return &value
}
