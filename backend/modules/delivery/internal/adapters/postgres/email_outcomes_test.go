package postgres_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

func stopGuardian(t *testing.T, ctx context.Context, db *bun.DB, id int64) {
	t.Helper()
	_, err := db.ExecContext(ctx, `UPDATE users.guardian_profiles SET emails_stopped_at = NOW() WHERE id = ?`, id)
	require.NoError(t, err)
}

// Decision A: the stop is the person's, so it follows a changed address, leaves
// the old one free and never touches the list.
func TestStopFollowsThePersonToANewAddress(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	guardian := testpkg.CreateTestGuardianProfile(t, db, "stop-person")
	old := *guardian.Email

	stopGuardian(t, ctx, db, guardian.ID)
	listed, stopped, err := store.SchoolEmailHolds(ctx, school, " "+old+" ")
	require.NoError(t, err)
	assert.False(t, listed, "the stop is not on the address list")
	assert.True(t, stopped)

	next := "neu-" + old
	_, err = db.ExecContext(ctx, `UPDATE users.guardian_profiles SET email = ? WHERE id = ?`, next, guardian.ID)
	require.NoError(t, err)
	_, stopped, err = store.SchoolEmailHolds(ctx, school, next)
	require.NoError(t, err)
	assert.True(t, stopped, "a new address does not end the stop")
	_, stopped, err = store.SchoolEmailHolds(ctx, school, old)
	require.NoError(t, err)
	assert.False(t, stopped, "nobody holds the old address any more")
}

// A person who holds the address through an account stays stopped, however often
// the account's address changes.
func TestStopCoversTheAccountAddress(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	guardian := testpkg.CreateTestGuardianProfile(t, db, "stop-account")
	account := testpkg.CreateTestAccount(t, db, "stop-account-login")
	_, err := db.ExecContext(ctx, `UPDATE users.guardian_profiles SET account_id = ?, has_account = TRUE WHERE id = ?`, account.ID, guardian.ID)
	require.NoError(t, err)
	stopGuardian(t, ctx, db, guardian.ID)

	_, stopped, err := store.SchoolEmailHolds(ctx, school, account.Email)
	require.NoError(t, err)
	assert.True(t, stopped)
}

// A Träger's mail to a parent honours the list and the stop of the Träger's schools,
// and no other Träger's.
func TestTraegerMailHoldsAnswerForTheTraegersSchools(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	var organizationID int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, school).Scan(ctx, &organizationID))
	guardian := testpkg.CreateTestGuardianProfile(t, db, "stop-traeger")
	stopGuardian(t, ctx, db, guardian.ID)

	_, stopped, err := store.TraegerEmailHolds(ctx, organizationID, *guardian.Email)
	require.NoError(t, err)
	assert.True(t, stopped)

	_, stopped, err = store.TraegerEmailHolds(ctx, -organizationID, *guardian.Email)
	require.NoError(t, err)
	assert.False(t, stopped)
}

// Two people can hold one address through an account: the mail still reaches the
// one nobody stopped.
func TestStopLeavesAnotherHolderOfTheAddressAlone(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	free := testpkg.CreateTestGuardianProfile(t, db, "stop-shared-free")
	account := testpkg.CreateTestAccount(t, db, "stop-shared-login")
	_, err := db.ExecContext(ctx, `UPDATE users.guardian_profiles SET account_id = ?, has_account = TRUE WHERE id = ?`, account.ID, free.ID)
	require.NoError(t, err)
	stopped := testpkg.CreateTestGuardianProfile(t, db, "stop-shared-stopped")
	_, err = db.ExecContext(ctx, `UPDATE users.guardian_profiles SET email = ? WHERE id = ?`, account.Email, stopped.ID)
	require.NoError(t, err)
	stopGuardian(t, ctx, db, stopped.ID)

	_, held, err := store.SchoolEmailHolds(ctx, school, account.Email)
	require.NoError(t, err)
	assert.False(t, held)
}

// Finding 8: a bounce and a stop no longer share a row, so neither hides the other.
func TestABounceAndAStopStaySeparate(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	guardian := testpkg.CreateTestGuardianProfile(t, db, "stop-bounce")
	mail := relatedMail(t, db, store, ctx, school, "stop-bounce", *guardian.Email, 61)
	stopGuardian(t, ctx, db, guardian.ID)

	_, err := store.RecordEmailDeliveryEvent(ctx, eventAt(eventMailToken(t, db, mail), "hard", domain.EmailDeliveryFailed, time.Now()))
	require.NoError(t, err)
	listed, stopped, err := store.SchoolEmailHolds(ctx, school, *guardian.Email)
	require.NoError(t, err)
	assert.True(t, listed)
	assert.True(t, stopped)

	inSchool(t, ctx, db, school, func(txCtx context.Context) error {
		return store.Store.DeleteEmailSuppression(txCtx, school, *guardian.Email)
	})
	listed, stopped, err = store.SchoolEmailHolds(ctx, school, *guardian.Email)
	require.NoError(t, err)
	assert.False(t, listed)
	assert.True(t, stopped, "lifting the bounce does not end the school's decision")
}

// Decision E and finding 13: what the provider or the worker held back is a status.
// It fills no list, so it cannot undo a lift a moment ago either.
func TestAHeldBackMailMarksNoAddress(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	for _, test := range []struct {
		key      string
		state    domain.EmailDeliveryState
		provider string
		want     deliveryContract.EmailOutcome
	}{
		{"held-provider", domain.EmailDeliverySuppressed, "lettermint", deliveryContract.EmailBlocked},
		{"held-worker", domain.EmailDeliverySuppressed, "moto", deliveryContract.EmailBlocked},
		{"held-unsubscribed", domain.EmailDeliveryUnsubscribed, "lettermint", deliveryContract.EmailUnsubscribed},
	} {
		address := test.key + "@example.com"
		mail := relatedMail(t, db, store, ctx, school, test.key, address, 71)
		event := eventAt(eventMailToken(t, db, mail), test.key, test.state, time.Now())
		event.Provider = test.provider
		_, err := store.RecordEmailDeliveryEvent(ctx, event)
		require.NoError(t, err)
		listed, _, err := store.SchoolEmailHolds(ctx, school, address)
		require.NoError(t, err)
		assert.False(t, listed, test.key)
		inSchool(t, ctx, db, school, func(txCtx context.Context) error {
			latest, err := store.LatestEmailOutcomes(txCtx, school, []string{"guardian_invitation"}, []string{address})
			require.NoError(t, err)
			assert.Equal(t, test.want, latest[address].Outcome, test.key)
			return nil
		})
	}
}

// Finding 4 and 5: a bounced letter mail can be sent again, to the person's current
// address, and the mark of the old address no longer speaks for the person.
func TestLetterDeliveryFollowsTheCurrentAddressAndCanBeSentAgain(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	guardian := testpkg.CreateTestGuardianProfile(t, db, "letter-correct")
	old := strings.ToLower(*guardian.Email)
	mail := relatedMail(t, db, store, ctx, school, "letter-correct", old, 81)
	_, err := db.ExecContext(ctx, `UPDATE platform.email_outbox SET status = 'sent', sent_at = NOW() WHERE id = ?`, mail)
	require.NoError(t, err)
	var delivery int64
	require.NoError(t, db.NewRaw(`INSERT INTO platform.delivery_email_deliveries
		(tenant_id, related_entity_type, related_entity_id, outbox_id, guardian_profile_id, recipient_email, reachability)
		VALUES (?, 'parent_announcement', 81, ?, ?, ?, 'ok') RETURNING id`, school, mail, guardian.ID, old).Scan(ctx, &delivery))

	statuses := func() deliveryContract.EmailDeliveryStatus {
		var rows []deliveryContract.EmailDeliveryStatus
		inSchool(t, ctx, db, school, func(txCtx context.Context) error {
			var err error
			rows, err = store.EmailDeliveryStatuses(txCtx, school, "parent_announcement", 81)
			return err
		})
		require.Len(t, rows, 1)
		return rows[0]
	}
	assert.Equal(t, "sent", statuses().EmailStatus, "accepted, nothing reported")

	_, err = store.RecordEmailDeliveryEvent(ctx, eventAt(eventMailToken(t, db, mail), "hard", domain.EmailDeliveryFailed, time.Now()))
	require.NoError(t, err)
	row := statuses()
	assert.Equal(t, "failed", row.EmailStatus, "a bounce is no sent mail")
	assert.Equal(t, deliveryContract.EmailMarkFailed, row.AddressMark)

	next := "neu-" + old
	_, err = db.ExecContext(ctx, `UPDATE users.guardian_profiles SET email = ? WHERE id = ?`, next, guardian.ID)
	require.NoError(t, err)
	row = statuses()
	require.NotNil(t, row.RecipientEmail)
	assert.Equal(t, next, *row.RecipientEmail, "the status names the current address")
	assert.Empty(t, row.AddressMark, "the old address's mark ended with the correction")

	inSchool(t, ctx, db, school, func(txCtx context.Context) error {
		claimed, err := store.ClaimFailedEmailDelivery(txCtx, school, delivery)
		require.NoError(t, err)
		assert.True(t, claimed, "a bounced mail can be sent again")
		return nil
	})
}

// A letter's delayed mail can be sent again once the wait has ended, and only then.
func TestAnOverdueLetterMailCanBeSentAgain(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	guardian := testpkg.CreateTestGuardianProfile(t, db, "letter-overdue")
	address := strings.ToLower(*guardian.Email)
	mail := relatedMail(t, db, store, ctx, school, "letter-overdue", address, 82)
	var delivery int64
	require.NoError(t, db.NewRaw(`INSERT INTO platform.delivery_email_deliveries
		(tenant_id, related_entity_type, related_entity_id, outbox_id, guardian_profile_id, recipient_email, reachability)
		VALUES (?, 'parent_announcement', 82, ?, ?, ?, 'ok') RETURNING id`, school, mail, guardian.ID, address).Scan(ctx, &delivery))
	delayedSince := func(age time.Duration) {
		_, err := db.ExecContext(ctx, `UPDATE platform.email_outbox
			SET status = 'sent', sent_at = ?0, delivery_state = 'delayed', delivery_state_at = ?0 WHERE id = ?1`, time.Now().Add(-age), mail)
		require.NoError(t, err)
	}

	delayedSince(time.Hour)
	inSchool(t, ctx, db, school, func(txCtx context.Context) error {
		rows, err := store.EmailDeliveryStatuses(txCtx, school, "parent_announcement", 82)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, "sent", rows[0].EmailStatus, "it is still on its way")
		claimed, err := store.ClaimFailedEmailDelivery(txCtx, school, delivery)
		require.NoError(t, err)
		assert.False(t, claimed)
		return nil
	})

	delayedSince(deliveryContract.EmailDelayLimit + time.Hour)
	inSchool(t, ctx, db, school, func(txCtx context.Context) error {
		rows, err := store.EmailDeliveryStatuses(txCtx, school, "parent_announcement", 82)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, "failed", rows[0].EmailStatus)
		assert.Equal(t, deliveryContract.EmailOverdue, rows[0].Outcome)
		claimed, err := store.ClaimFailedEmailDelivery(txCtx, school, delivery)
		require.NoError(t, err)
		assert.True(t, claimed)
		return nil
	})
}

// A delayed mail stops blocking once the wait has ended.
func TestADelayedMailBecomesOverdue(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	address := "overdue@example.com"
	mail := relatedMail(t, db, store, ctx, school, "overdue", address, 91)
	_, err := store.RecordEmailDeliveryEvent(ctx, eventAt(eventMailToken(t, db, mail), "slow", domain.EmailDeliveryDelayed, time.Now().Add(-deliveryContract.EmailDelayLimit-time.Hour)))
	require.NoError(t, err)
	inSchool(t, ctx, db, school, func(txCtx context.Context) error {
		latest, err := store.LatestEmailOutcomes(txCtx, school, []string{"guardian_invitation"}, []string{address})
		require.NoError(t, err)
		assert.Equal(t, deliveryContract.EmailOverdue, latest[address].Outcome)
		return nil
	})
}

// A mail keeps the address it went to. A new address does not take over its result,
// and another mail goes to the new one.
func TestALetterRowKeepsTheAddressItsMailWentTo(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	guardian := testpkg.CreateTestGuardianProfile(t, db, "letter-mailed-to")
	old := strings.ToLower(*guardian.Email)
	mail := relatedMail(t, db, store, ctx, school, "letter-mailed-to", old, 83)
	_, err := db.ExecContext(ctx, `UPDATE platform.email_outbox SET status = 'sent', sent_at = NOW(), delivery_state = 'delivered', delivery_state_at = NOW() WHERE id = ?`, mail)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO platform.delivery_email_deliveries
		(tenant_id, related_entity_type, related_entity_id, outbox_id, guardian_profile_id, recipient_email, reachability)
		VALUES (?, 'parent_announcement', 83, ?, ?, ?, 'ok')`, school, mail, guardian.ID, old)
	require.NoError(t, err)
	next := "neu-" + old
	_, err = db.ExecContext(ctx, `UPDATE users.guardian_profiles SET email = ? WHERE id = ?`, next, guardian.ID)
	require.NoError(t, err)

	inSchool(t, ctx, db, school, func(txCtx context.Context) error {
		rows, err := store.EmailDeliveryStatuses(txCtx, school, "parent_announcement", 83)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		assert.Equal(t, deliveryContract.EmailDelivered, rows[0].Outcome)
		require.NotNil(t, rows[0].MailedTo)
		assert.Equal(t, old, *rows[0].MailedTo, "delivered there, not at the new address")
		require.NotNil(t, rows[0].RecipientEmail)
		assert.Equal(t, next, *rows[0].RecipientEmail, "the next mail goes here")
		return nil
	})
}

// What the status calls failed is exactly what a claim takes. A mail cancelled on
// purpose is neither, and a held mail is both, whether its own event arrived or not.
func TestTheLetterStatusAndTheClaimAgree(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	for index, test := range []struct {
		name          string
		status        string
		deliveryState any
		lastError     any
		wantStatus    string
		claims        bool
	}{
		{"cancelled on purpose", "cancelled", nil, "the letter was withdrawn", "cancelled", false},
		{"held by the list", "cancelled", "suppressed", deliveryContract.MailHeldByList, "failed", true},
		{"held by the list, event lost", "cancelled", nil, deliveryContract.MailHeldByList, "failed", true},
		{"held by a stop, event lost", "cancelled", nil, deliveryContract.MailHeldByStop, "failed", true},
		{"given up", "dead_letter", nil, "smtp refused", "failed", true},
		{"withheld by the provider", "sent", "suppressed", nil, "failed", true},
		{"accepted", "sent", nil, nil, "sent", false},
	} {
		related := int64(8400 + index)
		address := fmt.Sprintf("claim-%d-%d@example.com", index, testpkg.UniqueTestTenantID(t))
		mail := relatedMail(t, db, store, ctx, school, fmt.Sprintf("claim-%d", index), address, related)
		_, err := db.ExecContext(ctx, `UPDATE platform.email_outbox SET status = ?, delivery_state = ?, delivery_state_at = NOW(), last_error = ?, sent_at = NOW(),
			dead_letter_at = CASE WHEN ? = 'dead_letter' THEN NOW() END, cancelled_at = CASE WHEN ? = 'cancelled' THEN NOW() END WHERE id = ?`,
			test.status, test.deliveryState, test.lastError, test.status, test.status, mail)
		require.NoError(t, err, test.name)
		var delivery int64
		require.NoError(t, db.NewRaw(`INSERT INTO platform.delivery_email_deliveries
			(tenant_id, related_entity_type, related_entity_id, outbox_id, recipient_email, reachability)
			VALUES (?, 'parent_announcement', ?, ?, ?, 'ok') RETURNING id`, school, related, mail, address).Scan(ctx, &delivery), test.name)
		inSchool(t, ctx, db, school, func(txCtx context.Context) error {
			rows, err := store.EmailDeliveryStatuses(txCtx, school, "parent_announcement", related)
			require.NoError(t, err, test.name)
			require.Len(t, rows, 1, test.name)
			assert.Equal(t, test.wantStatus, rows[0].EmailStatus, test.name)
			claimed, err := store.ClaimFailedEmailDelivery(txCtx, school, delivery)
			require.NoError(t, err, test.name)
			assert.Equal(t, test.claims, claimed, test.name)
			return nil
		})
	}
}

// A mark of an address only an open application names stays through the night; a
// decided or withdrawn application holds nothing, and nothing crosses to another school.
func TestRetentionKeepsTheMarkOfAnOpenApplication(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	apply := func(address, childStatus string, withdrawn bool) {
		t.Helper()
		var request int64
		require.NoError(t, db.NewRaw(`INSERT INTO enrollment.requests
			(tenant_id, phase_id, guardian_first_name, guardian_last_name, guardian_email, consent_flags, custom_data, status_token, submitted_at, withdrawn_at)
			VALUES (?, ?, 'Anmeldung', 'Test', ?, '{}'::jsonb, '{}'::jsonb, ?, NOW(), CASE WHEN ? THEN NOW() END) RETURNING id`,
			school, phase.ID, address, "token-"+address, withdrawn).Scan(ctx, &request))
		_, err := db.ExecContext(ctx, `INSERT INTO enrollment.request_children
			(tenant_id, request_id, first_name, last_name, date_of_birth, status, activation_mode, sort_order, custom_data)
			VALUES (?, ?, 'Kind', 'Test', '2018-04-15', ?, 'scheduled', 0, '{}'::jsonb)`, school, request, childStatus)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `INSERT INTO platform.email_suppressions (tenant_id, address, reason) VALUES (?, ?, 'failed')`, school, address)
		require.NoError(t, err)
	}
	unique := testpkg.UniqueTestTenantID(t)
	open := fmt.Sprintf("offen-%d@example.com", unique)
	waiting := fmt.Sprintf("warteliste-%d@example.com", unique)
	decided := fmt.Sprintf("zusage-%d@example.com", unique)
	gone := fmt.Sprintf("zurueckgezogen-%d@example.com", unique)
	apply(open, "submitted", false)
	apply(waiting, "waitlisted", false)
	apply(decided, "approved", false)
	apply(gone, "submitted", true)

	inSchool(t, ctx, db, school, func(txCtx context.Context) error {
		_, err := store.DeleteUnheldEmailSuppressions(txCtx, school)
		return err
	})

	var left []string
	require.NoError(t, db.NewRaw(`SELECT address FROM platform.email_suppressions WHERE tenant_id = ? AND address IN (?)`,
		school, bun.List([]string{open, waiting, decided, gone})).Scan(ctx, &left))
	assert.ElementsMatch(t, []string{open, waiting}, left)
}

// A mail the stop held back says so, also once the stop has ended and also without
// its event, and never blames the address.
func TestAMailHeldByTheStopKeepsItsReason(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	guardian := testpkg.CreateTestGuardianProfile(t, db, "held-by-stop")
	address := strings.ToLower(*guardian.Email)
	stopGuardian(t, ctx, db, guardian.ID)
	mail := relatedMail(t, db, store, ctx, school, "held-by-stop", address, 72)
	_, err := db.ExecContext(ctx, `UPDATE platform.email_outbox SET status = 'cancelled', cancelled_at = NOW(), last_error = ? WHERE id = ?`, deliveryContract.MailHeldByStop, mail)
	require.NoError(t, err)
	read := func() deliveryContract.EmailOutcomeReport {
		var report deliveryContract.EmailOutcomeReport
		inSchool(t, ctx, db, school, func(txCtx context.Context) error {
			latest, err := store.LatestEmailOutcomes(txCtx, school, []string{"guardian_invitation"}, []string{address})
			report = latest[address]
			return err
		})
		return report
	}
	assert.Equal(t, deliveryContract.EmailHeldByStop, read().Outcome, "without its event")

	_, err = store.RecordEmailDeliveryEvent(ctx, eventAt(eventMailToken(t, db, mail), "held-back", domain.EmailDeliverySuppressed, time.Now()))
	require.NoError(t, err)
	assert.Equal(t, deliveryContract.EmailHeldByStop, read().Outcome, "with its event")

	_, err = db.ExecContext(ctx, `UPDATE users.guardian_profiles SET emails_stopped_at = NULL WHERE id = ?`, guardian.ID)
	require.NoError(t, err)
	assert.Equal(t, deliveryContract.EmailHeldByStop, read().Outcome, "after the stop ended")
}

// A Träger reads the holds of its own schools in its own transaction, and nobody
// else's. Its role reads no school's rows: the function answers for the session.
func TestTraegerMailHoldsStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store, ctx := testStore(t, db)
	school := testpkg.Tenant(t)
	var organization int64
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, school).Scan(ctx, &organization))
	other := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Anderer Träger", Slug: fmt.Sprintf("anderer-%d", testpkg.UniqueTestTenantID(t)), Active: true,
	}).ID
	guardian := testpkg.CreateTestGuardianProfile(t, db, "holds-traeger")
	address := strings.ToLower(*guardian.Email)
	stopGuardian(t, ctx, db, guardian.ID)
	_, err := db.ExecContext(ctx, `INSERT INTO platform.email_suppressions (tenant_id, address, reason) VALUES (?, ?, 'failed')`, school, address)
	require.NoError(t, err)

	require.NoError(t, tenant.WithinTraeger(ctx, organization, func(txCtx context.Context) error {
		listed, stopped, err := store.Store.TraegerMailHolds(txCtx, organization, address)
		require.NoError(t, err)
		assert.True(t, listed)
		assert.True(t, stopped)
		return nil
	}))
	require.NoError(t, tenant.WithinTraeger(ctx, other, func(txCtx context.Context) error {
		listed, stopped, err := store.Store.TraegerMailHolds(txCtx, other, address)
		require.NoError(t, err)
		assert.False(t, listed, "another Träger learns nothing")
		assert.False(t, stopped)
		return nil
	}))
}
