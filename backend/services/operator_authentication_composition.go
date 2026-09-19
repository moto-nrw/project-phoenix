package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// Identity & Access owns operator login, the MFA-proven token issue, refresh,
// profile and password changes with their session revocation, and the
// operator-led school access of accounts (#3252). This file binds the seams
// those flows need to the retained owners the root still composes (the
// operator MFA service, the operator audit ledger, the e-mail change tokens,
// the password policy, Organisation & Tenancy, the People Directory and
// School Membership identity chain, the retained role rules) and serves the
// retained platform services' consumer-owned ports over the public module.

// operatorAuthenticationWiring is the retained material the operator seams
// are bound to.
type operatorAuthenticationWiring struct {
	repos         operatorRepositories
	organizations organizationtenancy.Query
	persons       peopledirectory.Query
	membership    schoolmembership.Query
	logger        *slog.Logger
}

// operatorRepositories are the retained repositories the operator seams
// read: the audit ledger and the identity chain rows a school access
// provisions.
type operatorRepositories struct {
	auditLog platformModels.OperatorAuditLogRepository
	persons  userModels.PersonRepository
	staff    userModels.StaffRepository
	teachers userModels.TeacherRepository
	students userModels.StudentRepository
}

func operatorRepositoriesOf(repos *repositories.Factory) operatorRepositories {
	if repos == nil {
		return operatorRepositories{}
	}
	return operatorRepositories{
		auditLog: repos.OperatorAuditLog,
		persons:  repos.Person, staff: repos.Staff, teachers: repos.Teacher, students: repos.Student,
	}
}

func newOperatorDependencies(wiring operatorAuthenticationWiring) (*identityaccessCompose.OperatorDependencies, error) {
	if wiring.repos.auditLog == nil || wiring.repos.persons == nil || wiring.repos.staff == nil || wiring.repos.teachers == nil || wiring.repos.students == nil ||
		wiring.organizations == nil || wiring.persons == nil || wiring.membership == nil {
		return nil, errors.New("operator authentication composition: repositories and owner capabilities are required")
	}
	return &identityaccessCompose.OperatorDependencies{
		Audit:         operatorAuditLedger{ledger: wiring.repos.auditLog},
		Passwords:     passwordHasher{},
		Organizations: tenancyDirectory{query: wiring.organizations},
		Identities: schoolIdentityProvisioner{
			persons: wiring.repos.persons, staff: wiring.repos.staff, teachers: wiring.repos.teachers, students: wiring.repos.students,
			directory: wiring.persons, membership: wiring.membership,
		},
		Logger: wiring.logger,
	}, nil
}

// --- retained owner seams -------------------------------------------------

type operatorAuditLedger struct {
	ledger platformModels.OperatorAuditLogRepository
}

func (a operatorAuditLedger) RecordOperatorAction(ctx context.Context, entry identityaccess.OperatorAuditEntry) error {
	row := &platformModels.OperatorAuditLog{
		OperatorID: entry.OperatorID, Action: entry.Action, ResourceType: entry.ResourceType,
		ResourceID: entry.ResourceID, RequestIP: securityruntime.ParseClientAddress(entry.IPAddress),
	}
	if changes := operatorAuditChanges(entry); len(changes) > 0 {
		if err := row.SetChanges(changes); err != nil {
			return fmt.Errorf("encode operator audit log changes: %w", err)
		}
	}
	return a.ledger.Create(ctx, row)
}

// operatorAuditChanges renders the typed change summary into the JSON keys
// the operator audit log has always stored for each action.
func operatorAuditChanges(entry identityaccess.OperatorAuditEntry) map[string]any {
	if evidence := entry.RevokedSessions; evidence != nil {
		return map[string]any{
			"portal_scope":        evidence.PortalScope,
			"family_fingerprint":  evidence.FamilyFingerprint,
			"reason":              evidence.Reason,
			"revoked_token_count": evidence.Count,
		}
	}
	change := entry.AccessChange
	if change == nil {
		return nil
	}
	changes := map[string]any{"schoolID": change.SchoolID, "email": change.Email}
	switch entry.Action {
	case platformModels.ActionCreate:
		changes["roleID"] = change.RoleID
		changes["roleName"] = change.RoleName
	case platformModels.ActionUpdate:
		changes["roleID"] = change.RoleID
		changes["roleName"] = change.RoleName
		changes["removedRoles"] = change.RemovedRoles
	case platformModels.ActionDelete:
		changes["accountDeactivated"] = change.AccountDeactivated
	}
	return changes
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

func (d tenancyDirectory) ListOrganizationNames(ctx context.Context, ids []int64) ([]identityaccessCompose.OrganizationName, error) {
	organizations, err := d.query.ListOrganizationsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]identityaccessCompose.OrganizationName, 0, len(organizations))
	for _, organization := range organizations {
		result = append(result, identityaccessCompose.OrganizationName{ID: organization.ID, Name: organization.Name})
	}
	return result, nil
}

// schoolIdentityProvisioner serves the person, staff and teacher chain a
// school access owes through the retained shared helpers every
// school-access path uses, so the guards cannot drift from the tenant RBAC
// path.
type schoolIdentityProvisioner struct {
	persons    userModels.PersonRepository
	staff      userModels.StaffRepository
	teachers   userModels.TeacherRepository
	students   userModels.StudentRepository
	directory  peopledirectory.Query
	membership schoolmembership.Query
}

func (p schoolIdentityProvisioner) HasLivePersonAtSchool(ctx context.Context, accountID int64) (bool, error) {
	person, err := p.persons.FindByAccountID(ctx, accountID)
	if err != nil {
		return false, err
	}
	return person != nil && person.DeletedAt == nil, nil
}

func (p schoolIdentityProvisioner) ListAccountPersons(ctx context.Context, accountID int64) ([]identityaccessCompose.AccountPersonIdentity, error) {
	persons, err := p.persons.List(ctx, map[string]interface{}{"account_id": accountID})
	if err != nil {
		return nil, err
	}
	result := make([]identityaccessCompose.AccountPersonIdentity, 0, len(persons))
	for _, person := range persons {
		if person == nil {
			continue
		}
		identity := identityaccessCompose.AccountPersonIdentity{
			ID: person.ID, TenantID: person.TenantID, FirstName: person.FirstName, LastName: person.LastName, Deleted: person.DeletedAt != nil,
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
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return student != nil, nil
}

func (p schoolIdentityProvisioner) HasLiveCaregiverProfile(ctx context.Context, accountID int64) (bool, error) {
	return hasLiveCaregiverProfile(ctx, p.persons, p.staff, p.teachers, accountID)
}

func (p schoolIdentityProvisioner) EnsureSchoolIdentity(context.Context, identityaccessCompose.SchoolIdentityRequest) error {
	// Production compositions bind the module's account-lifecycle chain
	// (#3225). This fallback only runs when that chain was not composed.
	return errors.New("school identity provisioning is not composed")
}

// ListAccountIdentityFacts resolves, per school, whether a person and a
// live staff record back the account: the most recently updated person per
// school counts, and its staff record only at that same school.
func (p schoolIdentityProvisioner) ListAccountIdentityFacts(ctx context.Context, accountID int64) ([]identityaccessCompose.AccountIdentityFact, error) {
	persons, err := p.directory.ListPersonsByAccount(ctx, []int64{accountID})
	if err != nil {
		return nil, fmt.Errorf("load persons by account: %w", err)
	}
	personByTenant := make(map[int64]peopledirectory.Person, len(persons))
	for _, person := range persons {
		if person.AccountID == nil || *person.AccountID != accountID {
			continue
		}
		current, found := personByTenant[person.TenantID]
		if !found || person.UpdatedAt.After(current.UpdatedAt) {
			personByTenant[person.TenantID] = person
		}
	}
	if len(personByTenant) == 0 {
		return []identityaccessCompose.AccountIdentityFact{}, nil
	}
	personIDs := make([]int64, 0, len(personByTenant))
	for _, person := range personByTenant {
		personIDs = append(personIDs, person.ID)
	}
	members, err := p.membership.ListStaff(ctx, schoolmembership.StaffFilter{PersonIDs: personIDs})
	if err != nil {
		return nil, fmt.Errorf("load staff for account identity facts: %w", err)
	}
	staffTenantByPerson := make(map[int64]int64, len(members))
	for _, member := range members {
		if _, found := staffTenantByPerson[member.PersonID]; !found {
			staffTenantByPerson[member.PersonID] = member.TenantID
		}
	}
	facts := make([]identityaccessCompose.AccountIdentityFact, 0, len(personByTenant))
	for tenantID, person := range personByTenant {
		staffTenant, hasStaff := staffTenantByPerson[person.ID]
		facts = append(facts, identityaccessCompose.AccountIdentityFact{TenantID: tenantID, HasPerson: true, HasStaff: hasStaff && staffTenant == tenantID})
	}
	return facts, nil
}
