package staffpostgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/uptrace/bun"
)

const (
	staffThreadsTable                    = "users.staff_message_threads"
	tableExprStaffMessageThreadsAsThread = staffThreadsTable + ` AS "staff_message_thread"`
	staffThreadAlias                     = "staff_message_thread"
)

// ThreadStore owns staff conversations, their participant rows and the
// denormalized last-activity preview.
type ThreadStore struct{ store }

// NewThreadStore builds the thread store over the ambient transaction runtime.
func NewThreadStore(database Database) *ThreadStore {
	return &ThreadStore{store: newStore(database)}
}

type staffThreadRow struct {
	bun.BaseModel       `bun:"table:users.staff_message_threads,alias:staff_message_thread"`
	ID                  int64      `bun:"id,pk,autoincrement"`
	CreatedAt           time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt           time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID            int64      `bun:"tenant_id,notnull"`
	ParticipantKey      string     `bun:"participant_key,notnull"`
	Kind                string     `bun:"kind,notnull"`
	LastMessageAt       *time.Time `bun:"last_message_at"`
	LastMessageID       *int64     `bun:"last_message_id"`
	LastMessageBody     string     `bun:"last_message_body"`
	LastSenderAccountID *int64     `bun:"last_sender_account_id"`
}

func (r *staffThreadRow) value() *domain.StaffMessageThread {
	return &domain.StaffMessageThread{
		ID: r.ID, TenantID: r.TenantID, ParticipantKey: r.ParticipantKey, Kind: r.Kind,
		LastMessageAt: r.LastMessageAt, LastMessageID: r.LastMessageID,
		LastMessageBody: r.LastMessageBody, LastSenderAccountID: r.LastSenderAccountID,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

type staffParticipantRow struct {
	bun.BaseModel `bun:"table:users.staff_message_participants,alias:smp"`
	TenantID      int64     `bun:"tenant_id,notnull"`
	ThreadID      int64     `bun:"thread_id,pk"`
	AccountID     int64     `bun:"account_id,pk"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
}

// FindByID returns the thread within the current tenant, or nil when absent.
func (s *ThreadStore) FindByID(ctx context.Context, id int64) (*domain.StaffMessageThread, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	row := new(staffThreadRow)
	query := db.NewSelect().
		Model(row).
		ModelTableExpr(tableExprStaffMessageThreadsAsThread).
		Where(`"staff_message_thread".id = ?`, id)
	query = withTenant(query, staffThreadAlias, tenantID)

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find staff message thread: %w", err)
	}
	return row.value(), nil
}

// findByParticipantKey loads the canonical thread row for a participant key in
// the current tenant, or nil when none exists yet.
func (s *ThreadStore) findByParticipantKey(ctx context.Context, db bun.IDB, tenantID int64, key string) (*staffThreadRow, error) {
	row := new(staffThreadRow)
	query := db.NewSelect().
		Model(row).
		ModelTableExpr(tableExprStaffMessageThreadsAsThread).
		Where(`"staff_message_thread".participant_key = ?`, key).
		Limit(1)
	query = withTenant(query, staffThreadAlias, tenantID)

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find staff message thread by participant key: %w", err)
	}
	return row, nil
}

// GetOrCreate returns the conversation identified by key in the current
// tenant, atomically creating it with the given kind and participants when
// absent.
//
// The INSERT ... ON CONFLICT DO NOTHING makes two concurrent first messages
// race-safe: the loser does NOT raise a unique violation (which would abort the
// surrounding transaction and surface as a 500 on a successful send), it simply
// inserts nothing and then loads the row the winner created. The conflict target
// matches uq_staff_message_threads_participants.
//
// Participants are inserted with the same conflict-tolerant shape, so a thread
// that already exists is not disturbed and a partially created one (thread row
// committed, participants not) heals on the next open.
func (s *ThreadStore) GetOrCreate(ctx context.Context, key, kind string, accountIDs ...int64) (*domain.StaffMessageThread, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	thread := &staffThreadRow{ParticipantKey: key, Kind: kind, TenantID: tenantID}

	// Qualify the table explicitly. Relying on the struct tag leaves the INSERT
	// unqualified, which the least-privilege phoenix_tenant role cannot resolve
	// (its search_path excludes the users schema) ->
	// "relation staff_message_threads does not exist".
	if _, err := db.NewInsert().
		Model(thread).
		ModelTableExpr(staffThreadsTable).
		On("CONFLICT (tenant_id, participant_key) DO NOTHING").
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("get-or-create staff message thread: %w", err)
	}

	existing, err := s.findByParticipantKey(ctx, db, tenantID, key)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("get-or-create staff message thread: thread missing after upsert")
	}

	if err := ensureParticipants(ctx, db, existing, accountIDs...); err != nil {
		return nil, err
	}
	return existing.value(), nil
}

// ensureParticipants inserts the membership rows for a thread, tolerating rows
// that already exist.
func ensureParticipants(ctx context.Context, db bun.IDB, thread *staffThreadRow, accountIDs ...int64) error {
	rows := make([]*staffParticipantRow, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		rows = append(rows, &staffParticipantRow{
			TenantID:  thread.TenantID,
			ThreadID:  thread.ID,
			AccountID: accountID,
		})
	}

	if _, err := db.NewInsert().
		Model(&rows).
		ModelTableExpr("users.staff_message_participants").
		On("CONFLICT (thread_id, account_id) DO NOTHING").
		Exec(ctx); err != nil {
		return fmt.Errorf("ensure staff message participants: %w", err)
	}
	return nil
}

// LockForMessageAppend serializes inserts into a thread within the caller's
// transaction, keeping message tuple order aligned with commit order.
func (s *ThreadStore) LockForMessageAppend(ctx context.Context, threadID int64) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	var id int64
	query := db.NewSelect().
		TableExpr(tableExprStaffMessageThreadsAsThread).
		ColumnExpr(`"staff_message_thread".id`).
		Where(`"staff_message_thread".id = ?`, threadID).
		For("UPDATE")
	query = withTenant(query, staffThreadAlias, tenantID)

	if err := query.Scan(ctx, &id); err != nil {
		return fmt.Errorf("lock staff message thread for append: %w", err)
	}
	return nil
}

// TouchLastMessage atomically advances the thread's denormalized last-activity
// fields — the columns the inbox projection sorts and previews by — but ONLY
// when the (at, messageID) composite is newer than the stored pair. This is the
// single monotonic write path for those fields.
//
// The composite matters because clock_timestamp() can produce ties: without the
// id comparison a second message sharing a created_at would leave the preview
// pointing at the first one.
func (s *ThreadStore) TouchLastMessage(ctx context.Context, threadID int64, at time.Time, messageID, senderAccountID int64, body string) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	query := db.NewUpdate().
		Model((*staffThreadRow)(nil)).
		ModelTableExpr(tableExprStaffMessageThreadsAsThread).
		Set("last_message_at = ?", at).
		Set("last_message_id = ?", messageID).
		Set("last_sender_account_id = ?", senderAccountID).
		Set("last_message_body = ?", body).
		Set("updated_at = ?", at).
		Where(`"staff_message_thread".id = ?`, threadID).
		Where(`("staff_message_thread".last_message_at IS NULL
			OR "staff_message_thread".last_message_at < ?
			OR ("staff_message_thread".last_message_at = ?
				AND ("staff_message_thread".last_message_id IS NULL
					OR "staff_message_thread".last_message_id < ?)))`, at, at, messageID)
	query = withTenant(query, staffThreadAlias, tenantID)

	if _, err := query.Exec(ctx); err != nil {
		return fmt.Errorf("touch staff message thread last message: %w", err)
	}
	return nil
}

// ParticipantAccountIDs returns every account in the thread, ascending.
func (s *ThreadStore) ParticipantAccountIDs(ctx context.Context, threadID int64) ([]int64, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var ids []int64
	query := db.NewSelect().
		TableExpr(`users.staff_message_participants AS "participant"`).
		ColumnExpr(`"participant".account_id`).
		Where(`"participant".thread_id = ?`, threadID).
		OrderExpr(`"participant".account_id ASC`)
	query = withTenant(query, "participant", tenantID)

	if err := query.Scan(ctx, &ids); err != nil {
		return nil, fmt.Errorf("list staff message thread participants: %w", err)
	}
	return ids, nil
}

// IsParticipant reports whether the account belongs to the thread. This is the
// authorization predicate for reading or posting: membership is the ONLY thing
// that grants access to an internal conversation — no role, no permission, and
// no admin flag substitutes for it.
func (s *ThreadStore) IsParticipant(ctx context.Context, threadID, accountID int64) (bool, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, err
	}
	query := db.NewSelect().
		TableExpr(`users.staff_message_participants AS "participant"`).
		ColumnExpr(`1`).
		Where(`"participant".thread_id = ?`, threadID).
		Where(`"participant".account_id = ?`, accountID).
		Limit(1)
	query = withTenant(query, "participant", tenantID)

	exists, err := query.Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("check staff message thread participation: %w", err)
	}
	return exists, nil
}

// DeleteEmpty removes threads that hold no messages any more, so retention
// cleanup does not leave orphaned conversations in the inbox. Participant and
// read-cursor rows follow via ON DELETE CASCADE.
//
// createdBefore is a GRACE PERIOD, not an optimization. OpenThread
// get-or-creates the thread BEFORE the first message is sent, so a conversation
// someone opened and has not written in yet is legitimately empty. Without the
// bound the daily sweep would delete it under the user, and their next send
// would fail with "Diese Unterhaltung gibt es nicht." on a chat they had open.
func (s *ThreadStore) DeleteEmpty(ctx context.Context, createdBefore time.Time) (int64, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, err
	}
	query := db.NewDelete().
		Model((*staffThreadRow)(nil)).
		ModelTableExpr(tableExprStaffMessageThreadsAsThread).
		Where(`"staff_message_thread".created_at < ?`, createdBefore).
		Where(`NOT EXISTS (
			SELECT 1 FROM users.staff_messages m
			WHERE m.thread_id = "staff_message_thread".id
		)`)
	query = withTenant(query, staffThreadAlias, tenantID)

	res, err := query.Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete empty staff message threads: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("delete empty staff message threads: %w", err)
	}
	return affected, nil
}
