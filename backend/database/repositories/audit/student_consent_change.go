package audit

import (
	"context"
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
)

const studentConsentChangeTableExpr = `audit.student_consent_changes AS "student_consent_change"`

type studentConsentChangeRepository struct {
	runtime Runtime
}

func NewStudentConsentChangeRepository(runtime Runtime) auditModels.StudentConsentChangeRepository {
	return &studentConsentChangeRepository{runtime: requireRuntime(runtime)}
}

func (r *studentConsentChangeRepository) Create(ctx context.Context, entry *auditModels.StudentConsentChange) error {
	return NewAppender(r.runtime).Append(ctx, entry)
}

// ListByStudentID is a custom ordered audit projection; the generic repository
// cannot express the stable created_at/id ordering needed by history readers.
func (r *studentConsentChangeRepository) ListByStudentID(ctx context.Context, studentID int64) ([]*auditModels.StudentConsentChange, error) {
	if studentID <= 0 {
		return nil, fmt.Errorf("student_id is required")
	}
	var rows []*auditModels.StudentConsentChange
	err := runtimeDB(ctx, r.runtime).NewSelect().
		Model(&rows).
		ModelTableExpr(studentConsentChangeTableExpr).
		Where(`"student_consent_change".student_id = ?`, studentID).
		OrderExpr(`"student_consent_change".created_at DESC, "student_consent_change".id DESC`).
		Scan(ctx)
	if err != nil {
		return nil, wrapDatabase("list student consent changes", err)
	}
	return rows, nil
}
