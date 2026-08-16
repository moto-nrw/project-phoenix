package migrations

import (
	"context"
	"testing"
	"time"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParentMessageTeamHandledMigrationLeavesExistingHistoryOpen(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	require.NoError(t, parentMessageTeamHandledDown(ctx, db))
	t.Cleanup(func() { require.NoError(t, parentMessageTeamHandledUp(ctx, db)) })

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	t.Cleanup(func() { testpkg.CleanupParentGuardianChain(t, db, chain) })
	staff, staffAccount := testpkg.CreateTestStaffWithAccount(t, db, "Olivia", "Berg")
	t.Cleanup(func() { testpkg.CleanupStaffFixtures(t, db, staff.ID) })
	t.Cleanup(func() { testpkg.CleanupAuthFixtures(t, db, staffAccount.ID) })
	t.Cleanup(func() { testpkg.CleanupParentMessagingForAccount(t, db, staffAccount.ID) })

	tenantCtx := tenant.WithTenantID(ctx, chain.TenantID)
	var threadID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO users.parent_message_threads (tenant_id, student_id, guardian_account_id)
		VALUES (?, ?, ?)
		RETURNING id
	`, chain.TenantID, chain.StudentID, chain.AccountID).Scan(ctx, &threadID))

	base := time.Now().Add(-time.Minute).Truncate(time.Microsecond)
	insert := func(senderID int64, senderKind, body string, at time.Time) *usersModels.ParentMessage {
		t.Helper()
		message := &usersModels.ParentMessage{
			ThreadID:        threadID,
			StudentID:       chain.StudentID,
			SenderAccountID: senderID,
			SenderKind:      senderKind,
			SenderName:      "Test",
			Body:            body,
			Kind:            usersModels.ParentMessageKindMessage,
		}
		message.SetTenantID(chain.TenantID)
		message.CreatedAt, message.UpdatedAt = at, at
		require.NoError(t, usersRepo.NewParentMessageRepository(db).Create(tenantCtx, message))
		return message
	}

	insert(chain.AccountID, usersModels.ParentMessageSenderGuardian, "Frage", base)
	insert(staffAccount.ID, usersModels.ParentMessageSenderStaff, "Antwort", base.Add(time.Second))

	require.NoError(t, parentMessageTeamHandledUp(ctx, db))

	var handledMessageID *int64
	require.NoError(t, db.NewRaw(`
		SELECT staff_handled_up_to_message_id
		FROM users.parent_message_threads
		WHERE id = ?
	`, threadID).Scan(ctx, &handledMessageID))
	assert.Nil(t, handledMessageID, "the migration must not guess which existing messages were visible before a reply")

	count, err := usersRepo.NewParentMessageReadRepository(db).
		UnreadMessageCountForStaff(tenantCtx, staffAccount.ID, true)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "existing guardian activity stays open until a new snapshot-bounded team reply")
}
