package services

import "context"

// CaregiverChainsForTests exposes the School Membership caregiver chains to
// the operator directory suite.
var CaregiverChainsForTests = caregiverChainsFromMembership

// ClassListEntryStudentsForTests exposes the class-list name lookup over a
// People Directory to its suite.
func ClassListEntryStudentsForTests(persons classListEntryPeople) ClassListEntryStudentLookup {
	return classListEntryStudents{persons: persons}
}

// ClassListEntryStudentLookup is the class-list name lookup under test.
type ClassListEntryStudentLookup interface {
	ListStudentIDsByNameAndClass(ctx context.Context, firstName, lastName, schoolClass string) ([]int64, error)
	IsEnrolledStudent(ctx context.Context, studentID int64) (bool, error)
}
