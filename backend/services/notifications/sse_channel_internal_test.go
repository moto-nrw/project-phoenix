package notifications

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// fakeGuardianAccess records the delivery-time access question and answers it
// from a fixed permitted set.
type fakeGuardianAccess struct {
	permitted  []int64
	err        error
	calls      int
	accountIDs []int64
	studentIDs []int64
	permission string
	tenantID   int64
}

func (f *fakeGuardianAccess) FilterAccountsWithStudentAccess(_ context.Context, guardianAccountIDs, studentIDs []int64, tenantID int64, permission string) ([]int64, error) {
	f.calls++
	f.accountIDs = guardianAccountIDs
	f.studentIDs = studentIDs
	f.tenantID = tenantID
	f.permission = permission
	return f.permitted, f.err
}

// recordingGuardianBroadcaster records guardian fan-out targets. It is local to
// this file because the shared RecordingBroadcaster lives in package test, which
// an internal test of this package cannot reach without an import cycle.
type recordingGuardianBroadcaster struct {
	guardians []int64
	err       error
}

func (b *recordingGuardianBroadcaster) BroadcastToGuardian(_, guardianAccountID int64, _ realtime.Event) error {
	b.guardians = append(b.guardians, guardianAccountID)
	return b.err
}

func (b *recordingGuardianBroadcaster) BroadcastToGroup(int64, string, realtime.Event) error {
	return nil
}
func (b *recordingGuardianBroadcaster) BroadcastToGroups(int64, []string, realtime.Event) error {
	return nil
}
func (b *recordingGuardianBroadcaster) BroadcastToTenant(int64, realtime.Event) error { return nil }
func (b *recordingGuardianBroadcaster) BroadcastToTenantAdmins(int64, realtime.Event) error {
	return nil
}
func (b *recordingGuardianBroadcaster) BroadcastToAll(realtime.Event) error { return nil }
func (b *recordingGuardianBroadcaster) BroadcastParentMessage(int64, int64, realtime.Event) error {
	return nil
}

func (b *recordingGuardianBroadcaster) BroadcastToStaffAccounts(int64, []int64, realtime.Event) error {
	return nil
}

// mockDB returns a bun DB backed by sqlmock, asserting at teardown that exactly
// the expected statements ran.
func mockDB(t *testing.T) (*bun.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := bun.NewDB(sqlDB, pgdialect.New())
	t.Cleanup(func() {
		_ = sqlDB.Close()
		require.NoError(t, mock.ExpectationsWereMet())
	})
	return db, mock
}

// mockTenantTxDB returns a bun DB whose tenant transaction (BEGIN, SET LOCAL
// ROLE, set_config, COMMIT/ROLLBACK) is satisfied by sqlmock; the access lookup
// itself is faked, so no statement inside the transaction is expected.
func mockTenantTxDB(t *testing.T, commits bool) *bun.DB {
	t.Helper()
	db, mock := mockDB(t)
	mock.ExpectBegin()
	mock.ExpectExec("SET LOCAL ROLE phoenix_tenant").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("SELECT set_config").WillReturnResult(sqlmock.NewResult(0, 0))
	if commits {
		mock.ExpectCommit()
	} else {
		mock.ExpectRollback()
	}
	return db
}

func guardianEvent(studentIDs []int64) Event {
	return Event{
		Type:     TypeParentMessage,
		Title:    "Neue Nachricht der OGS",
		Body:     "Sie haben eine neue Nachricht im Elternportal.",
		DeepLink: "/messages",
		Priority: PriorityNormal,
		Audience: Audience{
			TenantID:           41,
			Scope:              ScopeGuardian,
			GuardianAccountIDs: []int64{77, 88},
			StudentIDs:         studentIDs,
		},
	}
}

// TestSSEGuardianFanOutRechecksChildAccess is the SSE half of the #1671
// delivery-time authorization contract: an in-app notification carries the same
// title, body and deep link a push does, so a guardian whose access to the child
// was revoked after the producer resolved its audience must not be woken either.
func TestSSEGuardianFanOutRechecksChildAccess(t *testing.T) {
	t.Parallel()

	broadcaster := &recordingGuardianBroadcaster{}
	access := &fakeGuardianAccess{permitted: []int64{88}}
	channel := NewSSEChannel(broadcaster, WithGuardianChildAccess(mockTenantTxDB(t, true), access, nil))

	require.NoError(t, channel.Deliver(context.Background(), guardianEvent([]int64{55})))

	assert.Equal(t, []int64{88}, broadcaster.guardians,
		"only the guardians who still hold parent_portal.access may be woken")
	assert.Equal(t, 1, access.calls)
	assert.Equal(t, []int64{77, 88}, access.accountIDs)
	assert.Equal(t, []int64{55}, access.studentIDs)
	assert.Equal(t, int64(41), access.tenantID)
	assert.Equal(t, authorize.GuardianPermissionPortalAccess, access.permission)
}

// TestSSEGuardianFanOutSkipsRecheckWithoutChildren keeps events that are about no
// child at all (broadcast announcements, gated by their own producer) on the
// unchanged path — no lookup, every resolved recipient woken.
func TestSSEGuardianFanOutSkipsRecheckWithoutChildren(t *testing.T) {
	t.Parallel()

	broadcaster := &recordingGuardianBroadcaster{}
	access := &fakeGuardianAccess{}
	db, _ := mockDB(t) // no statement expected: the unchanged path opens no transaction
	channel := NewSSEChannel(broadcaster, WithGuardianChildAccess(db, access, nil))

	require.NoError(t, channel.Deliver(context.Background(), guardianEvent(nil)))

	assert.Equal(t, []int64{77, 88}, broadcaster.guardians)
	assert.Zero(t, access.calls, "an audience about no child has nothing to re-check")
}

// TestSSEGuardianFanOutFailsClosedWithoutAccessLookup covers a channel built
// without the lookup: a student-scoped event is dropped rather than delivered
// unchecked.
func TestSSEGuardianFanOutFailsClosedWithoutAccessLookup(t *testing.T) {
	t.Parallel()

	broadcaster := &recordingGuardianBroadcaster{}
	channel := NewSSEChannel(broadcaster)

	require.NoError(t, channel.Deliver(context.Background(), guardianEvent([]int64{55})))

	assert.Empty(t, broadcaster.guardians)
}

// TestSSEGuardianFanOutReportsLookupFailure keeps a failed access read from
// silently delivering to everyone: the router logs the returned error and no
// guardian is woken.
func TestSSEGuardianFanOutReportsLookupFailure(t *testing.T) {
	t.Parallel()

	broadcaster := &recordingGuardianBroadcaster{}
	access := &fakeGuardianAccess{err: errors.New("access lookup failed")}
	channel := NewSSEChannel(broadcaster, WithGuardianChildAccess(mockTenantTxDB(t, false), access, nil))

	err := channel.Deliver(context.Background(), guardianEvent([]int64{55}))

	require.ErrorContains(t, err, "access lookup failed")
	assert.Empty(t, broadcaster.guardians)
}
