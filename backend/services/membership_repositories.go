package services

import (
	"context"

	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceLegacy "github.com/moto-nrw/project-phoenix/modules/workforce/legacy"
)

// The retained staff, teacher, guest, teaching-assignment and group
// repositories are served over School Membership and School Structure
// (#2667, #2742) and built where they are used (#2743). The person half of a
// staff row comes from the People Directory repository, the account, role
// and permission half from Identity & Access, the employment-only writes from
// Workforce.

// MembershipTestRepositories are the People Directory and School Membership
// repositories the retained graphs read through. services composes them where
// it composes the owners; the test graphs build them through
// api.NewMembershipTestRepositories (#2743, #4005).
type MembershipTestRepositories struct {
	// People is the People Directory owner the student rows translate to.
	People          peopledirectory.Capability
	Person          userModels.PersonRepository
	Student         userModels.StudentRepository
	GuardianProfile userModels.GuardianProfileRepository
	StudentGuardian userModels.StudentGuardianRepository
	Staff           userModels.StaffRepository
	Teacher         userModels.TeacherRepository
	Guest           userModels.GuestRepository
	Group           EducationGroupRepository
	GroupTeacher    GroupTeacherRepository
	ClassTeacher    ClassTeacherRepository
	// GroupSubstitution is the Workforce group substitution contract over the
	// group store and School Membership.
	GroupSubstitution GroupSubstitutionRepository
	// Membership is the owner capability itself; the class-list entries
	// (#2382) are read and written through it.
	Membership schoolmembership.Capability
}

// bindMembershipRepositories binds the staff, teacher and guest adapters over
// membership, with the group store and the teaching assignments they resolve
// through. members.People, Person, Student and the guardian fields stay
// unset; the caller composes them.
func bindMembershipRepositories(persons userModels.PersonRepository, identity staffIdentityQuery, membership schoolmembership.Capability, group *EducationGroupStore, employment workforce.StaffEmployments) MembershipTestRepositories {
	groupTeachers := NewGroupTeacherRepository(membership, group)
	deps := NewStaffMembershipDeps(persons, identity, employment)
	deps.GroupTeachers = func() GroupTeacherRepository { return groupTeachers }
	return MembershipTestRepositories{
		Person:       persons,
		Staff:        StaffMembershipRepository{Membership: membership, Deps: deps},
		Teacher:      TeacherMembershipRepository{Membership: membership, Deps: deps},
		Guest:        guestMembershipRepository{membership: membership},
		Group:        group,
		GroupTeacher: groupTeachers,
		ClassTeacher: newClassTeacherRepository(membership),
		Membership:   membership,
	}
}

// NewGroupSubstitutionRepository serves the retained group substitution
// contract over Workforce (#2688): the rows are Workforce's, the groups
// resolve through the group store and the staff through School Membership.
func NewGroupSubstitutionRepository(groups EducationGroupRepository, membership staffLookup, workTime workforce.Capability) GroupSubstitutionRepository {
	return newGroupSubstitutions(
		workforceLegacy.NewGroupSubstitutionRepository(workTime, func(ctx context.Context, ids []int64) (map[int64]string, error) {
			return NewGroupNames(groups).GroupNamesByID(ctx, ids)
		}),
		groups.FindByIDs,
		membership,
	)
}

// groupSubstitutions serves the retained group substitution contract over
// the Workforce rows. The reads with relations attach the group through
// School Structure and the staff members through School Membership; the
// Workforce adapter names neither (#3556).
type groupSubstitutions struct {
	GroupSubstitutionRows
	groups     func(ctx context.Context, ids []int64) (map[int64]*educationRepo.Group, error)
	membership staffLookup
}

func newGroupSubstitutions(rows GroupSubstitutionRows, groups func(context.Context, []int64) (map[int64]*educationRepo.Group, error), membership staffLookup) groupSubstitutions {
	return groupSubstitutions{GroupSubstitutionRows: rows, groups: groups, membership: membership}
}

// FindGroupHandovers lists the substitutions of one group as School
// Structure values, the group service's deletion guard.
func (r groupSubstitutions) FindGroupHandovers(ctx context.Context, groupID int64) ([]*educationRepo.GroupSubstitution, error) {
	return educationHandovers(r.FindByGroup(ctx, groupID))
}

// ListWithRelations resolves the groups and then the staff members. Soft-
// deleted staff are included so historical substitutions keep resolving
// after offboarding, as the replaced WhereAllWithDeleted lookup did.
func (r groupSubstitutions) ListWithRelations(ctx context.Context, options *userModels.QueryOptions) ([]*educationRepo.GroupSubstitution, error) {
	rows, err := educationHandovers(r.ListWithOptions(ctx, options))
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	groupIDs := make([]int64, 0, len(rows))
	staffIDs := make([]int64, 0, 2*len(rows))
	for _, row := range rows {
		if row.GroupID > 0 {
			groupIDs = append(groupIDs, row.GroupID)
		}
		staffIDs = append(staffIDs, row.SubstituteStaffID)
		if row.RegularStaffID != nil {
			staffIDs = append(staffIDs, *row.RegularStaffID)
		}
	}
	if groupIDs = uniqueIDs(groupIDs); len(groupIDs) > 0 {
		groups, err := r.groups(ctx, groupIDs)
		if err != nil {
			return nil, &userModels.DatabaseError{Op: "load substitution groups", Err: err}
		}
		for _, row := range rows {
			row.Group = groups[row.GroupID]
		}
	}
	members, err := staffByID(ctx, r.membership, staffIDs, true)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if member, found := members[row.SubstituteStaffID]; found {
			row.SubstituteStaff = substitutionStaff(member)
		}
		if row.RegularStaffID == nil {
			continue
		}
		if member, found := members[*row.RegularStaffID]; found {
			row.RegularStaff = substitutionStaff(member)
		}
	}
	return rows, nil
}

// substitutionStaff is the staff reference a substitution row carries; the
// name follows from the People Directory where a reader needs it.
func substitutionStaff(value schoolmembership.Staff) *educationRepo.SubstitutionStaff {
	return &educationRepo.SubstitutionStaff{ID: value.ID, PersonID: value.PersonID}
}
