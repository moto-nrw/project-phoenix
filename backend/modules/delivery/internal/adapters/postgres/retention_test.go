package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// A guardian welcome cleared by the retention keeps its key: enqueuing the
// welcome again is refused as a reuse, so nobody is welcomed twice (#3534,
// #3955).
func TestRetentionKeepsTheWelcomeKeyWithoutItsContent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	key := "guardian_welcome:42"
	relatedType := "guardian_profile"
	relatedID := int64(42)
	welcome := func() domain.Intent {
		return domain.Intent{
			TenantID: testpkg.Tenant(t), Transport: domain.TransportEmail, Template: "guardian_welcome", IdempotencyKey: &key,
			RelatedEntityType: &relatedType, RelatedEntityID: &relatedID,
			Recipient: json.RawMessage(`{"address":"guardian@example.com"}`), Payload: json.RawMessage(`{"name":"Ada"}`),
			Status: "pending", NextRetryAt: time.Now().Add(-time.Minute),
		}
	}
	var first domain.Enqueued
	require.NoError(t, tenant.WithTenantTx(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		var err error
		first, err = store.Enqueue(txCtx, welcome())
		return err
	}))
	_, err := db.ExecContext(context.Background(), `UPDATE platform.email_outbox
		SET status = 'sent', sent_at = CURRENT_TIMESTAMP - INTERVAL '31 days' WHERE id = ?`, first.ID)
	require.NoError(t, err)

	var deleted, cleared int
	before := time.Now().Add(-domain.RetentionWindow)
	require.NoError(t, tenant.WithTenantTx(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		if deleted, err = store.DeleteExpiredEmailIntents(txCtx, testpkg.Tenant(t), before, domain.KeyOnlyTemplates); err != nil {
			return err
		}
		cleared, err = store.ClearExpiredKeyOnlyEmailIntents(txCtx, testpkg.Tenant(t), before, domain.KeyOnlyTemplates)
		return err
	}))
	assert.Zero(t, deleted, "the welcome is not deleted")
	assert.Equal(t, 1, cleared)

	err = tenant.WithTenantTx(ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		_, err := store.Enqueue(txCtx, welcome())
		return err
	})
	require.ErrorIs(t, err, domain.ErrIdempotencyConflict, "the cleared row still claims the key")
	var rows int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM platform.email_outbox WHERE tenant_id = ? AND idempotency_key = ?`,
		testpkg.Tenant(t), key).Scan(context.Background(), &rows))
	assert.Equal(t, 1, rows)

	var retainedType *string
	var retainedID *int64
	var recipient, payload string
	require.NoError(t, db.NewRaw(`SELECT related_entity_type, related_entity_id, recipient::text, payload::text
		FROM platform.email_outbox WHERE tenant_id = ? AND idempotency_key = ?`, testpkg.Tenant(t), key).
		Scan(context.Background(), &retainedType, &retainedID, &recipient, &payload))
	assert.Nil(t, retainedType, "the retained key keeps no guardian profile type")
	assert.Nil(t, retainedID, "the retained key keeps no guardian profile ID")
	assert.Equal(t, "{}", recipient)
	assert.Equal(t, "{}", payload)
}
