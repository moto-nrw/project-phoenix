package owners

import (
	"context"
	"errors"
	"fmt"
	"time"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	carePlanBindingPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	recordports "github.com/moto-nrw/project-phoenix/modules/communication/ports"
	staffStore "github.com/moto-nrw/project-phoenix/modules/communication/staffstore"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	enrollmentCapability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

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
func NewOperatorAuditLog(db transactionruntime.Database) OperatorAuditEntries {
	return NewOperatorAuditLogFromEntries(auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db.Pool()}).OperatorAuditLog)
}

// NewParentPortalReads builds Care Plan's guardian-portal repositories over
// the owners they read through: people answers the student, guardian and
// person reads; identity owns the accounts, their school memberships and
// role classes; enrollment owns the phases and applications; schools names
// the schools.
func NewParentPortalReads(
	db transactionruntime.Database, people peopledirectory.Capability, identity *identityaccess.Module,
	enrollment enrollmentCapability.Owner, schools organizationtenancy.Query,
) carePlanCompose.ParentPortalRepositories {
	if schools == nil {
		panic("parent portal repositories: organization tenancy query is required")
	}
	return ComposeParentPortalReads(db, people, identity, enrollment).WithSchools(ParentPortalSchools{Schools: schools})
}

// NewParentMessagingRepositories builds Communication's parent-portal and
// staff messaging repositories: groups names the groups of the inbox
// threads; membership resolves the live staff accounts behind the read
// cursors; identity owns the accounts; enrollment owns the announcement
// applicants; now is the announcement clock, nil keeps the store default;
// schools names the schools.
func NewParentMessagingRepositories(
	db transactionruntime.Database, groups schoolstructure.Query, membership schoolmembership.Capability,
	identity *identityaccess.Module, enrollment enrollmentCapability.Owner, now func() time.Time,
	schools organizationtenancy.Query,
) parentStore.MessagingRepositories {
	if schools == nil {
		panic("parent portal repositories: organization tenancy query is required")
	}
	return ComposeParentMessagingRepositories(db, groups, membership, identity, enrollment, now).WithSchoolNames(ParentPortalSchools{Schools: schools})
}

func ComposeParentPortalReads(db transactionruntime.Database, people peopledirectory.Capability, identity *identityaccess.Module, enrollment enrollmentCapability.Owner) carePlanCompose.ParentPortalRepositories {
	return carePlanCompose.NewParentPortalRepositories(carePlanCompose.ParentPortalDependencies{
		DB: db.Pool(), Directory: ParentPortalDirectory{People: people},
		AccountSchools: identity.ListActiveAccountSchoolIDs,
		Phases:         ParentPortalEnrollments{Enrollment: enrollment},
		Applications:   ParentPortalEnrollments{Enrollment: enrollment},
		Accounts:       IdentityAccountDirectory{Accounts: identity},
	})
}

func ComposeParentMessagingRepositories(
	db transactionruntime.Database, groups schoolstructure.Query, membership schoolmembership.Capability,
	identity *identityaccess.Module, enrollment enrollmentCapability.Owner, now func() time.Time,
) parentStore.MessagingRepositories {
	pool := db.Pool()
	members := ParentStaffMembers{Membership: membership}.List
	// The staff accounts are loaded from the person rows.
	persons := NewPersonRepository(db)
	personAccounts := func(ctx context.Context, ids []int64) (map[int64]*recordports.StaffPersonAccount, error) {
		rows, err := persons.FindByIDs(ctx, ids)
		if rows == nil {
			return nil, err
		}
		out := make(map[int64]*recordports.StaffPersonAccount, len(rows))
		for id, row := range rows {
			if row == nil {
				out[id] = nil
				continue
			}
			out[id] = &recordports.StaffPersonAccount{AccountID: row.AccountID}
		}
		return out, err
	}
	groupNames := parentGroupNames{groups: groups}.list
	return parentStore.MessagingRepositories{
		ParentAnnouncement:  parentStore.NewParentAnnouncementRepository(pool, NewParentAnnouncementAudience(db, ParentAnnouncementApplicants(enrollment), now)),
		ParentMessageThread: parentStore.NewParentMessageThreadRepository(pool, NewParentGuardianLister(db, identity.FindActiveSchoolMemberships)),
		ParentMessageRead: parentStore.WithInboxGroupNames(
			parentStore.NewStaffParentMessageReadRepository(pool, NewParentMessageInbox(db), members, personAccounts), groupNames),
		StaffMessageRead: staffStore.NewStaffMessageReadRepository(pool, NewCommunicationStaffColleagues(db,
			parentStore.CurrentTenantStaffAccounts(members, personAccounts), StaffMessageIdentity(identity)), NewStaffMessageInbox(db)),
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
	return func(ctx context.Context, accountID int64) (*peopledirectory.PersonAccountSnapshot, error) {
		account, err := accounts.FindAccountMetadata(ctx, accountID)
		if errors.Is(err, identityaccess.ErrAccountNotFound) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		result := peopledirectory.PersonAccountSnapshot(account)
		return &result, nil
	}
}

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
		RoleClasses: func(ctx context.Context, tenantID int64, accountIDs []int64) ([]peoplePorts.StaffMessageRole, error) {
			rows, err := accounts.ClassifySchoolRoles(ctx, tenantID, accountIDs)
			if err != nil {
				return nil, err
			}
			result := make([]peoplePorts.StaffMessageRole, 0, len(rows))
			for _, row := range rows {
				result = append(result, peoplePorts.StaffMessageRole{AccountID: row.AccountID, IsAdmin: row.IsAdmin, IsLehrkraft: row.IsLehrkraft})
			}
			return result, nil
		},
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

func (d ParentPortalSchools) ListParentPortalSchools(ctx context.Context, ids []int64) ([]carePlanBindingPorts.ParentPortalSchool, error) {
	schools, err := d.Schools.ListSchoolsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]carePlanBindingPorts.ParentPortalSchool, 0, len(schools))
	for _, school := range schools {
		result = append(result, carePlanBindingPorts.ParentPortalSchool{
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

func (d ParentStaffMembers) List(ctx context.Context, tenantIDs []int64) ([]recordports.StaffMember, error) {
	members, err := d.Membership.ListStaff(ctx, schoolmembership.StaffFilter{TenantIDs: tenantIDs})
	if err != nil {
		return nil, err
	}
	result := make([]recordports.StaffMember, 0, len(members))
	for _, member := range members {
		result = append(result, recordports.StaffMember{TenantID: member.TenantID, PersonID: member.PersonID})
	}
	return result, nil
}

// parentGroupNames serves the parent inbox's group port from School
// Structure.
type parentGroupNames struct{ groups schoolstructure.Query }

func (d parentGroupNames) list(ctx context.Context, ids []int64) (map[int64]string, error) {
	return GroupNamesByID(ctx, d.groups, ids)
}
