package users

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

const tableExprParentMessageThreadsAsThread = `users.parent_message_threads AS "parent_message_thread"`

// ParentMessageThreadRepository is the tenant-scoped data-access layer for
// parent-OGS message threads.
type ParentMessageThreadRepository struct {
	*base.Repository[*users.ParentMessageThread]
}

// NewParentMessageThreadRepository wires a fresh repository.
func NewParentMessageThreadRepository(db *bun.DB) users.ParentMessageThreadRepository {
	repo := base.NewRepository[*users.ParentMessageThread](db, "users.parent_message_threads", "ParentMessageThread")
	repo.TenantScoped = true
	return &ParentMessageThreadRepository{Repository: repo}
}

// FindByID returns the thread by id (tenant-scoped), or nil when absent.
// Delegates to the generic base helper so the schema-qualified query and tenant
// filter are not duplicated (backend-conventions Rule 2).
func (r *ParentMessageThreadRepository) FindByID(ctx context.Context, id int64) (*users.ParentMessageThread, error) {
	return r.FindByIDOrNil(ctx, id)
}

// FindByStudentGuardian returns the single conversation for a (student,
// guardian) pair in the current tenant, or nil when none exists yet.
func (r *ParentMessageThreadRepository) FindByStudentGuardian(ctx context.Context, studentID, guardianAccountID int64) (*users.ParentMessageThread, error) {
	thread := new(users.ParentMessageThread)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(thread).
		ModelTableExpr(tableExprParentMessageThreadsAsThread).
		Where(`"parent_message_thread".student_id = ?`, studentID).
		Where(`"parent_message_thread".guardian_account_id = ?`, guardianAccountID).
		Limit(1)

	query = base.WithTenantFilter(ctx, query, "parent_message_thread")

	err := query.Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find parent message thread by student+guardian", Err: err}
	}
	return thread, nil
}

// GetOrCreate returns the existing (student, guardian) thread in the tenant or
// atomically creates it. The INSERT ... ON CONFLICT DO NOTHING makes two
// concurrent first-messages race-safe: the loser does NOT raise a unique
// violation (which would abort the surrounding transaction and surface as a 500
// on a successful send), it simply inserts nothing; the row is then loaded
// either way. The conflict target matches uq_parent_message_threads_student_guardian.
func (r *ParentMessageThreadRepository) GetOrCreate(ctx context.Context, tenantID, studentID, guardianAccountID int64) (*users.ParentMessageThread, error) {
	thread := &users.ParentMessageThread{
		StudentID:         studentID,
		GuardianAccountID: guardianAccountID,
	}
	thread.SetTenantID(tenantID)
	// Qualify the table explicitly (users.parent_message_threads) like base.Create
	// does. Relying on the struct tag leaves the INSERT unqualified, which the
	// least-privilege phoenix_tenant role cannot resolve (its search_path excludes
	// the users schema) -> "relation parent_message_threads does not exist".
	if _, err := base.GetDB(ctx, r.DB).NewInsert().
		Model(thread).
		ModelTableExpr(r.TableName).
		On("CONFLICT (tenant_id, student_id, guardian_account_id) DO NOTHING").
		Exec(ctx); err != nil {
		return nil, &modelBase.DatabaseError{Op: "get-or-create parent message thread", Err: err}
	}
	// Whether we inserted or hit the conflict, the row now exists; load the
	// canonical row (tenant-scoped) so callers always get the persisted thread.
	existing, err := r.FindByStudentGuardian(ctx, studentID, guardianAccountID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, &modelBase.DatabaseError{
			Op:  "get-or-create parent message thread",
			Err: errors.New("thread missing after upsert"),
		}
	}
	return existing, nil
}

// LockForMessageAppend serializes inserts into a thread within the caller's
// transaction, keeping message tuple order aligned with commit order.
func (r *ParentMessageThreadRepository) LockForMessageAppend(ctx context.Context, threadID int64) error {
	var id int64
	query := base.GetDB(ctx, r.DB).NewSelect().
		TableExpr(tableExprParentMessageThreadsAsThread).
		ColumnExpr(`"parent_message_thread".id`).
		Where(`"parent_message_thread".id = ?`, threadID).
		For("UPDATE")
	query = base.WithTenantFilter(ctx, query, "parent_message_thread")
	if err := query.Scan(ctx, &id); err != nil {
		return &modelBase.DatabaseError{Op: "lock parent message thread for append", Err: err}
	}
	return nil
}

// TouchLastMessage atomically advances the thread's denormalized last-activity
// fields (last_message_at, last_sender_kind, last_message_body) — the columns the
// inbox projection sorts and previews by — but ONLY when `at` is newer than the
// stored last_message_at. This is the single write path for those fields on both
// portals.
//
// The composite guard also protects repair and legacy call sites from moving the
// preview backward. The append path separately locks the thread before insert,
// aligning message order with commit order.
func (r *ParentMessageThreadRepository) TouchLastMessage(ctx context.Context, threadID int64, at time.Time, messageID int64, senderKind, body string) error {
	query := base.GetDB(ctx, r.DB).NewUpdate().
		Model((*users.ParentMessageThread)(nil)).
		ModelTableExpr(tableExprParentMessageThreadsAsThread).
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

	query = base.WithTenantFilter(ctx, query, "parent_message_thread")

	if _, err := query.Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "touch parent message thread last message", Err: err}
	}
	return nil
}

// ClaimStaffMessageNotification atomically claims one staff-notification
// window for a thread. The database clock keeps the decision consistent across
// processes and avoids application-host clock skew.
func (r *ParentMessageThreadRepository) ClaimStaffMessageNotification(ctx context.Context, threadID int64, cooldown time.Duration) (bool, error) {
	if cooldown <= 0 {
		return false, errors.New("staff message notification cooldown must be positive")
	}
	query := base.GetDB(ctx, r.DB).NewUpdate().
		Model((*users.ParentMessageThread)(nil)).
		ModelTableExpr(tableExprParentMessageThreadsAsThread).
		Set("last_staff_message_notification_at = clock_timestamp()").
		Where(`"parent_message_thread".id = ?`, threadID).
		Where(`("parent_message_thread".last_staff_message_notification_at IS NULL
			OR "parent_message_thread".last_staff_message_notification_at < clock_timestamp() - (? * INTERVAL '1 microsecond'))`, cooldown.Microseconds())
	query = base.WithTenantFilter(ctx, query, "parent_message_thread")

	result, err := query.Exec(ctx)
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "claim staff parent-message notification", Err: err}
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, &modelBase.DatabaseError{Op: "count claimed staff parent-message notifications", Err: err}
	}
	return rows == 1, nil
}

// ListGuardiansForStudent returns the child's account-holding guardians who may
// actually use the parent portal for THIS child, primary first, for the staff
// "new conversation" recipient picker. A guardian is included only when the
// relationship grants parent_portal.access — pickup-only, emergency-contact and
// custom limited roles hold an account but no portal authority, and the
// parent-side reads reject those same threads (resolveOwnedChild requires
// parent_portal.access), so offering them as recipients would let staff send a
// message the chosen guardian can never see. Permissions are written as
// `{"<perm>": true}` (StudentGuardianPermissionSet), so JSONB containment of
// `{"parent_portal.access": true}` matches the same grant the Go check enforces;
// a NULL/empty permissions map fails containment and is correctly excluded.
//
// The account must ALSO hold an ACTIVE auth.account_tenants membership for this
// school. A guardian whose mapping is pending or inactive can no longer read its
// children — the parent-side reads resolve children through
// account_tenants.status = 'active' (see parent.ChildRepository) — so without
// this join staff could open/reply to a thread the recipient cannot see and
// BroadcastParentMessage would still wake that revoked account if it is logged
// in for another tenant. Matching at.tenant_id = sg.tenant_id keeps the
// membership check scoped to the same school as the relationship row.
func (r *ParentMessageThreadRepository) ListGuardiansForStudent(ctx context.Context, studentID int64) ([]*users.MessageableGuardian, error) {
	var rows []*users.MessageableGuardian
	query := base.GetDB(ctx, r.DB).NewSelect().
		TableExpr("users.students_guardians AS sg").
		ColumnExpr("gp.account_id AS account_id").
		ColumnExpr("btrim(COALESCE(gp.first_name,'') || ' ' || COALESCE(gp.last_name,'')) AS name").
		ColumnExpr("sg.relationship_type AS relationship_type").
		ColumnExpr("sg.is_primary AS is_primary").
		ColumnExpr("COALESCE(gp.portal_locale, 'de') AS portal_locale").
		Join("JOIN users.guardian_profiles AS gp ON gp.id = sg.guardian_profile_id").
		Join("JOIN auth.account_tenants AS at ON at.account_id = gp.account_id AND at.tenant_id = sg.tenant_id AND at.status = ?", auth.AccountTenantStatusActive).
		Where("sg.student_id = ?", studentID).
		Where("gp.account_id IS NOT NULL").
		Where("gp.has_account = true").
		Where(`sg.permissions @> ?::jsonb`, `{"parent_portal.access": true}`).
		OrderExpr("sg.is_primary DESC, name ASC")

	query = base.WithTenantFilter(ctx, query, "sg")

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list guardians for student", Err: err}
	}
	return rows, nil
}
