package migrations

// Data-repair coverage for step 4 of the #1803 cutover
// (migrateOpenCareRequestsUp): after an open chat care_schedule request is
// converted into a 'request_created' pill, a thread that had that request as its
// denormalized last_message_id must NOT keep pointing at the now-hidden pill —
// request_created pills are excluded from the Nachrichten preview/ordering, so a
// dangling pointer shows a stale "latest message". The migration re-derives the
// preview from the newest previewable message, or clears it when none remains.

import (
	"context"
	"database/sql"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// insertCareRequestThread creates a parent_message_threads row for the chain and
// returns its id.
func insertCareRequestThread(t *testing.T, db *bun.DB, tenantID, studentID, accountID int64) int64 {
	t.Helper()
	var id int64
	err := db.QueryRowContext(context.Background(), `
		INSERT INTO users.parent_message_threads
			(tenant_id, student_id, guardian_account_id, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
		RETURNING id
	`, tenantID, studentID, accountID).Scan(&id)
	require.NoError(t, err)
	return id
}

// insertCareChatMessage inserts a plain guardian chat message and returns its id.
func insertCareChatMessage(t *testing.T, db *bun.DB, tenantID, threadID, studentID, accountID int64, body string, createdAt time.Time) int64 {
	t.Helper()
	var id int64
	err := db.QueryRowContext(context.Background(), `
		INSERT INTO users.parent_messages
			(tenant_id, thread_id, student_id, sender_account_id, sender_kind, sender_name, body, kind, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'guardian', 'Sabine Schneider', ?, 'message', ?, ?)
		RETURNING id
	`, tenantID, threadID, studentID, accountID, body, createdAt, createdAt).Scan(&id)
	require.NoError(t, err)
	return id
}

// insertOpenCareRequestMessage inserts a legacy open care_schedule chat request
// (kind='request') and returns its id.
func insertOpenCareRequestMessage(t *testing.T, db *bun.DB, tenantID, threadID, studentID, accountID int64, createdAt time.Time) int64 {
	t.Helper()
	var id int64
	err := db.QueryRowContext(context.Background(), `
		INSERT INTO users.parent_messages
			(tenant_id, thread_id, student_id, sender_account_id, sender_kind, sender_name, body,
			 kind, request_type, request_status, payload, created_at, updated_at)
		VALUES (?, ?, ?, ?, 'guardian', 'Sabine Schneider', 'Änderungsanfrage',
			 'request', 'care_schedule', 'offen', ?::jsonb, ?, ?)
		RETURNING id
	`, tenantID, threadID, studentID, accountID,
		`{"weekdays":[{"weekday":1,"arrival":"08:00","pickup":"","mode":"alone"}]}`, createdAt, createdAt).Scan(&id)
	require.NoError(t, err)
	return id
}

// pointThreadPreviewAt sets the denormalized preview columns to a given message,
// mirroring the pre-cutover state where the open chat request had touched the
// thread as its latest activity.
func pointThreadPreviewAt(t *testing.T, db *bun.DB, threadID, messageID int64, at time.Time, body string) {
	t.Helper()
	_, err := db.ExecContext(context.Background(), `
		UPDATE users.parent_message_threads
		SET last_message_id = ?, last_message_at = ?, last_sender_kind = 'guardian', last_message_body = ?
		WHERE id = ?
	`, messageID, at, body, threadID)
	require.NoError(t, err)
}

type threadPreview struct {
	LastMessageID   sql.NullInt64  `bun:"last_message_id"`
	LastMessageAt   sql.NullTime   `bun:"last_message_at"`
	LastSenderKind  sql.NullString `bun:"last_sender_kind"`
	LastMessageBody string         `bun:"last_message_body"`
}

func loadThreadPreview(t *testing.T, db *bun.DB, threadID int64) threadPreview {
	t.Helper()
	var p threadPreview
	err := db.NewSelect().
		ColumnExpr("last_message_id").
		ColumnExpr("last_message_at").
		ColumnExpr("last_sender_kind").
		ColumnExpr("COALESCE(last_message_body,'') AS last_message_body").
		TableExpr("users.parent_message_threads").
		Where("id = ?", threadID).
		Scan(context.Background(), &p)
	require.NoError(t, err)
	return p
}

func assertMessageIsRequestCreatedPill(t *testing.T, db *bun.DB, messageID int64) {
	t.Helper()
	var kind, eventType string
	err := db.QueryRowContext(context.Background(),
		`SELECT kind, COALESCE(event_type,'') FROM users.parent_messages WHERE id = ?`, messageID).
		Scan(&kind, &eventType)
	require.NoError(t, err)
	assert.Equal(t, "event", kind, "the moved request row is now an event pill")
	assert.Equal(t, "request_created", eventType)
}

// TestMigrateOpenCareRequests_RecomputesStalePreview: a thread whose latest
// activity was the open request must fall back to the newest previewable
// message (the earlier chat message) once the request becomes a hidden pill.
func TestMigrateOpenCareRequests_RecomputesStalePreview(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	threadID := insertCareRequestThread(t, db, chain.TenantID, chain.StudentID, chain.AccountID)
	chatAt := time.Now().Add(-1 * time.Minute)
	chatID := insertCareChatMessage(t, db, chain.TenantID, threadID, chain.StudentID, chain.AccountID, "Guten Morgen", chatAt)
	requestAt := time.Now()
	requestID := insertOpenCareRequestMessage(t, db, chain.TenantID, threadID, chain.StudentID, chain.AccountID, requestAt)

	// Pre-cutover: the open request is the thread's denormalized latest message.
	pointThreadPreviewAt(t, db, threadID, requestID, requestAt, "Änderungsanfrage")

	require.NoError(t, migrateOpenCareRequestsUp(ctx, db))

	// The request row became a request_created pill and a pending domain row exists.
	assertMessageIsRequestCreatedPill(t, db, requestID)
	var pendingCount int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM schedule.care_schedule_change_requests WHERE student_id = ? AND status = 'pending'`,
		chain.StudentID).Scan(&pendingCount))
	assert.Equal(t, 1, pendingCount, "the open request moved into the schedule domain")

	// The preview no longer points at the hidden pill; it fell back to the chat.
	preview := loadThreadPreview(t, db, threadID)
	require.True(t, preview.LastMessageID.Valid)
	assert.Equal(t, chatID, preview.LastMessageID.Int64, "preview re-derives to the newest previewable message")
	assert.Equal(t, "Guten Morgen", preview.LastMessageBody)
	assert.Equal(t, "guardian", preview.LastSenderKind.String)
}

// TestMigrateOpenCareRequests_ClearsPreviewWhenOnlyMessage: a thread whose ONLY
// message was the open request has no previewable message left after the
// cutover, so the preview is cleared to the fresh-thread empty state.
func TestMigrateOpenCareRequests_ClearsPreviewWhenOnlyMessage(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	chain := testpkg.CreateTestParentGuardianChain(t, db)
	defer testpkg.CleanupParentGuardianChain(t, db, chain)

	threadID := insertCareRequestThread(t, db, chain.TenantID, chain.StudentID, chain.AccountID)
	requestAt := time.Now()
	requestID := insertOpenCareRequestMessage(t, db, chain.TenantID, threadID, chain.StudentID, chain.AccountID, requestAt)
	pointThreadPreviewAt(t, db, threadID, requestID, requestAt, "Änderungsanfrage")

	require.NoError(t, migrateOpenCareRequestsUp(ctx, db))

	assertMessageIsRequestCreatedPill(t, db, requestID)

	preview := loadThreadPreview(t, db, threadID)
	assert.False(t, preview.LastMessageID.Valid, "no previewable message remains → pointer cleared")
	assert.False(t, preview.LastMessageAt.Valid, "no previewable message remains → timestamp cleared")
	assert.Equal(t, "", preview.LastMessageBody, "preview body reset to the empty fresh-thread state")
}
