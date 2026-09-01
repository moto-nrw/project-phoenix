package audit

import (
	"context"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
)

type substitutionChangeRepository struct{ runtime Runtime }

func NewSubstitutionChangeRepository(runtime Runtime) auditModels.SubstitutionChangeCreator {
	return &substitutionChangeRepository{runtime: requireRuntime(runtime)}
}

func (r *substitutionChangeRepository) Create(ctx context.Context, change *auditModels.SubstitutionChange) error {
	return NewAppender(r.runtime).Append(ctx, change)
}
