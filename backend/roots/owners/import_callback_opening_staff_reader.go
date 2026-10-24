package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

func NewOpeningStaffReader(membership schoolmembership.Capability, people peopledirectory.Capability) func(ctx context.Context) ([]*dataimport.OpeningStaff, error) {
	return func(ctx context.Context) ([]*dataimport.OpeningStaff, error) {
		staff, err := membership.ListStaff(ctx, schoolmembership.StaffFilter{})
		if err != nil {
			return nil, err
		}
		ids := make([]int64, 0, len(staff))
		for _, member := range staff {
			ids = append(ids, member.PersonID)
		}
		persons, err := people.ListPersonsByID(ctx, ids)
		if err != nil {
			return nil, err
		}
		byID := make(map[int64]dataimport.OpeningPerson, len(persons))
		for _, person := range persons {
			byID[person.ID] = dataimport.OpeningPerson{FirstName: person.FirstName, LastName: person.LastName}
		}
		result := make([]*dataimport.OpeningStaff, 0, len(staff))
		for _, member := range staff {
			row := &dataimport.OpeningStaff{ID: member.ID, PersonnelNumber: member.PersonnelNumber}
			if person, found := byID[member.PersonID]; found {
				row.Person = &person
			}
			result = append(result, row)
		}
		return result, nil
	}
}
