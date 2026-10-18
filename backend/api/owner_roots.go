package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	guardianLinkView "github.com/moto-nrw/project-phoenix/modules/guardianlinkview/compose"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"

	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	staffStore "github.com/moto-nrw/project-phoenix/modules/communication/staffstore"
	enrollmentCapability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The roots below build owners from the database for the CLI roots, the
// service graph and the test graphs. NewParentPortalRepositories
// builds the stores of several owners over the database, so no single owner
// composition can host it (#4013).

// NewOrganizationTenancy composes the school owner, unobserved.
func NewOrganizationTenancy(db transactionruntime.Database) (organizationtenancy.Capability, error) {
	return organizationCompose.New(organizationCompose.Dependencies{
		DB:      db.Pool(),
		Observe: func(organizationCompose.Observation) {},
	})
}

// NewOperatorAuditLog composes the operator ledger over the Audit handoff on
// the context, with the root database as the fallback, for the roots that
// record operator actions.
func NewOperatorAuditLog(db transactionruntime.Database) operatorAuditEntries {
	return NewOperatorAuditLogFromEntries(auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db.Pool()}).OperatorAuditLog)
}

// NewParentPortalRepositories builds the retained guardian-portal and
// messaging repositories with the People Directory, School Membership,
// School Structure, Identity & Access and Enrollment reads bound. The school
// names are bound by WithSchools.
func NewParentPortalRepositories(db transactionruntime.Database, owners ParentPortalOwners) ParentPortalRepositories {
	pool, identity := db.Pool(), owners.Identity
	members := ParentStaffMembers{Membership: owners.Membership}.List
	// The staff accounts are loaded from the person rows.
	persons := NewPersonRepository(db)
	personAccounts := func(ctx context.Context, ids []int64) (map[int64]*parentStore.StaffPersonAccount, error) {
		rows, err := persons.FindByIDs(ctx, ids)
		if rows == nil {
			return nil, err
		}
		out := make(map[int64]*parentStore.StaffPersonAccount, len(rows))
		for id, row := range rows {
			if row == nil {
				out[id] = nil
				continue
			}
			out[id] = &parentStore.StaffPersonAccount{AccountID: row.AccountID}
		}
		return out, err
	}
	groupNames := parentGroupNames{groups: owners.Groups}.list
	return ParentPortalRepositories{
		ParentPortalRepositories: carePlanCompose.NewParentPortalRepositories(carePlanCompose.ParentPortalDependencies{
			DB: pool, Directory: ParentPortalDirectory{People: owners.People},
			AccountSchools: identity.ListActiveAccountSchoolIDs,
			Phases:         ParentPortalEnrollments{enrollment: owners.Enrollment},
			Applications:   ParentPortalEnrollments{enrollment: owners.Enrollment},
			Accounts:       IdentityAccountDirectory{Accounts: identity},
		}),
		MessagingRepositories: parentStore.MessagingRepositories{
			ParentAnnouncement:  parentStore.NewParentAnnouncementRepository(pool, ParentAnnouncementApplicants(owners.Enrollment), guardianLinkView.NewReader(), owners.Now),
			ParentMessageThread: parentStore.NewParentMessageThreadRepository(pool, newParentGuardianLister(db, identity.FindActiveSchoolMemberships)),
			ParentMessageRead: parentStore.WithInboxGroupNames(
				parentStore.NewStaffParentMessageReadRepository(pool, guardianLinkView.NewReader(), members, personAccounts), groupNames),
			StaffMessageRead: staffStore.NewStaffMessageReadRepository(pool, NewCommunicationStaffColleagues(db,
				parentStore.CurrentTenantStaffAccounts(members, personAccounts), StaffMessageIdentity(identity))),
		},
	}
}

// NewDeviceFleet composes the device owner for graphs that do not record
// observations and never serve the info-point dashboard. The serving root
// composes the observed module where it mounts the IoT routes, so runtime
// evidence and the dashboard collaborators are kept.
func NewDeviceFleet(db transactionruntime.Database, onlineWindowMinutes ...func(context.Context) int) (devicefleet.Capability, error) {
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return nil, err
	}
	var minutes func(context.Context) int
	if len(onlineWindowMinutes) > 0 {
		minutes = onlineWindowMinutes[0]
	}
	return devicefleetCompose.New(devicefleetCompose.Dependencies{
		DB:                  db.Pool(),
		Rooms:               rooms,
		OnlineWindowMinutes: minutes,
		APIKeys:             DeviceAPIKeys{},
		Observe:             func(devicefleetCompose.Observation) {},
	})
}

// NewPersonRepository is the person repository with the Identity & Access
// account lookup FindWithAccount attaches (#2720).
func NewPersonRepository(db transactionruntime.Database) peoplePorts.PersonRepository {
	identity, err := identityaccessCompose.New(identityaccessCompose.Dependencies{DB: db.Pool(), Observe: func(identityaccessCompose.Observation) {}})
	if err != nil {
		panic(fmt.Sprintf("compose identity access: %v", err))
	}
	persons := peopleCompose.NewPersonRepository(db.Pool(), accountLookup(identity))
	return persons
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

// Identity & Access owns auth.accounts, platform.operators and the operator
// refresh sessions. The legacy repositories that used to join or select
// those tables declare the narrow fact they need and the root binds it at
// construction to the owner (#2720): the parent dashboard reads the
// account's address through the public capability, and the People Directory
// repositories filter or attach accounts through the owner's queries.

// IdentityAccountDirectory adapts the public account lookup to the parent
// enrollment repository's directory port.
type IdentityAccountDirectory struct {
	Accounts identityaccess.GuardianAccessQuery
}

// AccountEmail resolves the address behind an account; an unknown account is
// an empty address, matching the applications by id only.
func (d IdentityAccountDirectory) AccountEmail(ctx context.Context, accountID int64) (string, error) {
	account, err := d.Accounts.FindAccount(ctx, accountID)
	if errors.Is(err, identityaccess.ErrAccountNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return account.Email, nil
}

// StaffMessageIdentity returns the Identity & Access owner queries the staff
// messaging reads filter through, including the role classification
// (#2721).
func StaffMessageIdentity(accounts identityaccess.StaffAccountQueries) peopleCompose.StaffMessageIdentity {
	return peopleCompose.StaffMessageIdentity{
		ActiveSchoolAccounts: accounts.ListActiveAccountIDsForTenant,
		RoleClasses: func(ctx context.Context, tenantID int64, accountIDs []int64) ([]peopleCompose.StaffMessageRole, error) {
			rows, err := accounts.ClassifySchoolRoles(ctx, tenantID, accountIDs)
			if err != nil {
				return nil, err
			}
			result := make([]peopleCompose.StaffMessageRole, 0, len(rows))
			for _, row := range rows {
				result = append(result, peopleCompose.StaffMessageRole{AccountID: row.AccountID, IsAdmin: row.IsAdmin, IsLehrkraft: row.IsLehrkraft})
			}
			return result, nil
		},
	}
}

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
	// Identity owns the accounts, their school memberships and role classes.
	Identity *identityaccess.Module
	// Enrollment owns the phases, applications and announcement applicants.
	Enrollment *enrollmentCapability.Module
	// Now is the announcement clock; nil keeps the store default.
	Now func() time.Time
}

// ParentPortalRepositories are the retained guardian-portal repositories
// (built by the Care Plan composition) and the parent-messaging and
// announcement repositories (built by Communication) over their owners.
type ParentPortalRepositories struct {
	carePlanCompose.ParentPortalRepositories
	parentStore.MessagingRepositories
}

// WithSchools returns the repositories with the school names and the school
// eligibility resolved through the Organisation & Tenancy owner.
func (r ParentPortalRepositories) WithSchools(schools organizationtenancy.Query) ParentPortalRepositories {
	if schools == nil {
		panic("parent portal repositories: organization tenancy query is required")
	}
	return ParentPortalRepositories{
		ParentPortalRepositories: r.ParentPortalRepositories.WithSchools(ParentPortalSchools{Schools: schools}),
		MessagingRepositories:    r.WithSchoolNames(ParentPortalSchools{Schools: schools}),
	}
}

// ParentPortalDirectory serves the guardian portal's People Directory port.
type ParentPortalDirectory struct{ People peopledirectory.Capability }

func (d ParentPortalDirectory) ListParentPortalStudents(ctx context.Context, ids []int64) ([]careplan.ParentPortalStudent, error) {
	students, err := d.People.ListStudentsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]careplan.ParentPortalStudent, 0, len(students))
	for _, student := range students {
		result = append(result, careplan.ParentPortalStudent{
			ID: student.ID, TenantID: student.TenantID, PersonID: student.PersonID,
			SchoolClass: student.SchoolClass, Status: student.Status,
			EnrolledFrom: calendar.Date(student.EnrolledFrom), EnrolledUntil: calendar.Date(student.EnrolledUntil),
		})
	}
	return result, nil
}

func (d ParentPortalDirectory) ListParentPortalGuardianLinks(ctx context.Context, accountID int64) ([]careplan.ParentPortalGuardianLink, error) {
	links, err := d.People.ListGuardianLinksByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	result := make([]careplan.ParentPortalGuardianLink, 0, len(links))
	for _, link := range links {
		result = append(result, careplan.ParentPortalGuardianLink{
			TenantID: link.TenantID, StudentID: link.StudentID, GuardianProfileID: link.GuardianProfileID, Permissions: link.Permissions,
		})
	}
	return result, nil
}

func (d ParentPortalDirectory) ListParentPortalPersons(ctx context.Context, ids []int64) ([]careplan.ParentPortalPerson, error) {
	persons, err := d.People.ListPersonsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]careplan.ParentPortalPerson, 0, len(persons))
	for _, person := range persons {
		result = append(result, careplan.ParentPortalPerson{ID: person.ID, FirstName: person.FirstName, LastName: person.LastName})
	}
	return result, nil
}

// ParentPortalSchools serves the guardian portal's and the parent inbox's
// school ports from the Organisation & Tenancy owner.
type ParentPortalSchools struct{ Schools organizationtenancy.Query }

func (d ParentPortalSchools) ListParentPortalSchools(ctx context.Context, ids []int64) ([]carePlanCompose.ParentPortalSchool, error) {
	schools, err := d.Schools.ListSchoolsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]carePlanCompose.ParentPortalSchool, 0, len(schools))
	for _, school := range schools {
		result = append(result, carePlanCompose.ParentPortalSchool{
			ID: school.ID, Name: school.Name, Slug: school.Slug, Subdomain: school.Subdomain,
			Active: school.Active, Hidden: school.Hidden, Deleted: school.IsDeleted(),
		})
	}
	return result, nil
}

func (d ParentPortalSchools) SchoolName(ctx context.Context, tenantID int64) (string, error) {
	school, err := d.Schools.FindSchool(ctx, tenantID)
	if err != nil {
		return "", err
	}
	return school.Name, nil
}

func (d ParentPortalSchools) SchoolNames(ctx context.Context, ids []int64) (map[int64]string, error) {
	schools, err := schoolsByID(ctx, d.Schools, ids)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(schools))
	for id, school := range schools {
		names[id] = school.Name
	}
	return names, nil
}

// ParentStaffMembers serves the parent inbox's staff port from School
// Membership.
type ParentStaffMembers struct{ Membership schoolmembership.Query }

func (d ParentStaffMembers) List(ctx context.Context, tenantIDs []int64) ([]parentStore.StaffMember, error) {
	members, err := d.Membership.ListStaff(ctx, schoolmembership.StaffFilter{TenantIDs: tenantIDs})
	if err != nil {
		return nil, err
	}
	result := make([]parentStore.StaffMember, 0, len(members))
	for _, member := range members {
		result = append(result, parentStore.StaffMember{TenantID: member.TenantID, PersonID: member.PersonID})
	}
	return result, nil
}

// parentGroupNames serves the parent inbox's group port from School
// Structure.
type parentGroupNames struct{ groups schoolstructure.Query }

func (d parentGroupNames) list(ctx context.Context, ids []int64) (map[int64]string, error) {
	return schoolStructureCompose.GroupNamesByID(ctx, d.groups, ids)
}
