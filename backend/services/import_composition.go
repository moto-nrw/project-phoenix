package services

import (
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	authsvc "github.com/moto-nrw/project-phoenix/services/auth"
	importService "github.com/moto-nrw/project-phoenix/services/import"
	"github.com/moto-nrw/project-phoenix/services/users"
)

// importWiring is everything the Data Import needs (#2708): the owner
// capabilities its ports are bound to, the few read-only legacy repositories
// it still consults, and the platform collaborators. The production root and
// the test composition build the same graph from it, so the two cannot drift.
type importWiring struct {
	Persons    peopledirectory.Capability
	Membership schoolmembership.Capability
	Workforce  workforceModule.Capability
	CarePlan   careplan.Capability
	Presence   studentpresence.Capability

	// Reads are the retained repositories the import still consults; none of
	// them is written. InvitationService issues the staff invitation.
	Reads             importService.LegacyReads
	InvitationService authsvc.InvitationService

	// Opening balances (#2132) still book through the retained Workforce
	// services; that row type's owner cutover is not part of #2708.
	OpeningBalance importService.OpeningBalanceImportDeps

	ConsentHistory users.StudentConsentChangeRecorder
	Audit          auditModels.Command
	Observe        func(importService.ImportObservation)

	// GuardianOverride replaces the guardian port of the student import.
	// Only the test composition sets it, to observe an owner failure
	// mid-batch; production leaves it nil and the People Directory serves
	// all three student ports.
	GuardianOverride importService.GuardianPort
}

// imports is the set of import services the API resource is wired with.
type imports struct {
	Student        *importService.ImportService[importModels.StudentImportRow]
	Staff          *importService.ImportService[importModels.StaffImportRow]
	ClassList      *importService.ImportService[importModels.ClassListEntryImportRow]
	OpeningBalance importService.OpeningBalanceImportFactory
}

// newImports composes the Data Import over its owner ports. Every accepted
// row is committed through the public commands of People Directory, School
// Membership, Workforce, Care Plan, Student Presence and the Audit platform;
// the import performs no write of its own.
func newImports(wiring importWiring) imports {
	runtime := importService.ImportRuntime{Audit: wiring.Audit, Observe: wiring.Observe}
	resolver := importService.NewRelationshipResolver(wiring.Reads.Groups, wiring.Reads.Rooms)
	guardians := importService.GuardianPort(wiring.Persons)
	if wiring.GuardianOverride != nil {
		guardians = wiring.GuardianOverride
	}

	student := importService.NewImportService(importService.NewStudentImportConfig(
		importService.StudentImportDeps{
			Persons:         wiring.Persons,
			Students:        wiring.Persons,
			Guardians:       guardians,
			Schedules:       wiring.CarePlan,
			PrivacyConsents: wiring.Presence,
			RFIDCardRepo:    wiring.Reads.RFIDCard,
			Resolver:        resolver,
			ConsentHistory:  wiring.ConsentHistory,
		},
	), runtime)

	// The staff import files the Stammdatensatz (person, staff, caregiver
	// profile, master data, qualifications) immediately and issues an
	// invitation for rows with an e-mail; accepting links the account to the
	// imported person (#2600).
	staff := importService.NewImportService(importService.NewStaffImportConfig(
		importService.StaffImportDeps{
			InvitationService: wiring.InvitationService,
			InvitationRepo:    wiring.Reads.InvitationToken,
			AccountRepo:       wiring.Reads.Account,
			AccountTenantRepo: wiring.Reads.AccountTenant,
			RoleRepo:          wiring.Reads.Role,
			PermissionRepo:    wiring.Reads.Permission,
			SchoolRepo:        wiring.Reads.School,
			Persons:           wiring.Persons,
			Membership:        wiring.Membership,
			Records:           wiring.Workforce,
		},
	), runtime)

	// Class-list entries (#2382) are created through the Membership owner, so
	// the duplicate guards and the audit trail apply to imported rows too.
	classList := importService.NewImportService(importService.NewClassListImportConfig(
		importService.ClassListImportDeps{
			Membership: wiring.Membership,
			Persons:    wiring.Persons,
			Students:   wiring.Persons,
			Audit:      wiring.Audit,
		},
	), runtime)

	// The opening-balance config is request-scoped (Stichtag, Begründung and
	// acting staff member come from the upload form), so the factory closes
	// over the request-independent dependencies and builds one service per
	// request.
	openingBalance := importService.OpeningBalanceImportFactory(
		func(effectiveDate timezone.Date, note string, decidedByStaffID int64) *importService.ImportService[importModels.OpeningBalanceImportRow] {
			config := importService.NewOpeningBalanceImportConfig(wiring.OpeningBalance, effectiveDate, note, decidedByStaffID)
			return importService.NewImportService(config, runtime)
		})

	return imports{Student: student, Staff: staff, ClassList: classList, OpeningBalance: openingBalance}
}
