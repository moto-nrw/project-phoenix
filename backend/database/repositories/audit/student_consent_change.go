package audit

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const studentConsentChangeTableExpr = `audit.student_consent_changes AS "student_consent_change"`

type studentConsentChangeRepository struct {
	*base.Repository[*auditModels.StudentConsentChange]
	db *bun.DB
}

func NewStudentConsentChangeRepository(db *bun.DB) auditModels.StudentConsentChangeRepository {
	repo := base.NewRepository[*auditModels.StudentConsentChange](db, "audit.student_consent_changes", "StudentConsentChange")
	repo.TenantScoped = true
	return &studentConsentChangeRepository{Repository: repo, db: db}
}

// ListByStudentID is a custom ordered audit projection; the generic repository
// cannot express the stable created_at/id ordering needed by history readers.
func (r *studentConsentChangeRepository) ListByStudentID(ctx context.Context, studentID int64) ([]*auditModels.StudentConsentChange, error) {
	if studentID <= 0 {
		return nil, fmt.Errorf("student_id is required")
	}
	var rows []*auditModels.StudentConsentChange
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(&rows).
		ModelTableExpr(studentConsentChangeTableExpr).
		Where(`"student_consent_change".student_id = ?`, studentID).
		OrderExpr(`"student_consent_change".created_at DESC, "student_consent_change".id DESC`).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "list student consent changes", Err: base.TranslateNotFound(err)}
	}
	return rows, nil
}
