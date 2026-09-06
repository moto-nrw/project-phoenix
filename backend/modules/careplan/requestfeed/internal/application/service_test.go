package application

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed"
	"github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	active       bool
	createdHash  string
	rotatedHash  string
	subscription domain.Subscription
	items        []domain.Item
	since        time.Time
	listAccess   domain.Access
}

func (s *fakeStore) Active(context.Context, int64, int64) (bool, error) { return s.active, nil }
func (s *fakeStore) Create(_ context.Context, _, _ int64, hash string) (bool, error) {
	if s.active {
		return false, nil
	}
	s.createdHash, s.active = hash, true
	return true, nil
}
func (s *fakeStore) Rotate(_ context.Context, _, _ int64, hash string) (bool, error) {
	if !s.active {
		return false, nil
	}
	s.rotatedHash = hash
	return true, nil
}
func (s *fakeStore) Resolve(_ context.Context, hash string) (domain.Subscription, bool, error) {
	return s.subscription, hash == s.createdHash, nil
}
func (s *fakeStore) List(_ context.Context, _ int64, since time.Time, access domain.Access) ([]domain.Item, error) {
	s.since, s.listAccess = since, access
	return s.items, nil
}

type fakeAccess struct{ value domain.Access }

func (a fakeAccess) Resolve(context.Context, int64, int64) (domain.Access, error) {
	return a.value, nil
}

type fakeTokens struct{}

const testRawToken = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func (fakeTokens) New() (string, string, error) { return testRawToken, "hash:" + testRawToken, nil }
func (fakeTokens) Hash(raw string) string       { return "hash:" + raw }

func newTestService(t *testing.T, store *fakeStore, access domain.Access, now time.Time) *Service {
	t.Helper()
	service, err := New(store, fakeAccess{value: access}, fakeTokens{}, func(ctx context.Context, fn func(context.Context) error) error {
		return fn(ctx)
	}, "https://moto-app.de", func() time.Time { return now })
	require.NoError(t, err)
	return service
}

func TestCreateReturnsTenantURLAndStoresOnlyHash(t *testing.T) {
	t.Parallel()
	store := &fakeStore{}
	service := newTestService(t, store, domain.Access{Active: true, GeneralRequests: true, Subdomain: "sonnenschule"}, time.Now())

	created, err := service.Provision(context.Background(), 7, 9)
	require.NoError(t, err)
	assert.Contains(t, created.URL, "https://sonnenschule.moto-app.de/api/request-feed/")
	raw := testRawToken
	assert.Equal(t, "https://sonnenschule.moto-app.de/api/request-feed/"+raw, created.URL)
	assert.Equal(t, "hash:"+raw, store.createdHash)
	assert.NotContains(t, created.URL, store.createdHash)
	assert.NotEqual(t, raw, store.createdHash)
}

func TestCreateRejectsGroupScopedAccess(t *testing.T) {
	t.Parallel()
	service := newTestService(t, &fakeStore{}, domain.Access{Active: true}, time.Now())
	_, err := service.Provision(context.Background(), 7, 9)
	assert.ErrorIs(t, err, requestfeed.ErrNotFound)
}

func TestFeedContainsOnlyGenericRequestMetadata(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 5, 12, 0, 0, 0, time.UTC)
	store := &fakeStore{
		active:       true,
		createdHash:  fakeTokens{}.Hash("raw-token"),
		subscription: domain.Subscription{TenantID: 7, AccountID: 9},
		items:        []domain.Item{{Kind: "master_data", ID: 41, CreatedAt: now.Add(-time.Hour)}},
	}
	access := domain.Access{Active: true, GeneralRequests: true, EnrollmentRequests: false, SchoolName: "Sonnenschule", Subdomain: "sonnenschule"}
	service := newTestService(t, store, access, now)

	feed, err := service.ByToken(context.Background(), "raw-token")
	require.NoError(t, err)
	assert.Contains(t, feed.XML, "Neue Anfrage: Stammdaten")
	assert.Contains(t, feed.XML, "urn:moto:parent-request:7:master_data:41")
	assert.Contains(t, feed.XML, "https://sonnenschule.moto-app.de/anfragen?tab=eltern")
	assert.NotContains(t, feed.XML, "raw-token")
	assert.Equal(t, now.Add(-30*24*time.Hour), store.since)
	assert.Equal(t, access, store.listAccess)
}

func TestFeedRechecksCurrentAccess(t *testing.T) {
	t.Parallel()
	store := &fakeStore{createdHash: fakeTokens{}.Hash("raw-token"), subscription: domain.Subscription{TenantID: 7, AccountID: 9}}
	service := newTestService(t, store, domain.Access{Active: false, GeneralRequests: true}, time.Now())
	_, err := service.ByToken(context.Background(), "raw-token")
	assert.ErrorIs(t, err, requestfeed.ErrNotFound)
}

func TestRotateInvalidatesPreviousHash(t *testing.T) {
	t.Parallel()
	store := &fakeStore{active: true, createdHash: fakeTokens{}.Hash("old")}
	service := newTestService(t, store, domain.Access{Active: true, EnrollmentRequests: true, Subdomain: "sonnenschule"}, time.Now())
	created, err := service.Rotate(context.Background(), 7, 9)
	require.NoError(t, err)
	assert.NotEmpty(t, store.rotatedHash)
	assert.NotEqual(t, store.createdHash, store.rotatedHash)
	assert.Contains(t, created.URL, "sonnenschule.moto-app.de")
}
