package audit

import (
	"context"
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
)

type EnrollmentDeletionRepository struct {
	runtime Runtime
}

func NewEnrollmentDeletionRepository(runtime Runtime) auditModels.EnrollmentDeletionRepository {
	return &EnrollmentDeletionRepository{runtime: requireRuntime(runtime)}
}

func (r *EnrollmentDeletionRepository) Create(ctx context.Context, event *auditModels.EnrollmentDeletion) error {
	if event == nil {
		return fmt.Errorf("enrollment deletion audit event is required")
	}
	return NewAppender(r.runtime).Append(ctx, event)
}
