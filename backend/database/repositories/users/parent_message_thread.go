package users

import (
	"context"
	"database/sql"
	"errors"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
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
func (r *ParentMessageThreadRepository) FindByID(ctx context.Context, id int64) (*users.ParentMessageThread, error) {
	thread := new(users.ParentMessageThread)
	query := base.GetDB(ctx, r.DB).NewSelect().
		Model(thread).
		ModelTableExpr(tableExprParentMessageThreadsAsThread).
		Where(`"parent_message_thread".id = ?`, id).
		Limit(1)

	if where, val, ok := base.TenantWhere(ctx, "parent_message_thread"); ok {
		query = query.Where(where, val)
	}

	err := query.Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find parent message thread", Err: err}
	}
	return thread, nil
}

// ListGuardiansForStudent returns the child's account-holding guardians,
// primary first, for the staff "new conversation" recipient picker.
func (r *ParentMessageThreadRepository) ListGuardiansForStudent(ctx context.Context, studentID int64) ([]*users.MessageableGuardian, error) {
	var rows []*users.MessageableGuardian
	query := base.GetDB(ctx, r.DB).NewSelect().
		TableExpr("users.students_guardians AS sg").
		ColumnExpr("gp.account_id AS account_id").
		ColumnExpr("btrim(COALESCE(gp.first_name,'') || ' ' || COALESCE(gp.last_name,'')) AS name").
		ColumnExpr("sg.relationship_type AS relationship_type").
		ColumnExpr("sg.is_primary AS is_primary").
		Join("JOIN users.guardian_profiles AS gp ON gp.id = sg.guardian_profile_id").
		Where("sg.student_id = ?", studentID).
		Where("gp.account_id IS NOT NULL").
		Where("gp.has_account = true").
		OrderExpr("sg.is_primary DESC, name ASC")

	if where, val, ok := base.TenantWhere(ctx, "sg"); ok {
		query = query.Where(where, val)
	}

	if err := query.Scan(ctx, &rows); err != nil {
		return nil, &modelBase.DatabaseError{Op: "list guardians for student", Err: err}
	}
	return rows, nil
}
