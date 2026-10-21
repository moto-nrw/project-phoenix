package peopledirectory

import "context"

// StaffMergePerson is a live person considered for an operator-reviewed repair.
// Student profiles, including former ones, always exclude a person.
type StaffMergePerson struct {
	Person
	NameKey    string `json:"-"`
	HasStudent bool   `json:"has_student"`
}

// StaffPersonMerge is an administrative capability. Callers must hold an
// administrative transaction. Locking reads lock people in ascending ID order.
type StaffPersonMerge interface {
	StaffMergePeople(context.Context, int64, bool) ([]StaffMergePerson, error)
	RetireMergedStaffPerson(context.Context, int64, int64, int64) error
}
