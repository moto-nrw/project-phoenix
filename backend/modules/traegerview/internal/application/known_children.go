package application

import (
	"context"
	"slices"

	"github.com/moto-ogs/project-phoenix/modules/traegerview"
)

// KnownHolidayChildren returns the registered children of a holiday period of
// the Träger that exactly one actively managed child of its schools matches
// (#6683, ADR 0214), in registration order. It reads the children at
// Enrollment, their namesakes at People Directory and the roster of the
// schools with a namesake at School Membership: three statements for any
// number of children and schools. The answer names a school per child and
// nothing of the child at the school; Enrollment records the comparison.
func (o *Overview) KnownHolidayChildren(ctx context.Context, organizationID, periodID, registrationID int64) ([]traegerview.KnownChild, error) {
	if organizationID <= 0 {
		return nil, traegerview.ErrTraegerRequired
	}
	known := []traegerview.KnownChild{}
	if periodID <= 0 || registrationID < 0 {
		return known, nil
	}
	err := o.read(ctx, organizationID, func(adminCtx context.Context, schools []int64) error {
		children, err := o.known.Children.RegisteredChildren(adminCtx, organizationID, periodID, registrationID)
		if err != nil || len(children) == 0 || len(schools) == 0 {
			return err
		}
		namesakes, err := o.known.Namesakes.StudentNamesakesByTenant(adminCtx, schools, children)
		if err != nil || len(namesakes) == 0 {
			return err
		}
		roster, err := o.rosters.ActiveStudentMembershipsByTenant(adminCtx, namesakeSchools(namesakes), o.now())
		if err != nil {
			return err
		}
		known = knownChildren(children, namesakes, roster)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return known, nil
}

// namesakeSchools are the schools with a namesake, each once.
func namesakeSchools(namesakes []traegerview.Namesake) []int64 {
	schools := []int64{}
	for _, namesake := range namesakes {
		if !slices.Contains(schools, namesake.SchoolID) {
			schools = append(schools, namesake.SchoolID)
		}
	}
	return schools
}

// knownChildren keeps the children with exactly one namesake on the roster
// of its school.
func knownChildren(children []traegerview.RegisteredChild, namesakes []traegerview.Namesake, roster map[int64]map[int64]int64) []traegerview.KnownChild {
	matches := make([][]traegerview.Namesake, len(children))
	for _, namesake := range namesakes {
		if namesake.Child < 0 || namesake.Child >= len(children) {
			continue
		}
		if _, managed := roster[namesake.SchoolID][namesake.StudentProfileID]; managed {
			matches[namesake.Child] = append(matches[namesake.Child], namesake)
		}
	}
	known := []traegerview.KnownChild{}
	for i, child := range children {
		if len(matches[i]) == 1 {
			known = append(known, traegerview.KnownChild{RegistrationID: child.RegistrationID, ChildID: child.ChildID, SchoolID: matches[i][0].SchoolID})
		}
	}
	return known
}
