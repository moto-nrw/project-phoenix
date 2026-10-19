package owners

import (
	"context"
	"fmt"

	realtime "github.com/moto-nrw/project-phoenix/realtime"

	fileStorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

// PeopleServices are the People Directory services the roots compose over
// the owner and the Identity & Access module: the person directory with Care
// Plan's participation decision, the guardian service, the caregiver
// capability, the change history, the consents, the photo lifecycle and the
// change-history retention.
type PeopleServices struct {
	Users               peoplePorts.PersonDirectory
	Guardian            peoplePorts.GuardianService
	CaregiverCapability peoplePorts.CaregiverCapabilityService
	StudentAudit        peoplePorts.StudentAuditService
	// StudentConsents resolves and records a child's consent states over the
	// People Directory and the Audit Platform trail.
	StudentConsents StudentConsentRecords
	// StudentPhotos is the photo lifecycle (#4007).
	StudentPhotos peoplePorts.StudentPhotoService
	// StudentChangeLogCleanup is the retention sweep of the per-child change
	// history (#1455).
	StudentChangeLogCleanup peoplePorts.StudentChangeLogCleanupService
}

// buildGuardianService binds the People Directory's guardian service over
// the guardian rows and Identity & Access's invitation reads, which need
// only the database: the People Directory module serves it as its guardian
// provider (#2663), and Identity & Access is composed over the module.
func BuildGuardianService(db transactionruntime.Database, people PeopleRows, audit AuditOwner) (peoplePorts.GuardianService, error) {
	invitations, err := identityaccessCompose.NewGuardianInvitationReads(db.Pool())
	if err != nil {
		return nil, err
	}
	return peopleCompose.NewGuardianService(peoplePorts.GuardianServiceDependencies{
		GuardianProfileRepo: people.GuardianProfiles, GuardianPhoneNumberRepo: peopleCompose.NewGuardianPhoneNumberRepository(db.Pool()),
		StudentGuardianRepo: people.StudentGuardians, GuardianInvitations: NewGuardianInvitationReads(invitations),
		StudentRepo: people.Students, PersonRepo: people.Persons,
		GuardianFinancialRepo:  peopleCompose.NewGuardianFinancialDataRepository(db.Pool()),
		GuardianFinancialAudit: NewGuardianFinancialAudit(audit.Persistence.GuardianFinancialChange),
		DataAccessLog:          NewGuardianDataAccessLog(audit.Persistence.DataAccessLog),
		RolePresets: peoplePorts.GuardianRolePresetFuncs{
			Default: securityruntime.DefaultStudentGuardianRole, Presets: securityruntime.StudentGuardianRolePreset,
		}}, db.Pool()), nil
}

// guardianDirectoryProvider is the guardian provider of the People
// Directory module over the guardian service.
func GuardianDirectoryProvider(db transactionruntime.Database, guardian peoplePorts.GuardianService) *peopleCompose.GuardianDirectoryProvider {
	return &peopleCompose.GuardianDirectoryProvider{
		Guardians: guardian, Runtime: peopleCompose.NewLegacyRepositoryRuntime(db.Pool()),
		PermissionGranted: securityruntime.GuardianPermissionGranted,
	}
}

// buildPeopleServices binds the person directory, the caregiver capability,
// the change history, the consents, the photo lifecycle and the
// change-history retention. The staff and teacher lookups and the staff
// writes are School Membership and Workforce territory (#3752); the person
// service takes them through the StaffDirectory port. Care Plan's lifecycle
// is composed first, so the directory receives its participation decision
// at construction.
func BuildPeopleServices(
	db transactionruntime.Database, env RootEnvironment, persons peopledirectory.Capability, people PeopleRows, peopleIdentity *identityaccess.Module,
	identity *identityaccess.Module, participation careplan.CareParticipation, audit AuditOwner, tokens TokenAdapter,
	guardian peoplePorts.GuardianService, studentAudit peoplePorts.StudentAuditService, photos StudentPhotoRuntime,
	caregiverLock peoplePorts.CaregiverBindingLocker, groupSupervisions studentpresence.SupervisionRecords,
	groupSubstitutions GroupSubstitutions, structure StructureRows, plannedSupervisors peoplePorts.PlannedSupervisionBlockers,
	settings settingsCompose.SettingsService,
) PeopleServices {
	staffDirectory := peopleCompose.NewStaffDirectory(peoplePorts.StaffDirectoryDependencies{
		Runtime: peopleCompose.NewLegacyRepositoryRuntime(db.Pool()), Persons: people.Persons, Staff: people.Staff,
		Teachers: people.Teachers, LehrkraftRoles: identity, MayManageStaff: staffAdministrationPermitted,
	})
	users := peopleCompose.NewPersonDirectory(peoplePorts.PersonDirectoryDependencies{
		PersonDirectory:  peopleCompose.NewPersonWrites(persons),
		StudentDirectory: peopleCompose.NewStudentDirectory(persons),
		PersonRepo:       people.Persons, RFIDRepo: peopleIdentity,
		AccountExists: identityaccessCompose.AccountExists(peopleIdentity),
		StudentRepo:   people.Students, TeacherRepo: people.Teachers, StaffDirectory: staffDirectory,
		CareParticipation: CareParticipationOf(participation),
	})
	caregiver := peopleCompose.NewCaregiverCapabilityService(peoplePorts.CaregiverCapabilityServiceDependencies{
		RequestActorScope: tokens.actorScope, Identity: caregiverIdentity{identity},
		Audit: newCaregiverCapabilityAudit(audit.Persistence.AuthEvent), PersonRepo: people.Persons, StaffRepo: people.Staff,
		CaregiverBindingLock: caregiverLock, TeacherRepo: people.Teachers, GroupTeacherRepo: structure.GroupTeachers,
		GroupSubstitutionRepo:  groupSubstitutions,
		GroupSupervisorRepo:    CaregiverSupervisionBlockers{Records: groupSupervisions},
		ActivitySupervisorRepo: plannedSupervisors,
		RoleAssignments:        identity,
	})
	return PeopleServices{
		Users: users, Guardian: guardian, CaregiverCapability: caregiver, StudentAudit: studentAudit,
		StudentConsents: NewStudentConsentsFor(persons, audit.Persistence.StudentConsentChange),
		StudentPhotos:   StudentPhotos{Directory: persons, Runtime: photos},
		StudentChangeLogCleanup: NewStudentChangeLogCleanup(audit.Persistence.StudentFieldEdit, audit.Persistence.DataDeletion,
			settings, env.Logger.With("service", "student-change-log-cleanup")),
	}
}

// buildWorkforceStaffAdmin builds Workforce's personnel record (#1417, #1423,
// #1424): the payroll number, the Stammdaten sections and the Dokumente tab;
// the staff and person rows and the trail are bound here.
func BuildWorkforceStaffAdmin(db transactionruntime.Database, env RootEnvironment, persons peopledirectory.Capability, membership schoolmembership.Capability, people PeopleRows, audit AuditOwner) (*workforceModule.StaffAdmin, error) {
	admin, err := ComposeStaffAdmin(workforceCompose.StaffAdminDependencies{
		DB: db.Pool(), Logger: env.Logger.With("service", "staff_documents"), Now: env.Now,
	}, people.Staff, people.Persons, persons, membership,
		audit.Persistence.StaffMasterDataChange, audit.Persistence.PersonnelNumberChange, audit.Persistence.DataAccessLog)
	if err != nil {
		return nil, fmt.Errorf("staff admin: %w", err)
	}
	return admin, nil
}

// buildStudentPhotoRuntime binds the five surfaces the People Directory photo
// lifecycle needs beyond its own rows (#3349): the tenant feature flag, the
// acting account, the stored-file cleanup with the live refresh and the
// Audit Platform consent trail. None of them needs the directory, so the
// module receives the runtime at construction.
func BuildStudentPhotoRuntime(db transactionruntime.Database, env RootEnvironment, settings settingsCompose.SettingsService, hub *realtime.Hub, audit AuditOwner, unlinker peoplePorts.PhotoUnlinker, actor func(context.Context) int64) StudentPhotoRuntime {
	return StudentPhotoRuntime{
		Cleanup:  fileStorageCompose.NewUploadCleanup(db.Pool(), "student-photos").Queue,
		Settings: settings, Broadcaster: hub, Unlinker: unlinker,
		Consents: studentConsentTrail{trail: audit.Persistence.StudentConsentChange},
		Logger:   env.Logger.With("service", "student-photo"), Actor: actor,
	}
}
