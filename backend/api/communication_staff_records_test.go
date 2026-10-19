package api

import (
	"context"
	"testing"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	staffStore "github.com/moto-nrw/project-phoenix/modules/communication/staffstore"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestNativeStaffRecordsUseTheSameParticipantScopedStoresAndClock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	_, first := testpkg.CreateTestStaffWithAccount(t, db, "Anna", "Berg")
	_, second := testpkg.CreateTestStaffWithAccount(t, db, "Ben", "Tal")
	_, outsider := testpkg.CreateTestStaffWithAccount(t, db, "Clara", "See")
	database := testpkg.NewDatabase(t, db)
	repos, err := NewStaffMessagingTestRepositories(database, owners.MustNewMembershipTestRepositories(database))
	require.NoError(t, err)
	ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)
	require.NoError(t, tenant.WithTenantTx(ctx, db, testpkg.Tenant(t), func(tx context.Context, _ bun.Tx) error {
		original, err := repos.Thread.GetOrCreateStaffThread(tx, first.ID, second.ID)
		require.NoError(t, err)
		native, err := repos.Thread.GetOrCreateStaffThread(tx, second.ID, first.ID)
		require.NoError(t, err)
		require.NotNil(t, native)
		assert.Equal(t, communication.StaffThreadRecord{ID: original.ID, TenantID: original.TenantID, ParticipantKey: original.ParticipantKey, Kind: original.Kind, LastMessageAt: original.LastMessageAt, LastMessageID: original.LastMessageID, LastMessageBody: original.LastMessageBody, LastSenderAccountID: original.LastSenderAccountID, CreatedAt: original.CreatedAt, UpdatedAt: original.UpdatedAt}, *native)
		participant, err := repos.Thread.HasStaffParticipant(tx, native.ID, outsider.ID)
		require.NoError(t, err)
		assert.False(t, participant, "an unrelated colleague must not gain conversation access")
		row := &communication.StaffMessageRecord{ThreadID: native.ID, SenderAccountID: first.ID, SenderName: "Anna Berg", Body: "native message"}
		require.NoError(t, repos.Thread.LockStaffThreadForAppend(tx, native.ID))
		require.NoError(t, repos.Message.CreateStaffMessage(tx, row))
		assert.Positive(t, row.ID)
		assert.Equal(t, testpkg.Tenant(t), row.TenantID)
		assert.False(t, row.CreatedAt.IsZero())
		require.NoError(t, repos.Thread.AdvanceStaffThreadPreview(tx, native.ID, row.CreatedAt, row.ID, first.ID, row.Body))
		nativeRows, err := repos.Message.ListStaffMessages(tx, native.ID)
		require.NoError(t, err)
		require.Len(t, nativeRows, 1)
		assert.Equal(t, *row, *nativeRows[0], "create must return the same DB-stamped clock and identity as a read")
		newInbox, err := repos.Read.ListStaffInbox(tx, second.ID, false)
		require.NoError(t, err)
		require.Len(t, newInbox, 1)
		assert.Equal(t, communication.StaffInboxRecord{ThreadID: native.ID, TenantID: testpkg.Tenant(t), CounterpartAccountID: first.ID, CounterpartName: "Anna Berg", LastMessageAt: &row.CreatedAt, LastMessageBody: row.Body, LastSenderAccountID: &first.ID, UnreadCount: 1}, *newInbox[0])
		require.NoError(t, repos.Read.MarkStaffReadUpTo(tx, native.ID, second.ID, row.CreatedAt, row.ID))
		unread, err := repos.Read.StaffUnreadCount(tx, second.ID)
		require.NoError(t, err)
		assert.Zero(t, unread)
		return nil
	}))
}

func TestNativeStaffCreateRetainsMetadataOnTheOriginalMissingDatabaseFailure(t *testing.T) {
	t.Parallel()
	source := staffStore.NewStaffMessageRepository(transactionruntime.Database{}.Pool())
	at := time.Date(2026, time.October, 17, 12, 30, 0, 0, time.UTC)
	original := communication.StaffMessageRecord{ID: 13, TenantID: 17, ThreadID: 19, SenderAccountID: 23, SenderName: "sender", Body: "body", CreatedAt: at, UpdatedAt: at.Add(time.Second)}
	row := original
	err := source.CreateStaffMessage(context.Background(), &row)
	require.Error(t, err)
	assert.EqualError(t, err, "communication staff store: database is required")
	assert.Equal(t, original, row)
}

// An insert failure happens after the SQL store resolves the tenant. The public
// store must retain the original success-only metadata copyback contract.
func TestNativeStaffFailedInsertDoesNotExposeTentativeTenantMetadata(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	_, first := testpkg.CreateTestStaffWithAccount(t, db, "Anna", "Berg")
	_, second := testpkg.CreateTestStaffWithAccount(t, db, "Ben", "Tal")
	threads := staffStore.NewStaffMessageThreadRepository(db)
	messages := staffStore.NewStaffMessageRepository(db)
	ctx := testpkg.WithTenantRuntime(t, testpkg.Ctx(t), db)
	var row communication.StaffMessageRecord
	var original communication.StaffMessageRecord
	err := tenant.WithTenantTx(ctx, db, testpkg.Tenant(t), func(tx context.Context, _ bun.Tx) error {
		thread, err := threads.GetOrCreateStaffThread(tx, first.ID, second.ID)
		require.NoError(t, err)
		deleted, err := threads.DeleteEmptyStaffThreads(tx, time.Now().Add(time.Hour))
		require.NoError(t, err)
		require.EqualValues(t, 1, deleted)
		row = communication.StaffMessageRecord{ThreadID: thread.ID, SenderAccountID: first.ID, SenderName: "Anna Berg", Body: "failed insert"}
		original = row
		return messages.CreateStaffMessage(tx, &row)
	})
	require.ErrorContains(t, err, "create staff message:")
	assert.Equal(t, original, row)
}
