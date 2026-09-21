package repositories

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/usercontext"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// NewUserContextStaffGroups binds the retained user-context read side to the
// School Structure staff group reads (#3499).
func NewUserContextStaffGroups(groups schoolstructure.Query, membership schoolmembership.Capability, workTime workforce.Capability) (usercontext.StaffGroupReads, error) {
	query, err := schoolStructureCompose.NewStaffGroups(schoolStructureCompose.StaffGroupsDependencies{
		Groups: groups, Assignments: membership, Substitutions: workTime,
	})
	if err != nil {
		return nil, err
	}
	return userContextStaffGroups{query: query}, nil
}

type userContextStaffGroups struct {
	query schoolstructure.StaffGroupQuery
}

func (r userContextStaffGroups) SubstitutedGroupIDs(ctx context.Context, staffID int64, day string) (map[int64]bool, error) {
	groups, err := r.query.ListSubstitutedGroups(ctx, staffID, day)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]bool, len(groups))
	for _, group := range groups {
		result[group.Group.ID] = group.ViaSubstitution
	}
	return result, nil
}

func (r userContextStaffGroups) SchoolClasses(ctx context.Context, staffID int64) ([]string, error) {
	return r.query.ListSchoolClassesByStaff(ctx, staffID)
}
