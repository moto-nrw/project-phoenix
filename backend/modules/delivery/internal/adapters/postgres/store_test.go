package postgres_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	postgresadapter "github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func testStore(t *testing.T, db *bun.DB) (*postgresadapter.Store, context.Context) {
	t.Helper()
	ctx := testpkg.WithTenantRuntime(t, context.Background(), db)
	store := postgresadapter.New(
		func(ctx context.Context, expectedTenantID int64) (bun.IDB, error) {
			tx, _ := tenant.TransactionFromContext(ctx)
			return tx.(bun.Tx), nil
		},
		func(ctx context.Context, callback func(context.Context, bun.IDB) error) error {
			return tenant.WithAdminTx(ctx, db, func(txCtx context.Context, tx bun.Tx) error { return callback(txCtx, tx) })
		},
	)
	return store, ctx
}

func enqueueEmail(t *testing.T, db *bun.DB, store *postgresadapter.Store, ctx context.Context, key string) domain.Enqueued {
	t.Helper()
	var enqueued domain.Enqueued
	err := tenant.WithTenantTx(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		var err error
		enqueued, err = store.Enqueue(txCtx, domain.Intent{
			TenantID: testpkg.Tenant(t), Transport: domain.TransportEmail, Template: "welcome", IdempotencyKey: &key,
			Recipient: json.RawMessage(`{"address":"guardian@example.com"}`), Payload: json.RawMessage(`{"name":"Ada"}`),
			Status: "pending", NextRetryAt: time.Now().Add(-time.Minute),
		})
		return err
	})
	require.NoError(t, err)
	return enqueued
}

func TestStoreEnqueueIsIdempotentPerTenant(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)

	first := enqueueEmail(t, db, store, ctx, "welcome:guardian")
	second := enqueueEmail(t, db, store, ctx, "welcome:guardian")

	assert.Positive(t, first.ID)
	assert.Equal(t, first.ID, second.ID)
	assert.False(t, first.Duplicate)
	assert.True(t, second.Duplicate)
}

func TestStoreRejectsIdempotencyKeyReuseWithDifferentSnapshot(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	enqueueEmail(t, db, store, ctx, "conflicting-intent")

	err := tenant.WithTenantTx(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		key := "conflicting-intent"
		_, err := store.Enqueue(txCtx, domain.Intent{
			TenantID: testpkg.Tenant(t), Transport: domain.TransportEmail, Template: "welcome", IdempotencyKey: &key,
			Recipient: json.RawMessage(`{"address":"different@example.com"}`), Payload: json.RawMessage(`{"name":"Ada"}`),
			Status: "pending", NextRetryAt: time.Now(),
		})
		return err
	})
	require.Error(t, err)
	assert.True(t, errors.Is(err, domain.ErrIdempotencyConflict))
}

func TestStoreCrashBeforeSendReclaimsExpiredLeaseAndFencesStaleWorker(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	enqueued := enqueueEmail(t, db, store, ctx, "lease-reclaim")
	base := time.Now()

	first, err := store.Claim(ctx, domain.TransportEmail, 1, base, base.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, first, 1)
	assert.Equal(t, enqueued.ID, first[0].ID)

	second, err := store.Claim(ctx, domain.TransportEmail, 1, base.Add(2*time.Minute), base.Add(3*time.Minute))
	require.NoError(t, err)
	require.Len(t, second, 1)

	finalized, err := store.FinalizeSent(ctx, domain.TransportEmail, enqueued.ID, *first[0].LeaseToken, json.RawMessage(`{"id":"stale"}`), base.Add(2*time.Minute))
	require.NoError(t, err)
	assert.False(t, finalized)
	finalized, err = store.FinalizeSent(ctx, domain.TransportEmail, enqueued.ID, *second[0].LeaseToken, json.RawMessage(`{"id":"current"}`), base.Add(2*time.Minute))
	require.NoError(t, err)
	assert.True(t, finalized)
}

func TestStoreCancellationLeavesClaimedWorkOwnedByWorker(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	relatedType := "announcement"
	relatedID := testpkg.Tenant(t)
	key := "cancel-claimed"
	var enqueued domain.Enqueued
	err := tenant.WithTenantTx(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		var err error
		enqueued, err = store.Enqueue(txCtx, domain.Intent{
			TenantID: testpkg.Tenant(t), Transport: domain.TransportEmail, Template: "announcement", IdempotencyKey: &key,
			RelatedEntityType: &relatedType, RelatedEntityID: &relatedID,
			Recipient: json.RawMessage(`{"address":"guardian@example.com"}`), Payload: json.RawMessage(`{}`),
			Status: "pending", NextRetryAt: time.Now().Add(-time.Minute),
		})
		return err
	})
	require.NoError(t, err)
	claimed, err := store.Claim(ctx, domain.TransportEmail, 1, time.Now(), time.Now().Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	err = tenant.WithTenantTx(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		count, err := store.Cancel(txCtx, testpkg.Tenant(t), domain.TransportEmail, relatedType, relatedID, "retracted", time.Now())
		assert.Zero(t, count)
		return err
	})
	require.NoError(t, err)
	finalized, err := store.FinalizeSent(ctx, domain.TransportEmail, enqueued.ID, *claimed[0].LeaseToken, json.RawMessage(`{}`), time.Now())
	require.NoError(t, err)
	assert.True(t, finalized)
}

func TestStoreRejectsCrossTenantEmailOutboxAttachment(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	tenantA := testpkg.NewTenantScope(t, db)
	tenantB := testpkg.NewTenantScope(t, db)
	relatedType, relatedID := "announcement", int64(41)
	address := "guardian@example.test"

	var deliveryID, outboxID int64
	err := tenant.WithTenantTx(ctx, db, tenantA.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		if err := store.ReplaceEmailDeliveries(txCtx, tenantA.TenantID, relatedType, relatedID, []domain.EmailDelivery{{
			RecipientEmail: &address, Reachability: "ok",
		}}); err != nil {
			return err
		}
		rows, err := store.EmailDeliveryStatuses(txCtx, tenantA.TenantID, relatedType, relatedID)
		if err != nil {
			return err
		}
		if len(rows) != 1 {
			return fmt.Errorf("expected one email delivery, got %d", len(rows))
		}
		deliveryID = rows[0].DeliveryID
		return nil
	})
	require.NoError(t, err)

	err = tenant.WithTenantTx(ctx, db, tenantB.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		key := "other-tenant-outbox"
		stored, enqueueErr := store.Enqueue(txCtx, domain.Intent{
			TenantID: tenantB.TenantID, Transport: domain.TransportEmail, Template: "announcement", IdempotencyKey: &key,
			Recipient: json.RawMessage(`{"address":"guardian@example.test"}`), Payload: json.RawMessage(`{}`), NextRetryAt: time.Now(),
		})
		outboxID = stored.ID
		return enqueueErr
	})
	require.NoError(t, err)

	err = tenant.WithTenantTx(ctx, db, tenantA.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		return store.AttachEmailOutbox(txCtx, tenantA.TenantID, deliveryID, outboxID)
	})
	require.EqualError(t, err, "delivery postgres: email delivery not found")
}

func TestStoreProviderCancellationFinalizesUnderLeaseToken(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	enqueued := enqueueEmail(t, db, store, ctx, "provider-cancelled")
	claimed, err := store.Claim(ctx, domain.TransportEmail, 1, time.Now(), time.Now().Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, claimed, 1)

	finalized, err := store.FinalizeCancelled(ctx, domain.TransportEmail, enqueued.ID, *claimed[0].LeaseToken, "guardian access revoked", time.Now())
	require.NoError(t, err)
	require.True(t, finalized)

	stale, err := store.FinalizeSent(ctx, domain.TransportEmail, enqueued.ID, *claimed[0].LeaseToken, json.RawMessage(`{}`), time.Now())
	require.NoError(t, err)
	assert.False(t, stale)
}

func TestStoreStatusReadTracksDeliveryState(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	tenantID := testpkg.Tenant(t)
	relatedType, relatedID := "announcement", tenantID
	key := "status-read"
	var enqueued domain.Enqueued
	err := tenant.WithTenantTx(ctx, db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		var enqueueErr error
		enqueued, enqueueErr = store.Enqueue(txCtx, domain.Intent{
			TenantID: tenantID, Transport: domain.TransportEmail, Template: "announcement", IdempotencyKey: &key,
			RelatedEntityType: &relatedType, RelatedEntityID: &relatedID,
			Recipient: json.RawMessage(`{"address":"guardian@example.com"}`), Payload: json.RawMessage(`{}`),
			Status: "pending", NextRetryAt: time.Now().Add(-time.Minute),
		})
		return enqueueErr
	})
	require.NoError(t, err)

	assertStatus := func(expected string) {
		t.Helper()
		err := tenant.WithTenantTx(ctx, db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			rows, statusErr := store.Statuses(txCtx, tenantID, domain.TransportEmail, relatedType, relatedID)
			require.NoError(t, statusErr)
			require.Len(t, rows, 1)
			assert.Equal(t, expected, rows[0].Status)
			return nil
		})
		require.NoError(t, err)
	}
	assertStatus("pending")

	claimed, err := store.Claim(ctx, domain.TransportEmail, 1, time.Now(), time.Now().Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	finalized, err := store.FinalizeSent(ctx, domain.TransportEmail, enqueued.ID, *claimed[0].LeaseToken, json.RawMessage(`{"message_id":"provider-1"}`), time.Now())
	require.NoError(t, err)
	require.True(t, finalized)
	assertStatus("sent")
}

func TestStoreEmailDeliveryStatusJoinsOutboxState(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	tenantID := testpkg.Tenant(t)
	relatedType, relatedID := "announcement", tenantID
	key := "delivery-status-join"
	address := "guardian@example.test"
	var enqueued domain.Enqueued
	err := tenant.WithTenantTx(ctx, db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		var enqueueErr error
		enqueued, enqueueErr = store.Enqueue(txCtx, domain.Intent{
			TenantID: tenantID, Transport: domain.TransportEmail, Template: "announcement", IdempotencyKey: &key,
			RelatedEntityType: &relatedType, RelatedEntityID: &relatedID,
			Recipient: json.RawMessage(`{"address":"guardian@example.test"}`), Payload: json.RawMessage(`{}`),
			Status: "pending", NextRetryAt: time.Now().Add(-time.Minute),
		})
		if enqueueErr != nil {
			return enqueueErr
		}
		return store.ReplaceEmailDeliveries(txCtx, tenantID, relatedType, relatedID, []domain.EmailDelivery{{
			OutboxID: &enqueued.ID, RecipientEmail: &address, Reachability: "ok",
		}})
	})
	require.NoError(t, err)

	assertEmailDeliveryState := func(expected string, attempts int) {
		t.Helper()
		err := tenant.WithTenantTx(ctx, db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			rows, statusErr := store.EmailDeliveryStatuses(txCtx, tenantID, relatedType, relatedID)
			require.NoError(t, statusErr)
			require.Len(t, rows, 1)
			assert.Equal(t, expected, rows[0].EmailStatus)
			assert.Equal(t, attempts, rows[0].Attempts)
			return nil
		})
		require.NoError(t, err)
	}
	assertEmailDeliveryState("pending", 0)

	claimed, err := store.Claim(ctx, domain.TransportEmail, 1, time.Now(), time.Now().Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	result, err := store.FinalizeFailure(ctx, domain.TransportEmail, enqueued.ID, *claimed[0].LeaseToken, 1, "mailbox rejected", time.Now(), 1)
	require.NoError(t, err)
	require.True(t, result.Finalized)
	assertEmailDeliveryState("failed", 1)
}

func TestStoreStatusReadIsTenantIsolatedByRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, runtimeCtx := testStore(t, db)
	tenantA := testpkg.NewTenantScope(t, db)
	tenantB := testpkg.NewTenantScope(t, db)
	relatedType, relatedID := "request", int64(77)

	for _, scope := range []testpkg.TenantScope{tenantA, tenantB} {
		scope := scope
		err := tenant.WithTenantTx(runtimeCtx, db, scope.TenantID, func(txCtx context.Context, _ bun.Tx) error {
			key := fmt.Sprintf("tenant-isolation:%d", scope.TenantID)
			_, err := store.Enqueue(txCtx, domain.Intent{
				TenantID: scope.TenantID, Transport: domain.TransportEmail, Template: "request", IdempotencyKey: &key,
				RelatedEntityType: &relatedType, RelatedEntityID: &relatedID,
				Recipient: json.RawMessage(`{"address":"guardian@example.com"}`), Payload: json.RawMessage(`{}`),
				Status: "pending", NextRetryAt: time.Now(),
			})
			return err
		})
		require.NoError(t, err)
	}

	err := tenant.WithTenantTx(runtimeCtx, db, tenantA.TenantID, func(txCtx context.Context, _ bun.Tx) error {
		own, err := store.Statuses(txCtx, tenantA.TenantID, domain.TransportEmail, relatedType, relatedID)
		require.NoError(t, err)
		require.Len(t, own, 1)
		other, err := store.Statuses(txCtx, tenantB.TenantID, domain.TransportEmail, relatedType, relatedID)
		require.NoError(t, err)
		assert.Empty(t, other, "tenant A transaction must not read tenant B even with tenant B in the predicate")
		return nil
	})
	require.NoError(t, err)
}
