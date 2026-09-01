package audit

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/models/audit"
)

// SQL clause constants
const (
	whereAccountIDEquals = "account_id = ?"
	whereCreatedAtGTE    = "created_at >= ?"
	whereSuccessEquals   = "success = ?"
)

// AuthEventRepository implements the audit.AuthEventRepository interface
type AuthEventRepository struct {
	runtime Runtime
}

// NewAuthEventRepository creates a new auth event repository
func NewAuthEventRepository(runtime Runtime) *AuthEventRepository {
	return &AuthEventRepository{runtime: requireRuntime(runtime)}
}

// Create creates a new auth event
func (r *AuthEventRepository) Create(ctx context.Context, event *audit.AuthEvent) error {
	return NewAppender(r.runtime).Append(ctx, event)
}

// FindByID finds an auth event by ID
func (r *AuthEventRepository) FindByID(ctx context.Context, id interface{}) (*audit.AuthEvent, error) {
	var event audit.AuthEvent
	err := runtimeDB(ctx, r.runtime).NewSelect().
		Model(&event).
		ModelTableExpr(`audit.auth_events AS "auth_event"`).
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("auth event not found")
		}
		return nil, err
	}

	return &event, nil
}

// FindByAccountID returns auth events for a specific account
func (r *AuthEventRepository) FindByAccountID(ctx context.Context, accountID int64, limit int) ([]*audit.AuthEvent, error) {
	var events []*audit.AuthEvent

	query := runtimeDB(ctx, r.runtime).NewSelect().
		Model(&events).
		ModelTableExpr(`audit.auth_events AS "auth_event"`).
		Where(whereAccountIDEquals, accountID).
		Order(orderByCreatedAtDesc)

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}

	return events, nil
}

// List returns all auth events matching the given filters
func (r *AuthEventRepository) List(ctx context.Context, filters map[string]interface{}) ([]*audit.AuthEvent, error) {
	var events []*audit.AuthEvent

	query := runtimeDB(ctx, r.runtime).NewSelect().
		Model(&events).
		ModelTableExpr(`audit.auth_events AS "auth_event"`).
		Order(orderByCreatedAtDesc)

	// Apply filters
	for key, value := range filters {
		switch key {
		case "account_id":
			query = query.Where(whereAccountIDEquals, value)
		case "event_type":
			query = query.Where("event_type = ?", value)
		case "success":
			query = query.Where(whereSuccessEquals, value)
		case "since":
			if t, ok := value.(time.Time); ok {
				query = query.Where(whereCreatedAtGTE, t)
			}
		case "until":
			if t, ok := value.(time.Time); ok {
				query = query.Where("created_at <= ?", t)
			}
		}
	}

	err := query.Scan(ctx)
	if err != nil {
		return nil, err
	}

	return events, nil
}

// CreateStaffPreviewEndOnce records the end of one staff-view preview
// instance and reports whether THIS call wrote the row (#2893).
//
// Ending a preview is idempotent and concurrent: the client may repeat the
// call, and two tabs can end the same preview in the same moment. A
// read-then-insert guard cannot cover the second case — both callers read
// "not ended yet" before either row lands. So uniqueness lives in the
// database (partial unique index idx_auth_events_staff_preview_end_once,
// migration 1.15.357) and the conflict is absorbed here: the second writer
// simply inserts nothing and gets false.
func (r *AuthEventRepository) CreateStaffPreviewEndOnce(ctx context.Context, event *audit.AuthEvent) (bool, error) {
	if event == nil || event.EventType != audit.EventTypeStaffPreviewEnded {
		return false, errors.New("staff preview end event is required")
	}
	return NewAppender(r.runtime).AppendOnce(ctx, event)
}

// StaffPreviewEnded reports whether the preview instance previewID of
// adminAccountID has already been ended (#2893).
//
// Read against the same partial unique index that makes the end one-shot, so
// the lookup is a single index probe. It answers one question only: may a
// token still be treated as the running preview instance, or does it belong
// to a preview that is already closed?
func (r *AuthEventRepository) StaffPreviewEnded(ctx context.Context, accountID int64, previewID string) (bool, error) {
	if previewID == "" {
		return false, errors.New("preview_id is required")
	}
	return runtimeDB(ctx, r.runtime).NewSelect().
		Model((*audit.AuthEvent)(nil)).
		ModelTableExpr(`audit.auth_events AS "auth_event"`).
		Where(`"auth_event".`+whereAccountIDEquals, accountID).
		Where(`"auth_event".event_type = ?`, audit.EventTypeStaffPreviewEnded).
		Where(`"auth_event".metadata->>'preview_id' = ?`, previewID).
		Exists(ctx)
}

// LockStaffPreview takes the transaction-scoped advisory lock of one preview
// instance (#2893). Two writers contend for it: the renewal, which asks
// whether the instance is still running and then mints its replacement token,
// and the end, which closes it. Without the lock those two straddle each
// other — the renewal reads "still running", the end commits, and the token
// minted a moment later revives a preview the admin had closed, with no start
// event of its own and an end the uniqueness index would swallow.
//
// The lock is released with the caller's transaction, so the renewal keeps it
// until its token is committed.
func (r *AuthEventRepository) LockStaffPreview(ctx context.Context, accountID int64, previewID string) error {
	if previewID == "" {
		return errors.New("preview_id is required")
	}
	_, err := runtimeDB(ctx, r.runtime).ExecContext(ctx,
		"SELECT pg_advisory_xact_lock(hashtextextended(?, 0))",
		fmt.Sprintf("staff-preview:%d:%s", accountID, previewID),
	)
	return err
}

// ListPendingAccountWideWipes returns the newest pending wipe per account.
// A zero since value lists every still-pending wipe so a failed after-commit
// revoke is retried for as long as the flag remains.
func (r *AuthEventRepository) ListPendingAccountWideWipes(ctx context.Context, since time.Time) ([]audit.PendingAccountWideWipe, error) {
	var rows []audit.PendingAccountWideWipe
	query := runtimeDB(ctx, r.runtime).NewSelect().
		ColumnExpr(`DISTINCT ON ("pending".account_id) "pending".id AS event_id`).
		ColumnExpr(`"pending".tenant_id`).
		ColumnExpr(`"pending".account_id`).
		ColumnExpr(`COALESCE("pending".metadata->>'reason', '') AS reason`).
		ColumnExpr(`"pending".created_at`).
		TableExpr(`audit.auth_events AS "pending"`).
		Where(`"pending".event_type = ?`, audit.EventTypeTokenRevoked).
		Where(`"pending".metadata @> ?`, `{"pending_account_wide_wipe":true}`).
		Where(`NOT EXISTS (
			SELECT 1 FROM audit.auth_events AS "completed"
			WHERE "completed".event_type = ?
			AND "completed".metadata->>'pending_event_id' = "pending".id::text
		)`, audit.EventTypeAccountWideWipeCompleted)
	if !since.IsZero() {
		query = query.Where(`"pending".created_at >= ?`, since)
	}
	err := query.
		OrderExpr(`"pending".account_id ASC, "pending".created_at DESC`).
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// ClaimPendingAccountWideWipes locks and returns incomplete pending events.
// Completion is represented by a later append-only event in the same
// transaction; rollback therefore releases the claim without mutating history.
func (r *AuthEventRepository) ClaimPendingAccountWideWipes(ctx context.Context, accountID int64) ([]audit.PendingAccountWideWipe, error) {
	var rows []audit.PendingAccountWideWipe
	err := runtimeDB(ctx, r.runtime).NewSelect().
		ColumnExpr(`"pending".id AS event_id`).
		ColumnExpr(`"pending".tenant_id`).
		ColumnExpr(`"pending".account_id`).
		ColumnExpr(`COALESCE("pending".metadata->>'reason', '') AS reason`).
		ColumnExpr(`"pending".created_at`).
		TableExpr(`audit.auth_events AS "pending"`).
		Where(`"pending".account_id = ?`, accountID).
		Where(`"pending".event_type = ?`, audit.EventTypeTokenRevoked).
		Where(`"pending".metadata @> ?`, `{"pending_account_wide_wipe":true}`).
		Where(`NOT EXISTS (
			SELECT 1 FROM audit.auth_events AS "completed"
			WHERE "completed".event_type = ?
			AND "completed".metadata->>'pending_event_id' = "pending".id::text
		)`, audit.EventTypeAccountWideWipeCompleted).
		For("UPDATE OF pending SKIP LOCKED").
		Scan(ctx, &rows)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return rows, nil
}
