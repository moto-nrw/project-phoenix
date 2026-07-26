package audit

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const guardianChangeTableExpr = `audit.guardian_changes AS "guardian_change"`

type guardianChangeRepository struct {
	*base.Repository[*audit.GuardianChange]
	db *bun.DB
}

func NewGuardianChangeRepository(db *bun.DB) audit.GuardianChangeRepository {
	repo := base.NewRepository[*audit.GuardianChange](db, "audit.guardian_changes", "GuardianChange")
	repo.TenantScoped = true
	return &guardianChangeRepository{Repository: repo, db: db}
}

func (r *guardianChangeRepository) ListByStudentID(ctx context.Context, studentID int64) ([]*audit.GuardianChange, error) {
	if studentID <= 0 {
		return nil, fmt.Errorf("student_id is required")
	}
	var rows []*audit.GuardianChange
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(guardianChangeTableExpr).
		Where(`"guardian_change".student_id = ?`, studentID).
		OrderExpr(`"guardian_change".changed_at DESC, "guardian_change".id DESC`).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list guardian changes", Err: err}
	}
	return rows, nil
}
