package contract

import "time"

// Teacher is a teacher assigned to a group, as the group reads show it.
type Teacher struct {
	ID             int64
	StaffID        int64
	Specialization string
	Role           string
	Qualifications string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	// Person is the staff member's person, nil when the staff member or
	// person is not resolvable.
	Person *TeacherPerson
}

// TeacherPerson is the name and login of a teacher's person.
type TeacherPerson struct {
	FirstName string
	LastName  string
	// Email is the person's login e-mail, empty without an account.
	Email string
}

// FullName returns "Vorname Nachname", or "" without a person.
func (t *Teacher) FullName() string {
	if t.Person == nil {
		return ""
	}
	return t.Person.FirstName + " " + t.Person.LastName
}
