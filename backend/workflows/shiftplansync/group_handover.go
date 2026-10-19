package shiftplansync

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// ErrHandoverExists reports a group handover that duplicates a stored one.
var ErrHandoverExists = errors.New("group handover already exists")

// HandoverSubstitutions is the Workforce command surface a group handover
// writes: education.group_substitution belongs to Workforce (#2688).
type HandoverSubstitutions interface {
	CreateGroupSubstitution(ctx context.Context, value workforce.GroupSubstitution) (workforce.GroupSubstitution, error)
	DeleteGroupSubstitution(ctx context.Context, id int64) error
}

// GroupHandovers writes the group handovers ("Gruppenübergabe") School
// Structure's substitution module plans into Workforce's substitution rows,
// on the caller's tenant transaction: the module's lock, duplicate check,
// write and audit stay one unit of work.
type GroupHandovers struct {
	substitutions HandoverSubstitutions
}

// NewGroupHandovers binds the handover writes to Workforce. A missing
// dependency is a composition error.
func NewGroupHandovers(substitutions HandoverSubstitutions) GroupHandovers {
	if substitutions == nil {
		panic("group handovers: Workforce substitutions are required")
	}
	return GroupHandovers{substitutions: substitutions}
}

// CreateHandover stores a handover. A substitution Workforce already holds
// for the same group, staff member and period is ErrHandoverExists.
func (h GroupHandovers) CreateHandover(ctx context.Context, value workforce.GroupSubstitution) (workforce.GroupSubstitution, error) {
	created, err := h.substitutions.CreateGroupSubstitution(ctx, value)
	if errors.Is(err, workforce.ErrGroupSubstitutionExists) {
		return workforce.GroupSubstitution{}, ErrHandoverExists
	}
	return created, err
}

// DeleteHandover removes a handover.
func (h GroupHandovers) DeleteHandover(ctx context.Context, id int64) error {
	return h.substitutions.DeleteGroupSubstitution(ctx, id)
}
