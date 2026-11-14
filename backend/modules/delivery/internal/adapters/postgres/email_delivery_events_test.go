package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
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

func TestClaimOrdersAMixedEmailBatch(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	var letters []int64
	for _, key := range []string{"letter-1", "letter-2", "letter-3"} {
		letters = append(letters, enqueueEventMail(t, db, store, ctx, school, key, false).ID)
	}
	code := enqueueEventMail(t, db, store, ctx, school, "code", true)
	invitation := enqueueEventMail(t, db, store, ctx, school, "invitation", true)
	// Equal due times also exercise the ID tie-breaker within each priority.
	now := time.Now()
	_, err := db.ExecContext(ctx, `UPDATE platform.email_outbox SET next_retry_at = ? WHERE tenant_id = ?`, now, school)
	require.NoError(t, err)

	claimed, err := store.Claim(ctx, domain.TransportEmail, 5, now, now.Add(time.Minute))

	require.NoError(t, err)
	require.Len(t, claimed, 5)
	want := append([]int64{code.ID, invitation.ID}, letters...)
	for i, mail := range claimed {
		assert.Equal(t, want[i], mail.ID, "dispatch position %d", i)
	}
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
	guardian := eventGuardian(t, db)
	child := testpkg.CreateTestStudent(t, db, "Kind", "Muster", "2a")
	testpkg.CreateTestStudentGuardianLink(t, db, child.ID, guardian.ID, "primary_guardian")
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
		if keptSuppressions, err = store.DeleteUnheldEmailSuppressions(txCtx, school); err != nil {
			return err
		}
		return err
	}))
	// However old the mark, it stays until someone at the school checked the
	// address (#5934).
	_, err = db.ExecContext(context.Background(), `UPDATE platform.email_suppressions SET updated_at = NOW() - INTERVAL '10 years' WHERE tenant_id = ?`, school)
	require.NoError(t, err)
	require.NoError(t, tenant.WithTenantTx(ctx, db, school, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		suppressions, err = store.DeleteUnheldEmailSuppressions(txCtx, school)
		return err
	}))
	assert.Equal(t, 1, events)
	assert.Zero(t, keptSuppressions, "a fresh entry of a family with a child in care stays")
	assert.Zero(t, suppressions, "an old entry of a known address stays too")
}

// Deleting a person must end the entry, whichever path deleted them (#5932).
func TestRetentionRemovesASuppressionNobodyAtTheSchoolHolds(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	mail := enqueueEventMail(t, db, store, ctx, school, "gone", false)
	_, err := store.RecordEmailDeliveryEvent(ctx, eventAt(eventMailToken(t, db, mail.ID), "hard", domain.EmailDeliveryFailed, time.Now()))
	require.NoError(t, err)

	var removed int
	require.NoError(t, tenant.WithTenantTx(ctx, db, school, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		removed, err = store.DeleteUnheldEmailSuppressions(txCtx, school)
		return err
	}))
	assert.Equal(t, 1, removed)
}

// eventGuardian is a guardian contact of the test's school holding eventAddress.
func eventGuardian(t *testing.T, db *bun.DB) *testpkg.GuardianProfile {
	t.Helper()
	guardian := testpkg.CreateTestGuardianProfileNamed(t, db, "Erika", "Muster", "erika")
	_, err := db.ExecContext(context.Background(), `UPDATE users.guardian_profiles SET email = ? WHERE id = ?`, eventAddress, guardian.ID)
	require.NoError(t, err)
	return guardian
}

// setChildStatus moves a child's membership the way the activate-students tick
// (inactive) or the grade transition (alumnus) does.
func setChildStatus(t *testing.T, db *bun.DB, school, studentID int64, status string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `UPDATE users.student_school_memberships SET status = ?
		WHERE tenant_id = ? AND student_profile_id = ?`, status, school, studentID)
	require.NoError(t, err)
}

func schoolHolds(t *testing.T, store *workerCommandsForTest, ctx context.Context, school int64) domain.SchoolMailHolds {
	t.Helper()
	holds, err := store.SchoolEmailHolds(ctx, school, eventAddress)
	require.NoError(t, err)
	return holds
}

// The school's mailings end with the family's last child in care, whether its
// care ended (inactive) or it left with the grade transition (alumnus) (#5960).
func TestSchoolMailHoldsDepartOnlyWithTheFamilysLastChild(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	assert.False(t, schoolHolds(t, store, ctx, school).Departed, "an address no guardian holds is no departed family")

	guardian := eventGuardian(t, db)
	first := testpkg.CreateTestStudent(t, db, "Erstes", "Kind", "2a")
	second := testpkg.CreateTestStudent(t, db, "Zweites", "Kind", "4b")
	testpkg.CreateTestStudentGuardianLink(t, db, first.ID, guardian.ID, "primary_guardian")
	testpkg.CreateTestStudentGuardianLink(t, db, second.ID, guardian.ID, "primary_guardian")
	assert.False(t, schoolHolds(t, store, ctx, school).Departed)

	setChildStatus(t, db, school, first.ID, "inactive")
	assert.False(t, schoolHolds(t, store, ctx, school).Departed, "one child is still in care")

	setChildStatus(t, db, school, second.ID, "alumnus")
	holds := schoolHolds(t, store, ctx, school)
	assert.True(t, holds.Departed, "no child of the family is in care any more")
	assert.False(t, holds.Listed)
	assert.False(t, holds.Stopped)

	setChildStatus(t, db, school, first.ID, "pending")
	assert.False(t, schoolHolds(t, store, ctx, school).Departed, "a child admitted again brings the family back")
}

// The parent's school account outlives the child, so its membership alone holds
// no address. The same person as staff of the school still does (#5960).
func TestSchoolMailHoldsReadAParentAccountLikeItsGuardian(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	guardian := testpkg.CreateTestGuardianProfileNamed(t, db, "Erika", "Muster", "erika")
	account := testpkg.CreateTestAccount(t, db, "parent")
	testpkg.EnsureAccountTenant(t, db, account.ID, school)
	_, err := db.ExecContext(context.Background(), `UPDATE auth.accounts SET email = ? WHERE id = ?`, eventAddress, account.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(), `UPDATE users.guardian_profiles SET account_id = ?, has_account = TRUE WHERE id = ?`, account.ID, guardian.ID)
	require.NoError(t, err)
	child := testpkg.CreateTestStudent(t, db, "Kind", "Muster", "2a")
	testpkg.CreateTestStudentGuardianLink(t, db, child.ID, guardian.ID, "primary_guardian")
	assert.False(t, schoolHolds(t, store, ctx, school).Departed)

	setChildStatus(t, db, school, child.ID, "alumnus")
	assert.True(t, schoolHolds(t, store, ctx, school).Departed, "the account's address departs with its guardian")

	testpkg.CreateTestStaffForExistingAccountInTenant(t, db, school, account.ID, "Erika", "Muster")
	assert.False(t, schoolHolds(t, store, ctx, school).Departed, "a staff member of the school keeps the address")
}

// After the last child left, the nightly run ends the school's mark of the
// family's address, a complaint too: the provider keeps its own block. A family
// with a child in care keeps its mark (#5960).
func TestRetentionEndsTheMarkOfADepartedFamily(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	guardian := eventGuardian(t, db)
	child := testpkg.CreateTestStudent(t, db, "Kind", "Muster", "2a")
	testpkg.CreateTestStudentGuardianLink(t, db, child.ID, guardian.ID, "primary_guardian")
	mail := enqueueEventMail(t, db, store, ctx, school, "complaint", false)
	_, err := store.RecordEmailDeliveryEvent(ctx, eventAt(eventMailToken(t, db, mail.ID), "spam", domain.EmailDeliveryComplained, time.Now()))
	require.NoError(t, err)
	sweep := func() int {
		var removed int
		require.NoError(t, tenant.WithTenantTx(ctx, db, school, func(txCtx context.Context, _ bun.Tx) error {
			removed, err = store.DeleteUnheldEmailSuppressions(txCtx, school)
			return err
		}))
		return removed
	}

	assert.Zero(t, sweep(), "the family still has a child in care")
	setChildStatus(t, db, school, child.ID, "inactive")
	assert.Equal(t, 1, sweep(), "the mark ends once the last child left")
	assert.False(t, schoolHolds(t, store, ctx, school).Listed)
}

// A provider that names two mails leaves the school open: nothing is recorded.
func TestEmailEventWithContradictingIdentitiesMatchesNothing(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school, other := testpkg.Tenant(t), testpkg.NewTenantScope(t, db).TenantID
	first := enqueueEventMail(t, db, store, ctx, school, "first", false)
	second := enqueueEventMail(t, db, store, ctx, other, "second", false)
	_, err := db.ExecContext(context.Background(), `UPDATE platform.email_outbox SET provider_message_id = 'provider-first' WHERE id = ?`, first.ID)
	require.NoError(t, err)

	event := eventAt(eventMailToken(t, db, second.ID), "hard", domain.EmailDeliveryFailed, time.Now())
	event.ProviderMessageID = "provider-first"
	result, err := store.RecordEmailDeliveryEvent(ctx, event)

	require.NoError(t, err)
	assert.False(t, result.Matched)
	for _, schoolID := range []int64{school, other} {
		suppressed, err := store.EmailAddressSuppressed(ctx, schoolID, eventAddress)
		require.NoError(t, err)
		assert.False(t, suppressed)
	}
}

// The rest of a guardian welcome keeps only its key: no state, no way to match a
// late event.
func TestRetentionClearsTheDeliveryDataOfAKeyOnlyMail(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	key := "guardian_welcome:7"
	var welcome domain.Enqueued
	require.NoError(t, tenant.WithTenantTx(ctx, db, school, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		welcome, err = store.Enqueue(txCtx, domain.Intent{
			TenantID: school, Transport: domain.TransportEmail, Template: "guardian_welcome", IdempotencyKey: &key,
			Recipient: json.RawMessage(`{"address":"guardian@example.com"}`), Payload: json.RawMessage(`{}`),
			Status: "pending", NextRetryAt: time.Now(),
		})
		return err
	}))
	token := eventMailToken(t, db, welcome.ID)
	_, err := store.RecordEmailDeliveryEvent(ctx, eventAt(token, "complaint", domain.EmailDeliveryComplained, time.Now()))
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(), `UPDATE platform.email_outbox
		SET status = 'sent', sent_at = NOW() - INTERVAL '31 days', message_id = 'x@mail.example', provider_message_id = 'p' WHERE id = ?`, welcome.ID)
	require.NoError(t, err)

	require.NoError(t, tenant.WithTenantTx(ctx, db, school, func(txCtx context.Context, _ bun.Tx) error {
		_, err := store.ClearExpiredKeyOnlyEmailIntents(txCtx, school, time.Now().Add(-domain.RetentionWindow), domain.KeyOnlyTemplates)
		return err
	}))

	var kept int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.email_outbox WHERE id = ?
		AND message_id IS NULL AND provider_message_id IS NULL AND delivery_state IS NULL AND delivery_state_at IS NULL`, welcome.ID).Scan(context.Background(), &kept))
	assert.Equal(t, 1, kept)
	late, err := store.RecordEmailDeliveryEvent(ctx, eventAt(token, "late", domain.EmailDeliveryFailed, time.Now()))
	require.NoError(t, err)
	assert.False(t, late.Matched)
}

func TestClaimKeepsSystemMailAmongSecurityMailByAge(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	systemKey := fmt.Sprintf("demo-link-%d", testpkg.Tenant(t))
	var system domain.Enqueued
	require.NoError(t, tenant.WithAdminTx(ctx, db, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		system, err = store.Enqueue(txCtx, domain.Intent{
			Transport: domain.TransportEmail, Template: "demo-access", IdempotencyKey: &systemKey,
			Recipient: json.RawMessage(`{"address":"visitor@example.com"}`), Payload: json.RawMessage(`{}`),
			Status: "pending", NextRetryAt: time.Now().Add(-time.Hour),
		})
		return err
	}))
	enqueueEventMail(t, db, store, ctx, testpkg.Tenant(t), "invitation", true)

	now := time.Now()
	first, err := store.Claim(ctx, domain.TransportEmail, 1, now, now.Add(time.Minute))

	require.NoError(t, err)
	require.Len(t, first, 1)
	assert.Equal(t, system.ID, first[0].ID, "the older demo link stays ahead of a newer invitation")
}

// sendEventMail claims and finalizes one queued mail as the worker does.
// The event can beat the provider's 250 answer, and that answer may name no ID. The
// open request decides, not the ID: the code reaches the person over the fallback,
// and only once (#5957).
func TestRerouteDuringTheSendWithoutAProviderIDStillTakesTheFallbackOnce(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	mail := enqueueEventMail(t, db, store, ctx, testpkg.Tenant(t), "race-no-id", true)
	fence := startEventMail(t, store, ctx, mail.ID)
	token := eventMailToken(t, db, mail.ID)
	event := eventAt(token, "suppressed", domain.EmailDeliverySuppressed, time.Now())
	event.Reroute, event.KeepsAddress = true, true
	result, err := store.RecordEmailDeliveryEvent(ctx, event)
	require.NoError(t, err)
	require.True(t, result.Rerouted)

	finalized, err := store.FinalizeSent(ctx, domain.TransportEmail, mail.ID, fence, json.RawMessage(`{"message_id":"a@mail.example"}`), time.Now())

	require.NoError(t, err)
	require.True(t, finalized)
	status, _, _ := mailStatus(t, db, mail.ID)
	assert.Equal(t, "pending", status, "the acceptance is what the event already refuted, with or without an ID")
	assert.Zero(t, mailAttempts(t, db, mail.ID), "the fallback has attempts of its own")

	now := time.Now()
	claimed, err := store.Claim(ctx, domain.TransportEmail, 50, now, now.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	assert.Equal(t, mail.ID, claimed[0].ID)
	assert.True(t, claimed[0].UseFallback, "this try goes over the fallback")
	fence, active, err := store.StartDelivery(ctx, domain.TransportEmail, mail.ID, *claimed[0].LeaseToken, now.Add(time.Minute))
	require.NoError(t, err)
	require.True(t, active)
	finalized, err = store.FinalizeSent(ctx, domain.TransportEmail, mail.ID, fence, json.RawMessage(`{"message_id":"fallback@mail.example","used_fallback":true}`), now)
	require.NoError(t, err)
	require.True(t, finalized)

	status, _, _ = mailStatus(t, db, mail.ID)
	assert.Equal(t, "sent", status, "left over the fallback: it stays sent")
	again, err := store.Claim(ctx, domain.TransportEmail, 50, time.Now(), time.Now().Add(time.Minute))
	require.NoError(t, err)
	assert.Empty(t, again, "at most one fallback send")
	second := eventAt(token, "suppressed-2", domain.EmailDeliverySuppressed, time.Now())
	second.Reroute, second.KeepsAddress = true, true
	result, err = store.RecordEmailDeliveryEvent(ctx, second)
	require.NoError(t, err)
	assert.False(t, result.Rerouted, "a late event sends nothing again")
}

func sendEventMail(t *testing.T, store *workerCommandsForTest, ctx context.Context, id int64, providerResult string) {
	t.Helper()
	fence := startEventMail(t, store, ctx, id)
	finalized, err := store.FinalizeSent(ctx, domain.TransportEmail, id, fence, json.RawMessage(providerResult), time.Now())
	require.NoError(t, err)
	require.True(t, finalized)
}

func startEventMail(t *testing.T, store *workerCommandsForTest, ctx context.Context, id int64) string {
	t.Helper()
	now := time.Now()
	claimed, err := store.Claim(ctx, domain.TransportEmail, 50, now, now.Add(time.Minute))
	require.NoError(t, err)
	for _, intent := range claimed {
		if intent.ID == id {
			fence, active, err := store.StartDelivery(ctx, domain.TransportEmail, id, *intent.LeaseToken, now.Add(time.Minute))
			require.NoError(t, err)
			require.True(t, active)
			return fence
		}
	}
	require.Fail(t, "mail was not claimed")
	return ""
}

func mailStatus(t *testing.T, db *bun.DB, id int64) (status string, state *string, requested bool) {
	t.Helper()
	require.NoError(t, db.NewRaw(`SELECT status, delivery_state, fallback_requested_at IS NOT NULL FROM platform.email_outbox WHERE id = ?`, id).
		Scan(context.Background(), &status, &state, &requested))
	return status, state, requested
}

const acceptedByProvider = `{"message_id":"a@mail.example","provider_message_id":"provider-1"}`

func TestFailureThatKeepsTheAddressStaysOffTheList(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	mail := enqueueEventMail(t, db, store, ctx, school, "blocked", false)
	event := eventAt(eventMailToken(t, db, mail.ID), "blocked", domain.EmailDeliveryFailed, time.Now())
	event.KeepsAddress = true

	result, err := store.RecordEmailDeliveryEvent(ctx, event)

	require.NoError(t, err)
	assert.Equal(t, domain.EmailDeliveryFailed, result.State, "the school still sees that it did not arrive")
	suppressed, err := store.EmailAddressSuppressed(ctx, school, eventAddress)
	require.NoError(t, err)
	assert.False(t, suppressed, "a blocklist entry of the provider is not the parent's fault")
}

func TestRerouteQueuesASecurityMailForTheFallbackOnce(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	mail := enqueueEventMail(t, db, store, ctx, school, "code", true)
	token := eventMailToken(t, db, mail.ID)
	sendEventMail(t, store, ctx, mail.ID, acceptedByProvider)
	suppressedEvent := eventAt(token, "suppressed", domain.EmailDeliverySuppressed, time.Now())
	suppressedEvent.Reroute = true

	result, err := store.RecordEmailDeliveryEvent(ctx, suppressedEvent)

	require.NoError(t, err)
	assert.True(t, result.Rerouted)
	status, state, requested := mailStatus(t, db, mail.ID)
	assert.Equal(t, "pending", status)
	assert.Nil(t, state, "the fallback's outcome is still open")
	assert.True(t, requested)

	now := time.Now()
	claimed, err := store.Claim(ctx, domain.TransportEmail, 1, now, now.Add(time.Minute))
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	assert.True(t, claimed[0].UseFallback)
	fence, active, err := store.StartDelivery(ctx, domain.TransportEmail, mail.ID, *claimed[0].LeaseToken, now.Add(time.Minute))
	require.NoError(t, err)
	require.True(t, active)
	finalized, err := store.FinalizeSent(ctx, domain.TransportEmail, mail.ID, fence, json.RawMessage(`{"message_id":"a@example.org","used_fallback":true}`), now)
	require.NoError(t, err)
	require.True(t, finalized)

	second := eventAt(token, "suppressed-2", domain.EmailDeliverySuppressed, time.Now())
	second.Reroute = true
	result, err = store.RecordEmailDeliveryEvent(ctx, second)
	require.NoError(t, err)
	assert.False(t, result.Rerouted, "one fallback per mail")
	status, state, _ = mailStatus(t, db, mail.ID)
	assert.Equal(t, "sent", status)
	assert.Nil(t, state, "the provider no longer speaks for this mail")

	late, err := store.RecordEmailDeliveryEvent(ctx, eventAt(token, "late-hard", domain.EmailDeliveryFailed, time.Now()))
	require.NoError(t, err)
	assert.True(t, late.Recorded, "the event stays in the history")
	suppressed, err := store.EmailAddressSuppressed(ctx, school, eventAddress)
	require.NoError(t, err)
	assert.False(t, suppressed)
}

func TestFallbackRecipientRejectionUpdatesStateAndSuppressesTheAddress(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	other := testpkg.NewTenantScope(t, db).TenantID
	mail := enqueueEventMail(t, db, store, ctx, school, "fallback-rejection", true)
	token := eventMailToken(t, db, mail.ID)
	sendEventMail(t, store, ctx, mail.ID, acceptedByProvider)
	reroute := eventAt(token, "reroute", domain.EmailDeliverySuppressed, time.Now())
	reroute.Reroute = true
	_, err := store.RecordEmailDeliveryEvent(ctx, reroute)
	require.NoError(t, err)

	rejection := eventAt(token, "recipient-rejected", domain.EmailDeliveryFailed, time.Now())
	rejection.Provider = "moto"
	result, err := store.RecordEmailDeliveryEvent(ctx, rejection)
	require.NoError(t, err)
	assert.True(t, result.Recorded)
	assert.Equal(t, domain.EmailDeliveryFailed, result.State)
	_, state, requested := mailStatus(t, db, mail.ID)
	require.NotNil(t, state)
	assert.Equal(t, string(domain.EmailDeliveryFailed), *state)
	assert.True(t, requested)
	suppressed, err := store.EmailAddressSuppressed(ctx, school, eventAddress)
	require.NoError(t, err)
	assert.True(t, suppressed)
	suppressed, err = store.EmailAddressSuppressed(ctx, other, eventAddress)
	require.NoError(t, err)
	assert.False(t, suppressed, "the rejection belongs only to the mail's school")

	late, err := store.RecordEmailDeliveryEvent(ctx, eventAt(token, "late-delivery", domain.EmailDeliveryDelivered, time.Now()))
	require.NoError(t, err)
	assert.True(t, late.Recorded)
	assert.Equal(t, domain.EmailDeliveryFailed, late.State, "the original provider cannot replace the fallback rejection")
}

func TestRerouteLeavesOtherMailAlone(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	mail := enqueueEventMail(t, db, store, ctx, testpkg.Tenant(t), "letter", false)
	sendEventMail(t, store, ctx, mail.ID, acceptedByProvider)
	event := eventAt(eventMailToken(t, db, mail.ID), "suppressed", domain.EmailDeliverySuppressed, time.Now())
	event.Reroute = true

	result, err := store.RecordEmailDeliveryEvent(ctx, event)

	require.NoError(t, err)
	assert.False(t, result.Rerouted)
	status, state, requested := mailStatus(t, db, mail.ID)
	assert.Equal(t, "sent", status)
	require.NotNil(t, state)
	assert.Equal(t, "suppressed", *state)
	assert.False(t, requested)
}

// An event can be faster than the worker's own write after the provider said 250.
func TestRerouteDuringTheSendQueuesTheMailAfterAcceptance(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	mail := enqueueEventMail(t, db, store, ctx, testpkg.Tenant(t), "race", true)
	fence := startEventMail(t, store, ctx, mail.ID)
	event := eventAt(eventMailToken(t, db, mail.ID), "suppressed", domain.EmailDeliverySuppressed, time.Now())
	event.Reroute = true
	_, err := store.RecordEmailDeliveryEvent(ctx, event)
	require.NoError(t, err)

	finalized, err := store.FinalizeSent(ctx, domain.TransportEmail, mail.ID, fence, json.RawMessage(acceptedByProvider), time.Now())

	require.NoError(t, err)
	require.True(t, finalized)
	status, _, _ := mailStatus(t, db, mail.ID)
	assert.Equal(t, "pending", status, "the provider's acceptance is what the event already refuted")
}

func TestOpenProviderMailsAreTheAcceptedOnesWithoutAnOutcome(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	sent := func(key, result string) int64 {
		mail := enqueueEventMail(t, db, store, ctx, school, key, false)
		sendEventMail(t, store, ctx, mail.ID, result)
		return mail.ID
	}
	open := sent("open", `{"provider_message_id":"p-open"}`)
	delayed := sent("delayed", `{"provider_message_id":"p-delayed"}`)
	delivered := sent("delivered", `{"provider_message_id":"p-delivered"}`)
	sent("plain-smtp", `{"message_id":"a@strato.example"}`)
	tooOld := sent("too-old", `{"provider_message_id":"p-old"}`)
	checked := sent("checked", `{"provider_message_id":"p-checked"}`)
	_, err := store.RecordEmailDeliveryEvent(ctx, eventAt(eventMailToken(t, db, delayed), "soft", domain.EmailDeliveryDelayed, time.Now()))
	require.NoError(t, err)
	_, err = store.RecordEmailDeliveryEvent(ctx, eventAt(eventMailToken(t, db, delivered), "ok", domain.EmailDeliveryDelivered, time.Now()))
	require.NoError(t, err)
	_, err = db.ExecContext(context.Background(), `UPDATE platform.email_outbox SET sent_at = NOW() - INTERVAL '30 days' WHERE id = ?`, tooOld)
	require.NoError(t, err)
	now := time.Now()
	require.NoError(t, store.MarkEmailDeliveryChecked(ctx, checked, now))

	query := domain.ProviderMailQuery{SentAfter: now.Add(-28 * 24 * time.Hour), SentBefore: now.Add(time.Minute), CheckedBefore: now.Add(-time.Hour), Limit: 10}
	mails, err := store.OpenProviderMails(ctx, query)

	require.NoError(t, err)
	found := map[int64]domain.ProviderMail{}
	for _, mail := range mails {
		found[mail.ID] = mail
	}
	require.Len(t, found, 2)
	assert.Equal(t, "p-open", found[open].ProviderMessageID)
	assert.Empty(t, found[open].State)
	assert.Equal(t, domain.EmailDeliveryDelayed, found[delayed].State)

	query.SentBefore = now.Add(-time.Hour)
	mails, err = store.OpenProviderMails(ctx, query)
	require.NoError(t, err)
	assert.Empty(t, mails, "the webhook gets its time first")
}

func TestLiftEmailSuppressionFreesOnlyABounceOfThatSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school, other := testpkg.Tenant(t), testpkg.NewTenantScope(t, db).TenantID
	insert := func(tenantID int64, address, reason string) {
		_, err := db.ExecContext(context.Background(), `INSERT INTO platform.email_suppressions (tenant_id, address, reason) VALUES (?, ?, ?)`, tenantID, address, reason)
		require.NoError(t, err)
	}
	insert(school, "bounce@example.com", "failed")
	insert(other, "bounce@example.com", "failed")
	insert(school, "complaint@example.com", "complained")

	inSchool := func(run func(txCtx context.Context) error) {
		require.NoError(t, tenant.WithTenantTx(ctx, db, school, func(txCtx context.Context, _ bun.Tx) error { return run(txCtx) }))
	}
	inSchool(func(txCtx context.Context) error {
		reason, listed, err := store.EmailSuppressionReason(txCtx, school, " Complaint@Example.com ")
		require.NoError(t, err)
		assert.True(t, listed)
		assert.Equal(t, domain.EmailSuppressionComplained, reason)
		_, listed, err = store.EmailSuppressionReason(txCtx, school, "nobody@example.com")
		require.NoError(t, err)
		assert.False(t, listed)
		return store.DeleteEmailSuppression(txCtx, school, "Bounce@Example.com")
	})

	stillListed, err := store.EmailAddressSuppressed(ctx, school, "bounce@example.com")
	require.NoError(t, err)
	assert.False(t, stillListed)
	stillListed, err = store.EmailAddressSuppressed(ctx, other, "bounce@example.com")
	require.NoError(t, err)
	assert.True(t, stillListed, "the other school keeps its own list")
}

func TestSynchronousFallbackFinalizationIgnoresLateProviderEvents(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	mail := enqueueEventMail(t, db, store, ctx, testpkg.Tenant(t), "sync-fallback", true)
	fence := startEventMail(t, store, ctx, mail.ID)
	token := eventMailToken(t, db, mail.ID)
	// The primary event can race with the successful fallback's finalization.
	_, err := store.RecordEmailDeliveryEvent(ctx, eventAt(token, "early-delivery", domain.EmailDeliveryDelivered, time.Now()))
	require.NoError(t, err)
	finalized, err := store.FinalizeSent(ctx, domain.TransportEmail, mail.ID, fence,
		json.RawMessage(`{"message_id":"fallback@example.org","used_fallback":true}`), time.Now())
	require.NoError(t, err)
	require.True(t, finalized)
	status, state, requested := mailStatus(t, db, mail.ID)
	assert.Equal(t, "sent", status)
	assert.Nil(t, state)
	assert.True(t, requested)
	late := eventAt(token, "late-failure", domain.EmailDeliveryFailed, time.Now())
	late.Reroute, late.KeepsAddress = true, true
	result, err := store.RecordEmailDeliveryEvent(ctx, late)
	require.NoError(t, err)
	assert.False(t, result.Rerouted)
	status, state, _ = mailStatus(t, db, mail.ID)
	assert.Equal(t, "sent", status)
	assert.Nil(t, state)
}

func TestDeliveredSecurityMailDoesNotRerouteOnALateBlocklistDelay(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	mail := enqueueEventMail(t, db, store, ctx, testpkg.Tenant(t), "delivered-security", true)
	token := eventMailToken(t, db, mail.ID)
	_, err := store.RecordEmailDeliveryEvent(ctx, eventAt(token, "delivered", domain.EmailDeliveryDelivered, time.Now()))
	require.NoError(t, err)
	late := eventAt(token, "late-blocklist", domain.EmailDeliveryDelayed, time.Now().Add(-time.Minute))
	late.Reroute, late.KeepsAddress = true, true
	result, err := store.RecordEmailDeliveryEvent(ctx, late)
	require.NoError(t, err)
	assert.False(t, result.Rerouted)
	assert.Equal(t, domain.EmailDeliveryDelivered, result.State)
	_, _, requested := mailStatus(t, db, mail.ID)
	assert.False(t, requested)
}

func TestComplaintAfterFallbackStillSuppressesTheAddress(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	mail := enqueueEventMail(t, db, store, ctx, school, "fallback-complaint", true)
	token := eventMailToken(t, db, mail.ID)
	reroute := eventAt(token, "reroute", domain.EmailDeliverySuppressed, time.Now())
	reroute.Reroute = true
	_, err := store.RecordEmailDeliveryEvent(ctx, reroute)
	require.NoError(t, err)
	result, err := store.RecordEmailDeliveryEvent(ctx, eventAt(token, "complaint", domain.EmailDeliveryComplained, time.Now()))
	require.NoError(t, err)
	assert.True(t, result.Recorded)
	assert.Empty(t, result.State)
	suppressed, err := store.EmailAddressSuppressed(ctx, school, eventAddress)
	require.NoError(t, err)
	assert.True(t, suppressed)
	require.NoError(t, tenant.WithTenantTx(ctx, db, school, func(txCtx context.Context, _ bun.Tx) error {
		assert.ErrorIs(t, store.DeleteEmailSuppression(txCtx, school, eventAddress), domain.ErrSuppressionNeedsReview)
		return nil
	}))
	suppressed, err = store.EmailAddressSuppressed(ctx, school, eventAddress)
	require.NoError(t, err)
	assert.True(t, suppressed)
}

func TestLiftSuppressionPreservesAComplaintRecordedAfterTheEligibilityCheck(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	mail := enqueueEventMail(t, db, store, ctx, school, "lift-complaint-race", false)
	token := eventMailToken(t, db, mail.ID)
	_, err := store.RecordEmailDeliveryEvent(ctx, eventAt(token, "bounce", domain.EmailDeliveryFailed, time.Now()))
	require.NoError(t, err)
	var reason domain.EmailSuppressionReason
	var listed bool
	readReason := func() {
		t.Helper()
		require.NoError(t, tenant.WithTenantTx(ctx, db, school, func(txCtx context.Context, _ bun.Tx) error {
			var readErr error
			reason, listed, readErr = store.EmailSuppressionReason(txCtx, school, eventAddress)
			return readErr
		}))
	}
	readReason()
	require.True(t, listed)
	require.Equal(t, domain.EmailSuppressionFailed, reason)
	// A webhook commits the complaint while the provider call is in progress.
	_, err = store.RecordEmailDeliveryEvent(ctx, eventAt(token, "complaint", domain.EmailDeliveryComplained, time.Now()))
	require.NoError(t, err)
	require.NoError(t, tenant.WithTenantTx(ctx, db, school, func(txCtx context.Context, _ bun.Tx) error {
		require.ErrorIs(t, store.DeleteEmailSuppression(txCtx, school, eventAddress), domain.ErrSuppressionNeedsReview)
		return nil
	}))
	readReason()
	assert.True(t, listed)
	assert.Equal(t, domain.EmailSuppressionComplained, reason)
}

// A mail that arrived or drew a complaint is not sent a second time, whatever
// reaches moto later.
func TestLateFailureDoesNotRerouteADeliveredOrComplainedMail(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	for _, first := range []domain.EmailDeliveryState{domain.EmailDeliveryDelivered, domain.EmailDeliveryComplained} {
		for _, late := range []domain.EmailDeliveryState{domain.EmailDeliveryDelayed, domain.EmailDeliverySuppressed, domain.EmailDeliveryFailed} {
			key := string(first) + "-" + string(late)
			mail := enqueueEventMail(t, db, store, ctx, school, key, true)
			sendEventMail(t, store, ctx, mail.ID, acceptedByProvider)
			token := eventMailToken(t, db, mail.ID)
			_, err := store.RecordEmailDeliveryEvent(ctx, eventAt(token, "first", first, time.Now()))
			require.NoError(t, err, key)
			event := eventAt(token, "late", late, time.Now())
			event.Reroute, event.KeepsAddress = true, true

			result, err := store.RecordEmailDeliveryEvent(ctx, event)

			require.NoError(t, err, key)
			assert.False(t, result.Rerouted, key)
			status, state, requested := mailStatus(t, db, mail.ID)
			assert.Equal(t, "sent", status, key)
			assert.NotNil(t, state, key)
			assert.False(t, requested, key)
		}
	}
}

// The reconciliation shows a bounce without knowing why. The webhook's retry
// that names a blocklist can still send the mail over the fallback.
func TestBlocklistRetryAfterAnAPIBounceReroutesAndLeavesTheList(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	mail := enqueueEventMail(t, db, store, ctx, school, "api-then-blocklist", true)
	sendEventMail(t, store, ctx, mail.ID, acceptedByProvider)
	token := eventMailToken(t, db, mail.ID)
	api := eventAt(token, "api:hard_bounced", domain.EmailDeliveryFailed, time.Now())
	api.KeepsAddress = true
	result, err := store.RecordEmailDeliveryEvent(ctx, api)
	require.NoError(t, err)
	require.Equal(t, domain.EmailDeliveryFailed, result.State)
	listed, err := store.EmailAddressSuppressed(ctx, school, eventAddress)
	require.NoError(t, err)
	require.False(t, listed, "the API names no answer, so it does not accuse the address")

	retry := eventAt(token, "webhook-1", domain.EmailDeliveryFailed, time.Now())
	retry.KeepsAddress, retry.Reroute = true, true
	result, err = store.RecordEmailDeliveryEvent(ctx, retry)

	require.NoError(t, err)
	assert.True(t, result.Rerouted)
	listed, err = store.EmailAddressSuppressed(ctx, school, eventAddress)
	require.NoError(t, err)
	assert.False(t, listed)
}

func TestWebhookBounceAfterAnAPIBounceListsTheAddress(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	mail := enqueueEventMail(t, db, store, ctx, school, "api-then-bounce", false)
	token := eventMailToken(t, db, mail.ID)
	api := eventAt(token, "api:hard_bounced", domain.EmailDeliveryFailed, time.Now())
	api.KeepsAddress = true
	_, err := store.RecordEmailDeliveryEvent(ctx, api)
	require.NoError(t, err)

	_, err = store.RecordEmailDeliveryEvent(ctx, eventAt(token, "webhook-1", domain.EmailDeliveryFailed, time.Now()))

	require.NoError(t, err)
	listed, err := store.EmailAddressSuppressed(ctx, school, eventAddress)
	require.NoError(t, err)
	assert.True(t, listed, "the server's answer accuses the address")
}

func mailAttempts(t *testing.T, db *bun.DB, id int64) int {
	t.Helper()
	var attempts int
	require.NoError(t, db.NewRaw(`SELECT attempts FROM platform.email_outbox WHERE id = ?`, id).Scan(context.Background(), &attempts))
	return attempts
}

// Attempts spent on the provider must not eat the ones of the fallback, and the
// fallback is still bounded: it runs out and goes no further.
func TestTheFallbackGetsItsOwnAttempts(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	mail := enqueueEventMail(t, db, store, ctx, testpkg.Tenant(t), "fallback-attempts", true)
	sendEventMail(t, store, ctx, mail.ID, acceptedByProvider)
	_, err := db.ExecContext(context.Background(), `UPDATE platform.email_outbox SET attempts = 2 WHERE id = ?`, mail.ID)
	require.NoError(t, err)
	token := eventMailToken(t, db, mail.ID)
	event := eventAt(token, "suppressed", domain.EmailDeliverySuppressed, time.Now())
	event.Reroute = true

	result, err := store.RecordEmailDeliveryEvent(ctx, event)

	require.NoError(t, err)
	require.True(t, result.Rerouted)
	assert.Zero(t, mailAttempts(t, db, mail.ID))

	const maxAttempts = 3
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		fence := startEventMail(t, store, ctx, mail.ID)
		finalize, err := store.FinalizeFailure(ctx, domain.TransportEmail, mail.ID, fence, attempt, "mailbox down", time.Now().Add(-time.Second), maxAttempts)
		require.NoError(t, err)
		require.True(t, finalize.Finalized)
	}
	status, _, _ := mailStatus(t, db, mail.ID)
	assert.Equal(t, "dead_letter", status)
}

func TestRerouteDuringTheSendAlsoResetsTheAttempts(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	mail := enqueueEventMail(t, db, store, ctx, testpkg.Tenant(t), "fallback-attempts-send", true)
	_, err := db.ExecContext(context.Background(), `UPDATE platform.email_outbox SET attempts = 2 WHERE id = ?`, mail.ID)
	require.NoError(t, err)
	fence := startEventMail(t, store, ctx, mail.ID)
	event := eventAt(eventMailToken(t, db, mail.ID), "suppressed", domain.EmailDeliverySuppressed, time.Now())
	event.Reroute = true
	_, err = store.RecordEmailDeliveryEvent(ctx, event)
	require.NoError(t, err)
	assert.Equal(t, 2, mailAttempts(t, db, mail.ID), "the worker's own attempt is still running")

	finalized, err := store.FinalizeSent(ctx, domain.TransportEmail, mail.ID, fence, json.RawMessage(acceptedByProvider), time.Now())

	require.NoError(t, err)
	require.True(t, finalized)
	status, _, _ := mailStatus(t, db, mail.ID)
	assert.Equal(t, "pending", status)
	assert.Zero(t, mailAttempts(t, db, mail.ID))
}

func TestOpenProviderMailsNameTheirLastCheck(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	store, ctx := testStore(t, db)
	mail := enqueueEventMail(t, db, store, ctx, testpkg.Tenant(t), "checked", false)
	sendEventMail(t, store, ctx, mail.ID, acceptedByProvider)
	now := time.Now()
	query := domain.ProviderMailQuery{SentAfter: now.Add(-time.Hour), SentBefore: now.Add(time.Hour), CheckedBefore: now.Add(time.Hour), Limit: 10}
	mails, err := store.OpenProviderMails(ctx, query)
	require.NoError(t, err)
	require.Len(t, mails, 1)
	assert.Nil(t, mails[0].CheckedAt)

	checkedAt := now.Add(-time.Minute).Truncate(time.Second)
	require.NoError(t, store.MarkEmailDeliveryChecked(ctx, mail.ID, checkedAt))

	mails, err = store.OpenProviderMails(ctx, query)
	require.NoError(t, err)
	require.Len(t, mails, 1)
	require.NotNil(t, mails[0].CheckedAt)
	assert.True(t, mails[0].CheckedAt.Equal(checkedAt))
}

// Only the bounce goes, under the school's own row-level security.
func TestDeleteEmailSuppressionLeavesAReasonItMustNotDelete(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	_, err := db.ExecContext(context.Background(), `INSERT INTO platform.email_suppressions (tenant_id, address, reason) VALUES (?, 'bounce@example.com', 'failed'), (?, 'complaint@example.com', 'complained')`, school, school)
	require.NoError(t, err)

	require.NoError(t, store.DeleteEmailSuppression(ctx, school, "nobody@example.com"), "nothing to do")
	require.NoError(t, store.DeleteEmailSuppression(ctx, school, "bounce@example.com"))
	require.ErrorIs(t, store.DeleteEmailSuppression(ctx, school, "complaint@example.com"), domain.ErrSuppressionNeedsReview)

	listed, err := store.EmailAddressSuppressed(ctx, school, "complaint@example.com")
	require.NoError(t, err)
	assert.True(t, listed)
}
