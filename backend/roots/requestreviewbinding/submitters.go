package requestreviewbinding

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
)

// SubmitterPeople is the People Directory read that names who filed a request:
// the profiles of the filing accounts and their links to children, both in
// the tenant in context.
type SubmitterPeople interface {
	ListGuardiansByAccount(context.Context, []int64) ([]peopledirectory.Guardian, error)
	ListGuardianLinksByAccounts(context.Context, []int64) ([]peopledirectory.GuardianLink, error)
}

// NewSubmitterDirectory binds the filer decoration (#4359) to the People
// Directory. A missing optional people source leaves the decoration disabled.
func NewSubmitterDirectory(guardians SubmitterPeople) requestreview.SubmitterDirectory {
	if guardians == nil {
		return nil
	}
	return submitterDirectory{guardians: guardians}
}

type submitterDirectory struct {
	guardians SubmitterPeople
}

// Submitters costs two reads per page, however many filings it carries. The
// name comes from the filer's profile at this school; the relationship only
// from the stored link to the child of that filing, so a parent of a sibling
// is named without a relationship rather than with a borrowed one.
func (d submitterDirectory) Submitters(ctx context.Context, keys []requestreview.SubmitterKey) (map[requestreview.SubmitterKey]requestreview.Submitter, error) {
	accountIDs := make([]int64, 0, len(keys))
	for _, key := range keys {
		accountIDs = append(accountIDs, key.AccountID)
	}
	profiles, err := d.guardians.ListGuardiansByAccount(ctx, accountIDs)
	if err != nil {
		return nil, err
	}
	profileByAccount := profilesByAccount(profiles)
	if len(profileByAccount) == 0 {
		return map[requestreview.SubmitterKey]requestreview.Submitter{}, nil
	}
	links, err := d.guardians.ListGuardianLinksByAccounts(ctx, accountIDs)
	if err != nil {
		return nil, err
	}
	relationships := make(map[profileChild]string, len(links))
	for _, link := range links {
		relationships[profileChild{link.GuardianProfileID, link.StudentID}] = link.RelationshipType
	}
	result := make(map[requestreview.SubmitterKey]requestreview.Submitter, len(keys))
	for _, key := range keys {
		profile, ok := profileByAccount[key.AccountID]
		if !ok || profile.FullName() == "" {
			continue
		}
		result[key] = requestreview.Submitter{Name: profile.FullName(), RelationshipType: relationships[profileChild{profile.ID, key.StudentID}]}
	}
	return result, nil
}

type profileChild struct{ profileID, studentID int64 }

// profilesByAccount keys the profiles by their account. One account has one
// profile per school; the lowest ID decides should a stale duplicate ever
// exist, so the name stays stable.
func profilesByAccount(profiles []peopledirectory.Guardian) map[int64]peopledirectory.Guardian {
	result := make(map[int64]peopledirectory.Guardian, len(profiles))
	for _, profile := range profiles {
		if profile.AccountID == nil {
			continue
		}
		if current, ok := result[*profile.AccountID]; !ok || profile.ID < current.ID {
			result[*profile.AccountID] = profile
		}
	}
	return result
}
