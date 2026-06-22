package audit

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const guardianPickupChangeTableExpr = `audit.guardian_pickup_changes AS "guardian_pickup_change"`

type guardianPickupChangeRepository struct {
	*base.Repository[*audit.GuardianPickupChange]
	db *bun.DB
}

func NewGuardianPickupChangeRepository(db *bun.DB) audit.GuardianPickupChangeRepository {
	repo := base.NewRepository[*audit.GuardianPickupChange](db, "audit.guardian_pickup_changes", "GuardianPickupChange")
	repo.TenantScoped = true
	return &guardianPickupChangeRepository{Repository: repo, db: db}
}

func (r *guardianPickupChangeRepository) ListByStudentID(ctx context.Context, studentID int64) ([]*audit.GuardianPickupChange, error) {
	if studentID <= 0 {
		return nil, fmt.Errorf("student_id is required")
	}
	var rows []*audit.GuardianPickupChange
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(guardianPickupChangeTableExpr).
		Where(`"guardian_pickup_change".student_id = ?`, studentID).
		OrderExpr(`"guardian_pickup_change".changed_at DESC, "guardian_pickup_change".id DESC`).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list guardian pickup changes", Err: err}
	}
	return rows, nil
}
