package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/platform"
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
// are bound to. mfa is read at call time because the operator MFA service
// is constructed after the module.
type operatorAuthenticationWiring struct {
	repos         operatorRepositories
	organizations organizationtenancy.Query
	persons       peopledirectory.Query
	membership    schoolmembership.Query
	mfa           func() platform.OperatorMFAService
	logger        *slog.Logger
}

// operatorRepositories are the retained repositories the operator seams
// read: the audit ledger, the e-mail change tokens and the identity chain
// rows a school access provisions.
type operatorRepositories struct {
	auditLog          platformModels.OperatorAuditLogRepository
	emailChangeTokens platformModels.OperatorEmailChangeTokenRepository
	persons           userModels.PersonRepository
	staff             userModels.StaffRepository
	teachers          userModels.TeacherRepository
	students          userModels.StudentRepository
}

func operatorRepositoriesOf(repos *repositories.Factory) operatorRepositories {
	if repos == nil {
		return operatorRepositories{}
	}
	return operatorRepositories{
		auditLog: repos.OperatorAuditLog, emailChangeTokens: repos.OperatorEmailChangeToken,
		persons: repos.Person, staff: repos.Staff, teachers: repos.Teacher, students: repos.Student,
	}
}

func newOperatorDependencies(wiring operatorAuthenticationWiring) (*identityaccessCompose.OperatorDependencies, error) {
	if wiring.repos.auditLog == nil || wiring.repos.persons == nil || wiring.repos.staff == nil || wiring.repos.teachers == nil || wiring.repos.students == nil ||
		wiring.organizations == nil || wiring.persons == nil || wiring.membership == nil {
		return nil, errors.New("operator authentication composition: repositories and owner capabilities are required")
	}
	mfa := wiring.mfa
	if mfa == nil {
		mfa = func() platform.OperatorMFAService { return nil }
	}
	return &identityaccessCompose.OperatorDependencies{
		MFA:           operatorMFAGate{current: mfa},
		Audit:         operatorAuditLedger{ledger: wiring.repos.auditLog},
		Credentials:   operatorCredentialCleanup{tokens: wiring.repos.emailChangeTokens},
		Passwords:     passwordHasher{},
		Organizations: tenancyDirectory{query: wiring.organizations},
		Identities: schoolIdentityProvisioner{
			persons: wiring.repos.persons, staff: wiring.repos.staff, teachers: wiring.repos.teachers, students: wiring.repos.students,
			directory: wiring.persons, membership: wiring.membership,
		},
		RolePolicy: schoolRolePolicy{},
		Logger:     wiring.logger,
	}, nil
}

// --- retained owner seams -------------------------------------------------

type operatorMFAGate struct {
	current func() platform.OperatorMFAService
}

func (g operatorMFAGate) Configured() bool { return g.current() != nil }

func (g operatorMFAGate) HasEnrollment(ctx context.Context, operatorID int64) (bool, error) {
	svc := g.current()
	if svc == nil {
		return false, nil
	}
	return svc.HasEnrollment(ctx, operatorID)
}

func (g operatorMFAGate) VerifyTrustedDevice(ctx context.Context, operatorID int64, cookie string) (bool, error) {
	svc := g.current()
	if svc == nil {
		return false, nil
	}
	return svc.VerifyTrustedDevice(ctx, operatorID, cookie)
}

func (g operatorMFAGate) StartChallenge(ctx context.Context, operatorID int64, ipAddress string) (string, error) {
	svc := g.current()
	if svc == nil {
		return "", errors.New("operator mfa service is not configured")
	}
	return svc.StartChallenge(ctx, operatorID, auth.ParseClientIP(ipAddress))
}

// TrustedDeviceDays derives from the hardcoded operator trusted-device
// duration; operators do not expose this as a configurable setting.
func (operatorMFAGate) TrustedDeviceDays() int {
	return int(platform.OperatorMFATrustedDeviceDuration.Hours() / 24)
}

type operatorAuditLedger struct {
	ledger platformModels.OperatorAuditLogRepository
}

func (a operatorAuditLedger) RecordOperatorAction(ctx context.Context, entry identityaccess.OperatorAuditEntry) error {
	row := &platformModels.OperatorAuditLog{
		OperatorID: entry.OperatorID, Action: entry.Action, ResourceType: entry.ResourceType,
		ResourceID: entry.ResourceID, RequestIP: auth.ParseClientIP(entry.IPAddress),
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

type operatorCredentialCleanup struct {
	tokens platformModels.OperatorEmailChangeTokenRepository
}

// InvalidateEmailChangeTokens is a no-op without the token repository, as
// the retained password change treated it.
func (c operatorCredentialCleanup) InvalidateEmailChangeTokens(ctx context.Context, operatorID int64) error {
	if c.tokens == nil {
		return nil
	}
	return c.tokens.InvalidateByOperatorID(ctx, operatorID)
}

type passwordHasher struct{}

func (passwordHasher) HashPassword(password string) (string, error) {
	return auth.HashPassword(password)
}

func (passwordHasher) ValidatePasswordStrength(password string) error {
	return auth.ValidatePasswordStrength(password)
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
	return auth.HasLiveCaregiverProfile(ctx, p.persons, p.staff, p.teachers, accountID)
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

// schoolRolePolicy serves the retained role assignment rules.
type schoolRolePolicy struct{}

func (schoolRolePolicy) ValidateAssignableSchoolRole(role identityaccess.SchoolRole, tenantID int64) error {
	if role.ID <= 0 {
		return auth.ErrRoleNotAssignable
	}
	_, err := auth.ValidateResolvedAssignableSchoolRole(auth.ResolvedSchoolRole(role.ID, role.TenantID, role.Name, role.IsSystem, role.BaseRole), tenantID)
	return err
}

func (schoolRolePolicy) IsLehrkraftSystemRole(role identityaccess.SchoolRole) bool {
	return auth.IsLehrkraftSystemRole(auth.ResolvedSchoolRole(role.ID, role.TenantID, role.Name, role.IsSystem, role.BaseRole))
}

func (schoolRolePolicy) RoleNeedsStaffRecord(role identityaccess.SchoolRole) bool {
	return identityaccess.RoleNeedsStaffRecord(&identityaccess.RoleFacts{
		ID: role.ID, TenantID: role.TenantID, Name: role.Name, IsSystem: role.IsSystem, BaseRole: role.BaseRole,
	})
}

func (schoolRolePolicy) LehrkraftRoleImmutable() error { return auth.ErrLehrkraftRoleImmutable }

// --- the retained platform services' consumer-owned ports -----------------

// operatorSessions serves platform.OperatorSessions over the public module
// and translates the public sentinels back into the retained error shapes
// the MFA and passkey routes render.
type operatorSessions struct {
	module identityaccess.OperatorAuthentication
}

func newOperatorSessions(module identityaccess.OperatorAuthentication) platform.OperatorSessions {
	return operatorSessions{module: module}
}

func (s operatorSessions) IssueTokensForAuthenticatedOperator(ctx context.Context, operatorID int64, ipAddress, userAgent string) (string, string, error) {
	access, refresh, err := s.module.IssueTokensForAuthenticatedOperator(ctx, operatorID, ipAddress, userAgent)
	switch {
	case err == nil:
		return access, refresh, nil
	case errors.Is(err, identityaccess.ErrOperatorNotFound):
		return "", "", &platform.OperatorNotFoundError{OperatorID: operatorID}
	case errors.Is(err, identityaccess.ErrOperatorInactive):
		return "", "", &platform.OperatorInactiveError{OperatorID: operatorID}
	default:
		return "", "", err
	}
}

// operatorDirectory serves platform.OperatorDirectory, the retained
// contract the e-mail change, invitation, MFA and passkey flows read
// operator rows through: a missing row is (nil, nil), validation runs on the
// retained model before the owner sees the value, and the identity and
// timestamps are written back into the caller's value.
type operatorDirectory struct{ identity identityaccess.OperatorAccess }

func newOperatorDirectory(identity identityaccess.OperatorAccess) platform.OperatorDirectory {
	return operatorDirectory{identity: identity}
}

func (r operatorDirectory) Create(ctx context.Context, operator *platformModels.Operator) error {
	if operator == nil {
		return fmt.Errorf("%s cannot be nil or zero value", "Operator")
	}
	if err := operator.Validate(); err != nil {
		return err
	}
	stored, err := r.identity.CreateOperator(ctx, identityOperator(operator))
	if err != nil {
		return operatorDatabaseError("create", err)
	}
	applyIdentityOperator(operator, stored)
	return nil
}

func (r operatorDirectory) FindByID(ctx context.Context, id int64) (*platformModels.Operator, error) {
	operator, err := r.identity.FindOperator(ctx, id)
	return operatorResult(operator, err, "find operator by id")
}

func (r operatorDirectory) FindByIDForUpdate(ctx context.Context, id int64) (*platformModels.Operator, error) {
	operator, err := r.identity.FindOperatorForUpdate(ctx, id)
	return operatorResult(operator, err, "find operator by id for update")
}

func (r operatorDirectory) FindByEmail(ctx context.Context, email string) (*platformModels.Operator, error) {
	operator, err := r.identity.FindOperatorByEmail(ctx, email)
	return operatorResult(operator, err, "find operator by email")
}

func operatorResult(operator identityaccess.Operator, err error, op string) (*platformModels.Operator, error) {
	if errors.Is(err, identityaccess.ErrOperatorNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, operatorDatabaseError(op, err)
	}
	return operatorModel(operator), nil
}

func (r operatorDirectory) Update(ctx context.Context, operator *platformModels.Operator) error {
	if operator == nil {
		return fmt.Errorf("%s cannot be nil or zero value", "Operator")
	}
	if err := operator.Validate(); err != nil {
		return err
	}
	stored, err := r.identity.UpdateOperator(ctx, identityOperator(operator))
	if err != nil {
		return operatorDatabaseError("update", err)
	}
	applyIdentityOperator(operator, stored)
	return nil
}

func (r operatorDirectory) List(ctx context.Context) ([]*platformModels.Operator, error) {
	operators, err := r.identity.ListOperators(ctx)
	if err != nil {
		return nil, operatorDatabaseError("list operators", err)
	}
	result := make([]*platformModels.Operator, 0, len(operators))
	for _, operator := range operators {
		result = append(result, operatorModel(operator))
	}
	return result, nil
}

func (r operatorDirectory) IncrementMFAAttempts(ctx context.Context, id int64, threshold int, lockoutDuration time.Duration) (platform.OperatorMFAAttempts, error) {
	attempts, err := r.identity.IncrementOperatorMFAAttempts(ctx, id, threshold, lockoutDuration)
	if err != nil {
		return platform.OperatorMFAAttempts{}, operatorDatabaseError("increment operator mfa attempts", err)
	}
	return platform.OperatorMFAAttempts{Attempts: attempts.Attempts, LockedUntil: attempts.LockedUntil}, nil
}

func (r operatorDirectory) ResetMFAAttempts(ctx context.Context, id int64) error {
	if err := r.identity.ResetOperatorMFAAttempts(ctx, id); err != nil {
		return operatorDatabaseError("reset operator mfa attempts", err)
	}
	return nil
}

func operatorDatabaseError(op string, err error) error {
	return fmt.Errorf("database error during %s: %w", op, err)
}

func identityOperator(operator *platformModels.Operator) identityaccess.Operator {
	return identityaccess.Operator{
		ID: operator.ID, Email: operator.Email, DisplayName: operator.DisplayName, PasswordHash: operator.PasswordHash, Active: operator.Active,
		LastLogin: operator.LastLogin, MFAAttempts: operator.MFAAttempts, MFALockedUntil: operator.MFALockedUntil,
		CreatedAt: operator.CreatedAt, UpdatedAt: operator.UpdatedAt,
	}
}

func applyIdentityOperator(dst *platformModels.Operator, src identityaccess.Operator) {
	dst.ID = src.ID
	dst.Email = src.Email
	dst.DisplayName = src.DisplayName
	dst.PasswordHash = src.PasswordHash
	dst.Active = src.Active
	dst.LastLogin = src.LastLogin
	dst.MFAAttempts = src.MFAAttempts
	dst.MFALockedUntil = src.MFALockedUntil
	dst.CreatedAt = src.CreatedAt
	dst.UpdatedAt = src.UpdatedAt
}

func operatorModel(src identityaccess.Operator) *platformModels.Operator {
	operator := &platformModels.Operator{}
	applyIdentityOperator(operator, src)
	return operator
}
