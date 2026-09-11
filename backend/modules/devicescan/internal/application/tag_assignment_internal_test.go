package application

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
)

type tagPeopleFake struct {
	fakePeople
	role            string
	specialization  string
	teacher         bool
	teacherErr      error
	normalized      bool
	staffRecord     *ports.StaffMember
	personRecord    *ports.Person
	staffLookupErr  error
	personLookupErr error
	linkErr         error
	linkedPerson    int64
	linkedTag       string
	unlinkedPerson  int64
}

func (p *tagPeopleFake) FindStaff(context.Context, int64) (*ports.StaffMember, error) {
	return p.staffRecord, p.staffLookupErr
}

func (p *tagPeopleFake) FindPerson(context.Context, int64) (*ports.Person, error) {
	return p.personRecord, p.personLookupErr
}

func (p *tagPeopleFake) LinkTag(_ context.Context, personID int64, tag string) error {
	p.linkedPerson, p.linkedTag = personID, tag
	return p.linkErr
}

func (p *tagPeopleFake) UnlinkTag(_ context.Context, personID int64) error {
	p.unlinkedPerson = personID
	return nil
}

func TestStaffTagAssignmentsUseTheStaffPersonAndPreservePreviousTag(t *testing.T) {
	t.Parallel()
	previous := "OLD-TAG-123"
	people := &tagPeopleFake{
		staffRecord:  &ports.StaffMember{ID: testStaffID, PersonID: testPersonID},
		personRecord: &ports.Person{ID: testPersonID, FirstName: "Ada", LastName: "Example", TagID: &previous},
	}
	commands := NewTagAssignments(people, fakePrincipals{device: testDevice()}, fakeClock{now: fixedNow}, nil)
	assigned, err := commands.AssignStaffTag(context.Background(), testStaffID, testTag)
	if err != nil || !assigned.Success || assigned.StudentID != testStaffID || assigned.StudentName != "Ada Example" || assigned.PreviousTag == nil || *assigned.PreviousTag != previous || people.linkedPerson != testPersonID || people.linkedTag != testTag {
		t.Fatalf("staff assignment contract changed: %+v, %v", assigned, err)
	}
	if assigned.Message != "RFID tag assigned successfully (previous tag replaced)" {
		t.Fatalf("previous-tag message changed: %q", assigned.Message)
	}
	unassigned, err := commands.UnassignStaffTag(context.Background(), testStaffID)
	if err != nil || !unassigned.Success || unassigned.RFIDTag != previous || people.unlinkedPerson != testPersonID {
		t.Fatalf("staff unassignment contract changed: %+v, %v", unassigned, err)
	}
}

func TestStaffTagAssignmentsRejectUnauthenticatedMutations(t *testing.T) {
	t.Parallel()
	people := &tagPeopleFake{}
	commands := NewTagAssignments(people, fakePrincipals{}, fakeClock{now: fixedNow}, nil)
	_, assignErr := commands.AssignStaffTag(context.Background(), testStaffID, testTag)
	_, unassignErr := commands.UnassignStaffTag(context.Background(), testStaffID)
	if !errors.Is(assignErr, devicescan.ErrDeviceUnauthorized) || !errors.Is(unassignErr, devicescan.ErrDeviceUnauthorized) || people.linkedPerson != 0 || people.unlinkedPerson != 0 {
		t.Fatalf("missing device must reject both writes: %v, %v", assignErr, unassignErr)
	}
}

func (p *tagPeopleFake) NormalizeTag(string) string {
	p.normalized = true
	return testTag
}

func (p *tagPeopleFake) TeacherRole(context.Context, int64) (string, string, bool, error) {
	return p.role, p.specialization, p.teacher, p.teacherErr
}

func TestTagAssignmentDistinguishesFreeBraceletsFromLookupFailures(t *testing.T) {
	t.Parallel()
	people := &tagPeopleFake{}
	query := NewTagAssignments(people, fakePrincipals{device: testDevice()}, fakeClock{now: fixedNow}, nil)
	assignment, err := query.LookupTagAssignment(context.Background(), "a1:b2:c3:d4")
	if err != nil || assignment.Assigned || !people.normalized {
		t.Fatalf("free bracelet must be normalized and reported unassigned: %+v, %v", assignment, err)
	}
	failure := errors.New("database unavailable")
	people.personErr = failure
	_, err = query.LookupTagAssignment(context.Background(), testTag)
	classified, ok := devicescan.IsFailure(err)
	if !ok || classified.Kind != devicescan.FailureInternal || classified.Message != "Internal server error" || !errors.Is(err, failure) {
		t.Fatalf("lookup failure must remain a server error with private cause: %v", err)
	}
}

func TestTagAssignmentRejectsMissingDevice(t *testing.T) {
	t.Parallel()
	people := &tagPeopleFake{}
	query := NewTagAssignments(people, fakePrincipals{}, fakeClock{now: fixedNow}, nil)
	_, err := query.LookupTagAssignment(context.Background(), testTag)
	if !errors.Is(err, devicescan.ErrDeviceUnauthorized) || people.normalized {
		t.Fatalf("missing device must be rejected before directory access: %v", err)
	}
}

func TestTagAssignmentHonorsCareEndDayAndGraduation(t *testing.T) {
	t.Parallel()
	tag := testTag
	clock := fakeClock{now: fixedNow}
	day := clock.Day(fixedNow)
	student := &ports.Student{ID: testStudentID, PersonID: testPersonID, SchoolClass: "1a", EnrolledUntil: &day}
	people := &tagPeopleFake{fakePeople: fakePeople{
		persons:  map[string]*ports.Person{tag: {ID: testPersonID, FirstName: "Ada", LastName: "Example", TagID: &tag}},
		students: map[int64]*ports.Student{testPersonID: student},
	}}
	query := NewTagAssignments(people, fakePrincipals{device: testDevice()}, clock, nil)
	assigned, err := query.LookupTagAssignment(context.Background(), tag)
	if err != nil || !assigned.Assigned || assigned.PersonType != "student" || assigned.Student == nil || assigned.Person == nil || assigned.Student.ID != testStudentID || assigned.Person.Name != "Ada Example" {
		t.Fatalf("student stays assigned through the inclusive last care day: %+v, %v", assigned, err)
	}
	previousDay := day.AddDays(-1)
	student.EnrolledUntil = &previousDay
	assigned, err = query.LookupTagAssignment(context.Background(), tag)
	if err != nil || assigned.Assigned {
		t.Fatalf("departed child must appear unassigned: %+v, %v", assigned, err)
	}
	student.EnrolledUntil = nil
	student.Alumnus = true
	assigned, err = query.LookupTagAssignment(context.Background(), tag)
	if err != nil || assigned.Assigned {
		t.Fatalf("graduate must appear unassigned: %+v, %v", assigned, err)
	}
}
