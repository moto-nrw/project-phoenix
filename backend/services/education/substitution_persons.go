package education

import (
	"context"

	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// Person is the display projection of a staff member's person row.
type Person struct {
	ID        int64
	FirstName string
	LastName  string
}

// PersonQuery resolves persons by ID. The substitution repository stops at
// the staff row (#2661); the names the overview shows come from the People
// Directory through this port.
type PersonQuery interface {
	ListPersonsByID(context.Context, []int64) ([]Person, error)
}

// attachSubstitutionPersons resolves Staff.Person for the regular and
// substitute staff of every row. Staff without a resolvable person keep a
// nil Person, so the projection falls back to the bare staff reference.
func attachSubstitutionPersons(ctx context.Context, persons PersonQuery, rows []*educationModels.GroupSubstitution) error {
	if persons == nil || len(rows) == 0 {
		return nil
	}
	staff := make([]*userModels.Staff, 0, 2*len(rows))
	seen := make(map[*userModels.Staff]struct{}, 2*len(rows))
	add := func(member *userModels.Staff) {
		if member == nil {
			return
		}
		if _, found := seen[member]; found {
			return
		}
		seen[member] = struct{}{}
		staff = append(staff, member)
	}
	for _, row := range rows {
		if row == nil {
			continue
		}
		add(row.RegularStaff)
		add(row.SubstituteStaff)
	}
	if len(staff) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(staff))
	for _, member := range staff {
		ids = append(ids, member.PersonID)
	}
	values, err := persons.ListPersonsByID(ctx, ids)
	if err != nil {
		return err
	}
	byID := make(map[int64]Person, len(values))
	for _, value := range values {
		byID[value.ID] = value
	}
	for _, member := range staff {
		value, found := byID[member.PersonID]
		if !found {
			continue
		}
		person := &userModels.Person{FirstName: value.FirstName, LastName: value.LastName}
		person.ID = value.ID
		member.Person = person
	}
	return nil
}
