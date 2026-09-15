package audit

import (
	"context"
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
)

type EnrollmentRestorationRepository struct {
	runtime Runtime
}

func NewEnrollmentRestorationRepository(runtime Runtime) auditModels.EnrollmentRestorationRepository {
	return &EnrollmentRestorationRepository{runtime: requireRuntime(runtime)}
}

func (r *EnrollmentRestorationRepository) Create(ctx context.Context, event *auditModels.EnrollmentRestoration) error {
	if event == nil {
		return fmt.Errorf("enrollment restoration audit event is required")
	}
	return NewAppender(r.runtime).Append(ctx, event)
}
