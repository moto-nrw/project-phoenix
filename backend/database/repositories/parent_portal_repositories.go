package repositories

import (
	"context"
	"time"

	parentRepo "github.com/moto-nrw/project-phoenix/database/repositories/parent"
	"github.com/moto-nrw/project-phoenix/database/repositories/users"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	staffStore "github.com/moto-nrw/project-phoenix/modules/communication/staffstore"
	enrollmentCapability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/uptrace/bun"
)

// ParentPortalOwners are the owners the retained guardian-portal and
// messaging repositories read through. Every one is bound at construction
// (#2743).
type ParentPortalOwners struct {
	// People answers the student, guardian and person reads that used to
	// join users.students, users.guardian_profiles and users.persons.
	People peopledirectory.Capability
	// Groups names the groups of the inbox threads.
	Groups schoolstructure.Query
	// Membership resolves the live staff accounts behind the read cursors.
	Membership schoolmembership.Capability
	// Persons is the person repository those staff accounts are loaded from.
	Persons userModels.PersonRepository
	// Identity owns the accounts, their school memberships and role classes.
	Identity *identityaccess.Module
	// Enrollment owns the phases, applications and announcement applicants.
	Enrollment *enrollmentCapability.Module
	// Now is the announcement clock; nil keeps the store default.
	Now func() time.Time
}

// ParentPortalRepositories are the retained guardian-portal and messaging
// repositories over their owners.
type ParentPortalRepositories struct {
	ParentChild             parentModels.ChildRepository
	ParentEnrollablePhase   parentModels.EnrollablePhaseRepository
	ParentEnrollmentRequest parentModels.EnrollmentRequestRepository
	ParentAnnouncement      userModels.ParentAnnouncementRepository
	ParentMessageThread     userModels.ParentMessageThreadRepository
	ParentMessageRead       userModels.ParentMessageReadRepository
	StaffMessageRead        userModels.StaffMessageReadRepository
}

// NewParentPortalRepositories builds the retained guardian-portal and
// messaging repositories with the People Directory, School Membership,
// School Structure, Identity & Access and Enrollment reads bound. The school
// names are bound by WithSchools.
func NewParentPortalRepositories(db *bun.DB, owners ParentPortalOwners) ParentPortalRepositories {
	identity := owners.Identity
	phases := parentRepo.NewEnrollablePhaseRepository(owners.Enrollment, identity.ListActiveAccountSchoolIDs)
	requests := parentRepo.NewEnrollmentRequestRepository(parentRepo.RuntimeFunc(carePlanCompose.TenantAmbientDatabase(db)), owners.Enrollment, identityAccountDirectory{accounts: identity})
	if repo, ok := phases.(*parentRepo.EnrollablePhaseRepository); ok {
		repo.BindStudentDirectory(parentStudentDirectory{owners.People})
		repo.BindGuardianDirectory(parentGuardianDirectory{owners.People})
	}
	if repo, ok := requests.(*parentRepo.EnrollmentRequestRepository); ok {
		repo.BindGuardianDirectory(parentGuardianDirectory{owners.People})
	}
	membership := func() schoolmembership.Capability { return owners.Membership }
	persons := func() userModels.PersonRepository { return owners.Persons }
	return ParentPortalRepositories{
		ParentChild:             NewParentChildRepository(identity.ListActiveAccountSchoolIDs, owners.People),
		ParentEnrollablePhase:   personEnrollablePhaseRepository{EnrollablePhaseRepository: phases, persons: owners.People},
		ParentEnrollmentRequest: requests,
		ParentAnnouncement:      NewParentAnnouncementRepository(db, owners.Enrollment, owners.Now),
		ParentMessageThread:     parentStore.NewParentMessageThreadRepository(db, users.NewMessageableGuardianRepository(peopleRuntime(db), identity.FindActiveSchoolMemberships)),
		ParentMessageRead: groupParentMessageReadRepository{
			ParentMessageReadRepository: parentMessageStaffRepository{
				ParentMessageReads: parentStore.NewParentMessageReadRepository(db), membership: membership, persons: persons,
			},
			groups: owners.Groups,
		},
		StaffMessageRead: staffStore.NewStaffMessageReadRepository(db, users.NewMessageableStaffRepository(peopleRuntime(db), func(ctx context.Context) ([]int64, error) {
			return currentTenantStaffAccounts(ctx, membership(), persons())
		}, staffMessageIdentity(identity))),
	}
}

// NewParentChildRepository lists a guardian account's children across its
// active schools, with the students, guardian links and person names read
// through the People Directory. The school names are bound by WithSchools.
func NewParentChildRepository(schools parentRepo.ActiveSchoolMemberships, people peopledirectory.Capability) parentModels.ChildRepository {
	child := parentRepo.NewChildRepository(schools)
	if repo, ok := child.(*parentRepo.ChildRepository); ok {
		repo.BindStudentDirectory(parentStudentDirectory{people})
		repo.BindGuardianDirectory(parentGuardianDirectory{people})
	}
	return personChildRepository{ChildRepository: child, persons: people}
}

// WithSchools returns the repositories with the school names and the school
// eligibility resolved through the Organisation & Tenancy owner.
func (r ParentPortalRepositories) WithSchools(schools organizationtenancy.Query) ParentPortalRepositories {
	if schools == nil {
		panic("parent portal repositories: organization tenancy query is required")
	}
	return ParentPortalRepositories{
		ParentChild:             schoolChildRepository{ChildRepository: r.ParentChild, schools: schools},
		ParentEnrollablePhase:   schoolEnrollablePhaseRepository{EnrollablePhaseRepository: r.ParentEnrollablePhase, schools: schools},
		ParentEnrollmentRequest: schoolEnrollmentRequestRepository{EnrollmentRequestRepository: r.ParentEnrollmentRequest, schools: schools},
		ParentAnnouncement:      schoolParentAnnouncementRepository{ParentAnnouncementRepository: r.ParentAnnouncement, schools: schools},
		ParentMessageThread:     r.ParentMessageThread,
		ParentMessageRead:       schoolParentMessageReadRepository{ParentMessageReadRepository: r.ParentMessageRead, schools: schools},
		StaffMessageRead:        r.StaffMessageRead,
	}
}
