package peopledirectory

import "context"

// EnrollmentCare is the care profile of one child an enrollment batch
// created: the membership its enrollment returned, the created child and the
// input it was created from.
type EnrollmentCare struct {
	MembershipID int64
	Student      CreatedEnrollmentStudent
	Input        EnrollmentStudent
}

// StudentCareSave is one care profile the student owners save in a batch:
// what SaveCare receives for one membership.
type StudentCareSave struct {
	MembershipID int64
	Record       StudentRecord
	Plan         StudentPlan
	Note         *string
	Touched      bool
}

// EnrollmentBatches creates new children for the data import in three
// steps instead of one CreateEnrollmentStudent per child (#5552, ADR 0067
// addendum of 2026-11-08, step 7; exception E3 in its batch form): the
// profiles of all children, then each child's membership where the import
// would create it, then the care profiles of all children. Every step joins
// the caller's transaction, opens none and no savepoint; the caller's chunk
// savepoint is the unit that rolls back. It stays outside Capability and
// Owner; the data import binds it on its own.
type EnrollmentBatches interface {
	// CreateEnrollmentProfiles inserts the student profiles of the inputs in
	// one statement under the shared class-writes gate, with the columns
	// and validation of CreateEnrollmentStudent; the result is index-aligned.
	CreateEnrollmentProfiles(context.Context, []EnrollmentStudent) ([]CreatedEnrollmentStudent, error)
	// EnrollCreatedStudent writes the membership of a child the batch
	// created, from the same input (its status now), as
	// CreateEnrollmentStudent does, and returns the membership.
	EnrollCreatedStudent(context.Context, CreatedEnrollmentStudent, EnrollmentStudent) (int64, error)
	// SaveEnrollmentCares writes the care profiles of the children in one
	// statement, with the values CreateEnrollmentStudent saves.
	SaveEnrollmentCares(context.Context, []EnrollmentCare) error
}
