package audit

import (
	"context"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

type substitutionChangeRepository struct{ db *bun.DB }

func NewSubstitutionChangeRepository(db *bun.DB) auditModels.SubstitutionChangeCreator {
	return &substitutionChangeRepository{db: db}
}

func (r *substitutionChangeRepository) Create(ctx context.Context, change *auditModels.SubstitutionChange) error {
	base.EnsureTenantID(ctx, change)
	if _, err := base.GetDB(ctx, r.db).NewInsert().
		Model(change).
		ModelTableExpr(`audit.substitution_changes AS "substitution_change"`).
		Returning("*").
		Exec(ctx); err != nil {
		return &modelBase.DatabaseError{Op: "create substitution change", Err: err}
	}
	return nil
}
