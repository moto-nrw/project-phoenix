package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

const eventAddress = "Guardian@Example.com"

func enqueueEventMail(t *testing.T, db *bun.DB, store *workerCommandsForTest, ctx context.Context, schoolID int64, key string, security bool) domain.Enqueued {
	t.Helper()
	var enqueued domain.Enqueued
	require.NoError(t, tenant.WithTenantTx(ctx, db, schoolID, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		enqueued, err = store.Enqueue(txCtx, domain.Intent{
			TenantID: schoolID, Transport: domain.TransportEmail, Template: "welcome", IdempotencyKey: &key, Security: security,
			Recipient: json.RawMessage(`{"address":"` + eventAddress + `"}`), Payload: json.RawMessage(`{}`),
			Status: "pending", NextRetryAt: time.Now().Add(-time.Minute),
		})
		return err
	}))
	return enqueued
}

func eventMailToken(t *testing.T, db *bun.DB, id int64) string {
	t.Helper()
	var token string
	require.NoError(t, db.NewRaw(`SELECT delivery_token FROM platform.email_outbox WHERE id = ?`, id).Scan(context.Background(), &token))
	return token
}

func eventAt(token, eventID string, state domain.EmailDeliveryState, at time.Time) domain.EmailDeliveryEvent {
	return domain.EmailDeliveryEvent{Token: token, Provider: "lettermint", ProviderEventID: eventID, State: state, OccurredAt: at}
}

func TestEmailEventWithoutMatchChangesNothing(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)

	result, err := store.RecordEmailDeliveryEvent(ctx, eventAt("00000000-0000-4000-8000-000000000000", "e1", domain.EmailDeliveryFailed, time.Now()))

	require.NoError(t, err)
	assert.False(t, result.Matched)
	assert.False(t, result.Recorded)
}

func TestEmailEventsFindTheMailByEachIdentity(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	mail := enqueueEventMail(t, db, store, ctx, testpkg.Tenant(t), "identity", false)
	token := eventMailToken(t, db, mail.ID)
	_, err := db.ExecContext(context.Background(), `UPDATE platform.email_outbox SET message_id = ?, provider_message_id = ? WHERE id = ?`,
		token+"@mail.example", "provider-"+token, mail.ID)
	require.NoError(t, err)

	for name, event := range map[string]domain.EmailDeliveryEvent{
		"token":       {Token: token},
		"message id":  {MessageID: token + "@mail.example"},
		"provider id": {ProviderMessageID: "provider-" + token},
	} {
		event.Provider, event.ProviderEventID, event.State, event.OccurredAt = "lettermint", name, domain.EmailDeliveryDelayed, time.Now()
		result, err := store.RecordEmailDeliveryEvent(ctx, event)
		require.NoError(t, err, name)
		assert.True(t, result.Matched, name)
	}
}

func TestEmailEventsRepeatedOrLateKeepTheState(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	mail := enqueueEventMail(t, db, store, ctx, testpkg.Tenant(t), "state", false)
	token := eventMailToken(t, db, mail.ID)
	first := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

	record := func(id string, state domain.EmailDeliveryState, at time.Time) domain.EmailDeliveryEventResult {
		t.Helper()
		result, err := store.RecordEmailDeliveryEvent(ctx, eventAt(token, id, state, at))
		require.NoError(t, err)
		return result
	}
	record("soft-1", domain.EmailDeliveryDelayed, first)
	for _, id := range []string{"soft-2", "soft-3", "soft-4", "soft-5", "soft-6", "soft-7"} {
		assert.Equal(t, domain.EmailDeliveryDelayed, record(id, domain.EmailDeliveryDelayed, first.Add(time.Hour)).State)
	}
	var since time.Time
	require.NoError(t, db.NewRaw(`SELECT delivery_state_at FROM platform.email_outbox WHERE id = ?`, mail.ID).Scan(context.Background(), &since))
	assert.True(t, since.Equal(first), "seven soft bounces are one delay, which started with the first")

	duplicate := record("soft-1", domain.EmailDeliveryDelayed, first)
	assert.True(t, duplicate.Matched)
	assert.False(t, duplicate.Recorded, "a repeated event is stored once")
	var stored int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.email_delivery_events WHERE outbox_id = ?`, mail.ID).Scan(context.Background(), &stored))
	assert.Equal(t, 7, stored)

	assert.Equal(t, domain.EmailDeliveryDelivered, record("delivered", domain.EmailDeliveryDelivered, first.Add(2*time.Hour)).State)
	assert.Equal(t, domain.EmailDeliveryDelivered, record("late-soft", domain.EmailDeliveryDelayed, first.Add(-time.Hour)).State, "a late event keeps the newer state")
	assert.Equal(t, domain.EmailDeliveryComplained, record("complaint", domain.EmailDeliveryComplained, first.Add(3*time.Hour)).State, "delivered is not the end")
}

func TestEmailEventsFillTheSuppressionListOfTheMailsSchoolOnly(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school, other := testpkg.Tenant(t), testpkg.NewTenantScope(t, db).TenantID
	mail := enqueueEventMail(t, db, store, ctx, school, "bounce", false)
	token := eventMailToken(t, db, mail.ID)

	result, err := store.RecordEmailDeliveryEvent(ctx, eventAt(token, "soft", domain.EmailDeliveryDelayed, time.Now()))
	require.NoError(t, err)
	assert.True(t, result.Recorded)
	suppressed, err := store.EmailAddressSuppressed(ctx, school, eventAddress)
	require.NoError(t, err)
	assert.False(t, suppressed, "a delay says nothing about the address")

	_, err = store.RecordEmailDeliveryEvent(ctx, eventAt(token, "hard", domain.EmailDeliveryFailed, time.Now()))
	require.NoError(t, err)
	suppressed, err = store.EmailAddressSuppressed(ctx, school, "guardian@example.com")
	require.NoError(t, err)
	assert.True(t, suppressed, "the list ignores the address's case")
	suppressed, err = store.EmailAddressSuppressed(ctx, other, eventAddress)
	require.NoError(t, err)
	assert.False(t, suppressed, "another school keeps its own list")

	read := func(schoolID int64, table string) int {
		var visible int
		require.NoError(t, tenant.WithTenantTx(ctx, db, schoolID, func(txCtx context.Context, tx bun.Tx) error {
			return tx.NewRaw(`SELECT count(*) FROM ?`, bun.Safe(table)).Scan(txCtx, &visible)
		}))
		return visible
	}
	assert.Equal(t, 1, read(school, "platform.email_suppressions"))
	assert.Zero(t, read(other, "platform.email_suppressions"), "another school read the list")
	assert.Equal(t, 2, read(school, "platform.email_delivery_events"))
	assert.Zero(t, read(other, "platform.email_delivery_events"), "another school read the events")
}

func TestEmailEventOfSystemMailStaysOffTheSchoolLists(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	var system domain.Enqueued
	idempotency := fmt.Sprintf("system-event-%d", testpkg.Tenant(t))
	require.NoError(t, tenant.WithAdminTx(ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		system, err = store.Enqueue(txCtx, domain.Intent{
			Transport: domain.TransportEmail, Template: "demo-access", IdempotencyKey: &idempotency,
			Recipient: json.RawMessage(`{"address":"visitor-event@example.com"}`), Payload: json.RawMessage(`{}`),
			Status: "pending", NextRetryAt: time.Now(),
		})
		return err
	}))

	result, err := store.RecordEmailDeliveryEvent(ctx, eventAt(eventMailToken(t, db, system.ID), "hard", domain.EmailDeliveryFailed, time.Now()))

	require.NoError(t, err)
	assert.Equal(t, domain.EmailDeliveryFailed, result.State)
	var listed int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.email_suppressions WHERE address = 'visitor-event@example.com'`).Scan(context.Background(), &listed))
	assert.Zero(t, listed)
}

func TestSecurityMailIsClaimedBeforeAnOlderLetterQueue(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	for _, key := range []string{"letter-1", "letter-2", "letter-3"} {
		enqueueEventMail(t, db, store, ctx, school, key, false)
	}
	code := enqueueEventMail(t, db, store, ctx, school, "code", true)

	now := time.Now()
	first, err := store.Claim(ctx, domain.TransportEmail, 1, now, now.Add(time.Minute))

	require.NoError(t, err)
	require.Len(t, first, 1)
	assert.Equal(t, code.ID, first[0].ID)
	assert.True(t, first[0].Security)
	assert.NotEmpty(t, first[0].DeliveryToken)
}

func TestFinalizeSentStoresTheMailsIdentity(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	enqueueEventMail(t, db, store, ctx, testpkg.Tenant(t), "identity", false)
	now := time.Now()
	claimed, err := store.Claim(ctx, domain.TransportEmail, 1, now, now.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	fence, active, err := store.StartDelivery(ctx, domain.TransportEmail, claimed[0].ID, *claimed[0].LeaseToken, now.Add(time.Minute))
	require.NoError(t, err)
	require.True(t, active)

	finalized, err := store.FinalizeSent(ctx, domain.TransportEmail, claimed[0].ID, fence,
		json.RawMessage(`{"message_id":"abc@mail.example","provider_message_id":"provider-1"}`), now)

	require.NoError(t, err)
	require.True(t, finalized)
	var messageID, providerID string
	require.NoError(t, db.NewRaw(`SELECT message_id, provider_message_id FROM platform.email_outbox WHERE id = ?`, claimed[0].ID).Scan(context.Background(), &messageID, &providerID))
	assert.Equal(t, "abc@mail.example", messageID)
	assert.Equal(t, "provider-1", providerID)
}

func TestRetentionRemovesExpiredEventsAndSuppressions(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	mail := enqueueEventMail(t, db, store, ctx, school, "retention", false)
	_, err := store.RecordEmailDeliveryEvent(ctx, eventAt(eventMailToken(t, db, mail.ID), "hard", domain.EmailDeliveryFailed, time.Now()))
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(), `UPDATE platform.email_outbox SET status = 'sent', sent_at = NOW() - INTERVAL '31 days' WHERE id = ?`, mail.ID)
	require.NoError(t, err)

	var events, keptSuppressions, suppressions int
	require.NoError(t, tenant.WithTenantTx(ctx, db, school, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		if events, err = store.DeleteExpiredEmailDeliveryEvents(txCtx, school, time.Now().Add(-domain.RetentionWindow)); err != nil {
			return err
		}
		if keptSuppressions, err = store.DeleteExpiredEmailSuppressions(txCtx, school, time.Now().Add(-domain.SuppressionRetention)); err != nil {
			return err
		}
		suppressions, err = store.DeleteExpiredEmailSuppressions(txCtx, school, time.Now().Add(time.Minute))
		return err
	}))
	assert.Equal(t, 1, events)
	assert.Zero(t, keptSuppressions, "a fresh entry stays for the period")
	assert.Equal(t, 1, suppressions)
}
