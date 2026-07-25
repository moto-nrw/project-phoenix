package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	platformRepo "github.com/moto-nrw/project-phoenix/database/repositories/platform"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// seedOutboxRow inserts a dispatched outbox row for a tenant and returns it.
func seedOutboxRow(t *testing.T, db *bun.DB, tenantID int64, messageID string) *platformModels.EmailOutbox {
	t.Helper()

	row := &platformModels.EmailOutbox{
		Kind:        platformModels.EmailKindGuardianInvitation,
		Payload:     map[string]any{},
		Status:      platformModels.EmailOutboxStatusSent,
		NextRetryAt: time.Now(),
	}
	row.SetTenantID(tenantID)

	_, err := db.NewInsert().
		Model(row).
		ModelTableExpr(`platform.email_outbox AS "email_outbox"`).
		Exec(context.Background())
	require.NoError(t, err)

	repo := platformRepo.NewEmailOutboxRepository(db)
	require.NoError(t, repo.SetDispatchIdentifiers(context.Background(), row.ID, messageID, nil))
	return row
}

// TestApplyDeliveryStatus_IsMonotone walks the precedence lattice against the
// real guarded UPDATE. This is the single mechanism that makes duplicate and
// out-of-order provider deliveries safe, so it is exercised end to end rather
// than only in the pure rank test.
func TestApplyDeliveryStatus_IsMonotone(t *testing.T) {
	db := SetupTestDB(t)
	defer func() { _ = db.Close() }()

	scope := NewTenantScope(t, db)
	defer CleanupTenantTestData(t, db, scope.TenantID)

	repo := platformRepo.NewEmailOutboxRepository(db)
	ctx := context.Background()

	base := time.Now().Truncate(time.Second) //nolint:forbidigo // second-alignment for stable timestamp comparison

	tests := []struct {
		name        string
		first       platformModels.DeliveryTransition
		second      platformModels.DeliveryTransition
		wantApplied bool
		wantStatus  string
	}{
		{
			// The 452 4.2.2 case from the issue: a delay arriving after the
			// delivery confirmation must not undo it.
			name:        "late deferred does not downgrade delivered",
			first:       transition(platformModels.EmailDeliveryStatusDelivered, base.Add(10*time.Minute)),
			second:      transition(platformModels.EmailDeliveryStatusDeferred, base.Add(5*time.Minute)),
			wantApplied: false,
			wantStatus:  platformModels.EmailDeliveryStatusDelivered,
		},
		{
			name:        "delivered supersedes deferred",
			first:       transition(platformModels.EmailDeliveryStatusDeferred, base),
			second:      transition(platformModels.EmailDeliveryStatusDelivered, base.Add(time.Minute)),
			wantApplied: true,
			wantStatus:  platformModels.EmailDeliveryStatusDelivered,
		},
		{
			// A complaint outranks delivery: the mail arrived AND was
			// rejected, and that is the fact operations must act on.
			name:        "complaint supersedes delivered",
			first:       transition(platformModels.EmailDeliveryStatusDelivered, base),
			second:      transition(platformModels.EmailDeliveryStatusComplained, base.Add(time.Minute)),
			wantApplied: true,
			wantStatus:  platformModels.EmailDeliveryStatusComplained,
		},
		{
			name:        "hard bounce outranks everything",
			first:       transition(platformModels.EmailDeliveryStatusComplained, base),
			second:      transition(platformModels.EmailDeliveryStatusBounced, base.Add(time.Minute)),
			wantApplied: true,
			wantStatus:  platformModels.EmailDeliveryStatusBounced,
		},
		{
			name:        "same rank with older timestamp is rejected",
			first:       transition(platformModels.EmailDeliveryStatusDeferred, base.Add(10*time.Minute)),
			second:      transition(platformModels.EmailDeliveryStatusDeferred, base.Add(time.Minute)),
			wantApplied: false,
			wantStatus:  platformModels.EmailDeliveryStatusDeferred,
		},
		{
			name:        "same rank with newer timestamp is applied",
			first:       transition(platformModels.EmailDeliveryStatusDeferred, base),
			second:      transition(platformModels.EmailDeliveryStatusDeferred, base.Add(10*time.Minute)),
			wantApplied: true,
			wantStatus:  platformModels.EmailDeliveryStatusDeferred,
		},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row := seedOutboxRow(t, db, scope.TenantID, messageIDFor(scope.TenantID, i))

			applied, err := repo.ApplyDeliveryStatus(ctx, row.ID, tt.first)
			require.NoError(t, err)
			require.True(t, applied, "the first transition always applies")

			applied, err = repo.ApplyDeliveryStatus(ctx, row.ID, tt.second)
			require.NoError(t, err)
			assert.Equal(t, tt.wantApplied, applied)

			stored, err := repo.FindByID(ctx, row.ID)
			require.NoError(t, err)
			assert.Equal(t, tt.wantStatus, stored.DeliveryStatus)
		})
	}
}

func transition(status string, at time.Time) platformModels.DeliveryTransition {
	return platformModels.DeliveryTransition{
		Status:     status,
		Rank:       rankFor(status),
		OccurredAt: at,
		Detail:     "",
	}
}

// rankFor mirrors the service-side lattice. Duplicated deliberately: if the
// service's ranks ever change, this test should fail rather than silently
// follow along.
func rankFor(status string) int {
	switch status {
	case platformModels.EmailDeliveryStatusUnknown:
		return 0
	case platformModels.EmailDeliveryStatusQueued:
		return 10
	case platformModels.EmailDeliveryStatusDeferred:
		return 20
	case platformModels.EmailDeliveryStatusDelivered:
		return 30
	case platformModels.EmailDeliveryStatusSoftBounced:
		return 40
	case platformModels.EmailDeliveryStatusSuppressed:
		return 50
	case platformModels.EmailDeliveryStatusComplained:
		return 60
	case platformModels.EmailDeliveryStatusBounced:
		return 70
	default:
		return 0
	}
}

func messageIDFor(tenantID int64, i int) string {
	return fmt.Sprintf("ob.%d.test%d.uuid@test.local", tenantID, i)
}

// A replayed webhook body must be stored once and applied once. The unique
// index is the wire-level idempotency guard.
func TestCreateIfNew_RejectsDuplicateProviderEvent(t *testing.T) {
	db := SetupTestDB(t)
	defer func() { _ = db.Close() }()

	scope := NewTenantScope(t, db)
	defer CleanupTenantTestData(t, db, scope.TenantID)

	row := seedOutboxRow(t, db, scope.TenantID, fmt.Sprintf("ob.%d.dup.uuid@test.local", scope.TenantID))
	repo := platformRepo.NewEmailDeliveryEventRepository(db)
	ctx := context.Background()

	build := func() *platformModels.EmailDeliveryEvent {
		event := &platformModels.EmailDeliveryEvent{
			OutboxID:        row.ID,
			Provider:        "generic",
			ProviderEventID: "evt_duplicate_1",
			EventType:       platformModels.DeliveryEventDelivered,
			OccurredAt:      time.Now(),
		}
		event.SetTenantID(scope.TenantID)
		return event
	}

	inserted, err := repo.CreateIfNew(ctx, build())
	require.NoError(t, err)
	assert.True(t, inserted)

	inserted, err = repo.CreateIfNew(ctx, build())
	require.NoError(t, err)
	assert.False(t, inserted, "a replayed event must not be stored twice")

	events, err := repo.ListByOutboxIDs(ctx, []int64{row.ID})
	require.NoError(t, err)
	assert.Len(t, events, 1)
}

// The webhook resolves the tenant from the outbox row, which is only possible
// because the lookup runs cross-tenant under phoenix_admin.
func TestFindByMessageID_ResolvesAcrossTenants(t *testing.T) {
	db := SetupTestDB(t)
	defer func() { _ = db.Close() }()

	scopeA := NewTenantScope(t, db)
	defer CleanupTenantTestData(t, db, scopeA.TenantID)
	scopeB := NewTenantScope(t, db)
	defer CleanupTenantTestData(t, db, scopeB.TenantID)

	messageID := fmt.Sprintf("ob.%d.xtenant.uuid@test.local", scopeB.TenantID)
	row := seedOutboxRow(t, db, scopeB.TenantID, messageID)

	repo := platformRepo.NewEmailOutboxRepository(db)

	// Ambient context is tenant A; the admin lookup must still find B's row,
	// because the webhook has no tenant of its own to scope by.
	err := tenant.WithAdminTx(tenant.WithTenantID(context.Background(), scopeA.TenantID), db,
		func(adminCtx context.Context, _ bun.Tx) error {
			found, err := repo.FindByMessageID(adminCtx, messageID)
			if err != nil {
				return err
			}
			require.NotNil(t, found)
			assert.Equal(t, row.ID, found.ID)
			assert.Equal(t, scopeB.TenantID, found.GetTenantID())
			return nil
		})
	require.NoError(t, err)
}

// An unmatched message is a normal outcome on a shared provider account and
// must not surface as an error — otherwise the webhook answers 500 and the
// provider retries forever.
func TestFindByMessageID_UnmatchedIsNotAnError(t *testing.T) {
	db := SetupTestDB(t)
	defer func() { _ = db.Close() }()

	repo := platformRepo.NewEmailOutboxRepository(db)
	found, err := repo.FindByMessageID(context.Background(), "ob.999999.does.not.exist@test.local")

	require.NoError(t, err)
	assert.Nil(t, found)
}
