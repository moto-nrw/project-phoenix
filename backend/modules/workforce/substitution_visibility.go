package workforce

import "context"

// SubstitutionOverviewScope answers whether the school's operational
// overview setting opens the substitution list to every staff member. The
// consumer binds it to the Settings Platform.
type SubstitutionOverviewScope func(ctx context.Context) (allStaff bool, err error)

// CanSeeAllSubstitutions decides whether the caller sees every group
// substitution of the school rather than only the ones of its own groups.
// An assignment-bound caller (a school-scoped account) never does. Everyone
// else needs the admin role, or a staff record while the overview scope is
// open to all staff. The scope is resolved for every other caller, so a
// settings failure is reported even to an admin.
func CanSeeAllSubstitutions(ctx context.Context, scope SubstitutionOverviewScope, assignmentBound, admin, hasStaff bool) (bool, error) {
	if assignmentBound {
		return false, nil
	}
	allStaff, err := scope(ctx)
	if err != nil {
		return false, err
	}
	return admin || (allStaff && hasStaff), nil
}
