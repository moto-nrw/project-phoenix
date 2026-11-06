package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func relatedMail(t *testing.T, db *bun.DB, store *workerCommandsForTest, ctx context.Context, school int64, key, address string, relatedID int64) int64 {
	t.Helper()
	var mail domain.Enqueued
	relatedType := "invitation"
	require.NoError(t, tenant.WithTenantTx(ctx, db, school, func(txCtx context.Context, _ bun.Tx) error {
		var err error
		mail, err = store.Enqueue(txCtx, domain.Intent{
			TenantID: school, Transport: domain.TransportEmail, Template: "guardian_invitation", IdempotencyKey: &key,
			RelatedEntityType: &relatedType, RelatedEntityID: &relatedID,
			Recipient: json.RawMessage(`{"address":"` + address + `"}`), Payload: json.RawMessage(`{}`),
			Status: "pending", NextRetryAt: time.Now().Add(-time.Minute),
		})
		return err
	}))
	return mail.ID
}

func inSchool(t *testing.T, ctx context.Context, db *bun.DB, school int64, run func(txCtx context.Context) error) {
	t.Helper()
	require.NoError(t, tenant.WithTenantTx(ctx, db, school, func(txCtx context.Context, _ bun.Tx) error { return run(txCtx) }))
}

// A school reads its own mails only, under its own row-level security.
func TestEmailOutcomesStayInsideTheSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school, other := testpkg.Tenant(t), testpkg.NewTenantScope(t, db).TenantID
	mine := relatedMail(t, db, store, ctx, school, "outcome-mine", "Eltern@Example.com", 41)
	theirs := relatedMail(t, db, store, ctx, other, "outcome-theirs", "Eltern@Example.com", 41)
	_, err := store.RecordEmailDeliveryEvent(ctx, eventAt(eventMailToken(t, db, mine), "ok", domain.EmailDeliveryDelivered, time.Now()))
	require.NoError(t, err)
	_, err = store.RecordEmailDeliveryEvent(ctx, eventAt(eventMailToken(t, db, theirs), "bad", domain.EmailDeliveryFailed, time.Now()))
	require.NoError(t, err)

	inSchool(t, ctx, db, school, func(txCtx context.Context) error {
		reports, err := store.EmailOutcomes(txCtx, school, deliveryContract.EmailOutcomeQuery{RelatedType: "invitation", RelatedIDs: []int64{41}})
		require.NoError(t, err)
		require.Len(t, reports, 1)
		assert.Equal(t, mine, reports[0].MailID)
		assert.Equal(t, deliveryContract.EmailDelivered, reports[0].Outcome)
		assert.Empty(t, reports[0].Mark, "the mark of the other school does not count here")

		latest, err := store.LatestEmailOutcomes(txCtx, school, []string{"guardian_invitation"}, []string{" ELTERN@example.com "})
		require.NoError(t, err)
		assert.Equal(t, mine, latest["eltern@example.com"].MailID)

		marks, err := store.EmailAddressMarks(txCtx, school, []string{"eltern@example.com"})
		require.NoError(t, err)
		assert.Empty(t, marks)
		return nil
	})

	// Even naming the other school in this school's transaction reads nothing.
	inSchool(t, ctx, db, school, func(txCtx context.Context) error {
		reports, err := store.EmailOutcomes(txCtx, other, deliveryContract.EmailOutcomeQuery{RelatedType: "invitation", RelatedIDs: []int64{41}})
		require.NoError(t, err)
		assert.Empty(t, reports)
		return nil
	})
}

func TestLatestEmailOutcomeIsTheNewestMail(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	first := relatedMail(t, db, store, ctx, school, "latest-1", "eltern@example.com", 51)
	_, err := store.RecordEmailDeliveryEvent(ctx, eventAt(eventMailToken(t, db, first), "slow", domain.EmailDeliveryDelayed, time.Now()))
	require.NoError(t, err)
	second := relatedMail(t, db, store, ctx, school, "latest-2", "eltern@example.com", 52)

	inSchool(t, ctx, db, school, func(txCtx context.Context) error {
		latest, err := store.LatestEmailOutcomes(txCtx, school, []string{"guardian_invitation"}, []string{"eltern@example.com"})
		require.NoError(t, err)
		assert.Equal(t, second, latest["eltern@example.com"].MailID)
		assert.Equal(t, deliveryContract.EmailQueued, latest["eltern@example.com"].Outcome)
		return nil
	})
}

// The stop of a school is written in the school's own transaction and survives
// what the mail server reports later; a complaint outranks it.
func TestSchoolStopIsKeptAndComplaintOutranksIt(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	mail := relatedMail(t, db, store, ctx, school, "stop-1", "stop@example.com", 61)

	mark := func() deliveryContract.EmailAddressMark {
		var found deliveryContract.EmailAddressMark
		inSchool(t, ctx, db, school, func(txCtx context.Context) error {
			marks, err := store.EmailAddressMarks(txCtx, school, []string{"stop@example.com"})
			found = marks["stop@example.com"]
			return err
		})
		return found
	}
	inSchool(t, ctx, db, school, func(txCtx context.Context) error {
		return store.StopEmailsToAddress(txCtx, school, " Stop@Example.com ")
	})
	assert.Equal(t, deliveryContract.EmailMarkSchoolStop, mark())

	_, err := store.RecordEmailDeliveryEvent(ctx, eventAt(eventMailToken(t, db, mail), "hard", domain.EmailDeliveryFailed, time.Now()))
	require.NoError(t, err)
	assert.Equal(t, deliveryContract.EmailMarkSchoolStop, mark(), "a bounce does not turn the school's decision into something else")

	_, err = store.RecordEmailDeliveryEvent(ctx, eventAt(eventMailToken(t, db, mail), "spam", domain.EmailDeliveryComplained, time.Now()))
	require.NoError(t, err)
	assert.Equal(t, deliveryContract.EmailMarkComplained, mark())

	inSchool(t, ctx, db, school, func(txCtx context.Context) error {
		return store.StopEmailsToAddress(txCtx, school, "stop@example.com")
	})
	assert.Equal(t, deliveryContract.EmailMarkComplained, mark(), "the school's stop does not wash a complaint away")
}

// A provider that holds a mail back knows the address from elsewhere; the school
// must see why its mail did not leave.
func TestProviderHoldBackMarksTheAddress(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	mail := relatedMail(t, db, store, ctx, school, "held-1", "held@example.com", 71)
	_, err := store.RecordEmailDeliveryEvent(ctx, eventAt(eventMailToken(t, db, mail), "suppressed", domain.EmailDeliverySuppressed, time.Now()))
	require.NoError(t, err)
	inSchool(t, ctx, db, school, func(txCtx context.Context) error {
		marks, err := store.EmailAddressMarks(txCtx, school, []string{"held@example.com"})
		require.NoError(t, err)
		assert.Equal(t, deliveryContract.EmailMarkFailed, marks["held@example.com"])
		return nil
	})
}
