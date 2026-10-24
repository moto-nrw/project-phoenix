package staffinbox

import (
	"context"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestListInboxAnswersOnlyTheNamedSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	school := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, db)
	_, anna := testpkg.CreateTestStaffWithAccount(t, db, "Anna", "Inbox")
	_, ben := testpkg.CreateTestStaffWithAccount(t, db, "Ben", "Inbox")
	var threadID int64
	require.NoError(t, db.NewRaw(`INSERT INTO users.staff_message_threads
		(tenant_id, participant_key, last_message_at, last_message_body, last_sender_account_id)
		VALUES (?, ?, NOW(), 'Hallo', ?) RETURNING id`, school, fmt.Sprintf("%d:%d", anna.ID, ben.ID), ben.ID).Scan(ctx, &threadID))
	_, err := db.NewRaw(`INSERT INTO users.staff_message_participants (tenant_id, thread_id, account_id)
		VALUES (?, ?, ?), (?, ?, ?)`, school, threadID, anna.ID, school, threadID, ben.ID).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`INSERT INTO users.staff_messages (tenant_id, thread_id, sender_account_id, sender_name, body)
		VALUES (?, ?, ?, 'Ben Inbox', 'Hallo')`, school, threadID, ben.ID).Exec(ctx)
	require.NoError(t, err)
	forSchool := func(schoolID int64) *Projection {
		return New(func(context.Context) (bun.IDB, int64, error) { return db, schoolID, nil })
	}

	inbox, err := forSchool(school).ListInbox(ctx, anna.ID, false)
	require.NoError(t, err)
	require.Len(t, inbox, 1)
	assert.Equal(t, threadID, inbox[0].ThreadID)
	assert.Equal(t, school, inbox[0].TenantID)
	assert.Equal(t, ben.ID, inbox[0].CounterpartAccountID)
	assert.Equal(t, "Hallo", inbox[0].LastMessageBody)
	assert.Equal(t, 1, inbox[0].UnreadCount)

	foreign, err := forSchool(otherSchool).ListInbox(ctx, anna.ID, false)
	require.NoError(t, err)
	assert.Empty(t, foreign, "another school must not list the conversation")
	unread, err := forSchool(otherSchool).UnreadCount(ctx, anna.ID)
	require.NoError(t, err)
	assert.Zero(t, unread, "another school must not count the message")
}
