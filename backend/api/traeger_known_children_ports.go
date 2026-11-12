package api

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-ogs/project-phoenix/modules/enrollment/compose"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	peopleDirectoryCompose "github.com/moto-ogs/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-ogs/project-phoenix/modules/traegerview"
)

// traegerKnownChildrenPorts binds the reads behind the hint on a known child
// of a holiday registration (#6683, ADR 0214): the registered children at
// Enrollment and their namesakes among the student profiles at People
// Directory. The roster is the projection's School Membership port.
func traegerKnownChildrenPorts() traegerview.KnownChildrenPorts {
	return traegerview.KnownChildrenPorts{
		Children:  traegerRegisteredChildren{children: enrollmentCompose.NewHolidayRegisteredChildren()},
		Namesakes: traegerStudentNamesakes{namesakes: peopleDirectoryCompose.NewStudentNamesakes()},
	}
}

type traegerRegisteredChildren struct {
	children enrollment.HolidayRegisteredChildren
}

func (c traegerRegisteredChildren) RegisteredChildren(ctx context.Context, organizationID, periodID, registrationID int64) ([]traegerview.RegisteredChild, error) {
	children, err := c.children.HolidayRegisteredChildren(ctx, organizationID, periodID, registrationID)
	if err != nil {
		return nil, err
	}
	result := make([]traegerview.RegisteredChild, 0, len(children))
	for _, child := range children {
		result = append(result, traegerview.RegisteredChild(child))
	}
	return result, nil
}

type traegerStudentNamesakes struct {
	namesakes peopledirectory.StudentNamesakes
}

func (n traegerStudentNamesakes) StudentNamesakesByTenant(ctx context.Context, schools []int64, children []traegerview.RegisteredChild) ([]traegerview.Namesake, error) {
	identities := make([]peopledirectory.StudentIdentity, 0, len(children))
	for _, child := range children {
		identities = append(identities, peopledirectory.StudentIdentity{FirstName: child.FirstName, LastName: child.LastName, Birthday: child.BirthDate})
	}
	namesakes, err := n.namesakes.StudentNamesakesByTenant(ctx, schools, identities)
	if err != nil {
		return nil, err
	}
	result := make([]traegerview.Namesake, 0, len(namesakes))
	for _, namesake := range namesakes {
		result = append(result, traegerview.Namesake{Child: namesake.Identity, SchoolID: namesake.SchoolID, StudentProfileID: namesake.StudentProfileID})
	}
	return result, nil
}

// holidayKnownChildren binds Enrollment's known children to the Träger
// projection, which reads them in its administrative transaction before the
// route opens the Träger transaction.
type holidayKnownChildren struct {
	overview traegerview.Overview
}

func (k holidayKnownChildren) KnownHolidayChildren(ctx context.Context, organizationID, periodID, registrationID int64) ([]enrollment.HolidayKnownChild, error) {
	known, err := k.overview.KnownHolidayChildren(ctx, organizationID, periodID, registrationID)
	if err != nil {
		return nil, err
	}
	result := make([]enrollment.HolidayKnownChild, 0, len(known))
	for _, child := range known {
		result = append(result, enrollment.HolidayKnownChild(child))
	}
	return result, nil
}
