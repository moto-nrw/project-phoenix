package notifications

import (
	"context"
	"errors"
	"fmt"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

var errPushRepository = errors.New("push repository failed")

type recordingPushRepository struct {
	iot.PushSubscriptionRepository
	upserted        []*iot.PushSubscription
	deletedAccount  int64
	deletedEndpoint string
	deletedTenants  []int64
	reboundEndpoint string
	operations      []string
	err             error
	deleteParentErr error
	failAfter       int
	deleteFailAfter int
}

func (r *recordingPushRepository) Upsert(_ context.Context, sub *iot.PushSubscription) error {
	r.upserted = append(r.upserted, sub)
	r.operations = append(r.operations, fmt.Sprintf("upsert:%d", sub.TenantID))
	if r.failAfter > 0 && len(r.upserted) < r.failAfter {
		return nil
	}
	return r.err
}

func (r *recordingPushRepository) DeleteByEndpoint(ctx context.Context, accountID int64, endpoint string) error {
	r.deletedAccount = accountID
	r.deletedEndpoint = endpoint
	r.deletedTenants = append(r.deletedTenants, tenant.FromContext(ctx))
	if r.deleteFailAfter > 0 && len(r.deletedTenants) < r.deleteFailAfter {
		return nil
	}
	return r.err
}

func (r *recordingPushRepository) DeleteParentByEndpoint(_ context.Context, endpoint string) error {
	r.reboundEndpoint = endpoint
	r.operations = append(r.operations, "clear")
	return r.deleteParentErr
}

type accountTenantRepositoryStub struct {
	authModels.AccountTenantRepository
	mappings []authModels.AccountTenant
	err      error
	find     func(context.Context, int64) ([]authModels.AccountTenant, error)
}

func (r accountTenantRepositoryStub) FindActiveGuardianByAccountID(ctx context.Context, accountID int64) ([]authModels.AccountTenant, error) {
	if r.find != nil {
		return r.find(ctx, accountID)
	}
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
	t.Parallel()
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

		input := validPushInput()
		input.TokenFamilyID = "family-1"
		require.NoError(t, service.Subscribe(context.Background(), 42, input))
		require.Len(t, repo.upserted, 1)
		assert.Equal(t, int64(42), repo.upserted[0].AccountID)
		assert.Equal(t, iot.PushPortalStaff, repo.upserted[0].Portal)
		assert.Equal(t, "test-agent", repo.upserted[0].UserAgent)
		assert.Equal(t, "family-1", repo.upserted[0].TokenFamilyID)

		invalid := validPushInput()
		invalid.Endpoint = "http://fcm.googleapis.com/fcm/send/device"
		err := service.Subscribe(context.Background(), 42, invalid)
		require.ErrorIs(t, err, ErrInvalidPushSubscription)
		assert.ErrorContains(t, err, "endpoint must be an https URL")
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
	t.Parallel()
	t.Run("rejects subscribe without VAPID keys", func(t *testing.T) {
		service := NewPushSubscriptionService(nil, nil, nil, VAPIDConfig{}, nil)
		err := service.SubscribeParent(context.Background(), 42, validPushInput())
		require.ErrorIs(t, err, ErrWebPushNotConfigured)
	})

	db := testpkg.SetupTestDB(t)

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

	t.Run("reports mapping lookup errors and missing mappings", func(t *testing.T) {
		service := NewPushSubscriptionService(db, nil, accountTenantRepositoryStub{err: errPushRepository}, testVAPID(), nil)
		err := service.SubscribeParent(context.Background(), 42, validPushInput())
		require.ErrorIs(t, err, errPushRepository)
		assert.ErrorContains(t, err, "resolving guardian tenant mappings")

		service = NewPushSubscriptionService(db, nil, accountTenantRepositoryStub{}, testVAPID(), nil)
		err = service.SubscribeParent(context.Background(), 42, validPushInput())
		require.EqualError(t, err, "account has no active school mapping")
	})

	t.Run("stores and removes a parent subscription in each tenant", func(t *testing.T) {
		repo := &recordingPushRepository{}
		service := NewPushSubscriptionService(db, repo, mappings, testVAPID(), nil)

		require.NoError(t, service.SubscribeParent(context.Background(), 42, validPushInput()))
		require.Len(t, repo.upserted, 1)
		assert.Equal(t, tenantID, repo.upserted[0].TenantID)
		assert.Equal(t, iot.PushPortalParent, repo.upserted[0].Portal)
		assert.Equal(t, validPushInput().Endpoint, repo.reboundEndpoint)
		assert.Equal(t, []string{"clear", fmt.Sprintf("upsert:%d", tenantID)}, repo.operations)

		require.NoError(t, service.UnsubscribeParent(context.Background(), 42, validPushInput().Endpoint))
		assert.Equal(t, int64(42), repo.deletedAccount)
		assert.Equal(t, validPushInput().Endpoint, repo.deletedEndpoint)
	})

	t.Run("reports validation and tenant repository errors", func(t *testing.T) {
		repo := &recordingPushRepository{}
		service := NewPushSubscriptionService(db, repo, mappings, testVAPID(), nil)
		invalid := validPushInput()
		invalid.Endpoint = "invalid"
		err := service.SubscribeParent(context.Background(), 42, invalid)
		require.ErrorIs(t, err, ErrInvalidPushSubscription)
		assert.ErrorContains(t, err, "endpoint must be an https URL")

		repo.err = errPushRepository
		err = service.SubscribeParent(context.Background(), 42, validPushInput())
		require.ErrorIs(t, err, errPushRepository)
		assert.ErrorContains(t, err, "registering push subscription for tenant")

		err = service.UnsubscribeParent(context.Background(), 42, validPushInput().Endpoint)
		require.ErrorIs(t, err, errPushRepository)
		assert.ErrorContains(t, err, "removing push subscription for tenant")
	})

	t.Run("reports previous binding cleanup errors", func(t *testing.T) {
		repo := &recordingPushRepository{deleteParentErr: errPushRepository}
		service := NewPushSubscriptionService(db, repo, mappings, testVAPID(), nil)

		err := service.SubscribeParent(context.Background(), 42, validPushInput())
		require.ErrorIs(t, err, errPushRepository)
		assert.ErrorContains(t, err, "clearing previous parent push subscription bindings")
		assert.Empty(t, repo.upserted)
	})

	t.Run("reports unsubscribe mapping lookup errors", func(t *testing.T) {
		service := NewPushSubscriptionService(db, nil, accountTenantRepositoryStub{err: errPushRepository}, testVAPID(), nil)
		err := service.UnsubscribeParent(context.Background(), 42, validPushInput().Endpoint)
		require.ErrorIs(t, err, errPushRepository)
		assert.ErrorContains(t, err, "resolving guardian tenant mappings")
	})
}

func TestPushSubscriptionServiceParentFiltersNonGuardianMappings(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	account := testpkg.CreateTestAccount(t, db, "push-parent-mixed-roles")
	guardianTenantID := account.ID + 4000
	staffTenantID := account.ID + 4001
	testpkg.EnsureTestTenant(t, db, guardianTenantID)
	testpkg.EnsureTestTenant(t, db, staffTenantID)
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), `DELETE FROM platform.schools WHERE id IN (?, ?)`, guardianTenantID, staffTenantID)
		_, _ = db.ExecContext(context.Background(), `DELETE FROM platform.organizations WHERE id IN (?, ?)`, guardianTenantID, staffTenantID)
	})

	testpkg.MapAccountToTenant(t, db, account.ID, guardianTenantID)
	testpkg.MapAccountToTenant(t, db, account.ID, staffTenantID)

	var guardianRoleID, staffRoleID int64
	require.NoError(t, db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("name = ?", authModels.BaseRoleGuardian).
		Scan(context.Background(), &guardianRoleID))
	require.NoError(t, db.NewSelect().
		ColumnExpr("id").
		TableExpr("auth.roles").
		Where("name = ?", authModels.BaseRoleUser).
		Scan(context.Background(), &staffRoleID))

	_, err := db.ExecContext(context.Background(), `
		INSERT INTO auth.account_roles (account_id, role_id, tenant_id)
		VALUES (?, ?, ?), (?, ?, ?)`,
		account.ID, guardianRoleID, guardianTenantID,
		account.ID, staffRoleID, staffTenantID)
	require.NoError(t, err)

	repo := &recordingPushRepository{}
	service := NewPushSubscriptionService(
		db,
		repo,
		authRepo.NewAccountTenantRepository(db),
		testVAPID(),
		nil,
	)

	require.NoError(t, service.SubscribeParent(context.Background(), account.ID, validPushInput()))
	require.Len(t, repo.upserted, 1)
	assert.Equal(t, guardianTenantID, repo.upserted[0].TenantID)

	require.NoError(t, service.UnsubscribeParent(context.Background(), account.ID, validPushInput().Endpoint))
	assert.Equal(t, []int64{guardianTenantID}, repo.deletedTenants)
}

func TestPushSubscriptionServiceParentSubscribeIsAtomic(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	repo := &recordingPushRepository{err: errPushRepository, failAfter: 2}
	mappingLookupInTx := false
	mappings := accountTenantRepositoryStub{
		find: func(ctx context.Context, _ int64) ([]authModels.AccountTenant, error) {
			_, mappingLookupInTx = modelBase.TxFromContext(ctx)
			return []authModels.AccountTenant{
				{TenantID: 41},
				{TenantID: 42},
			}, nil
		},
	}
	service := NewPushSubscriptionService(db, repo, mappings, testVAPID(), nil)

	err = service.SubscribeParent(context.Background(), 77, validPushInput())

	require.ErrorIs(t, err, errPushRepository)
	assert.True(t, mappingLookupInTx)
	require.Len(t, repo.upserted, 2)
	assert.Equal(t, int64(41), repo.upserted[0].TenantID)
	assert.Equal(t, int64(42), repo.upserted[1].TenantID)
	require.NoError(t, mock.ExpectationsWereMet(), "a later tenant failure must roll back the whole registration")
}

func TestPushSubscriptionServiceParentUnsubscribeIsAtomic(t *testing.T) {
	t.Parallel()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_admin").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	repo := &recordingPushRepository{err: errPushRepository, deleteFailAfter: 2}
	mappingLookupInTx := false
	mappings := accountTenantRepositoryStub{
		find: func(ctx context.Context, _ int64) ([]authModels.AccountTenant, error) {
			_, mappingLookupInTx = modelBase.TxFromContext(ctx)
			return []authModels.AccountTenant{
				{TenantID: 41},
				{TenantID: 42},
			}, nil
		},
	}
	service := NewPushSubscriptionService(db, repo, mappings, testVAPID(), nil)

	err = service.UnsubscribeParent(context.Background(), 77, validPushInput().Endpoint)

	require.ErrorIs(t, err, errPushRepository)
	assert.True(t, mappingLookupInTx)
	assert.Equal(t, []int64{41, 42}, repo.deletedTenants)
	require.NoError(t, mock.ExpectationsWereMet(), "a later tenant failure must roll back the whole removal")
}
