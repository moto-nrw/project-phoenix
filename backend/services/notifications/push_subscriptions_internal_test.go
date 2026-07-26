package notifications

import (
	"context"
	"errors"
	"testing"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errPushRepository = errors.New("push repository failed")

type recordingPushRepository struct {
	iot.PushSubscriptionRepository
	upserted        []*iot.PushSubscription
	upsertTenants   []int64
	deletedAccount  int64
	deletedEndpoint string
	err             error
}

func (r *recordingPushRepository) Upsert(ctx context.Context, sub *iot.PushSubscription) error {
	r.upserted = append(r.upserted, sub)
	r.upsertTenants = append(r.upsertTenants, tenant.FromContext(ctx))
	return r.err
}

func (r *recordingPushRepository) DeleteByEndpoint(_ context.Context, accountID int64, endpoint string) error {
	r.deletedAccount = accountID
	r.deletedEndpoint = endpoint
	return r.err
}

type accountTenantRepositoryStub struct {
	authModels.AccountTenantRepository
	mappings []authModels.AccountTenant
	err      error
}

func (r accountTenantRepositoryStub) FindActiveByAccountID(context.Context, int64) ([]authModels.AccountTenant, error) {
	return r.mappings, r.err
}

func validPushInput() PushSubscriptionInput {
	return PushSubscriptionInput{
		Endpoint:  "https://fcm.googleapis.com/fcm/send/device",
		P256dh:    "p256dh-key",
		Auth:      "auth-key",
		UserAgent: "test-agent",
	}
}

func TestPushSubscriptionServiceStaffLifecycle(t *testing.T) {
	t.Run("reports configuration state", func(t *testing.T) {
		unconfigured := NewPushSubscriptionService(nil, nil, nil, VAPIDConfig{}, nil)
		_, err := unconfigured.PublicKey()
		require.ErrorIs(t, err, ErrWebPushNotConfigured)

		configured := NewPushSubscriptionService(nil, nil, nil, testVAPID(), nil)
		key, err := configured.PublicKey()
		require.NoError(t, err)
		assert.Equal(t, "pub", key)
	})

	t.Run("rejects subscribe without VAPID keys", func(t *testing.T) {
		service := NewPushSubscriptionService(nil, nil, nil, VAPIDConfig{}, nil)
		err := service.Subscribe(context.Background(), 42, validPushInput())
		require.ErrorIs(t, err, ErrWebPushNotConfigured)
	})

	t.Run("validates and stores staff subscription", func(t *testing.T) {
		repo := &recordingPushRepository{}
		service := NewPushSubscriptionService(nil, repo, nil, testVAPID(), nil)

		require.NoError(t, service.Subscribe(context.Background(), 42, validPushInput()))
		require.Len(t, repo.upserted, 1)
		assert.Equal(t, int64(42), repo.upserted[0].AccountID)
		assert.Equal(t, iot.PushPortalStaff, repo.upserted[0].Portal)
		assert.Equal(t, "test-agent", repo.upserted[0].UserAgent)

		invalid := validPushInput()
		invalid.Endpoint = "http://fcm.googleapis.com/fcm/send/device"
		require.EqualError(t, service.Subscribe(context.Background(), 42, invalid), "endpoint must be an https URL")
	})

	t.Run("forwards repository errors and unsubscribe", func(t *testing.T) {
		repo := &recordingPushRepository{err: errPushRepository}
		service := NewPushSubscriptionService(nil, repo, nil, testVAPID(), nil)

		require.ErrorIs(t, service.Subscribe(context.Background(), 42, validPushInput()), errPushRepository)
		require.ErrorIs(t, service.Unsubscribe(context.Background(), 42, "https://fcm.googleapis.com/fcm/send/device"), errPushRepository)
		assert.Equal(t, int64(42), repo.deletedAccount)
		assert.Equal(t, "https://fcm.googleapis.com/fcm/send/device", repo.deletedEndpoint)
	})
}

func TestPushSubscriptionServiceParentLifecycle(t *testing.T) {
	t.Run("rejects subscribe without VAPID keys", func(t *testing.T) {
		service := NewPushSubscriptionService(nil, nil, nil, VAPIDConfig{}, nil)
		err := service.SubscribeParent(context.Background(), 42, validPushInput())
		require.ErrorIs(t, err, ErrWebPushNotConfigured)
	})

	t.Run("reports mapping lookup errors and missing mappings", func(t *testing.T) {
		service := NewPushSubscriptionService(nil, nil, accountTenantRepositoryStub{err: errPushRepository}, testVAPID(), nil)
		err := service.SubscribeParent(context.Background(), 42, validPushInput())
		require.ErrorIs(t, err, errPushRepository)
		assert.ErrorContains(t, err, "resolving guardian tenant mappings")

		service = NewPushSubscriptionService(nil, nil, accountTenantRepositoryStub{}, testVAPID(), nil)
		err = service.SubscribeParent(context.Background(), 42, validPushInput())
		require.EqualError(t, err, "account has no active school mapping")
	})

	db := testpkg.SetupTestDB(t)
	defer func() { _ = db.Close() }()

	var tenantID int64
	require.NoError(t, db.NewSelect().
		ColumnExpr("id").
		TableExpr("platform.schools").
		OrderExpr("id ASC").
		Limit(1).
		Scan(context.Background(), &tenantID))
	require.Positive(t, tenantID)

	mappings := accountTenantRepositoryStub{
		mappings: []authModels.AccountTenant{{TenantID: tenantID}},
	}

	t.Run("stores and removes a parent subscription in each tenant", func(t *testing.T) {
		repo := &recordingPushRepository{}
		service := NewPushSubscriptionService(db, repo, mappings, testVAPID(), nil)

		require.NoError(t, service.SubscribeParent(context.Background(), 42, validPushInput()))
		require.Len(t, repo.upserted, 1)
		assert.Equal(t, tenantID, repo.upserted[0].TenantID)
		assert.Equal(t, tenantID, repo.upsertTenants[0])
		assert.Equal(t, iot.PushPortalParent, repo.upserted[0].Portal)

		require.NoError(t, service.UnsubscribeParent(context.Background(), 42, validPushInput().Endpoint))
		assert.Equal(t, int64(42), repo.deletedAccount)
		assert.Equal(t, validPushInput().Endpoint, repo.deletedEndpoint)
	})

	t.Run("reports validation and tenant repository errors", func(t *testing.T) {
		repo := &recordingPushRepository{}
		service := NewPushSubscriptionService(db, repo, mappings, testVAPID(), nil)
		invalid := validPushInput()
		invalid.Endpoint = "invalid"
		require.EqualError(t, service.SubscribeParent(context.Background(), 42, invalid), "endpoint must be an https URL")

		repo.err = errPushRepository
		err := service.SubscribeParent(context.Background(), 42, validPushInput())
		require.ErrorIs(t, err, errPushRepository)
		assert.ErrorContains(t, err, "registering push subscription for tenant")

		err = service.UnsubscribeParent(context.Background(), 42, validPushInput().Endpoint)
		require.ErrorIs(t, err, errPushRepository)
		assert.ErrorContains(t, err, "removing push subscription for tenant")
	})

	t.Run("reports unsubscribe mapping lookup errors", func(t *testing.T) {
		service := NewPushSubscriptionService(db, nil, accountTenantRepositoryStub{err: errPushRepository}, testVAPID(), nil)
		err := service.UnsubscribeParent(context.Background(), 42, validPushInput().Endpoint)
		require.ErrorIs(t, err, errPushRepository)
		assert.ErrorContains(t, err, "resolving guardian tenant mappings")
	})
}
