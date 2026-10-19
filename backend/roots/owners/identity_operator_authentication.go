package owners

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
)

// operatorRepositories are the retained repositories the operator seams
// read: the audit ledger and the identity chain rows a school access
// provisions.
type operatorRepositories struct {
	auditLog OperatorAuditEntries
	persons  personRows
	staff    staffRows
	teachers teacherRows
	students studentRows
}

func OperatorRepositoriesOf(people LifecycleRepositories, auditLog OperatorAuditEntries) operatorRepositories {
	if people.Persons == nil {
		return operatorRepositories{}
	}
	return operatorRepositories{
		auditLog: auditLog,
		persons:  people.Persons, staff: people.Staff, teachers: people.Teachers, students: people.Students,
	}
}

// NewOperatorDependencies binds the operator seams to the operator audit
// ledger and identity chain rows in repos and to the owners a school access
// provisions through.
func NewOperatorDependencies(repos operatorRepositories, organizations organizationtenancy.Query, persons peopledirectory.Query, membership schoolmembership.Capability, logger *slog.Logger) (*identityaccessCompose.OperatorDependencies, error) {
	if repos.auditLog == nil || repos.persons == nil || repos.staff == nil || repos.teachers == nil || repos.students == nil ||
		organizations == nil || persons == nil || membership == nil {
		return nil, errors.New("operator authentication composition: repositories and owner capabilities are required")
	}
	return &identityaccessCompose.OperatorDependencies{
		Audit:         operatorAuditLedger{ledger: repos.auditLog},
		Passwords:     passwordHasher{},
		Organizations: tenancyDirectory{query: organizations},
		Identities: schoolIdentityProvisioner{
			persons: repos.persons, staff: repos.staff, teachers: repos.teachers, students: repos.students,
			directory: persons, membership: membership,
			caregivers: CaregiverProfiles{Persons: persons, Membership: membership},
		},
		Logger: logger,
	}, nil
}

type operatorAuditLedger struct {
	ledger OperatorAuditEntries
}

func (a operatorAuditLedger) RecordOperatorAction(ctx context.Context, entry identityaccess.OperatorAuditEntry) error {
	row := &auditModels.OperatorAuditEntry{
		OperatorID: entry.OperatorID, Action: entry.Action, ResourceType: entry.ResourceType,
		ResourceID: entry.ResourceID, RequestIP: securityruntime.ParseClientAddress(entry.IPAddress),
	}
	changes, err := entry.AuditChanges()
	if err != nil {
		return fmt.Errorf("encode operator audit log changes: %w", err)
	}
	row.Changes = changes
	return a.ledger.Create(ctx, row)
}

// passwordHasher binds the credential policy Security Runtime owns to the
// operator flows and reports the module's public sentinel.
type passwordHasher struct{}

func (passwordHasher) HashPassword(password string) (string, error) {
	return securityruntime.HashPassword(password)
}

func (passwordHasher) ValidatePasswordStrength(password string) error {
	if securityruntime.PasswordTooWeak(password) {
		return identityaccess.ErrPasswordTooWeak
	}
	return nil
}

type tenancyDirectory struct{ query organizationtenancy.Query }

func (d tenancyDirectory) ListSchools(ctx context.Context, ids []int64) ([]identityaccess.School, error) {
	schools, err := d.query.ListSchoolsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]identityaccess.School, 0, len(schools))
	for _, school := range schools {
		result = append(result, identityaccess.School{
			ID: school.ID, OrganizationID: school.OrganizationID, Name: school.Name, Slug: school.Slug,
			Active: school.Active, Deleted: school.IsDeleted(),
		})
	}
	return result, nil
}

func (d tenancyDirectory) ListOrganizationNames(ctx context.Context, ids []int64) ([]identityPorts.OrganizationName, error) {
	organizations, err := d.query.ListOrganizationsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]identityPorts.OrganizationName, 0, len(organizations))
	for _, organization := range organizations {
		result = append(result, identityPorts.OrganizationName{ID: organization.ID, Name: organization.Name})
	}
	return result, nil
}

// schoolIdentityProvisioner serves the person, staff and teacher chain a
// school access owes through the retained shared helpers every
// school-access path uses, so the guards cannot drift from the tenant RBAC
// path.
type schoolIdentityProvisioner struct {
	persons    personRows
	staff      staffRows
	teachers   teacherRows
	students   studentRows
	directory  peopledirectory.Query
	membership staffMemberQuery
	caregivers liveCaregiverProfile
}

func (p schoolIdentityProvisioner) HasLivePersonAtSchool(ctx context.Context, accountID int64) (bool, error) {
	person, err := p.persons.FindByAccountID(ctx, accountID)
	if err != nil {
		return false, err
	}
	return person != nil && peopledirectory.PersonIsLive(person.DeletedAt), nil
}

func (p schoolIdentityProvisioner) ListAccountPersons(ctx context.Context, accountID int64) ([]identityPorts.AccountPersonIdentity, error) {
	persons, err := p.persons.List(ctx, map[string]interface{}{"account_id": accountID})
	if err != nil {
		return nil, err
	}
	result := make([]identityPorts.AccountPersonIdentity, 0, len(persons))
	for _, person := range persons {
		if person == nil {
			continue
		}
		identity := identityPorts.AccountPersonIdentity{
			ID: person.ID, TenantID: person.TenantID, FirstName: person.FirstName, LastName: person.LastName, Deleted: !peopledirectory.PersonIsLive(person.DeletedAt),
		}
		if !identity.Deleted {
			isStudent, err := p.personIsStudent(ctx, person.ID)
			if err != nil {
				return nil, err
			}
			identity.IsStudent = isStudent
		}
		result = append(result, identity)
	}
	return result, nil
}

// personIsStudent reports whether the person record belongs to a child. The
// repository reports "no student" as sql.ErrNoRows wrapped in a
// DatabaseError.
func (p schoolIdentityProvisioner) personIsStudent(ctx context.Context, personID int64) (bool, error) {
	student, err := p.students.FindByPersonID(ctx, personID)
	if err != nil {
		if errors.Is(err, transactionruntime.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return student != nil, nil
}

func (p schoolIdentityProvisioner) HasLiveCaregiverProfile(ctx context.Context, accountID int64) (bool, error) {
	return p.caregivers.hasLive(ctx, accountID)
}

func (p schoolIdentityProvisioner) EnsureSchoolIdentity(context.Context, identityPorts.SchoolIdentityRequest) error {
	// Production compositions bind the module's account-lifecycle chain
	// (#3225). This fallback only runs when that chain was not composed.
	return errors.New("school identity provisioning is not composed")
}

// ListAccountIdentityFacts resolves, per school, whether a person and a
// live staff record back the account: the People Directory's latest person
// per school, and the staff record Identity & Access counts for it.
func (p schoolIdentityProvisioner) ListAccountIdentityFacts(ctx context.Context, accountID int64) ([]identityPorts.AccountIdentityFact, error) {
	persons, err := p.directory.ListPersonsByAccount(ctx, []int64{accountID})
	if err != nil {
		return nil, fmt.Errorf("load persons by account: %w", err)
	}
	personByTenant := latestPersonPerSchool(persons, accountID)
	if len(personByTenant) == 0 {
		return []identityPorts.AccountIdentityFact{}, nil
	}
	personIDs := make([]int64, 0, len(personByTenant))
	for _, person := range personByTenant {
		personIDs = append(personIDs, person.ID)
	}
	members, err := p.membership.ListStaff(ctx, schoolmembership.StaffFilter{PersonIDs: personIDs, MembershipOnly: true})
	if err != nil {
		return nil, fmt.Errorf("load staff for account identity facts: %w", err)
	}
	records := make([]identityaccess.PersonStaffRecord, 0, len(members))
	for _, member := range members {
		records = append(records, identityaccess.PersonStaffRecord{PersonID: member.PersonID, SchoolID: member.TenantID})
	}
	staffTenantByPerson := identityaccess.StaffSchoolByPerson(records)
	facts := make([]identityPorts.AccountIdentityFact, 0, len(personByTenant))
	for tenantID, person := range personByTenant {
		staffTenant, hasStaff := staffTenantByPerson[person.ID]
		facts = append(facts, identityPorts.AccountIdentityFact{
			TenantID: tenantID, HasPerson: true, HasStaff: identityaccess.StaffRecordCountsAtSchool(hasStaff, staffTenant, tenantID),
		})
	}
	return facts, nil
}

// latestPersonPerSchool is the People Directory's person of the account at
// each school.
func latestPersonPerSchool(persons []peopledirectory.Person, accountID int64) map[int64]peopledirectory.Person {
	personByTenant := make(map[int64]peopledirectory.Person, len(persons))
	for key, person := range peopledirectory.LatestPersonsByAccountSchool(persons) {
		if key.AccountID == accountID {
			personByTenant[key.TenantID] = person
		}
	}
	return personByTenant
}
