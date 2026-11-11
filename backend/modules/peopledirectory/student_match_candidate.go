package peopledirectory

import "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

// StudentMatchCandidate is an enrolled (active or pending) child of the
// school with the first name, last name and birthday an enrollment names
// (#6053). The office decides whether it is the same child.
type StudentMatchCandidate struct {
	StudentID   int64
	FirstName   string
	LastName    string
	Birthday    *calendar.Date
	SchoolClass string
	Status      string
}

// StudentIdentity is an exact name-and-birthday probe for enrolled children.
type StudentIdentity struct {
	FirstName string
	LastName  string
	Birthday  calendar.Date
}
