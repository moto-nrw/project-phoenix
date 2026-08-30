package migrations

import (
	"context"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParentMessageTeamHandledMigrationLeavesExistingHistoryOpen(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	require.NoError(t, parentMessageTeamHandledDown(ctx, db))
	t.Cleanup(func() { require.NoError(t, parentMessageTeamHandledUp(ctx, db)) })

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	_, staffAccount := testpkg.CreateTestStaffWithAccount(t, db, "Olivia", "Berg")

	var threadID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO users.parent_message_threads (tenant_id, student_id, guardian_account_id)
		VALUES (?, ?, ?)
		RETURNING id
	`, chain.TenantID, chain.StudentID, chain.AccountID).Scan(ctx, &threadID))

	base := time.Now().Add(-time.Minute).Truncate(time.Microsecond)
	insert := func(senderID int64, senderKind, body string, at time.Time) int64 {
		t.Helper()
		var id int64
		require.NoError(t, db.NewRaw(`
			INSERT INTO users.parent_messages
				(tenant_id, thread_id, student_id, sender_account_id, sender_kind,
				 sender_name, body, kind, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, 'Test', ?, 'message', ?, ?)
			RETURNING id
		`, chain.TenantID, threadID, chain.StudentID, senderID, senderKind, body, at, at).Scan(ctx, &id))
		return id
	}

	insert(chain.AccountID, "guardian", "Frage", base)
	insert(staffAccount.ID, "staff", "Antwort", base.Add(time.Second))

	require.NoError(t, parentMessageTeamHandledUp(ctx, db))

	var handledMessageID *int64
	require.NoError(t, db.NewRaw(`
		SELECT staff_handled_up_to_message_id
		FROM users.parent_message_threads
		WHERE id = ?
	`, threadID).Scan(ctx, &handledMessageID))
	assert.Nil(t, handledMessageID, "the migration must not guess which existing messages were visible before a reply")

	var count int
	require.NoError(t, db.NewRaw(`
		SELECT COUNT(*) FROM users.parent_messages
		WHERE tenant_id = ? AND thread_id = ? AND sender_kind = 'guardian' AND kind = 'message'
	`, chain.TenantID, threadID).Scan(ctx, &count))
	assert.Equal(t, 1, count, "existing guardian activity stays open until a new snapshot-bounded team reply")
}
