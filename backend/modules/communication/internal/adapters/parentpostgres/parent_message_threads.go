package parentpostgres

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
	parentThreadTable     = "users.parent_message_threads"
	parentThreadTableExpr = `users.parent_message_threads AS "parent_message_thread"`
	parentThreadAlias     = "parent_message_thread"
)

// ThreadStore owns the one conversation per (school, student, guardian) and
// its denormalized last-activity preview.
type ThreadStore struct{ store }

// NewThreadStore builds the thread store over the ambient transaction runtime.
func NewThreadStore(database Database) *ThreadStore {
	return &ThreadStore{store: newStore(database)}
}

type parentThreadRow struct {
	bun.BaseModel                  `bun:"table:users.parent_message_threads,alias:parent_message_thread"`
	ID                             int64      `bun:"id,pk,autoincrement"`
	TenantID                       int64      `bun:"tenant_id,notnull"`
	CreatedAt                      time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt                      time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	StudentID                      int64      `bun:"student_id,notnull"`
	GuardianAccountID              int64      `bun:"guardian_account_id,notnull"`
	LastMessageAt                  *time.Time `bun:"last_message_at"`
	LastMessageID                  *int64     `bun:"last_message_id"`
	LastSenderKind                 *string    `bun:"last_sender_kind"`
	LastMessageBody                string     `bun:"last_message_body"`
	StaffHandledUpToAt             *time.Time `bun:"staff_handled_up_to_at"`
	StaffHandledUpToMessageID      *int64     `bun:"staff_handled_up_to_message_id"`
	LastStaffMessageNotificationAt *time.Time `bun:"last_staff_message_notification_at"`
}

func (r *parentThreadRow) value() *domain.ParentMessageThread {
	if r == nil {
		return nil
	}
	return &domain.ParentMessageThread{
		ID: r.ID, TenantID: r.TenantID, StudentID: r.StudentID,
		GuardianAccountID: r.GuardianAccountID, LastMessageAt: r.LastMessageAt,
		LastMessageID: r.LastMessageID, LastSenderKind: r.LastSenderKind,
		LastMessageBody: r.LastMessageBody, StaffHandledUpToAt: r.StaffHandledUpToAt,
		StaffHandledUpToMessageID:      r.StaffHandledUpToMessageID,
		LastStaffMessageNotificationAt: r.LastStaffMessageNotificationAt,
		CreatedAt:                      r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func threadRow(value *domain.ParentMessageThread) *parentThreadRow {
	return &parentThreadRow{
		ID: value.ID, TenantID: value.TenantID, StudentID: value.StudentID,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		GuardianAccountID: value.GuardianAccountID, LastMessageAt: value.LastMessageAt,
		LastMessageID: value.LastMessageID, LastSenderKind: value.LastSenderKind,
		LastMessageBody: value.LastMessageBody, StaffHandledUpToAt: value.StaffHandledUpToAt,
		StaffHandledUpToMessageID:      value.StaffHandledUpToMessageID,
		LastStaffMessageNotificationAt: value.LastStaffMessageNotificationAt,
	}
}

// Create inserts one thread row.
func (s *ThreadStore) Create(ctx context.Context, thread *domain.ParentMessageThread) error {
	if thread == nil {
		return errors.New("create parent message thread: thread is required")
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	if thread.TenantID == 0 {
		thread.TenantID = tenantID
	}
	row := threadRow(thread)
	if _, err := db.NewInsert().
		Model(row).
		ModelTableExpr(parentThreadTable).
		Returning("id, created_at, updated_at").
		Exec(ctx); err != nil {
		return fmt.Errorf("create parent message thread: %w", err)
	}
	thread.ID = row.ID
	thread.CreatedAt = row.CreatedAt
	thread.UpdatedAt = row.UpdatedAt
	return nil
}

// Update rewrites one thread row by primary key.
func (s *ThreadStore) Update(ctx context.Context, thread *domain.ParentMessageThread) error {
	if thread == nil || thread.ID <= 0 {
		return errors.New("update parent message thread: thread id is required")
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	query := db.NewUpdate().
		Model(threadRow(thread)).
		ModelTableExpr(parentThreadTableExpr).
		WherePK()
	query = withTenant(query, parentThreadAlias, tenantID)
	result, err := query.Exec(ctx)
	if err != nil {
		return fmt.Errorf("update parent message thread: %w", err)
	}
	return assertOneRow(result, "update parent message thread")
}

// FindByID returns the thread within the current tenant, or nil when absent.
func (s *ThreadStore) FindByID(ctx context.Context, id int64) (*domain.ParentMessageThread, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	row := new(parentThreadRow)
	query := db.NewSelect().
		Model(row).
		ModelTableExpr(parentThreadTableExpr).
		Where(`"parent_message_thread".id = ?`, id).
		Limit(1)
	query = withTenant(query, parentThreadAlias, tenantID)
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find parent message thread: %w", err)
	}
	return row.value(), nil
}

// FindByStudentGuardian returns the single conversation for a (student,
// guardian) pair in the current tenant, or nil when none exists yet.
func (s *ThreadStore) FindByStudentGuardian(ctx context.Context, studentID, guardianAccountID int64) (*domain.ParentMessageThread, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	row := new(parentThreadRow)
	query := db.NewSelect().
		Model(row).
		ModelTableExpr(parentThreadTableExpr).
		Where(`"parent_message_thread".student_id = ?`, studentID).
		Where(`"parent_message_thread".guardian_account_id = ?`, guardianAccountID).
		Limit(1)
	query = withTenant(query, parentThreadAlias, tenantID)
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find parent message thread by student+guardian: %w", err)
	}
	return row.value(), nil
}

// GetOrCreate returns the existing (student, guardian) thread in the school or
// atomically creates it. INSERT ... ON CONFLICT DO NOTHING makes two concurrent
// first messages race-safe: the loser inserts nothing instead of raising a
// unique violation that would abort the surrounding transaction and surface as
// a 500 on a successful send. The conflict target matches
// uq_parent_message_threads_student_guardian.
func (s *ThreadStore) GetOrCreate(ctx context.Context, tenantID, studentID, guardianAccountID int64) (*domain.ParentMessageThread, error) {
	db, _, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	row := &parentThreadRow{TenantID: tenantID, StudentID: studentID, GuardianAccountID: guardianAccountID}
	// Qualify the table explicitly: relying on the struct tag leaves the INSERT
	// unqualified, which the least-privilege phoenix_tenant role cannot resolve
	// (its search_path excludes the users schema).
	if _, err := db.NewInsert().
		Model(row).
		ModelTableExpr(parentThreadTable).
		On("CONFLICT (tenant_id, student_id, guardian_account_id) DO NOTHING").
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("get-or-create parent message thread: %w", err)
	}
	// Whether we inserted or hit the conflict, the row now exists; load the
	// canonical row so callers always get the persisted thread.
	existing, err := s.FindByStudentGuardian(ctx, studentID, guardianAccountID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errors.New("get-or-create parent message thread: thread missing after upsert")
	}
	return existing, nil
}

// LockForMessageAppend serializes inserts into one thread within the caller's
// transaction, keeping message order aligned with commit order.
func (s *ThreadStore) LockForMessageAppend(ctx context.Context, threadID int64) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	var id int64
	query := db.NewSelect().
		TableExpr(parentThreadTableExpr).
		ColumnExpr(`"parent_message_thread".id`).
		Where(`"parent_message_thread".id = ?`, threadID).
		For("UPDATE")
	query = withTenant(query, parentThreadAlias, tenantID)
	if err := query.Scan(ctx, &id); err != nil {
		return fmt.Errorf("lock parent message thread for append: %w", err)
	}
	return nil
}

// TouchLastMessage advances the denormalized last-activity fields the inbox
// sorts and previews by, but ONLY when the message's (at, messageID) composite
// is newer than the stored one. This is the single monotonic write path for
// those fields, including when two messages share a timestamp and the higher id
// wins.
func (s *ThreadStore) TouchLastMessage(ctx context.Context, threadID int64, at time.Time, messageID int64, senderKind, body string) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	query := db.NewUpdate().
		Model((*parentThreadRow)(nil)).
		ModelTableExpr(parentThreadTableExpr).
		Set("last_message_at = ?", at).
		Set("last_message_id = ?", messageID).
		Set("last_sender_kind = ?", senderKind).
		Set("last_message_body = ?", body).
		Set("updated_at = ?", at).
		Where(`"parent_message_thread".id = ?`, threadID).
		Where(`("parent_message_thread".last_message_at IS NULL
			OR "parent_message_thread".last_message_at < ?
			OR ("parent_message_thread".last_message_at = ?
				AND ("parent_message_thread".last_message_id IS NULL
					OR "parent_message_thread".last_message_id < ?)))`, at, at, messageID)
	query = withTenant(query, parentThreadAlias, tenantID)
	if _, err := query.Exec(ctx); err != nil {
		return fmt.Errorf("touch parent message thread last message: %w", err)
	}
	return nil
}

// ClaimStaffMessageNotification atomically claims one staff-notification window
// for a thread. PostgreSQL supplies the clock, so multiple application hosts
// agree and host clock skew cannot open a second window.
func (s *ThreadStore) ClaimStaffMessageNotification(ctx context.Context, threadID int64, cooldown time.Duration) (bool, error) {
	if cooldown <= 0 {
		return false, errors.New("staff message notification cooldown must be positive")
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, err
	}
	query := db.NewUpdate().
		Model((*parentThreadRow)(nil)).
		ModelTableExpr(parentThreadTableExpr).
		Set("last_staff_message_notification_at = clock_timestamp()").
		Where(`"parent_message_thread".id = ?`, threadID).
		Where(`("parent_message_thread".last_staff_message_notification_at IS NULL
			OR "parent_message_thread".last_staff_message_notification_at < clock_timestamp() - (? * INTERVAL '1 microsecond'))`, cooldown.Microseconds())
	query = withTenant(query, parentThreadAlias, tenantID)
	result, err := query.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("claim staff parent-message notification: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("count claimed staff parent-message notifications: %w", err)
	}
	return rows == 1, nil
}

// MarkStaffHandledUpTo advances the shared team boundary for guardian activity
// already covered by a staff reply. The composite guard keeps a stale or
// concurrent reply from moving it backward.
func (s *ThreadStore) MarkStaffHandledUpTo(ctx context.Context, tenantID, threadID int64, handledAt time.Time, handledMessageID int64) error {
	db, contextTenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	query := db.NewUpdate().
		Model((*parentThreadRow)(nil)).
		ModelTableExpr(`users.parent_message_threads AS "thread"`).
		Set("staff_handled_up_to_at = ?", handledAt).
		Set("staff_handled_up_to_message_id = ?", handledMessageID).
		Where(`"thread".id = ?`, threadID).
		Where(`"thread".tenant_id = ?`, tenantID).
		Where(`("thread".staff_handled_up_to_at IS NULL OR
			("thread".staff_handled_up_to_at, "thread".staff_handled_up_to_message_id) < (?, ?))`, handledAt, handledMessageID)
	query = withTenant(query, "thread", contextTenantID)
	if _, err := query.Exec(ctx); err != nil {
		return fmt.Errorf("mark parent message thread handled for staff: %w", err)
	}
	return nil
}
