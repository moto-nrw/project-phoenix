package application

import (
	context "context"
	fmt "fmt"

	filestorage "github.com/moto-nrw/project-phoenix/modules/filestorage"
	ports "github.com/moto-nrw/project-phoenix/modules/filestorage/internal/ports"
	identityaccess "github.com/moto-nrw/project-phoenix/modules/identityaccess"
	peopledirectory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

type IdentityAdapter struct{ Module Identity }

func (i IdentityAdapter) HasActiveMembership(ctx context.Context, accountID int64) (bool, error) {
	return i.Module.HasActiveSchoolMembership(ctx, accountID)
}

func (i IdentityAdapter) ListAccountRoleIDs(ctx context.Context, accountID int64) ([]int64, error) {
	return i.Module.ListAccountRoleIDs(ctx, accountID)
}

// ListShareableRoles drops the guardian tier: parents never reach the tenant
// portal, so a folder shared with them would be shared with nobody.
func (i IdentityAdapter) ListShareableRoles(ctx context.Context) ([]filestorage.AudienceRole, error) {
	roles, err := i.Module.ListSchoolRoles(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]filestorage.AudienceRole, 0, len(roles))
	for _, role := range roles {
		if role == nil || role.Name == "guardian" || (role.BaseRole != nil && *role.BaseRole == "guardian") {
			continue
		}
		result = append(result, filestorage.AudienceRole{ID: role.ID, Name: role.Name})
	}
	return result, nil
}

func (i IdentityAdapter) ListActiveAccountIDs(ctx context.Context) ([]int64, error) {
	return i.Module.ListActiveSchoolAccountIDs(ctx)
}

type Identity interface {
	identityaccess.SchoolMembershipQuery
	identityaccess.SchoolRoleQuery
}

type PeopleAdapter struct{ Query People }

func (p PeopleAdapter) PersonNames(ctx context.Context, accountIDs []int64) (map[int64]ports.PersonName, error) {
	persons, err := p.Query.ListPersonsByAccount(ctx, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("load persons by account: %w", err)
	}
	newest := make(map[int64]peopledirectory.Person, len(persons))
	for _, person := range persons {
		if person.AccountID == nil {
			continue
		}
		current, found := newest[*person.AccountID]
		if !found || person.UpdatedAt.After(current.UpdatedAt) {
			newest[*person.AccountID] = person
		}
	}
	result := make(map[int64]ports.PersonName, len(newest))
	for accountID, person := range newest {
		result[accountID] = ports.PersonName{FirstName: person.FirstName, LastName: person.LastName}
	}
	return result, nil
}

type People interface {
	ListPersonsByAccount(context.Context, []int64) ([]peopledirectory.Person, error)
}
