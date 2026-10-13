package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/uptrace/bun"
)

// The retained People Directory repositories are built where they are used
// (#2743): modules/peopledirectory/compose constructs them over the owner's
// legacy runtime, and this composition binds what other owners hold — the
// accounts and memberships of Identity & Access, the enrollment and teacher
// assignments of School Membership, the group names of School Structure.

// NewPersonRepository is the person repository with the Identity & Access
// account lookup FindWithAccount attaches (#2720).
func NewPersonRepository(db *bun.DB) userModels.PersonRepository {
	return peopleCompose.NewPersonRepository(db, accountLookup(newPeopleIdentityAccess(db, nil)))
}

// accountLookup adapts the owner's by-id read to the People Directory
// lookup: a missing account resolves to nil, as the former LEFT JOIN did.
func accountLookup(accounts identityaccess.AccountProfiles) peopleCompose.AccountLookup {
	return func(ctx context.Context, accountID int64) (*userModels.PersonAccount, error) {
		account, err := accounts.FindAccountMetadata(ctx, accountID)
		if errors.Is(err, identityaccess.ErrAccountNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		result := userModels.PersonAccount(account)
		return &result, nil
	}
}

// NewGuardianProfileRepository is the guardian profile repository with the
// Identity & Access active-membership and login-ready-membership queries its
// portal reachability check joins (#2720, #2721).
func NewGuardianProfileRepository(db *bun.DB) userModels.GuardianProfileRepository {
	identity := newPeopleIdentityAccess(db, nil)
	return peopleCompose.NewGuardianProfileRepository(db,
		identity.FindActiveGuardianMemberships, identity.FindLoginReadyGuardianMemberships)
}

// NewStudentGuardianRepository is the relationship store over native Identity
// membership facts, unobserved, for graphs that do not compose the serving
// root.
func NewStudentGuardianRepository(db *bun.DB) userModels.StudentGuardianRepository {
	return newStudentGuardianRepository(db, newPeopleIdentityAccess(db, nil), nil)
}

// newStudentGuardianRepository is the relationship store with its membership
// lookup, both owner halves and the default role bound.
func newStudentGuardianRepository(db *bun.DB, memberships activeSchoolMemberships, observe IdentityAccessObserver) userModels.StudentGuardianRepository {
	pickup, access := newGuardianRelationshipOwners(db, observe)
	return peopleCompose.NewGuardianRelationshipRepository(db, peopleCompose.GuardianRelationshipDependencies{
		Memberships: memberships.FindActiveSchoolMemberships,
		Pickup:      pickup, Access: access, DefaultRole: applyDefaultGuardianRole,
	})
}

// applyDefaultGuardianRole gives a link without a role or permissions the
// authorization policy's default preset and the permissions it grants.
func applyDefaultGuardianRole(relationship *userModels.StudentGuardian) {
	relationshipType, _, primary, emergency, pickup, _ := relationship.GuardianAuthorizationData()
	role, granted := securityruntime.StudentGuardianRolePreset(
		securityruntime.DefaultStudentGuardianRole(relationshipType, primary, emergency, pickup))
	permissions := make(map[string]interface{}, len(granted))
	for _, permission := range granted {
		permissions[permission] = true
	}
	relationship.SetGuardianAuthorizationData(role, permissions)
}

// NewStudentRepository composes the retained student repository over an
// unobserved People Directory and School Membership, the owners its reads and
// writes translate to (#3349). The teacher-scoped roster reads resolve their
// groups through the given membership; the roster and birthday reads name the
// groups through School Structure.
func NewStudentRepository(db *bun.DB, membership groupAssignmentLister, groups schoolstructure.Query) userModels.StudentRepository {
	enrollment, err := NewSchoolMembership(db)
	if err != nil {
		panic(fmt.Sprintf("student repository: compose membership: %v", err))
	}
	repository := peopleCompose.NewStudentRepository(peopleCompose.StudentRepositoryDependencies{
		DB: db, Directory: MustNewPeopleDirectory(db), Enrollment: enrollment,
		TeacherGroupIDs:      teacherGroupIDs(membership),
		TeacherStaffGroupIDs: teacherStaffGroupIDs(membership),
	})
	return overlappingRosterGroupNames{StudentRepository: groupStudentRepository{StudentRepository: repository, groups: groups}, groups: groups}
}

// groupAssignmentLister is the School Membership teacher-assignment read the
// student roster resolves teacher groups through.
type groupAssignmentLister interface {
	ListGroupAssignments(context.Context, schoolmembership.GroupAssignmentFilter) ([]schoolmembership.GroupAssignment, error)
}

// activeSchoolMemberships is the Identity & Access membership read the
// relationship store's permission checks filter through.
type activeSchoolMemberships interface {
	FindActiveSchoolMemberships(context.Context, []int64, []int64) (map[int64][]int64, error)
}

func teacherGroupIDs(membership groupAssignmentLister) func(context.Context, int64) ([]int64, error) {
	return func(ctx context.Context, teacherID int64) ([]int64, error) {
		assignments, err := membership.ListGroupAssignments(ctx, schoolmembership.GroupAssignmentFilter{TeacherIDs: []int64{teacherID}})
		if err != nil {
			return nil, err
		}
		ids := make([]int64, 0, len(assignments))
		for _, assignment := range assignments {
			ids = append(ids, assignment.GroupID)
		}
		return ids, nil
	}
}

func teacherStaffGroupIDs(membership groupAssignmentLister) func(context.Context, []int64) ([]int64, error) {
	return func(ctx context.Context, staffIDs []int64) ([]int64, error) {
		assignments, err := membership.ListGroupAssignments(ctx, schoolmembership.GroupAssignmentFilter{TeacherStaffIDs: staffIDs})
		if err != nil {
			return nil, err
		}
		ids := make([]int64, 0, len(assignments))
		for _, assignment := range assignments {
			ids = append(ids, assignment.GroupID)
		}
		return ids, nil
	}
}

// NewCaregiverBindingLocker serializes the caregiver capability re-check over
// the four owners whose tables can retain caregiver capability: School
// Membership, Workforce, Student Presence and Timetable & Activities.
func NewCaregiverBindingLocker(membership schoolmembership.Capability, workTime workforce.Capability, presence studentpresence.Capability, timetableCapability timetable.Capability) userModels.CaregiverBindingLocker {
	return peopleCompose.NewCaregiverBindingLocker(caregiverBindingOwners{
		membership: membership, workforce: workTime, presence: presence, timetable: timetableCapability,
	})
}

// caregiverBindingOwners adapts the four binding owners to the People
// Directory port that serializes the caregiver capability re-check.
type caregiverBindingOwners struct {
	membership schoolmembership.Capability
	workforce  workforce.Capability
	presence   studentpresence.Capability
	timetable  timetable.Capability
}

func (o caregiverBindingOwners) LockGroupAssignments(ctx context.Context) error {
	return o.membership.LockGroupAssignments(ctx)
}

func (o caregiverBindingOwners) LockGroupSubstitutions(ctx context.Context) error {
	return o.workforce.LockGroupSubstitutions(ctx)
}

func (o caregiverBindingOwners) LockGroupSupervisions(ctx context.Context) error {
	return o.presence.LockGroupSupervisions(ctx)
}

func (o caregiverBindingOwners) LockPlannedSupervisors(ctx context.Context) error {
	return o.timetable.LockPlannedSupervisors(ctx)
}

// GuardianSeedRepositories are the guardian rows the parent-portal seed
// command promotes to portal accounts.
type GuardianSeedRepositories struct {
	GuardianProfile userModels.GuardianProfileRepository
	StudentGuardian userModels.StudentGuardianRepository
}

// NewGuardianSeedRepositories composes the guardian rows over unobserved
// owners, for the seed command.
func NewGuardianSeedRepositories(db *bun.DB) GuardianSeedRepositories {
	return GuardianSeedRepositories{GuardianProfile: NewGuardianProfileRepository(db), StudentGuardian: NewStudentGuardianRepository(db)}
}
