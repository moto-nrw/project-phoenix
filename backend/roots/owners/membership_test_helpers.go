//go:build testhooks

package owners

import (
	"fmt"
	"time"

	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// NewMembershipTestRepositories composes the People Directory and School
// Membership repositories the repository test graphs read through, over
// unobserved owners (#2743).
func NewMembershipTestRepositories(db transactionruntime.Database) (MembershipTestRepositories, error) {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return MembershipTestRepositories{}, err
	}
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return MembershipTestRepositories{}, err
	}
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return MembershipTestRepositories{}, err
	}
	workTime, err := NewWorkforce(db, membership)
	if err != nil {
		return MembershipTestRepositories{}, err
	}
	groups, err := NewSchoolStructure(db)
	if err != nil {
		return MembershipTestRepositories{}, err
	}
	persons, identity := NewPersonRepository(db), NewPeopleIdentityAccess(db, nil)
	members := NewMembershipRepositories(db, persons, identity, membership, rooms, workTime)
	members.People = people
	members.Student = NewStudentRepository(db, membership, groups)
	members.GuardianProfile = NewGuardianProfileRepository(db)
	members.StudentGuardian = NewStudentGuardianRepository(db)
	members.GroupSubstitution = NewGroupSubstitutionRepository(NewSchoolStructureRepositories(db, persons, identity, membership, rooms, workTime).Group, membership, workTime)
	return members, nil
}

// MustNewMembershipTestRepositories is NewMembershipTestRepositories for test
// graphs that cannot return an error; composing only fails on missing
// dependencies, which is a programming error at wiring time.
func MustNewMembershipTestRepositories(db transactionruntime.Database) MembershipTestRepositories {
	members, err := NewMembershipTestRepositories(db)
	if err != nil {
		panic(fmt.Sprintf("compose membership test repositories: %v", err))
	}
	return members
}

// NewWorkforce composes the work-time owner unobserved for the test graphs
// whose test roles may not name the Workforce composition. Production roots compose the
// module themselves (api/base.go) so runtime evidence is kept.
func NewWorkforce(db transactionruntime.Database, membership schoolmembership.Capability) (workforce.Capability, error) {
	return NewWorkforceWithClock(db, membership, nil)
}

// NewWorkforceWithClock is NewWorkforce with a pinned clock for the live
// work-session window and the calendar day; nil means the wall clock.
func NewWorkforceWithClock(db transactionruntime.Database, membership schoolmembership.Capability, now func() time.Time) (workforce.Capability, error) {
	return workforceCompose.New(workforceCompose.Dependencies{
		LockStaffAssignment: WorkforceStaffAssignmentLock(membership),
		DB:                  db.Pool(),
		LiveStaffIDs:        WorkforceLiveStaffIDs(membership),
		Observe:             func(workforceCompose.Observation) {},
		Now:                 now,
	})
}

// MembershipTestRepositories are the People Directory repositories over
// School Membership the retained graphs read through. services composes them
// where it composes the owners; the test graphs build them through
// api.NewMembershipTestRepositories (#2743, #4005). The group store and the
// teaching assignments come from SchoolStructureSuiteRepositories, the owner
// itself from MustNewSchoolMembership (#4080).
type MembershipTestRepositories struct {
	// People is the People Directory owner the student rows translate to.
	People          peopledirectory.Capability
	Person          peoplePorts.PersonRepository
	Student         peoplePorts.StudentRepository
	GuardianProfile peoplePorts.GuardianProfileRepository
	StudentGuardian peoplePorts.StudentGuardianRepository
	Staff           peoplePorts.StaffRepository
	Teacher         peoplePorts.TeacherRepository
	Guest           peoplePorts.GuestRepository
	// GroupSubstitution is the Workforce group substitution contract over the
	// group store and School Membership.
	GroupSubstitution GroupSubstitutions
}

// NewMembershipRepositories composes the staff, teacher and guest adapters
// over membership, with the group store and the teaching assignments they
// resolve through. members.People, Student and the guardian fields stay
// unset; the caller composes them.
func NewMembershipRepositories(db transactionruntime.Database, persons peoplePorts.PersonRepository, identity staffIdentityQuery, membership schoolmembership.Capability, rooms facilitiesModule.Query, workTime workforce.Capability) MembershipTestRepositories {
	staff, teacher, guest, _, _, _ := newMembershipAdapters(db, persons, identity, membership, rooms, workTime)
	return MembershipTestRepositories{Person: persons, Staff: staff, Teacher: teacher, Guest: guest}
}

// SchoolStructureSuiteRepositories are the School Structure group store and
// the teaching assignments over School Membership the retained graphs read
// through.
type SchoolStructureSuiteRepositories struct {
	Group        EducationGroupRows
	GroupTeacher GroupTeacherRepository
	ClassTeacher ClassTeacherRepository
}

// NewSchoolStructureRepositories composes the group store and the teaching
// assignments over membership, as NewMembershipRepositories composes the
// staff adapters beside them.
func NewSchoolStructureRepositories(db transactionruntime.Database, persons peoplePorts.PersonRepository, identity staffIdentityQuery, membership schoolmembership.Capability, rooms facilitiesModule.Query, workTime workforce.Capability) *SchoolStructureSuiteRepositories {
	_, _, _, group, groupTeacher, classTeacher := newMembershipAdapters(db, persons, identity, membership, rooms, workTime)
	return &SchoolStructureSuiteRepositories{Group: group, GroupTeacher: groupTeacher, ClassTeacher: classTeacher}
}

// NewSchoolStructureSuiteRepositories composes the group store and the
// teaching assignments over the unobserved owners of db.
func NewSchoolStructureSuiteRepositories(db transactionruntime.Database) *SchoolStructureSuiteRepositories {
	membership := MustNewSchoolMembership(db)
	return NewSchoolStructureRepositories(db, NewPersonRepository(db), NewPeopleIdentityAccess(db, nil), membership, MustNewUnobservedRooms(db), MustNewWorkforce(db))
}
