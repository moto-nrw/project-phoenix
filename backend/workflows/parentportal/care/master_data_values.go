package care

import (
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The field-state helpers compare snapshots, not persistence rows. The
// departure value still uses the retained canonical policy; its production
// dependency remains tracked until the owner-policy cutover.
type masterDataPerson struct {
	FirstName string
	LastName  string
	Birthday  *calendar.Date
}

type masterDataStudent struct {
	SchoolClass           string
	AllowedDepartureModes AllowedDepartureModes
}

func masterDataPersonValues(person *usersModels.Person) *masterDataPerson {
	if person == nil {
		return nil
	}
	return &masterDataPerson{FirstName: person.FirstName, LastName: person.LastName, Birthday: person.Birthday}
}

func masterDataStudentValues(student *usersModels.Student) *masterDataStudent {
	if student == nil {
		return nil
	}
	return &masterDataStudent{SchoolClass: student.SchoolClass, AllowedDepartureModes: student.AllowedDepartureModes}
}
