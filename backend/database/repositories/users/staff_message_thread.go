package users

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

const tableExprStaffMessageThreadsAsThread = `users.staff_message_threads AS "staff_message_thread"`

// StaffMessageThreadRepository is the tenant-scoped data-access layer for
// OGS-internal colleague conversations (#2598).
type StaffMessageThreadRepository struct {
	*base.Repository[*users.StaffMessageThread]
}

// NewStaffMessageThreadRepository wires a fresh repository.
func NewStaffMessageThreadRepository(db *bun.DB) users.StaffMessageThreadRepository {
	repo := base.NewRepository[*users.StaffMessageThread](db, "users.staff_message_threads", "StaffMessageThread")
	repo.TenantScoped = true
	return &StaffMessageThreadRepository{Repository: repo}
}

// FindByID returns the thread by id (tenant-scoped), or nil when absent.
func (r *StaffMessageThreadRepository) FindByID(ctx context.Context, id int64) (*users.StaffMessageThread, error) {
	return r.FindByIDOrNil(ctx, id)
}

// findByParticipantKey loads the canonical thread row for a participant key in
// the current tenant, or nil when none exists yet.
func (r *StaffMessageThreadRepository) findByParticipantKey(ctx context.Context, key string) (*users.StaffMessageThread, error) {
	thread := new(users.StaffMessageThread)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(thread).
		ModelTableExpr(tableExprStaffMessageThreadsAsThread).
		Where(`"staff_message_thread".participant_key = ?`, key).
		Limit(1)

	query = base.WithTenantFilter(ctx, query, "staff_message_thread")

	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find staff message thread by participant key", Err: err}
	}
	return thread, nil
}

// GetOrCreateDirect returns the conversation between the two accounts in the
// current tenant, atomically creating it when absent.
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
func (r *StaffMessageThreadRepository) GetOrCreateDirect(ctx context.Context, accountA, accountB int64) (*users.StaffMessageThread, error) {
	key := users.DirectParticipantKey(accountA, accountB)

	thread := &users.StaffMessageThread{
		ParticipantKey: key,
		Kind:           users.StaffMessageThreadKindDirect,
	}
	base.EnsureTenantID(ctx, thread)

	// Qualify the table explicitly like base.Create does. Relying on the struct
	// tag leaves the INSERT unqualified, which the least-privilege phoenix_tenant
	// role cannot resolve (its search_path excludes the users schema) ->
	// "relation staff_message_threads does not exist".
	if _, err := base.GetDB(ctx, r.DB).NewInsert().
		Model(thread).
		ModelTableExpr(r.TableName).
		On("CONFLICT (tenant_id, participant_key) DO NOTHING").
		Exec(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "get-or-create staff message thread", Err: err}
	}

	existing, err := r.findByParticipantKey(ctx, key)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get-or-create staff message thread",
			Err: errors.New("thread missing after upsert"),
		}
	}

	if err := r.ensureParticipants(ctx, existing, accountA, accountB); err != nil {
		return nil, err
	}
	return existing, nil
}

// ensureParticipants inserts the membership rows for a direct thread, tolerating
// rows that already exist.
func (r *StaffMessageThreadRepository) ensureParticipants(ctx context.Context, thread *users.StaffMessageThread, accountIDs ...int64) error {
	rows := make([]*users.StaffMessageParticipant, 0, len(accountIDs))
	for _, accountID := range accountIDs {
		row := &users.StaffMessageParticipant{
			ThreadID:  thread.ID,
			AccountID: accountID,
		}
		row.SetTenantID(thread.TenantID)
		rows = append(rows, row)
	}

	if _, err := base.GetDB(ctx, r.DB).NewInsert().
		Model(&rows).
		ModelTableExpr("users.staff_message_participants").
		On("CONFLICT (thread_id, account_id) DO NOTHING").
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "ensure staff message participants", Err: err}
	}
	return nil
}

// LockForMessageAppend serializes inserts into a thread within the caller's
// transaction, keeping message tuple order aligned with commit order.
func (r *StaffMessageThreadRepository) LockForMessageAppend(ctx context.Context, threadID int64) error {
	var id int64
	query := base.GetDB(ctx, r.DB).NewSelect().
		TableExpr(tableExprStaffMessageThreadsAsThread).
		ColumnExpr(`"staff_message_thread".id`).
		Where(`"staff_message_thread".id = ?`, threadID).
		For("UPDATE")

	query = base.WithTenantFilter(ctx, query, "staff_message_thread")

	if err := query.Scan(ctx, &id); err != nil {
		return &modelBase.DatabaseError{Op: "lock staff message thread for append", Err: err}
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
func (r *StaffMessageThreadRepository) TouchLastMessage(ctx context.Context, threadID int64, at time.Time, messageID, senderAccountID int64, body string) error {
	query := base.GetDB(ctx, r.DB).NewUpdate().
		Model((*users.StaffMessageThread)(nil)).
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

	query = base.WithTenantFilter(ctx, query, "staff_message_thread")

	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "touch staff message thread last message", Err: err}
	}
	return nil
}

// ParticipantAccountIDs returns every account in the thread, ascending.
func (r *StaffMessageThreadRepository) ParticipantAccountIDs(ctx context.Context, threadID int64) ([]int64, error) {
	var ids []int64
	query := base.GetDB(ctx, r.DB).NewSelect().
		TableExpr(`users.staff_message_participants AS "participant"`).
		ColumnExpr(`"participant".account_id`).
		Where(`"participant".thread_id = ?`, threadID).
		OrderExpr(`"participant".account_id ASC`)

	query = base.WithTenantFilter(ctx, query, "participant")

	if err := query.Scan(ctx, &ids); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list staff message thread participants", Err: err}
	}
	return ids, nil
}

// IsParticipant reports whether the account belongs to the thread. This is the
// authorization predicate for reading or posting: membership is the ONLY thing
// that grants access to an internal conversation — no role, no permission, and
// no admin flag substitutes for it.
func (r *StaffMessageThreadRepository) IsParticipant(ctx context.Context, threadID, accountID int64) (bool, error) {
	query := base.GetDB(ctx, r.DB).NewSelect().
		TableExpr(`users.staff_message_participants AS "participant"`).
		ColumnExpr(`1`).
		Where(`"participant".thread_id = ?`, threadID).
		Where(`"participant".account_id = ?`, accountID).
		Limit(1)

	query = base.WithTenantFilter(ctx, query, "participant")

	exists, err := query.Exists(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "check staff message thread participation", Err: err}
	}
	return exists, nil
}

// DeleteEmpty removes threads that hold no messages any more, so retention
// cleanup does not leave orphaned conversations in the inbox. Participant and
// read-cursor rows follow via ON DELETE CASCADE.
func (r *StaffMessageThreadRepository) DeleteEmpty(ctx context.Context) (int64, error) {
	query := base.GetDB(ctx, r.DB).NewDelete().
		Model((*users.StaffMessageThread)(nil)).
		ModelTableExpr(tableExprStaffMessageThreadsAsThread).
		Where(`NOT EXISTS (
			SELECT 1 FROM users.staff_messages m
			WHERE m.thread_id = "staff_message_thread".id
		)`)

	query = base.WithTenantFilter(ctx, query, "staff_message_thread")

	res, err := query.Exec(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete empty staff message threads", Err: err}
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "delete empty staff message threads", Err: err}
	}
	return affected, nil
}
