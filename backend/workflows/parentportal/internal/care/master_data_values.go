package care

import (
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The field-state helpers compare snapshots, not persistence rows. The
// departure value still uses the retained canonical policy; its production
// dependency remains tracked until the owner-policy cutover.
type MasterDataPerson struct {
	FirstName string
	LastName  string
	Birthday  *calendar.Date
}

type MasterDataStudent struct {
	SchoolClass           string
	AllowedDepartureModes AllowedDepartureModes
}

func MasterDataPersonValues(person *Person) *MasterDataPerson {
	if person == nil {
		return nil
	}
	return &MasterDataPerson{FirstName: person.FirstName, LastName: person.LastName, Birthday: person.Birthday}
}

func MasterDataStudentValues(student *PortalStudent) *MasterDataStudent {
	if student == nil {
		return nil
	}
	return &MasterDataStudent{SchoolClass: student.SchoolClass, AllowedDepartureModes: student.AllowedDepartureModes}
}
