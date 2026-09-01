package audit

import (
	"context"
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
)

type StudentDeletionRepository struct {
	runtime Runtime
}

func NewStudentDeletionRepository(runtime Runtime) auditModels.StudentDeletionRepository {
	return &StudentDeletionRepository{runtime: requireRuntime(runtime)}
}

func (r *StudentDeletionRepository) Create(ctx context.Context, event *auditModels.StudentDeletion) error {
	if event == nil {
		return fmt.Errorf("student deletion audit event is required")
	}
	return NewAppender(r.runtime).Append(ctx, event)
}

func (r *StudentDeletionRepository) CountStudentReferences(ctx context.Context, studentID int64) (int, error) {
	if studentID <= 0 {
		return 0, fmt.Errorf("student ID must be positive")
	}
	tenantID := runtimeTenantID(ctx, r.runtime)
	if tenantID <= 0 {
		return 0, fmt.Errorf("tenant context is required")
	}
	var count int
	err := runtimeDB(ctx, r.runtime).NewSelect().ColumnExpr(`
		(SELECT COUNT(*) FROM audit.enrollment_offering_adjustments WHERE tenant_id = ? AND student_id = ?) +
		(SELECT COUNT(*) FROM audit.guardian_changes WHERE tenant_id = ? AND student_id = ?) +
		(SELECT COUNT(*) FROM audit.student_field_edits WHERE tenant_id = ? AND student_id = ?)
	`, tenantID, studentID, tenantID, studentID, tenantID, studentID).Scan(ctx, &count)
	if err != nil {
		return 0, wrapDatabase("count audit references for student deletion", err)
	}
	return count, nil
}
