package staffdirectoryview

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// BirthdaysOn lists the live staff members whose stored birthday recurs on
// one of days, leaving out those who opted out of the birthday display. No
// day lists nobody.
func (d *Directory) BirthdaysOn(ctx context.Context, days []MonthDay) ([]StaffBirthday, error) {
	if len(days) == 0 {
		return nil, nil
	}
	return d.staffBirthdays(ctx, days, true)
}

// AllBirthdays lists every live staff member with a stored birthday for the
// export, which deliberately ignores the display opt-out.
func (d *Directory) AllBirthdays(ctx context.Context) ([]StaffBirthday, error) {
	return d.staffBirthdays(ctx, nil, false)
}

// staffBirthdays projects live staff with a stored birth date. days nil means
// every stored birthday; otherwise only the annually recurring days are kept.
func (d *Directory) staffBirthdays(ctx context.Context, days []MonthDay, applyOptOut bool) ([]StaffBirthday, error) {
	values, err := d.listStaff(ctx, schoolmembership.StaffFilter{})
	if err != nil {
		return nil, err
	}
	members := make([]schoolmembership.Staff, 0, len(values))
	personIDs := make([]int64, 0, len(values))
	for _, value := range values {
		if applyOptOut && value.BirthdayDisplayOptOut {
			continue
		}
		members = append(members, value)
		personIDs = append(personIDs, value.PersonID)
	}
	persons, err := d.deps.Persons.PersonsByID(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	wanted := make(map[MonthDay]bool, len(days))
	for _, day := range days {
		wanted[day] = true
	}
	entries := make([]StaffBirthday, 0, len(members))
	for _, member := range members {
		person, found := persons[member.PersonID]
		if !found || person.Birthday == nil || person.Birthday.IsZero() {
			continue
		}
		if days != nil && !wanted[MonthDay{Month: person.Birthday.Month(), Day: person.Birthday.Day()}] {
			continue
		}
		entries = append(entries, StaffBirthday{
			StaffID: member.ID, FirstName: person.FirstName, LastName: person.LastName, Birthday: *person.Birthday,
		})
	}
	return entries, nil
}
