package staffmessaging_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
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

	// Age the survivor too — now the empty conversation must go as well.
	_, err = db.NewUpdate().
		Table("users.staff_messages").
		Set("created_at = ?", time.Now().AddDate(0, 0, -40)).
		Where("thread_id = ?", thread.ThreadID).
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
