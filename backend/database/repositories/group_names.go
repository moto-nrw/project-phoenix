package repositories

import (
	"context"

	educationModels "github.com/moto-nrw/project-phoenix/models/education"
)

// GroupNames resolves education group names by id for the enrollment class
// roster, which prints a child's group next to its name (#3563). A group id
// without a row is absent from the result.
type GroupNames struct {
	groups educationModels.GroupRepository
}

// NewGroupNames binds the lookup to the retained group repository.
func NewGroupNames(groups educationModels.GroupRepository) GroupNames {
	return GroupNames{groups: groups}
}

// GroupNamesByID returns the names of the groups in one query.
func (g GroupNames) GroupNamesByID(ctx context.Context, groupIDs []int64) (map[int64]string, error) {
	groups, err := g.groups.FindByIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(groups))
	for id, group := range groups {
		if group != nil {
			names[id] = group.Name
		}
	}
	return names, nil
}
