package services

import (
	"github.com/moto-nrw/project-phoenix/database/repositories"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	auditService "github.com/moto-nrw/project-phoenix/services/audit"
	importService "github.com/moto-nrw/project-phoenix/services/import"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type ImportTestModule struct {
	Import          *importService.ImportService[importModels.StudentImportRow]
	StaffImport     *importService.ImportService[importModels.StaffImportRow]
	ClassListImport *importService.ImportService[importModels.ClassListEntryImportRow]
	Users           users.PersonService
}

func NewImportTestModule(db *bun.DB, unit tenant.UnitOfWork) (ImportTestModule, error) {
	command, err := auditService.NewCommand(repositories.NewTestAuditStore(db), func(auditService.AppendObservation) {})
	if err != nil {
		return ImportTestModule{}, err
	}
	repos, err := repositories.NewStudentTestRepositories(db, command)
	if err != nil {
		return ImportTestModule{}, err
	}
	identity, err := repositories.NewAuthTestRepositories(db, command)
	if err != nil {
		return ImportTestModule{}, err
	}
	work, err := repositories.NewWorkforceTestRepositories(db, command)
	if err != nil {
		return ImportTestModule{}, err
	}
	classes, err := repositories.NewClassListTestRepositories(db, command)
	if err != nil {
		return ImportTestModule{}, err
	}
	auth, err := NewAuthTestModule(db, unit)
	if err != nil {
		return ImportTestModule{}, err
	}
	people, err := NewRFIDTestModule(db)
	if err != nil {
		return ImportTestModule{}, err
	}
	invitationService := auth.Invitation
	studentConsentService := users.NewStudentConsentService(repos.StudentConsentChange)
	importAudit := repositories.NewImportAuditTestRepository(db, command)
	relationshipResolver := importService.NewRelationshipResolver(repos.Group, repos.Room)
	studentImportConfig := importService.NewStudentImportConfig(
		importService.StudentImportDeps{
			PersonRepo:          repos.Person,
			StudentRepo:         repos.Student,
			GuardianRepo:        repos.GuardianProfile,
			GuardianPhoneRepo:   repos.GuardianPhoneNumber,
			RelationRepo:        repos.StudentGuardian,
			PrivacyRepo:         repos.PrivacyConsent,
			ArrivalScheduleRepo: repos.StudentArrivalSchedule,
			PickupScheduleRepo:  repos.StudentPickupSchedule,
			RFIDCardRepo:        identity.RFIDCard,
			Resolver:            relationshipResolver,
			Consents:            studentConsentService,
		},
		db,
	)
	studentImportService := importService.NewImportService(studentImportConfig)
	studentImportService.SetAuditRepository(importAudit)

	// Staff import files the Stammdatensatz (Person/Staff/Teacher/master
	// data) immediately and issues an invitation for rows with an e-mail;
	// accepting links the account to the imported person (#2600).
	staffImportConfig := importService.NewStaffImportConfig(
		importService.StaffImportDeps{
			InvitationService: invitationService,
			InvitationRepo:    identity.InvitationToken,
			AccountRepo:       repos.Account,
			AccountTenantRepo: repos.AccountTenant,
			RoleRepo:          repos.Role,
			PermissionRepo:    identity.Permission,
			SchoolRepo:        repos.School,
			PersonRepo:        repos.Person,
			StaffRepo:         repos.Staff,
			TeacherRepo:       repos.Teacher,
			MasterDataRepo:    work.StaffMasterData,
			QualificationRepo: work.StaffQualification,
		},
	)
	staffImportService := importService.NewImportService(staffImportConfig)
	staffImportService.SetAuditRepository(importAudit)

	// Class-list entry import (#2382): creates through the entry service so
	// the duplicate guards and the audit trail apply to imported rows too.
	classListImportConfig := importService.NewClassListImportConfig(importService.ClassListImportDeps{
		EntryService: users.NewClassListEntryService(repos.ClassListEntry, repos.Student, classes.Audit),
		EntryRepo:    repos.ClassListEntry,
		StudentRepo:  repos.Student,
	})
	classListImportService := importService.NewImportService(classListImportConfig)
	classListImportService.SetAuditRepository(importAudit)
	return ImportTestModule{Import: studentImportService, StaffImport: staffImportService, ClassListImport: classListImportService, Users: people.Users}, nil
}
