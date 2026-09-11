package audit

import (
	"context"
	"fmt"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
)

type PersonnelNumberChangeRepository struct {
	runtime Runtime
}

func NewPersonnelNumberChangeRepository(runtime Runtime) auditModels.PersonnelNumberChangeCreator {
	return &PersonnelNumberChangeRepository{runtime: requireRuntime(runtime)}
}

func (r *PersonnelNumberChangeRepository) Create(ctx context.Context, event *auditModels.PersonnelNumberChange) error {
	if event == nil {
		return fmt.Errorf("personnel number change audit event is required")
	}
	if err := event.Validate(); err != nil {
		return fmt.Errorf("invalid personnel number change audit event: %w", err)
	}
	return NewAppender(r.runtime).Append(ctx, event)
}
