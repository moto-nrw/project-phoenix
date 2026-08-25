package staffmessaging_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/services/staffmessaging"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestCrossTenantIsolation is the security test for this feature: a staff
// account at school B must never see school A's conversations, not in the
// inbox, not in the unread badge, not by opening a thread id directly, and not
// in the recipient picker.
//
// It runs the SAME service against two different tenant contexts, which is how
// the tenant predicate (and RLS behind it) gets exercised rather than assumed.
func TestCrossTenantIsolation(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc := newService(t, db)

	// School A is this test's own tenant; school B is a second one.
	schoolA := testpkg.Tenant(t)
	schoolB, _ := testpkg.CreateTestTenant(t, db)

	annaA, benA := twoColleagues(t, db)

	// Someone who belongs to school B ONLY. The fixture creates the person and
	// staff rows in school B but auto-claims the ACCOUNT for this test's own
	// tenant (school A), so both halves have to be corrected explicitly —
	// otherwise the "outsider" is a legitimate member of school A and this test
	// would assert the opposite of what it means. Dual membership is a real
	// thing in this codebase; it just must not be what this test measures.
	_, outsiderB := testpkg.CreateTestStaffWithAccountForTenant(t, db, schoolB, "Bea", "Fremd")
	testpkg.EnsureAccountTenant(t, db, outsiderB.ID, schoolB)
	testpkg.UnclaimTestAccount(t, db, outsiderB.ID)

	ctxA := func(accountID int64) context.Context {
		return context.WithValue(tenant.WithTenantID(context.Background(), schoolA), jwt.CtxClaims, jwt.AppClaims{ID: int(accountID)})
	}
	ctxB := func(accountID int64) context.Context {
		return context.WithValue(tenant.WithTenantID(context.Background(), schoolB), jwt.CtxClaims, jwt.AppClaims{ID: int(accountID)})
	}

	// A conversation inside school A.
	thread, err := svc.OpenThread(ctxA(annaA), benA)
	require.NoError(t, err)
	_, err = svc.PostMessage(ctxA(annaA), thread.ThreadID, "Interne Absprache Schule A")
	require.NoError(t, err)

	// School B sees nothing of it.
	inbox, err := svc.ListInbox(ctxB(outsiderB.ID), false)
	require.NoError(t, err)
	assert.Empty(t, inbox, "school B must not see school A's conversations")

	count, err := svc.UnreadMessageCount(ctxB(outsiderB.ID))
	require.NoError(t, err)
	assert.Zero(t, count, "school A's messages must not reach school B's badge")

	// Opening school A's thread id from school B must fail as "not found",
	// never leak the conversation.
	_, err = svc.GetThread(ctxB(outsiderB.ID), thread.ThreadID)
	require.Error(t, err)
	assert.True(t,
		assert.ObjectsAreEqual(staffmessaging.ErrThreadNotFound, err) ||
			assert.ObjectsAreEqual(staffmessaging.ErrNotParticipant, err),
		"expected a not-found style error, got %v", err)

	// And school A cannot address school B's staff.
	_, err = svc.OpenThread(ctxA(annaA), outsiderB.ID)
	require.ErrorIs(t, err, staffmessaging.ErrRecipientNotAvailable)

	// The picker at school A never offers school B's staff.
	recipients, err := svc.ListMessageableStaff(ctxA(annaA))
	require.NoError(t, err)
	for _, r := range recipients {
		assert.NotEqual(t, outsiderB.ID, r.AccountID, "picker must stay inside the school")
	}
}

// TestRetentionSweep verifies the GDPR window actually deletes, and that a
// conversation left without any message disappears with it.
func TestRetentionSweep(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc := newServiceWithEnabled(t, db, true, 30)
	anna, ben := twoColleagues(t, db)
	ctx := testpkg.Ctx(t)

	thread, err := svc.OpenThread(asAccount(t, anna), ben)
	require.NoError(t, err)

	old, err := svc.PostMessage(asAccount(t, anna), thread.ThreadID, "Alte Nachricht")
	require.NoError(t, err)
	_, err = svc.PostMessage(asAccount(t, anna), thread.ThreadID, "Neue Nachricht")
	require.NoError(t, err)

	// Age the first message past the 30-day window.
	_, err = db.NewUpdate().
		Table("users.staff_messages").
		Set("created_at = ?", time.Now().AddDate(0, 0, -40)).
		Where("id = ?", old.ID).
		Exec(ctx)
	require.NoError(t, err)

	result, err := svc.CleanupExpiredMessages(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, result.MessagesDeleted)
	assert.Equal(t, 30, result.RetentionDays)
	assert.Zero(t, result.ThreadsDeleted, "the conversation still holds a message")

	detail, err := svc.GetThread(asAccount(t, anna), thread.ThreadID)
	require.NoError(t, err)
	require.Len(t, detail.Messages, 1)
	assert.Equal(t, "Neue Nachricht", detail.Messages[0].Body)

	// Age the survivor AND the thread itself. The thread needs ageing too: since
	// the grace period (DeleteEmpty's cutoff) a freshly created conversation is
	// protected even once its messages are gone, so only a thread that is itself
	// older than the window may be swept.
	_, err = db.NewUpdate().
		Table("users.staff_messages").
		Set("created_at = ?", time.Now().AddDate(0, 0, -40)).
		Where("thread_id = ?", thread.ThreadID).
		Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewUpdate().
		Table("users.staff_message_threads").
		Set("created_at = ?", time.Now().AddDate(0, 0, -40)).
		Where("id = ?", thread.ThreadID).
		Exec(ctx)
	require.NoError(t, err)

	result, err = svc.CleanupExpiredMessages(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, result.MessagesDeleted)
	assert.Equal(t, 1, result.ThreadsDeleted, "a conversation with no messages left must not linger")

	_, err = svc.GetThread(asAccount(t, anna), thread.ThreadID)
	require.ErrorIs(t, err, staffmessaging.ErrThreadNotFound)
}

// TestRetentionRunsForDisabledSchool pins that retention is NOT gated on the
// feature switch: a school that turned the chat off must still have its old
// messages aged out rather than frozen forever.
func TestRetentionRunsForDisabledSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	enabled := newServiceWithEnabled(t, db, true, 30)
	anna, ben := twoColleagues(t, db)
	ctx := testpkg.Ctx(t)

	thread, err := enabled.OpenThread(asAccount(t, anna), ben)
	require.NoError(t, err)
	msg, err := enabled.PostMessage(asAccount(t, anna), thread.ThreadID, "Alt")
	require.NoError(t, err)

	_, err = db.NewUpdate().
		Table("users.staff_messages").
		Set("created_at = ?", time.Now().AddDate(0, 0, -40)).
		Where("id = ?", msg.ID).
		Exec(ctx)
	require.NoError(t, err)

	disabled := newServiceWithEnabled(t, db, false, 30)
	result, err := disabled.CleanupExpiredMessages(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, result.MessagesDeleted, "retention must not depend on the feature switch")
}

// TestGuardianAccountIsNotAddressable is the regression guard for the quorum
// finding: a guardian who accepted an invitation has an ACTIVE
// auth.account_tenants row for the school but no users.persons row. Checking
// membership alone would let a caller pass that account id straight to
// POST /threads/open and open a "colleague" chat with a parent, bypassing a
// picker that never offered them.
func TestGuardianAccountIsNotAddressable(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc := newService(t, db)
	anna, _ := twoColleagues(t, db)
	ctx := testpkg.Ctx(t)

	// A guardian-shaped account: active tenant mapping, no persons row.
	guardian := testpkg.CreateTestAccount(t, db, "erika.sorgeberechtigt")
	testpkg.EnsureAccountTenant(t, db, guardian.ID, testpkg.Tenant(t))
	_, err := db.NewUpdate().
		Table("auth.account_tenants").
		Set("status = ?", authModels.AccountTenantStatusActive).
		Where("account_id = ? AND tenant_id = ?", guardian.ID, testpkg.Tenant(t)).
		Exec(ctx)
	require.NoError(t, err)

	// Never offered by the picker...
	recipients, err := svc.ListMessageableStaff(asAccount(t, anna))
	require.NoError(t, err)
	for _, r := range recipients {
		assert.NotEqual(t, guardian.ID, r.AccountID, "picker must not offer a guardian account")
	}

	// ...and the direct API path must refuse it too, not just the UI.
	_, err = svc.OpenThread(asAccount(t, anna), guardian.ID)
	require.ErrorIs(t, err, staffmessaging.ErrRecipientNotAvailable,
		"an account without a staff person row must not be addressable")
}

// TestFreshEmptyThreadSurvivesSweep pins the grace period: OpenThread creates
// the thread before the first message, so the daily sweep must not delete a
// conversation someone just opened and has not written in yet.
func TestFreshEmptyThreadSurvivesSweep(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc := newServiceWithEnabled(t, db, true, 30)
	anna, ben := twoColleagues(t, db)
	ctx := testpkg.Ctx(t)

	thread, err := svc.OpenThread(asAccount(t, anna), ben)
	require.NoError(t, err)

	result, err := svc.CleanupExpiredMessages(ctx)
	require.NoError(t, err)
	assert.Zero(t, result.ThreadsDeleted, "a just-opened conversation must survive the sweep")

	// Still there, and still writable.
	_, err = svc.PostMessage(asAccount(t, anna), thread.ThreadID, "Doch noch was")
	require.NoError(t, err, "the thread must still exist after the sweep")

	// An OLD empty thread is a different matter and does go.
	_, err = db.NewUpdate().
		Table("users.staff_message_threads").
		Set("created_at = ?", time.Now().AddDate(0, 0, -40)).
		Where("id = ?", thread.ThreadID).
		Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewDelete().
		Table("users.staff_messages").
		Where("thread_id = ?", thread.ThreadID).
		Exec(ctx)
	require.NoError(t, err)

	result, err = svc.CleanupExpiredMessages(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, result.ThreadsDeleted, "an aged-out empty conversation must go")
}

// TestNonStaffPersonIsNotAddressable is the second half of the guardian guard:
// users.persons also holds children and guests, who can carry an account and an
// active tenant mapping. Checking persons alone would let such an account be
// opened as a "colleague" chat. Only the users.staff relation makes someone one.
func TestNonStaffPersonIsNotAddressable(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	svc := newService(t, db)
	anna, _ := twoColleagues(t, db)

	// A person WITH an account and an active tenant mapping, but no staff row -
	// the shape a child or guest account has.
	_, account := testpkg.CreateTestPersonWithAccount(t, db, "Mila", "Kindkonto")
	testpkg.EnsureAccountTenant(t, db, account.ID, testpkg.Tenant(t))

	recipients, err := svc.ListMessageableStaff(asAccount(t, anna))
	require.NoError(t, err)
	for _, r := range recipients {
		assert.NotEqual(t, account.ID, r.AccountID,
			"a person without a staff row must not be offered as a recipient")
	}

	_, err = svc.OpenThread(asAccount(t, anna), account.ID)
	require.ErrorIs(t, err, staffmessaging.ErrRecipientNotAvailable,
		"the direct API path must apply the same staff-only rule as the picker")
}

// TestRetentionSkipsWhenWindowUnresolvable pins that the sweep never deletes on
// a guessed window: a fallback that is too short destroys messages the school
// was entitled to keep, one that is too long keeps employee data past its window
// and hides the misconfiguration behind a green job.
func TestRetentionSkipsWhenWindowUnresolvable(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	anna, ben := twoColleagues(t, db)
	ctx := testpkg.Ctx(t)

	enabled := newServiceWithEnabled(t, db, true, 30)
	thread, err := enabled.OpenThread(asAccount(t, anna), ben)
	require.NoError(t, err)
	msg, err := enabled.PostMessage(asAccount(t, anna), thread.ThreadID, "Uralt")
	require.NoError(t, err)
	_, err = db.NewUpdate().
		Table("users.staff_messages").
		Set("created_at = ?", time.Now().AddDate(0, 0, -400)).
		Where("id = ?", msg.ID).
		Exec(ctx)
	require.NoError(t, err)

	broken := newServiceWithBrokenRetention(t, db)
	result, err := broken.CleanupExpiredMessages(ctx)
	require.ErrorIs(t, err, staffmessaging.ErrRetentionUnresolved)
	assert.Zero(t, result.MessagesDeleted, "nothing may be deleted on an unknown window")

	// Und die Nachricht ist wirklich noch da.
	detail, err := enabled.GetThread(asAccount(t, anna), thread.ThreadID)
	require.NoError(t, err)
	assert.Len(t, detail.Messages, 1)
}
