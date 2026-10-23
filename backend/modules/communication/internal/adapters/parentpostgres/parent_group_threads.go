package parentpostgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/uptrace/bun"
)

// GroupThreadStore owns the group threads (#4784): the thread, its OGS
// messages, the recipients frozen per message and the guardians' read
// cursors. A guardian reads only the messages a recipient row addresses to
// them; no query here returns another family's account, name or count to a
// guardian.
type GroupThreadStore struct{ store }

// NewGroupThreadStore builds the group thread store over the ambient
// transaction runtime.
func NewGroupThreadStore(database Database) *GroupThreadStore {
	return &GroupThreadStore{store: newStore(database)}
}

type groupThreadRow struct {
	bun.BaseModel      `bun:"table:users.parent_group_threads,alias:gt"`
	ID                 int64      `bun:"id,pk,autoincrement"`
	TenantID           int64      `bun:"tenant_id,notnull"`
	TargetType         string     `bun:"target_type,notnull"`
	TargetRefID        *int64     `bun:"target_ref_id"`
	TargetRefText      *string    `bun:"target_ref_text"`
	TargetLabel        string     `bun:"target_label,notnull"`
	Title              string     `bun:"title,notnull"`
	CreatedByAccountID *int64     `bun:"created_by_account_id"`
	ClosedAt           *time.Time `bun:"closed_at"`
	ClosedByAccountID  *int64     `bun:"closed_by_account_id"`
	LastMessageAt      *time.Time `bun:"last_message_at"`
	CreatedAt          time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt          time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

func (r *groupThreadRow) value() *communication.ParentGroupThread {
	if r == nil {
		return nil
	}
	return &communication.ParentGroupThread{
		ID: r.ID, TenantID: r.TenantID, Title: r.Title,
		Target: communication.ParentGroupTarget{
			Type: r.TargetType, RefID: r.TargetRefID, RefText: r.TargetRefText, Label: r.TargetLabel,
		},
		ClosedAt: r.ClosedAt, LastMessageAt: r.LastMessageAt, CreatedAt: r.CreatedAt,
	}
}

type groupMessageRow struct {
	bun.BaseModel   `bun:"table:users.parent_group_messages,alias:gm"`
	ID              int64     `bun:"id,pk,autoincrement"`
	TenantID        int64     `bun:"tenant_id,notnull"`
	ThreadID        int64     `bun:"thread_id,notnull"`
	SenderAccountID *int64    `bun:"sender_account_id"`
	SenderName      string    `bun:"sender_name,notnull"`
	Body            string    `bun:"body,notnull"`
	CreatedAt       time.Time `bun:"created_at,nullzero,notnull,default:clock_timestamp()"`
}

func (r *groupMessageRow) value() communication.ParentGroupMessage {
	return communication.ParentGroupMessage{ID: r.ID, SenderName: r.SenderName, Body: r.Body, CreatedAt: r.CreatedAt}
}

type groupRecipientRow struct {
	bun.BaseModel     `bun:"table:users.parent_group_message_recipients"`
	TenantID          int64 `bun:"tenant_id,notnull"`
	ThreadID          int64 `bun:"thread_id,notnull"`
	MessageID         int64 `bun:"message_id,notnull"`
	StudentID         int64 `bun:"student_id,notnull"`
	GuardianAccountID int64 `bun:"guardian_account_id,notnull"`
}

func (s *GroupThreadStore) tenantDB(ctx context.Context, operation string) (bun.IDB, int64, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, 0, err
	}
	if scope.AcrossSchools || scope.TenantID <= 0 {
		return nil, 0, fmt.Errorf("%s: school transaction is required", operation)
	}
	return db, scope.TenantID, nil
}

// CreateThread inserts a new open thread in the caller's school.
func (s *GroupThreadStore) CreateThread(ctx context.Context, target communication.ParentGroupTarget, title string, createdBy *int64) (*communication.ParentGroupThread, error) {
	db, tenantID, err := s.tenantDB(ctx, "create parent group thread")
	if err != nil {
		return nil, err
	}
	row := &groupThreadRow{
		TenantID: tenantID, TargetType: target.Type, TargetRefID: target.RefID,
		TargetRefText: target.RefText, TargetLabel: target.Label, Title: title,
		CreatedByAccountID: createdBy,
	}
	if _, err := db.NewInsert().Model(row).Returning("id, created_at, updated_at").Exec(ctx); err != nil {
		return nil, fmt.Errorf("create parent group thread: %w", err)
	}
	return row.value(), nil
}

// FindThread returns one thread of the caller's school, nil when absent.
// lock takes the row lock that serializes the messages and the closing of a
// thread, so message ids grow in commit order within it.
func (s *GroupThreadStore) FindThread(ctx context.Context, id int64, lock bool) (*communication.ParentGroupThread, error) {
	db, tenantID, err := s.tenantDB(ctx, "find parent group thread")
	if err != nil {
		return nil, err
	}
	row := new(groupThreadRow)
	query := db.NewSelect().Model(row).Where("gt.id = ? AND gt.tenant_id = ?", id, tenantID)
	if lock {
		query = query.For("UPDATE")
	}
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find parent group thread: %w", err)
	}
	return row.value(), nil
}

// AppendMessage stores one OGS message with the recipients it reached and
// advances the thread's activity. The caller holds the thread lock.
func (s *GroupThreadStore) AppendMessage(ctx context.Context, threadID int64, sender *int64, senderName, body string, recipients []communication.ParentGroupRecipient) (*communication.ParentGroupMessage, error) {
	db, tenantID, err := s.tenantDB(ctx, "append parent group message")
	if err != nil {
		return nil, err
	}
	row := &groupMessageRow{TenantID: tenantID, ThreadID: threadID, SenderAccountID: sender, SenderName: senderName, Body: body}
	if _, err := db.NewInsert().Model(row).Returning("id, created_at").Exec(ctx); err != nil {
		return nil, fmt.Errorf("append parent group message: %w", err)
	}
	if len(recipients) > 0 {
		rows := make([]groupRecipientRow, 0, len(recipients))
		for _, recipient := range recipients {
			rows = append(rows, groupRecipientRow{
				TenantID: tenantID, ThreadID: threadID, MessageID: row.ID,
				StudentID: recipient.StudentID, GuardianAccountID: recipient.GuardianAccountID,
			})
		}
		if _, err := db.NewInsert().Model(&rows).On("CONFLICT DO NOTHING").Exec(ctx); err != nil {
			return nil, fmt.Errorf("store parent group message recipients: %w", err)
		}
	}
	result, err := db.NewUpdate().TableExpr("users.parent_group_threads").
		Set("last_message_at = ?", row.CreatedAt).
		Where("id = ? AND tenant_id = ?", threadID, tenantID).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("touch parent group thread: %w", err)
	}
	if err := assertOneRow(result, "touch parent group thread"); err != nil {
		return nil, err
	}
	message := row.value()
	return &message, nil
}

// CloseThread closes an open thread. It reports false when the thread was
// already closed.
func (s *GroupThreadStore) CloseThread(ctx context.Context, id, accountID int64) (bool, error) {
	db, tenantID, err := s.tenantDB(ctx, "close parent group thread")
	if err != nil {
		return false, err
	}
	result, err := db.NewUpdate().TableExpr("users.parent_group_threads").
		Set("closed_at = clock_timestamp()").
		Set("closed_by_account_id = ?", accountID).
		Where("id = ? AND tenant_id = ? AND closed_at IS NULL", id, tenantID).
		Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("close parent group thread: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("close parent group thread: %w", err)
	}
	return affected == 1, nil
}

// RecipientAccountIDs returns every account that has received at least one
// message in the thread. These are the only parent tabs that need an update
// when the thread closes.
func (s *GroupThreadStore) RecipientAccountIDs(ctx context.Context, threadID int64) ([]int64, error) {
	db, tenantID, err := s.tenantDB(ctx, "list parent group thread recipients")
	if err != nil {
		return nil, err
	}
	var accountIDs []int64
	if err := db.NewSelect().
		TableExpr("users.parent_group_message_recipients AS r").
		ColumnExpr("DISTINCT r.guardian_account_id").
		Where("r.thread_id = ? AND r.tenant_id = ?", threadID, tenantID).
		OrderExpr("r.guardian_account_id ASC").
		Scan(ctx, &accountIDs); err != nil {
		return nil, fmt.Errorf("list parent group thread recipients: %w", err)
	}
	return accountIDs, nil
}

// ListMessages returns every message of a thread, oldest first. Staff only.
func (s *GroupThreadStore) ListMessages(ctx context.Context, threadID int64) ([]communication.ParentGroupMessage, error) {
	db, tenantID, err := s.tenantDB(ctx, "list parent group messages")
	if err != nil {
		return nil, err
	}
	var rows []groupMessageRow
	if err := db.NewSelect().Model(&rows).
		Where("gm.thread_id = ? AND gm.tenant_id = ?", threadID, tenantID).
		OrderExpr("gm.id ASC").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list parent group messages: %w", err)
	}
	out := make([]communication.ParentGroupMessage, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].value())
	}
	return out, nil
}

// staffThreadListSQL lists the school's threads, newest activity first, with
// the newest message and the family read count. A family is one reached
// guardian account; it has read the thread after it read the newest message
// that reached the account. The detail page keeps its separate per-child
// status below.
const staffThreadListSQL = `
SELECT gt.*,
	COALESCE(latest.body, '') AS last_message_body,
	COALESCE(stats.message_count, 0) AS message_count,
	COALESCE(families.family_count, 0) AS family_count,
	COALESCE(families.read_count, 0) AS read_count
FROM users.parent_group_threads AS gt
LEFT JOIN LATERAL (
	SELECT gm.body FROM users.parent_group_messages AS gm
	WHERE gm.thread_id = gt.id AND gm.tenant_id = gt.tenant_id
	ORDER BY gm.id DESC LIMIT 1
) AS latest ON TRUE
LEFT JOIN LATERAL (
	SELECT COUNT(*) AS message_count FROM users.parent_group_messages AS gm
	WHERE gm.thread_id = gt.id AND gm.tenant_id = gt.tenant_id
) AS stats ON TRUE
LEFT JOIN LATERAL (
	SELECT COUNT(*) AS family_count, COUNT(*) FILTER (WHERE family.read) AS read_count
	FROM (` + groupFamilyReadSQL + `) AS family
) AS families ON TRUE
WHERE gt.tenant_id = ?
ORDER BY gt.last_message_at DESC NULLS LAST, gt.id DESC`

// groupChildReadSQL is one row per child a thread reached with whether a
// guardian read the newest message that reached the child. It correlates
// with gt.
const groupChildReadSQL = `
	SELECT per_child.student_id, EXISTS (
		SELECT 1 FROM users.parent_group_message_recipients AS r
		JOIN users.parent_group_thread_reads AS rd
			ON rd.thread_id = r.thread_id AND rd.account_id = r.guardian_account_id
		WHERE r.message_id = per_child.latest_message_id
			AND r.student_id = per_child.student_id
			AND rd.last_read_message_id >= per_child.latest_message_id
	) AS read
	FROM (
		SELECT r.student_id, MAX(r.message_id) AS latest_message_id
		FROM users.parent_group_message_recipients AS r
		WHERE r.thread_id = gt.id AND r.tenant_id = gt.tenant_id
		GROUP BY r.student_id
	) AS per_child`

// groupFamilyReadSQL is one row per guardian account a thread reached with
// whether it read the newest message that reached the account. It correlates
// with gt and is only used for the staff list's family totals.
const groupFamilyReadSQL = `
	SELECT per_family.guardian_account_id,
		COALESCE(rd.last_read_message_id >= per_family.latest_message_id, FALSE) AS read
	FROM (
		SELECT r.guardian_account_id, MAX(r.message_id) AS latest_message_id
		FROM users.parent_group_message_recipients AS r
		WHERE r.thread_id = gt.id AND r.tenant_id = gt.tenant_id
		GROUP BY r.guardian_account_id
	) AS per_family
	LEFT JOIN users.parent_group_thread_reads AS rd
		ON rd.thread_id = gt.id AND rd.account_id = per_family.guardian_account_id`

type staffThreadListRow struct {
	groupThreadRow
	LastMessageBody string `bun:"last_message_body"`
	MessageCount    int    `bun:"message_count"`
	FamilyCount     int    `bun:"family_count"`
	ReadCount       int    `bun:"read_count"`
}

// ListThreads returns the school's threads for the staff list.
func (s *GroupThreadStore) ListThreads(ctx context.Context) ([]communication.ParentGroupThreadSummary, error) {
	db, tenantID, err := s.tenantDB(ctx, "list parent group threads")
	if err != nil {
		return nil, err
	}
	var rows []staffThreadListRow
	if err := db.NewRaw(staffThreadListSQL, tenantID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("list parent group threads: %w", err)
	}
	out := make([]communication.ParentGroupThreadSummary, 0, len(rows))
	for i := range rows {
		out = append(out, communication.ParentGroupThreadSummary{
			Thread: *rows[i].value(), LastMessageBody: rows[i].LastMessageBody,
			MessageCount: rows[i].MessageCount, FamilyCount: rows[i].FamilyCount, ReadCount: rows[i].ReadCount,
		})
	}
	return out, nil
}

// TitlesByIDs returns the titles of the given threads of the caller's school,
// for the reference an answer in a 1:1 conversation carries.
func (s *GroupThreadStore) TitlesByIDs(ctx context.Context, ids []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	db, tenantID, err := s.tenantDB(ctx, "load parent group thread titles")
	if err != nil {
		return nil, err
	}
	var rows []groupThreadRow
	if err := db.NewSelect().Model(&rows).Column("gt.id", "gt.title").
		Where("gt.tenant_id = ? AND gt.id IN (?)", tenantID, bun.List(ids)).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("load parent group thread titles: %w", err)
	}
	for _, row := range rows {
		out[row.ID] = row.Title
	}
	return out, nil
}

// readStatusSQL lists every child the thread reached with its read state.
// Bind order: thread, school.
const readStatusSQL = `
SELECT child.student_id, child.read
FROM users.parent_group_threads AS gt, LATERAL (` + groupChildReadSQL + `) AS child
WHERE gt.id = ? AND gt.tenant_id = ?
ORDER BY child.student_id`

type childReadRow struct {
	StudentID int64 `bun:"student_id"`
	Read      bool  `bun:"read"`
}

// ReadStatus returns every child the thread reached with whether one of its
// guardians read the newest message that reached it.
func (s *GroupThreadStore) ReadStatus(ctx context.Context, threadID int64) ([]communication.ParentGroupChildRead, error) {
	db, tenantID, err := s.tenantDB(ctx, "load parent group thread read status")
	if err != nil {
		return nil, err
	}
	var rows []childReadRow
	if err := db.NewRaw(readStatusSQL, threadID, tenantID).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("load parent group thread read status: %w", err)
	}
	out := make([]communication.ParentGroupChildRead, 0, len(rows))
	for _, row := range rows {
		out = append(out, communication.ParentGroupChildRead{StudentID: row.StudentID, Read: row.Read})
	}
	return out, nil
}
