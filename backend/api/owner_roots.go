package api

import (
	"context"
	"errors"
	"fmt"
	"time"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"
	staffStore "github.com/moto-nrw/project-phoenix/modules/communication/staffstore"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	devicefleetCompose "github.com/moto-nrw/project-phoenix/modules/devicefleet/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	platform "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose/vocabulary"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

// The roots below build owners from the database for the CLI roots, the
// legacy service graph and the test graphs. NewParentPortalRepositories
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
func NewOperatorAuditLog(db transactionruntime.Database) platform.OperatorAuditLogRepository {
	return NewOperatorAuditLogFromEntries(auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db.Pool()}).OperatorAuditLog)
}

// NewParentPortalRepositories builds the retained guardian-portal and
// messaging repositories with the People Directory, School Membership,
// School Structure, Identity & Access and Enrollment reads bound. The school
// names are bound by WithSchools.
func NewParentPortalRepositories(db transactionruntime.Database, owners services.ParentPortalOwners) services.ParentPortalRepositories {
	pool, identity := db.Pool(), owners.Identity
	members := services.ParentStaffMembers{Membership: owners.Membership}.List
	// The staff accounts are loaded from the person rows.
	persons := NewPersonRepository(db)
	groupNames := func(ctx context.Context, ids []int64) (map[int64]string, error) {
		return schoolStructureCompose.GroupNamesByID(ctx, owners.Groups, ids)
	}
	return services.ParentPortalRepositories{
		ParentPortalRepositories: carePlanCompose.NewParentPortalRepositories(carePlanCompose.ParentPortalDependencies{
			DB: pool, Directory: services.ParentPortalDirectory{People: owners.People},
			AccountSchools: identity.ListActiveAccountSchoolIDs,
			Phases:         owners.Enrollment, Applications: owners.Enrollment,
			Accounts: IdentityAccountDirectory{Accounts: identity},
		}),
		MessagingRepositories: parentStore.MessagingRepositories{
			ParentAnnouncement:  parentStore.NewParentAnnouncementRepository(pool, ParentAnnouncementApplicants(owners.Enrollment), owners.Now),
			ParentMessageThread: parentStore.NewParentMessageThreadRepository(pool, peopleCompose.NewMessageableGuardians(pool, identity.FindActiveSchoolMemberships)),
			ParentMessageRead: parentStore.WithInboxGroupNames(
				parentStore.NewStaffParentMessageReadRepository(pool, members, persons), groupNames),
			StaffMessageRead: staffStore.NewStaffMessageReadRepository(pool, peopleCompose.NewMessageableStaff(pool,
				parentStore.CurrentTenantStaffAccounts(members, persons), StaffMessageIdentity(identity))),
		},
	}
}

// NewDeviceFleet composes the device owner for graphs that do not record
// observations and never serve the info-point dashboard. The serving root
// composes the observed module where it mounts the IoT routes, so runtime
// evidence and the dashboard collaborators are kept.
func NewDeviceFleet(db transactionruntime.Database, onlineWindows ...func(context.Context) time.Duration) (devicefleet.Capability, error) {
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return nil, err
	}
	var onlineWindow func(context.Context) time.Duration
	if len(onlineWindows) > 0 {
		onlineWindow = onlineWindows[0]
	}
	return devicefleetCompose.New(devicefleetCompose.Dependencies{
		DB:           db.Pool(),
		Rooms:        rooms,
		OnlineWindow: onlineWindow,
		Observe:      func(devicefleetCompose.Observation) {},
	})
}

// NewPersonRepository is the person repository with the Identity & Access
// account lookup FindWithAccount attaches (#2720).
func NewPersonRepository(db transactionruntime.Database) userModels.PersonRepository {
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
