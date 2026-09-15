package audit

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/audit"
)

const guardianChangeTableExpr = `audit.guardian_changes AS "guardian_change"`

type guardianChangeRepository struct {
	runtime Runtime
}

func NewGuardianChangeRepository(runtime Runtime) audit.GuardianChangeRepository {
	return &guardianChangeRepository{runtime: requireRuntime(runtime)}
}

func (r *guardianChangeRepository) Create(ctx context.Context, entry *audit.GuardianChange) error {
	return NewAppender(r.runtime).Append(ctx, entry)
}

func (r *guardianChangeRepository) ListByStudentID(ctx context.Context, studentID int64) ([]*audit.GuardianChange, error) {
	if studentID <= 0 {
		return nil, fmt.Errorf("student_id is required")
	}
	var rows []*audit.GuardianChange
	err := runtimeDB(ctx, r.runtime).NewSelect().
		Model(&rows).
		ModelTableExpr(guardianChangeTableExpr).
		Where(`"guardian_change".student_id = ?`, studentID).
		OrderExpr(`"guardian_change".changed_at DESC, "guardian_change".id DESC`).
		Scan(ctx)
	if err != nil {
		return nil, wrapDatabase("list guardian changes", err)
	}
	return rows, nil
}
