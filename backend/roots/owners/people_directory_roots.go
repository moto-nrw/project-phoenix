package owners

import (
	"fmt"

	guardianLinkView "github.com/moto-nrw/project-phoenix/modules/guardianlinkview/compose"

	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	membershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// GuardianRolePresets is the Security Runtime's role-preset policy as People
// Directory's preset port.
var GuardianRolePresets = peoplePorts.GuardianRolePresetFuncs{
	Default: securityruntime.DefaultStudentGuardianRole,
	Presets: securityruntime.StudentGuardianRolePreset,
}

// NewPeopleDirectory composes the person and student owner for test graphs
// and CLI roots, without guardian provider and photo runtime. The serve root
// composes the observed instance through buildPeopleDirectory.
func NewPeopleDirectory(db transactionruntime.Database) (*peopleCompose.Directory, error) {
	return NewPeopleDirectoryWithObserver(db, func(peopleCompose.Observation) {}, nil, nil)
}

// MustNewPeopleDirectory is NewPeopleDirectory for composition seams that
// cannot return an error; composing only fails on missing dependencies, which
// is a programming error at wiring time.
func MustNewPeopleDirectory(db transactionruntime.Database) *peopleCompose.Directory {
	capability, err := NewPeopleDirectory(db)
	if err != nil {
		panic(fmt.Sprintf("compose people directory: %v", err))
	}
	return capability
}

// NewPeopleDirectoryWithObserver composes the owner for a graph that records
// People Directory observations, over the graph's guardian provider and photo
// runtime (nil for none). The Identity & Access membership query, the School
// Membership and Care Plan write owners and the Audit seams are built here
// over unobserved owners.
func NewPeopleDirectoryWithObserver(
	db transactionruntime.Database, observe func(peopleCompose.Observation),
	guardians *peopleCompose.GuardianDirectoryProvider, photos peopleCompose.StudentPhotoRuntime,
) (*peopleCompose.Directory, error) {
	// Care Plan's companion slice is constructible from the database alone, so
	// the directory's write path gets it here rather than waiting for the full
	// Care Plan — which needs this directory and therefore cannot be built yet.
	companions, err := careplanCompose.NewCompanionRecords(db.Pool(), func(careplanCompose.Observation) {})
	if err != nil {
		return nil, err
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return nil, err
	}
	careProfiles, err := careplanCompose.NewStudentProfiles(db.Pool(), studentDirectoryView.NewReader(), func(careplanCompose.Observation) {})
	if err != nil {
		return nil, err
	}
	return ComposePeopleDirectory(peopleCompose.Dependencies{
		StudentClassWriteGateQuery: membershipCompose.StudentClassWriteGate(db.Pool()),
		DB:                         db.Pool(),
		Observe:                    observe,
		StudentPhotos:              photos,
		GuardianProvider:           guardians,
	}, membership, careProfiles, companions, peopleDirectoryAudit(db), newGuardianLinkOwners(db),
		NewPeopleIdentityAccess(db, nil).FindActiveGuardianMemberships)
}

// NewPeopleIdentityAccess composes the Identity & Access module the People
// Directory repositories read accounts and memberships through. A nil
// observer composes it unobserved, which is what test graphs and CLI roots
// use.
func NewPeopleIdentityAccess(db transactionruntime.Database, observe IdentityAccessObserver) *identityaccess.Module {
	module, err := identityCompose.New(identityCompose.Dependencies{DB: db.Pool(), Observe: identityAccessObservation(observe)})
	if err != nil {
		panic(fmt.Sprintf("compose identity access: %v", err))
	}
	return module
}

// peopleDirectoryAudit builds the Audit Platform persistence behind the People
// Directory's audit seams (note deletion audit, field audit, consent history)
// (ADR 0049). The repositories read and append through the Audit handoff on
// the context, falling back to db.
func peopleDirectoryAudit(db transactionruntime.Database) peopleDirectoryAuditTrails {
	repositories := auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db.Pool()})
	return peopleDirectoryAuditTrails{
		DataDeletion: repositories.DataDeletion, StudentFieldEdit: repositories.StudentFieldEdit,
		StudentConsentChange: repositories.StudentConsentChange,
	}
}

// newGuardianRelationshipOwners composes the Care Plan and Identity & Access
// commands behind the relationship's unit of work. Both halves are
// constructible from the database alone, so every graph that builds the
// relationship store also gets its owners.
func newGuardianRelationshipOwners(db transactionruntime.Database, observeIdentity IdentityAccessObserver) (guardianPickupPort, guardianAccessPort) {
	pickup, err := careplanCompose.NewGuardianPickupPermissions(db.Pool(), func(careplanCompose.Observation) {})
	if err != nil {
		panic(fmt.Sprintf("compose guardian pickup permissions: %v", err))
	}
	access, err := identityCompose.NewGuardianStudentAccess(db.Pool(), identityAccessObservation(observeIdentity))
	if err != nil {
		panic(fmt.Sprintf("compose guardian student access: %v", err))
	}
	return newGuardianRelationshipPorts(pickup, access)
}

func newGuardianLinkOwners(db transactionruntime.Database) guardianLinkOwners {
	pickup, access := newGuardianRelationshipOwners(db, nil)
	return guardianLinkOwners{guardianPickupPort: pickup, guardianAccessPort: access}
}

// NewGuardianProfileRepository is the guardian profile repository with the
// Identity & Access active-membership and login-ready-membership queries its
// portal reachability check joins (#2720, #2721).
func NewGuardianProfileRepository(db transactionruntime.Database) peoplePorts.GuardianProfileRepository {
	identity := NewPeopleIdentityAccess(db, nil)
	profiles := peopleCompose.NewGuardianProfileRepository(db.Pool(), studentDirectoryView.NewReader(), guardianLinkView.NewReader(),
		identity.FindActiveGuardianMemberships, identity.FindLoginReadyGuardianMemberships)
	return profiles
}

// NewStudentGuardianRepository is the relationship store over native Identity
// membership facts, unobserved, for graphs that do not compose the serving
// root.
func NewStudentGuardianRepository(db transactionruntime.Database) peoplePorts.StudentGuardianRepository {
	return newStudentGuardianRepository(db, NewPeopleIdentityAccess(db, nil), nil)
}

// newStudentGuardianRepository is the relationship store with its membership
// lookup, both owner halves and the default role preset bound.
func newStudentGuardianRepository(db transactionruntime.Database, memberships activeSchoolMemberships, observe IdentityAccessObserver) peoplePorts.StudentGuardianRepository {
	pickup, access := newGuardianRelationshipOwners(db, observe)
	relationships := peopleCompose.NewGuardianRelationshipRepository(db.Pool(), peopleCompose.GuardianRelationshipDependencies{
		Students: studentDirectoryView.NewReader(), Links: guardianLinkView.NewReader(),
		Memberships: memberships.FindActiveSchoolMemberships,
		Pickup:      pickup, Access: access, RolePresets: GuardianRolePresets,
	})
	return relationships
}

// NewStudentRepository composes the retained student repository over an
// unobserved People Directory and School Membership, the owners its reads and
// writes translate to (#3349). The teacher-scoped roster reads resolve their
// groups through the given membership; the roster and birthday reads name the
// groups through School Structure.
func NewStudentRepository(db transactionruntime.Database, membership groupAssignmentLister, groups schoolstructure.Query) peoplePorts.StudentRepository {
	enrollment, err := NewSchoolMembership(db)
	if err != nil {
		panic(fmt.Sprintf("student repository: compose membership: %v", err))
	}
	return BindStudentRepository(peopleCompose.StudentRepositoryDependencies{
		DB: db.Pool(), Directory: MustNewPeopleDirectory(db), Enrollment: enrollment,
	}, membership, groups)
}

// PresenceStudents reads Directory records and writes only Care Plan's live
// flags, over an unobserved Care Plan student profile owner.
func PresenceStudents(db transactionruntime.Database, source presenceStudentSource) presenceservice.PresenceStudents {
	care, err := careplanCompose.NewStudentProfiles(db.Pool(), studentDirectoryView.NewReader(), func(careplanCompose.Observation) {})
	if err != nil {
		panic(err)
	}
	return BindPresenceStudents(source, care)
}
