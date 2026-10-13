package services

import (
	"context"
	"fmt"
	"log/slog"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	membershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	offboardingcompose "github.com/moto-nrw/project-phoenix/workflows/staffoffboarding/compose"
	"github.com/uptrace/bun"
)

// The People Directory, School Membership and Workforce glue of this package
// takes built owners. The roots below build those owners from the database
// for the serving root, the CLI roots and the test graphs. Each composes an
// owner of another module than the one it serves, so no single owner
// composition can host it; they leave with the composition root (#2747).

// NewPeopleDirectory composes the person and student owner for test graphs
// and CLI roots. The serve root composes the observed instance through
// NewPeopleDirectoryWithPhotosAndObserver.
func NewPeopleDirectory(db *bun.DB) (peopledirectory.Capability, error) {
	capability, _, err := NewPeopleDirectoryWithPhotos(db)
	return capability, err
}

// MustNewPeopleDirectory is NewPeopleDirectory for composition seams that
// cannot return an error; composing only fails on missing dependencies, which
// is a programming error at wiring time.
func MustNewPeopleDirectory(db *bun.DB) peopledirectory.Capability {
	capability, err := NewPeopleDirectory(db)
	if err != nil {
		panic(fmt.Sprintf("compose people directory: %v", err))
	}
	return capability
}

// NewPeopleDirectoryWithPhotos additionally hands back the slot the photo
// lifecycle resolves its runtime from, so a caller that has the feature gate,
// the caller access and the file cleanup fills it after the directory exists.
func NewPeopleDirectoryWithPhotos(db *bun.DB) (peopledirectory.Capability, *peopleCompose.StudentPhotoRuntime, error) {
	module, photoRuntime, err := NewPeopleDirectoryWithPhotosAndObserver(db, func(peopleCompose.Observation) {})
	return module, photoRuntime, err
}

// NewPeopleDirectoryWithPhotosAndObserver composes the owner for a graph that
// records People Directory observations. The Identity & Access membership
// query, the School Membership and Care Plan write owners and the Audit seams
// are built here over unobserved owners.
func NewPeopleDirectoryWithPhotosAndObserver(
	db *bun.DB,
	observe func(peopleCompose.Observation),
) (*peopledirectory.Module, *peopleCompose.StudentPhotoRuntime, error) {
	// Care Plan's companion slice is constructible from the database alone, so
	// the directory's write path gets it here rather than waiting for the full
	// Care Plan — which needs this directory and therefore cannot be built yet.
	companions, err := careplanCompose.NewCompanionRecords(db, func(careplanCompose.Observation) {})
	if err != nil {
		return nil, nil, err
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return nil, nil, err
	}
	careProfiles, err := careplanCompose.NewStudentProfiles(db, func(careplanCompose.Observation) {})
	if err != nil {
		return nil, nil, err
	}
	return ComposePeopleDirectory(peopleCompose.Dependencies{
		StudentClassWriteGateQuery: func(ctx context.Context) (*bun.SelectQuery, error) {
			return membershipCompose.StudentClassWriteGateQuery(ctx, db)
		},
		DB:      db,
		Observe: observe,
	}, PeopleDirectoryOwners{
		Membership: membership, CareProfiles: careProfiles, Companions: companions,
		Audit:               peopleDirectoryAudit(db),
		GuardianLinks:       newGuardianLinkOwners(db),
		GuardianMemberships: newPeopleIdentityAccess(db, nil).FindActiveGuardianMemberships,
	})
}

// newPeopleIdentityAccess composes the Identity & Access module the People
// Directory repositories read accounts and memberships through. A nil
// observer composes it unobserved, which is what test graphs and CLI roots
// use.
func newPeopleIdentityAccess(db *bun.DB, observe IdentityAccessObserver) *identityaccess.Module {
	module, err := identityCompose.New(identityCompose.Dependencies{DB: db, Observe: identityAccessObservation(observe)})
	if err != nil {
		panic(fmt.Sprintf("compose identity access: %v", err))
	}
	return module
}

// peopleDirectoryAudit builds the Audit Platform persistence behind the People
// Directory's audit seams (note deletion audit, field audit, consent history)
// (ADR 0049). The repositories read and append through the Audit handoff on
// the context, falling back to db.
func peopleDirectoryAudit(db *bun.DB) peopleDirectoryAuditRepositories {
	repositories := auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db})
	return peopleDirectoryAuditRepositories{
		DataDeletion: repositories.DataDeletion, StudentFieldEdit: repositories.StudentFieldEdit,
		StudentConsentChange: repositories.StudentConsentChange,
	}
}

// newGuardianRelationshipOwners composes the Care Plan and Identity & Access
// commands behind the relationship's unit of work. Both halves are
// constructible from the database alone, so every graph that builds the
// relationship store also gets its owners.
func newGuardianRelationshipOwners(db *bun.DB, observeIdentity IdentityAccessObserver) (guardianPickupPort, guardianAccessPort) {
	pickup, err := careplanCompose.NewGuardianPickupPermissions(db, func(careplanCompose.Observation) {})
	if err != nil {
		panic(fmt.Sprintf("compose guardian pickup permissions: %v", err))
	}
	access, err := identityCompose.NewGuardianStudentAccess(db, identityAccessObservation(observeIdentity))
	if err != nil {
		panic(fmt.Sprintf("compose guardian student access: %v", err))
	}
	return newGuardianRelationshipPorts(pickup, access)
}

func newGuardianLinkOwners(db *bun.DB) guardianLinkOwners {
	pickup, access := newGuardianRelationshipOwners(db, nil)
	return guardianLinkOwners{guardianPickupPort: pickup, guardianAccessPort: access}
}

// NewPersonRepository is the person repository with the Identity & Access
// account lookup FindWithAccount attaches (#2720).
func NewPersonRepository(db *bun.DB) userModels.PersonRepository {
	return peopleCompose.NewPersonRepository(db, accountLookup(newPeopleIdentityAccess(db, nil)))
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
	return BindStudentRepository(peopleCompose.StudentRepositoryDependencies{
		DB: db, Directory: MustNewPeopleDirectory(db), Enrollment: enrollment,
	}, membership, groups)
}

// NewGuardianSeedRepositories composes the guardian rows over unobserved
// owners, for the seed command.
func NewGuardianSeedRepositories(db *bun.DB) GuardianSeedRepositories {
	return GuardianSeedRepositories{GuardianProfile: NewGuardianProfileRepository(db), StudentGuardian: NewStudentGuardianRepository(db)}
}

// newMembershipRepositories composes the staff, teacher and guest adapters
// over membership, with the group store and the teaching assignments they
// resolve through. members.People, Person, Student and the guardian fields
// stay unset; the caller composes them.
func newMembershipRepositories(db *bun.DB, persons userModels.PersonRepository, identity staffIdentityQuery, membership schoolmembership.Capability, rooms facilitiesModule.Query, workTime workforce.Capability) MembershipTestRepositories {
	return bindMembershipRepositories(persons, identity, membership,
		NewGroupRepository(educationRepo.NewLegacyRepositoryRuntime(db), rooms, membership, workTime),
		workforceCompose.MustNewStaffEmployment(db))
}

// NewStaffRepository is the retained staff repository over School
// Membership, for the roots that read staff rows outside the service graph
// (the Worker's reminder recipients). observe records the Identity & Access
// operations the account half reads through; nil composes it unobserved.
func NewStaffRepository(db *bun.DB, membership schoolmembership.Capability, rooms facilitiesModule.Query, workTime workforce.Capability, observe IdentityAccessObserver) userModels.StaffRepository {
	return newMembershipRepositories(db, NewPersonRepository(db), newPeopleIdentityAccess(db, observe), membership, rooms, workTime).Staff
}

// PresenceStudents reads Directory records and writes only Care Plan's live
// flags, over an unobserved Care Plan student profile owner.
func PresenceStudents(db *bun.DB, source presenceStudentSource) presenceservice.PresenceStudents {
	care, err := careplanCompose.NewStudentProfiles(db, func(careplanCompose.Observation) {})
	if err != nil {
		panic(err)
	}
	return BindPresenceStudents(source, care)
}

// NewEducationGroupRepository is the retained group store on the tenant
// runtime of the School Structure owner over db, with only the given
// dependencies bound.
func NewEducationGroupRepository(db *bun.DB, options ...educationRepo.GroupRepositoryDependencies) *educationGroupRepository {
	return newEducationGroupStore(educationRepo.NewLegacyRepositoryRuntime(db), options...)
}

// NewStaffMembershipRuntime composes the staff membership closures over the
// service factory, with the offboarding workflow over db and an unobserved
// Workforce employment owner.
func (f *Factory) NewStaffMembershipRuntime(db *bun.DB, logger *slog.Logger, hooks StaffMembershipHooks) StaffMembershipRuntime {
	return f.bindStaffMembershipRuntime(offboardingcompose.Dependencies{
		DB: db, Employment: MembershipStaffEmployment(workforceCompose.MustNewStaffEmployment(db)),
	}, logger, hooks)
}
