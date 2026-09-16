package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
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
			repos:     auth.SchoolIdentityRepos{Persons: wiring.repos.persons, Staff: wiring.repos.staff, Teachers: wiring.repos.teachers, Students: wiring.repos.students},
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
	repos      auth.SchoolIdentityRepos
	directory  peopledirectory.Query
	membership schoolmembership.Query
}

func (p schoolIdentityProvisioner) HasLivePersonAtSchool(ctx context.Context, accountID int64) (bool, error) {
	person, err := p.repos.Persons.FindByAccountID(ctx, accountID)
	if err != nil {
		return false, err
	}
	return person != nil && person.DeletedAt == nil, nil
}

func (p schoolIdentityProvisioner) ListAccountPersons(ctx context.Context, accountID int64) ([]identityaccessCompose.AccountPersonIdentity, error) {
	persons, err := p.repos.Persons.List(ctx, map[string]interface{}{"account_id": accountID})
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
	student, err := p.repos.Students.FindByPersonID(ctx, personID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return student != nil, nil
}

func (p schoolIdentityProvisioner) HasLiveCaregiverProfile(ctx context.Context, accountID int64) (bool, error) {
	return auth.HasLiveCaregiverProfile(ctx, p.repos.Persons, p.repos.Staff, p.repos.Teachers, accountID)
}

func (p schoolIdentityProvisioner) EnsureSchoolIdentity(ctx context.Context, request identityaccessCompose.SchoolIdentityRequest) error {
	_, err := auth.EnsureSchoolIdentity(ctx, p.repos, auth.SchoolIdentityInput{
		AccountID: request.AccountID, TenantID: request.TenantID, Role: auth.ResolvedSchoolRole(request.Role.ID, request.Role.TenantID, request.Role.Name, request.Role.IsSystem, request.Role.BaseRole),
		FirstName: request.FirstName, LastName: request.LastName, Position: request.Position, CreatePerson: request.CreatePerson,
	})
	if errors.Is(err, auth.ErrSchoolIdentityNamesRequired) || errors.Is(err, auth.ErrSchoolIdentityPersonIsStudent) {
		return &identityaccess.InvalidInputError{Err: err}
	}
	return err
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
	return auth.RoleNeedsStaffRecord(auth.ResolvedSchoolRole(role.ID, role.TenantID, role.Name, role.IsSystem, role.BaseRole))
}

func (schoolRolePolicy) LehrkraftRoleImmutable() error { return auth.ErrLehrkraftRoleImmutable }

// --- the retained platform services' consumer-owned ports -----------------

// operatorSessions serves platform.OperatorSessions over the public module
// and translates the public outcomes back into the retained shapes the
// operator routes and the MFA and passkey exchanges render.
type operatorSessions struct {
	module operatorIdentity
}

// operatorIdentity is the public operator capability the adapter serves:
// the authentication flows plus the operator lookup that names the refused
// operator of an inactive login.
type operatorIdentity interface {
	identityaccess.OperatorAuthentication
	FindOperatorByEmail(ctx context.Context, email string) (identityaccess.Operator, error)
}

func newOperatorSessions(module operatorIdentity) platform.OperatorSessions {
	return operatorSessions{module: module}
}

func (s operatorSessions) LoginWithMFAGate(ctx context.Context, email, password, ipAddress, userAgent, trustedDeviceCookie string) (*platform.OperatorLoginResult, error) {
	result, err := s.module.LoginOperatorWithMFAGate(ctx, email, password, ipAddress, userAgent, trustedDeviceCookie)
	if err != nil {
		// The retained inactive-operator error names the operator; the
		// public refusal does not, so the verified e-mail resolves it.
		var operatorID int64
		if errors.Is(err, identityaccess.ErrOperatorInactive) {
			if operator, lookupErr := s.module.FindOperatorByEmail(ctx, email); lookupErr == nil {
				operatorID = operator.ID
			}
		}
		return nil, retainedOperatorError(err, operatorID)
	}
	if result == nil {
		return nil, nil
	}
	retained := &platform.OperatorLoginResult{
		Status:      platform.OperatorLoginStatus(result.Status),
		AccessToken: result.AccessToken, RefreshToken: result.RefreshToken,
		ChallengeToken: result.ChallengeToken, MaskedEmail: result.MaskedEmail,
		MFAEnrollmentRequired: result.MFAEnrollmentRequired,
		TrustedDeviceEnabled:  result.TrustedDeviceEnabled, TrustedDeviceDays: result.TrustedDeviceDays,
	}
	if result.Operator != nil {
		retained.Operator = operatorModel(*result.Operator)
	}
	return retained, nil
}

func (s operatorSessions) IssueTokensForAuthenticatedOperator(ctx context.Context, operatorID int64, ipAddress, userAgent string) (string, string, error) {
	access, refresh, err := s.module.IssueTokensForAuthenticatedOperator(ctx, operatorID, ipAddress, userAgent)
	return access, refresh, retainedOperatorError(err, operatorID)
}

func (s operatorSessions) RefreshToken(ctx context.Context, operatorID int64, refreshTokenValue string) (string, string, error) {
	access, refresh, err := s.module.RefreshOperatorToken(ctx, operatorID, refreshTokenValue)
	return access, refresh, retainedOperatorError(err, operatorID)
}

func (s operatorSessions) UpdateProfile(ctx context.Context, operatorID int64, displayName string) (*platformModels.Operator, error) {
	operator, err := s.module.UpdateOperatorProfile(ctx, operatorID, displayName)
	if err != nil {
		return nil, retainedOperatorError(err, operatorID)
	}
	return operatorModel(operator), nil
}

func (s operatorSessions) ChangePassword(ctx context.Context, operatorID int64, currentPassword, newPassword string) error {
	return retainedOperatorError(s.module.ChangeOperatorPassword(ctx, operatorID, currentPassword, newPassword), operatorID)
}

// retainedOperatorError maps the public operator outcomes onto the error
// types the retained operator contract has always returned.
func retainedOperatorError(err error, operatorID int64) error {
	if err == nil {
		return nil
	}
	if invalid, ok := errors.AsType[*identityaccess.InvalidInputError](err); ok {
		return &platform.InvalidDataError{Err: invalid.Err}
	}
	switch {
	case errors.Is(err, identityaccess.ErrOperatorInvalidCredentials):
		return &platform.InvalidCredentialsError{}
	case errors.Is(err, identityaccess.ErrOperatorNotFound):
		return &platform.OperatorNotFoundError{OperatorID: operatorID}
	case errors.Is(err, identityaccess.ErrOperatorInactive):
		return &platform.OperatorInactiveError{OperatorID: operatorID}
	case errors.Is(err, identityaccess.ErrOperatorRefreshTokenInvalid):
		return &platform.OperatorRefreshTokenInvalidError{}
	case errors.Is(err, identityaccess.ErrOperatorPasswordMismatch):
		return &platform.PasswordMismatchError{}
	default:
		return err
	}
}

// operatorAccountAccess serves platform.OperatorAccountAccess over the
// public module and translates its outcomes into the retained shapes the
// operator school-access routes render.
type operatorAccountAccess struct {
	module identityaccess.OperatorAccountAccess
}

func newOperatorAccountAccess(module identityaccess.OperatorAccountAccess) platform.OperatorAccountAccess {
	return operatorAccountAccess{module: module}
}

func (a operatorAccountAccess) ListAccountTenantAccess(ctx context.Context, accountID int64) ([]platform.AccountTenantAccessEntry, error) {
	entries, err := a.module.ListAccountTenantAccess(ctx, accountID)
	return a.result(entries, err, accountID, 0)
}

func (a operatorAccountAccess) ListAssignableSchoolRoles(ctx context.Context, schoolID int64) ([]platform.AccountTenantRole, error) {
	roles, err := a.module.ListAssignableSchoolRoles(ctx, schoolID)
	if err != nil {
		return nil, retainedAccountAccessError(err, 0, schoolID)
	}
	return retainedTenantRoles(roles), nil
}

func (a operatorAccountAccess) GrantAccountTenantAccess(ctx context.Context, accountID, schoolID int64, req platform.GrantAccountTenantAccessRequest, operatorID int64, clientIP net.IP) ([]platform.AccountTenantAccessEntry, error) {
	entries, err := a.module.GrantAccountTenantAccess(ctx, identityaccess.GrantAccountTenantAccess{
		AccountID: accountID, SchoolID: schoolID, RoleID: req.RoleID,
		FirstName: req.FirstName, LastName: req.LastName, Position: req.Position,
		OperatorID: operatorID, ClientIP: clientIPString(clientIP),
	})
	return a.result(entries, err, accountID, schoolID)
}

func (a operatorAccountAccess) UpdateAccountTenantRole(ctx context.Context, accountID, schoolID, roleID, operatorID int64, clientIP net.IP) ([]platform.AccountTenantAccessEntry, error) {
	entries, err := a.module.UpdateAccountTenantRole(ctx, accountID, schoolID, roleID, operatorID, clientIPString(clientIP))
	return a.result(entries, err, accountID, schoolID)
}

func (a operatorAccountAccess) RevokeAccountTenantAccess(ctx context.Context, accountID, schoolID, operatorID int64, clientIP net.IP) ([]platform.AccountTenantAccessEntry, error) {
	entries, err := a.module.RevokeAccountTenantAccess(ctx, accountID, schoolID, operatorID, clientIPString(clientIP))
	return a.result(entries, err, accountID, schoolID)
}

func (operatorAccountAccess) result(entries []identityaccess.AccountTenantAccess, err error, accountID, schoolID int64) ([]platform.AccountTenantAccessEntry, error) {
	if err != nil {
		return nil, retainedAccountAccessError(err, accountID, schoolID)
	}
	result := make([]platform.AccountTenantAccessEntry, 0, len(entries))
	for _, entry := range entries {
		var retained platform.AccountTenantAccessEntry
		retained.TenantID = entry.TenantID
		retained.SchoolName = entry.SchoolName
		retained.SchoolSlug = entry.SchoolSlug
		retained.SchoolActive = entry.SchoolActive
		retained.OrganizationID = entry.OrganizationID
		retained.OrganizationName = entry.OrganizationName
		retained.Status = entry.Status
		retained.ActivatedAt = entry.ActivatedAt
		retained.DeactivatedAt = entry.DeactivatedAt
		retained.HasPerson = entry.HasPerson
		retained.HasStaff = entry.HasStaff
		retained.Roles = retainedTenantRoles(entry.Roles)
		result = append(result, retained)
	}
	return result, nil
}

// retainedTenantRoles keeps the retained wire shape: an entry without roles
// renders an empty list, never null.
func retainedTenantRoles(roles []identityaccess.AccountTenantRole) []platform.AccountTenantRole {
	result := make([]platform.AccountTenantRole, 0, len(roles))
	for _, role := range roles {
		result = append(result, platform.AccountTenantRole{ID: role.ID, Name: role.Name, IsSystem: role.IsSystem, BaseRole: role.BaseRole})
	}
	return result
}

// retainedAccountAccessError maps the public account access outcomes onto
// the error types the retained provisioning contract has always returned.
func retainedAccountAccessError(err error, accountID, schoolID int64) error {
	if invalid, ok := errors.AsType[*identityaccess.InvalidInputError](err); ok {
		return &platform.InvalidDataError{Err: invalid.Err}
	}
	switch {
	case errors.Is(err, identityaccess.ErrAccountNotFound):
		return &platform.AccountNotFoundError{AccountID: accountID}
	case errors.Is(err, identityaccess.ErrAccountTenantAccessNotFound):
		return &platform.AccountTenantAccessNotFoundError{AccountID: accountID, SchoolID: schoolID}
	case errors.Is(err, identityaccess.ErrAccountTenantAccessExists):
		return &platform.ConflictError{Err: errors.New("account already has access to this school")}
	case errors.Is(err, identityaccess.ErrSchoolNotFound):
		return &platform.SchoolNotFoundError{SchoolID: schoolID}
	case errors.Is(err, identityaccess.ErrSchoolDeleted):
		return &platform.SchoolAlreadyDeletedError{SchoolID: schoolID}
	default:
		return err
	}
}

// clientIPString renders the request address for the audit entries; an
// absent address stays empty, as the retained audit log stored it.
func clientIPString(ip net.IP) string {
	if ip == nil {
		return ""
	}
	return ip.String()
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
