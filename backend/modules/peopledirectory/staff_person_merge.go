package peopledirectory

import (
	"context"
	"errors"
	"fmt"
)

// ErrStaffPersonMergeNoLongerEligible reports a live person group that no
// longer satisfies the reviewed repair predicates.
var ErrStaffPersonMergeNoLongerEligible = errors.New("staff person merge no longer eligible")

// StaffPersonMergeLockKey serializes a reviewed repair with creating a person
// or staff membership in the same school. The merge takes it exclusively;
// creation takes it shared, so different schools remain independent.
func StaffPersonMergeLockKey(schoolID int64) string {
	return fmt.Sprintf("staff-person-merge:%d", schoolID)
}

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
