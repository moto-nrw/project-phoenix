package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	jwx "github.com/lestrrat-go/jwx/v3/jwt"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// Login authenticates a user and returns access and refresh tokens
func (s *Service) Login(ctx context.Context, email, password string) (string, string, error) {
	return s.LoginWithAudit(ctx, email, password, "", "", "")
}

// LoginWithAudit authenticates a user and returns access and refresh tokens with audit logging.
// tenantSlug is optional: when non-empty the account is resolved to the matching tenant;
// when empty the first active tenant mapping is used (Phase 3 fallback).
func (s *Service) LoginWithAudit(ctx context.Context, email, password, ipAddress, userAgent, tenantSlug string) (string, string, error) {
	// Validate credentials and get account
	account, err := s.validateLoginCredentials(ctx, email, password, ipAddress, userAgent)
	if err != nil {
		return "", "", err
	}

	// Load account metadata first (includes tenant resolution from DB)
	// so the refresh token gets the correct tenant_id on creation.
	// Login is a public route — tenant.FromContext(ctx) would return 0.
	metadata, err := s.loadAccountMetadata(ctx, account, tenantSlug)
	if err != nil {
		return "", "", err
	}

	// Create refresh token with resolved tenant ID
	token, err := s.createRefreshTokenWithRetry(ctx, account, metadata.tenantID)
	if err != nil {
		return "", "", err
	}

	// Build JWT claims from account and metadata
	appClaims, refreshClaims := s.buildJWTClaims(account, token, metadata, email)

	// Generate token pair and log success
	return s.generateAndLogTokens(ctx, account.ID, appClaims, refreshClaims, ipAddress, userAgent, audit.EventTypeLogin)
}

// validateLoginCredentials validates email, password, and account status
func (s *Service) validateLoginCredentials(ctx context.Context, email, password, ipAddress, userAgent string) (*auth.Account, error) {
	email = strings.TrimSpace(strings.ToLower(email))

	account, err := s.repos.Account.FindByEmail(ctx, email)
	if err != nil {
		s.logFailedLogin(ctx, 0, ipAddress, userAgent, "Account not found")
		return nil, &AuthError{Op: "login", Err: ErrAccountNotFound}
	}

	if !account.Active {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "Account inactive")
		return nil, &AuthError{Op: "login", Err: ErrAccountInactive}
	}

	if err := s.verifyPassword(account, password); err != nil {
		s.logFailedLogin(ctx, account.ID, ipAddress, userAgent, "Invalid password")
		return nil, err
	}

	return account, nil
}

// verifyPassword checks if the provided password matches the account's hash
func (s *Service) verifyPassword(account *auth.Account, password string) error {
	if account.PasswordHash == nil || *account.PasswordHash == "" {
		return &AuthError{Op: "login", Err: ErrInvalidCredentials}
	}

	valid, err := userpass.VerifyPassword(password, *account.PasswordHash)
	if err != nil || !valid {
		return &AuthError{Op: "login", Err: ErrInvalidCredentials}
	}

	return nil
}

// createRefreshTokenWithRetry creates a refresh token with retry logic for concurrent logins
func (s *Service) createRefreshTokenWithRetry(ctx context.Context, account *auth.Account, tenantID int64) (*auth.Token, error) {
	token := s.newRefreshToken(account.ID)

	maxRetries := 3
	for attempt := 0; attempt < maxRetries; attempt++ {
		err := s.persistTokenInTransaction(ctx, account, token, tenantID)

		if err == nil {
			return token, nil
		}

		if !s.isTokenFamilyConflict(err) {
			return nil, &AuthError{Op: "login transaction", Err: err}
		}

		// Regenerate family ID and retry
		token.FamilyID = uuid.Must(uuid.NewV4()).String()
		s.getLogger().Warn("login race condition detected, retrying",
			slog.Int64("account_id", account.ID),
			slog.Int("attempt", attempt+1),
			slog.Int("max_retries", maxRetries))
	}

	return nil, &AuthError{Op: "login transaction", Err: fmt.Errorf("max retries exceeded")}
}

// newRefreshToken creates a new refresh token for the given account
func (s *Service) newRefreshToken(accountID int64) *auth.Token {
	identifier := "Service login"
	return &auth.Token{
		Token:      uuid.Must(uuid.NewV4()).String(),
		AccountID:  accountID,
		Expiry:     time.Now().Add(s.jwtRefreshExpiry),
		Mobile:     false,
		Identifier: &identifier,
		FamilyID:   uuid.Must(uuid.NewV4()).String(),
		Generation: 0,
	}
}

// persistTokenInTransaction saves the token and updates last login in a transaction.
//
// Uses WithAdminTx (BYPASSRLS) because this is a public login route with no JWT/tenant
// context. The phoenix_auth connection role cannot pass RLS policies on auth.tokens,
// so we switch to phoenix_admin for the token write.
func (s *Service) persistTokenInTransaction(ctx context.Context, account *auth.Account, token *auth.Token, tenantID int64) error {
	return tenant.WithAdminTx(ctx, s.db, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(*Service)

		// Clean up old tokens (keep 5 most recent)
		const maxTokensPerAccount = 5
		if err := txService.repos.Token.CleanupOldTokensForAccount(ctx, account.ID, maxTokensPerAccount); err != nil {
			s.getLogger().Warn("failed to clean up old tokens",
				slog.Int64("account_id", account.ID),
				slog.Any("error", err),
			)
		}

		// Set tenant ID from DB resolution (not from context — login is a public route)
		token.SetTenantID(tenantID)

		// Create new token
		if err := txService.repos.Token.Create(ctx, token); err != nil {
			if s.isTokenFamilyConflict(err) {
				return err // Will retry with new family ID
			}
			return err
		}

		// Update last login
		loginTime := time.Now()
		account.LastLogin = &loginTime
		return txService.repos.Account.Update(ctx, account)
	})
}

// isTokenFamilyConflict checks if error is due to token family conflict
func (s *Service) isTokenFamilyConflict(err error) bool {
	return err != nil && strings.Contains(err.Error(), "uk_tokens_family_generation")
}

// accountMetadata holds account-related metadata for JWT claims
type accountMetadata struct {
	roleNames      []string
	permissionStrs []string
	username       string
	firstName      string
	lastName       string
	isAdmin        bool
	tenantID       int64
	orgID          int64
	scope          string
}

// loadAccountMetadata loads roles, permissions, person information, and tenant mapping.
// tenantSlug is optional: when non-empty the account is resolved to the matching tenant.
// Returns partial data with logged warnings if any lookups fail.
// Returns an error only when tenantSlug is provided but resolution fails (tenant not found
// or account not mapped to that tenant).
//
// D13 revision: tenant is resolved FIRST so that roles and permissions can be scoped to
// the resolved tenant, preventing cross-tenant privilege leakage in JWT tokens.
func (s *Service) loadAccountMetadata(ctx context.Context, account *auth.Account, tenantSlug string) (*accountMetadata, error) {
	// Use WithAdminTx (BYPASSRLS) because this runs during login/switch/refresh
	// where no tenant context exists yet. RLS policies on auth.account_roles and
	// auth.account_permissions require app.current_tenant_id to be set, which only
	// happens inside tenant-scoped transactions. Without BYPASSRLS the role/permission
	// queries return zero rows and the JWT gets empty permissions.
	var result *accountMetadata
	err := tenant.WithAdminTx(ctx, s.db, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(*Service)

		// Step 1: Resolve tenant FIRST — roles/permissions depend on the target tenant.
		tenantID, orgID, err := txService.resolveAccountTenant(ctx, account.ID, tenantSlug)
		if err != nil {
			return err
		}

		// Step 2: Load roles scoped to the resolved tenant (D13 §6.1 step 6).
		txService.ensureAccountRolesLoadedForTenant(ctx, account, tenantID)

		// Step 3: Load permissions scoped to the resolved tenant (D13 §6.1 step 7).
		permissions := txService.loadAccountPermissionsForTenant(ctx, account.ID, tenantID)
		roleNames := txService.extractRoleNames(account.Roles)
		permissionStrs := txService.extractPermissionNames(permissions)

		username := txService.extractUsername(account)
		firstName, lastName := txService.loadPersonNames(ctx, account.ID)
		isAdmin := txService.checkRoleFlags(roleNames)

		result = &accountMetadata{
			roleNames:      roleNames,
			permissionStrs: permissionStrs,
			username:       username,
			firstName:      firstName,
			lastName:       lastName,
			isAdmin:        isAdmin,
			tenantID:       tenantID,
			orgID:          orgID,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ensureAccountRolesLoaded loads account roles if not already loaded.
// Uses context-based tenant filtering (for authenticated requests).
func (s *Service) ensureAccountRolesLoaded(ctx context.Context, account *auth.Account) {
	if len(account.Roles) > 0 {
		return
	}

	accountRoles, err := s.repos.AccountRole.FindByAccountID(ctx, account.ID)
	if err != nil {
		s.getLogger().Warn("failed to load roles",
			slog.Int64("account_id", account.ID),
			slog.Any("error", err),
		)
		return
	}

	for _, ar := range accountRoles {
		if ar.Role != nil {
			account.Roles = append(account.Roles, ar.Role)
		}
	}
}

// ensureAccountRolesLoadedForTenant loads account roles scoped to a specific tenant.
// Used during login/switch flows where no tenant context exists yet (D13 §6.1 step 6).
func (s *Service) ensureAccountRolesLoadedForTenant(ctx context.Context, account *auth.Account, tenantID int64) {
	// Clear any previously loaded roles to ensure fresh tenant-scoped loading
	account.Roles = nil

	accountRoles, err := s.repos.AccountRole.FindByAccountIDForTenant(ctx, account.ID, tenantID)
	if err != nil {
		s.getLogger().Warn("failed to load tenant-scoped roles",
			slog.Int64("account_id", account.ID),
			slog.Int64("tenant_id", tenantID),
			slog.Any("error", err),
		)
		return
	}

	for _, ar := range accountRoles {
		if ar.Role != nil {
			account.Roles = append(account.Roles, ar.Role)
		}
	}
}

// loadAccountPermissionsForTenant retrieves permissions scoped to a specific tenant.
// Used during login/switch flows where no tenant context exists yet (D13 §6.1 step 7).
func (s *Service) loadAccountPermissionsForTenant(ctx context.Context, accountID int64, tenantID int64) []*auth.Permission {
	permissions, err := s.repos.Permission.FindByAccountIDForTenant(ctx, accountID, tenantID)
	if err != nil {
		s.getLogger().Warn("failed to load tenant-scoped permissions",
			slog.Int64("account_id", accountID),
			slog.Int64("tenant_id", tenantID),
			slog.Any("error", err),
		)
		return []*auth.Permission{}
	}
	return permissions
}

// ensureAccountPermissionsLoaded loads account permissions if not already loaded
func (s *Service) ensureAccountPermissionsLoaded(ctx context.Context, account *auth.Account) {
	if len(account.Permissions) > 0 {
		return
	}

	permissions, err := s.getAccountPermissions(ctx, account.ID)
	if err != nil {
		s.getLogger().Warn("failed to load permissions",
			slog.Int64("account_id", account.ID),
			slog.Any("error", err),
		)
		return
	}

	account.Permissions = permissions
}

// extractRoleNames converts roles to string slice
func (s *Service) extractRoleNames(roles []*auth.Role) []string {
	roleNames := make([]string, 0, len(roles))
	for _, role := range roles {
		roleNames = append(roleNames, role.Name)
	}
	return roleNames
}

// extractPermissionNames converts permissions to string slice
func (s *Service) extractPermissionNames(permissions []*auth.Permission) []string {
	permissionStrs := make([]string, 0, len(permissions))
	for _, perm := range permissions {
		permissionStrs = append(permissionStrs, perm.GetFullName())
	}
	return permissionStrs
}

// extractUsername safely extracts username from account
func (s *Service) extractUsername(account *auth.Account) string {
	if account.Username != nil {
		return *account.Username
	}
	return ""
}

// loadPersonNames retrieves first and last name from person record
func (s *Service) loadPersonNames(ctx context.Context, accountID int64) (string, string) {
	person, err := s.repos.Person.FindByAccountID(ctx, accountID)
	if err != nil || person == nil {
		return "", ""
	}
	return person.FirstName, person.LastName
}

// checkRoleFlags determines if account has admin role
func (s *Service) checkRoleFlags(roleNames []string) bool {
	for _, roleName := range roleNames {
		if roleName == "admin" {
			return true
		}
	}
	return false
}

// resolveAccountTenant resolves the tenant ID and organization ID for an account.
// When tenantSlug is non-empty the school is looked up by subdomain and verified
// against the account's active tenant mappings. When empty, the first active
// mapping is used (Phase 3 fallback).
// Returns a non-nil error only when tenantSlug is provided but resolution fails.
func (s *Service) resolveAccountTenant(ctx context.Context, accountID int64, tenantSlug string) (int64, int64, error) {
	if tenantSlug != "" {
		return s.resolveAccountTenantBySlug(ctx, accountID, tenantSlug)
	}

	return s.resolveAccountTenantDefault(ctx, accountID)
}

// resolveAccountTenantBySlug resolves tenant by subdomain slug, then verifies
// the account has access to that tenant.
//
// Access check depends on the caller's scope (spec §6.3):
//   - Normal scope (""): check account_tenants for explicit mapping
//   - Org scope ("org"): check school.organization_id matches the caller's org_id
//     (Träger-Büro auto-access to all schools in their organization)
func (s *Service) resolveAccountTenantBySlug(ctx context.Context, accountID int64, tenantSlug string) (int64, int64, error) {
	// Look up the school by subdomain (includes soft-deleted schools so we can
	// distinguish "deleted" from "not found" and return appropriate errors).
	school, err := s.repos.School.FindBySubdomain(ctx, tenantSlug)
	if err != nil {
		s.getLogger().Warn("tenant slug lookup failed",
			slog.Int64("account_id", accountID),
			slog.String("tenant_slug", tenantSlug),
			slog.Any("error", err),
		)
		return 0, 0, &AuthError{Op: "resolve tenant", Err: err}
	}
	if school == nil {
		s.getLogger().Warn("tenant slug not found",
			slog.Int64("account_id", accountID),
			slog.String("tenant_slug", tenantSlug),
		)
		return 0, 0, &AuthError{Op: "resolve tenant", Err: ErrTenantNotFound}
	}

	if school.IsDeleted() {
		s.getLogger().Warn("tenant is soft-deleted",
			slog.Int64("account_id", accountID),
			slog.String("tenant_slug", tenantSlug),
			slog.Int64("tenant_id", school.ID),
		)
		return 0, 0, &AuthError{Op: "resolve tenant", Err: ErrTenantNotFound}
	}

	if !school.Active {
		s.getLogger().Warn("tenant is inactive",
			slog.Int64("account_id", accountID),
			slog.String("tenant_slug", tenantSlug),
			slog.Int64("tenant_id", school.ID),
		)
		return 0, 0, &AuthError{Op: "resolve tenant", Err: ErrTenantNotFound}
	}

	// Org-scope check (§6.3): Träger-Büro users access any school in their org
	// without needing an explicit account_tenants entry.
	callerScope := tenant.ScopeFromContext(ctx)
	callerOrgID := tenant.OrgFromContext(ctx)

	if callerScope == tenant.ScopeOrg && callerOrgID > 0 {
		if school.OrganizationID == callerOrgID {
			return school.ID, school.OrganizationID, nil
		}
		s.getLogger().Warn("org-scope account tried to access school outside their organization",
			slog.Int64("account_id", accountID),
			slog.Int64("school_org_id", school.OrganizationID),
			slog.Int64("caller_org_id", callerOrgID),
			slog.String("tenant_slug", tenantSlug),
		)
		return 0, 0, &AuthError{Op: "resolve tenant", Err: ErrTenantAccessDenied}
	}

	// Normal scope: verify the account has an explicit active mapping to this tenant
	exists, err := s.repos.AccountTenant.ExistsByAccountAndTenant(ctx, accountID, school.ID)
	if err != nil {
		s.getLogger().Warn("failed to verify account tenant mapping",
			slog.Int64("account_id", accountID),
			slog.Int64("tenant_id", school.ID),
			slog.Any("error", err),
		)
		return 0, 0, &AuthError{Op: "resolve tenant", Err: ErrTenantAccessDenied}
	}

	if !exists {
		s.getLogger().Warn("account does not have access to requested tenant",
			slog.Int64("account_id", accountID),
			slog.Int64("tenant_id", school.ID),
			slog.String("tenant_slug", tenantSlug),
		)
		return 0, 0, &AuthError{Op: "resolve tenant", Err: ErrTenantAccessDenied}
	}

	return school.ID, school.OrganizationID, nil
}

// resolveAccountTenantDefault resolves the tenant using the first active, non-deleted mapping
// (Phase 3 fallback). Iterates through all tenant mappings to handle cases where some schools
// are deleted or inactive. Returns (0, 0, nil) if no valid mapping is found (non-fatal).
func (s *Service) resolveAccountTenantDefault(ctx context.Context, accountID int64) (int64, int64, error) {
	tenants, err := s.repos.AccountTenant.FindActiveByAccountID(ctx, accountID)
	if err != nil || len(tenants) == 0 {
		if err != nil {
			s.getLogger().Warn("failed to resolve account tenant",
				slog.Int64("account_id", accountID),
				slog.Any("error", err),
			)
		}
		return 0, 0, nil
	}

	// Iterate all mappings — skip deleted or inactive schools, use the first valid one.
	for _, t := range tenants {
		school, err := s.repos.School.FindByID(ctx, t.TenantID)
		if err != nil {
			s.getLogger().Warn("failed to resolve school for tenant",
				slog.Int64("tenant_id", t.TenantID),
				slog.Any("error", err),
			)
			continue
		}
		if school == nil {
			continue
		}
		if school.IsDeleted() || !school.Active {
			s.getLogger().Debug("skipping deleted or inactive tenant during default resolution",
				slog.Int64("account_id", accountID),
				slog.Int64("tenant_id", t.TenantID),
				slog.Bool("deleted", school.IsDeleted()),
				slog.Bool("active", school.Active),
			)
			continue
		}
		return t.TenantID, school.OrganizationID, nil
	}

	// All mappings point to deleted/inactive schools — force explicit tenant selection.
	return 0, 0, nil
}

// buildJWTClaims constructs JWT claims from account and metadata
func (s *Service) buildJWTClaims(
	account *auth.Account,
	token *auth.Token,
	metadata *accountMetadata,
	email string,
) (jwt.AppClaims, jwt.RefreshClaims) {
	appClaims := jwt.AppClaims{
		ID:          int(account.ID),
		Sub:         email,
		Username:    metadata.username,
		FirstName:   metadata.firstName,
		LastName:    metadata.lastName,
		Roles:       metadata.roleNames,
		Permissions: metadata.permissionStrs,
		IsAdmin:     metadata.isAdmin,
		Scope:       metadata.scope,
		TenantID:    metadata.tenantID,
		OrgID:       metadata.orgID,
	}

	refreshClaims := jwt.RefreshClaims{
		ID:       int(account.ID),
		Token:    token.Token,
		TenantID: metadata.tenantID,
	}

	return appClaims, refreshClaims
}

// generateAndLogTokens generates JWT token pair and logs the authentication event
func (s *Service) generateAndLogTokens(
	ctx context.Context,
	accountID int64,
	appClaims jwt.AppClaims,
	refreshClaims jwt.RefreshClaims,
	ipAddress, userAgent, eventType string,
) (string, string, error) {
	accessToken, refreshToken, err := s.tokenAuth.GenTokenPair(appClaims, refreshClaims)
	if err != nil {
		return "", "", &AuthError{Op: "generate tokens", Err: err}
	}

	if ipAddress != "" {
		s.logAuthEvent(ctx, accountID, eventType, true, ipAddress, userAgent, "")
	}

	return accessToken, refreshToken, nil
}

// logFailedLogin logs a failed login attempt if IP address is provided
func (s *Service) logFailedLogin(ctx context.Context, accountID int64, ipAddress, userAgent, reason string) {
	if ipAddress != "" {
		s.logAuthEvent(ctx, accountID, audit.EventTypeLogin, false, ipAddress, userAgent, reason)
	}
}

// Register creates a new user account
func (s *Service) Register(ctx context.Context, email, username, password string, roleID *int64, tenantID int64) (*auth.Account, error) {
	// Validate and normalize registration inputs
	if err := s.validateRegistrationInputs(ctx, email, username, password); err != nil {
		return nil, err
	}

	if roleID != nil && *roleID > 0 && tenantID <= 0 {
		return nil, &AuthError{Op: "register", Err: ErrTenantRequiredForRoleAssignment}
	}

	// Create account object with hashed password
	account, err := s.createAccountObject(email, username, password)
	if err != nil {
		return nil, err
	}

	// Persist account and assign role in transaction
	if err := s.persistAccountWithRole(ctx, account, roleID, tenantID); err != nil {
		return nil, err
	}

	return account, nil
}

// validateRegistrationInputs validates registration data and checks for conflicts
func (s *Service) validateRegistrationInputs(ctx context.Context, email, username, password string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	username = strings.TrimSpace(username)

	if err := ValidatePasswordStrength(password); err != nil {
		return &AuthError{Op: "register", Err: err}
	}

	// Check if email already exists
	if _, err := s.repos.Account.FindByEmail(ctx, email); err == nil {
		return &AuthError{Op: "register", Err: ErrEmailAlreadyExists}
	}

	// Check if username already exists
	if _, err := s.repos.Account.FindByUsername(ctx, username); err == nil {
		return &AuthError{Op: "register", Err: ErrUsernameAlreadyExists}
	}

	return nil
}

// createAccountObject creates a new account with hashed password
func (s *Service) createAccountObject(email, username, password string) (*auth.Account, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	username = strings.TrimSpace(username)

	passwordHash, err := HashPassword(password)
	if err != nil {
		return nil, &AuthError{Op: opHashPassword, Err: err}
	}

	usernamePtr := &username
	now := time.Now()

	return &auth.Account{
		Email:        email,
		Username:     usernamePtr,
		Active:       true,
		PasswordHash: &passwordHash,
		LastLogin:    &now,
	}, nil
}

// persistAccountWithRole saves account, maps it to a tenant, and assigns a role.
// Uses WithTenantTx so that RLS on auth.account_roles enforces tenant isolation
// at the database level. phoenix_tenant has CRUD on all tables in the auth schema
// (including auth.accounts which has no RLS), so no admin escalation is needed.
// The WITH CHECK policy on auth.account_roles guarantees the inserted tenant_id
// matches the transaction's app.current_tenant_id — a code bug cannot silently
// create cross-tenant role assignments.
func (s *Service) persistAccountWithRole(ctx context.Context, account *auth.Account, roleID *int64, tenantID int64) error {
	if tenantID <= 0 {
		// No tenant context (e.g. tests) — fall back to admin tx for the account insert only.
		return tenant.WithAdminTx(ctx, s.db, func(ctx context.Context, tx bun.Tx) error {
			return s.WithTx(tx).(*Service).repos.Account.Create(ctx, account)
		})
	}

	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(*Service)

		// Create account (auth.accounts has no tenant_id, no RLS — plain INSERT)
		if err := txService.repos.Account.Create(ctx, account); err != nil {
			return err
		}

		// Map account to tenant so the user can log into this school
		now := time.Now()
		mapping := &auth.AccountTenant{
			AccountID:   account.ID,
			TenantID:    tenantID,
			Status:      auth.AccountTenantStatusActive,
			ActivatedAt: &now,
		}
		if err := txService.repos.AccountTenant.Create(ctx, mapping); err != nil {
			return fmt.Errorf("failed to create account-tenant mapping: %w", err)
		}

		// Assign role scoped to this tenant (RLS WITH CHECK enforces tenant_id match)
		if roleID != nil && *roleID > 0 {
			accountRole := &auth.AccountRole{
				AccountID: account.ID,
				RoleID:    *roleID,
			}
			accountRole.SetTenantID(tenantID)
			if err := txService.repos.AccountRole.Create(ctx, accountRole); err != nil {
				return fmt.Errorf("failed to assign role to account: %w", err)
			}
		}

		return nil
	})
}

// LinkAccountToTenant links an existing account to a tenant with an optional role assignment.
// The password is NOT changed — the user keeps their current credentials.
// Returns ErrAccountNotFound if no account exists with the given email.
// Returns ErrAccountInactive if the account is deactivated.
func (s *Service) LinkAccountToTenant(ctx context.Context, email string, roleID *int64, tenantID int64) (*auth.Account, error) {
	const op = "link-to-tenant"
	email = strings.TrimSpace(strings.ToLower(email))

	if tenantID <= 0 {
		return nil, &AuthError{Op: op, Err: ErrTenantRequiredForRoleAssignment}
	}

	// Find existing account
	account, err := s.repos.Account.FindByEmail(ctx, email)
	if err != nil {
		return nil, &AuthError{Op: op, Err: ErrAccountNotFound}
	}

	if !account.Active {
		return nil, &AuthError{Op: op, Err: ErrAccountInactive}
	}

	// Link to tenant (idempotent — handles already-linked case)
	if err := s.performAccountTenantLink(ctx, account, roleID, tenantID); err != nil {
		return nil, &AuthError{Op: op, Err: fmt.Errorf("link failed: %w", err)}
	}

	s.getLogger().Info("account linked to tenant",
		slog.Int64("account_id", account.ID),
		slog.Int64("tenant_id", tenantID))

	return account, nil
}

// performAccountTenantLink creates a tenant mapping and role assignment for an existing account.
func (s *Service) performAccountTenantLink(ctx context.Context, account *auth.Account, roleID *int64, tenantID int64) error {
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(*Service)

		if err := txService.ensureTenantMapping(ctx, account.ID, tenantID); err != nil {
			return err
		}
		return txService.ensureRoleAssignment(ctx, account.ID, roleID, tenantID)
	})
}

// ensureTenantMapping creates an account-tenant mapping if one does not already exist.
func (s *Service) ensureTenantMapping(ctx context.Context, accountID, tenantID int64) error {
	now := time.Now()
	mapping := &auth.AccountTenant{
		AccountID:   accountID,
		TenantID:    tenantID,
		Status:      auth.AccountTenantStatusActive,
		ActivatedAt: &now,
	}
	if err := s.repos.AccountTenant.Create(ctx, mapping); err != nil {
		if !isDuplicateKeyError(err) {
			return fmt.Errorf("failed to create account-tenant mapping: %w", err)
		}
	}
	return nil
}

// ensureRoleAssignment assigns a role to an account for a tenant, ignoring duplicates.
func (s *Service) ensureRoleAssignment(ctx context.Context, accountID int64, roleID *int64, tenantID int64) error {
	if roleID == nil || *roleID <= 0 {
		return nil
	}
	accountRole := &auth.AccountRole{
		AccountID: accountID,
		RoleID:    *roleID,
	}
	accountRole.SetTenantID(tenantID)
	if err := s.repos.AccountRole.Create(ctx, accountRole); err != nil {
		if !isDuplicateKeyError(err) {
			return fmt.Errorf("failed to assign role to account: %w", err)
		}
	}
	return nil
}

// isDuplicateKeyError checks if a database error is a unique constraint violation (PG code 23505).
func isDuplicateKeyError(err error) bool {
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) {
		return pgErr.Field('C') == "23505"
	}
	return false
}

// ValidateToken validates an access token and returns the associated account and parsed claims.
func (s *Service) ValidateToken(ctx context.Context, tokenString string) (*auth.Account, *jwt.AppClaims, error) {
	// Parse and validate JWT token
	jwtToken, err := s.tokenAuth.JwtAuth.Decode(tokenString)
	if err != nil {
		return nil, nil, &AuthError{Op: "validate token", Err: ErrInvalidToken}
	}

	// Extract claims
	claims := extractClaims(jwtToken)

	// Parse claims into AppClaims
	var appClaims jwt.AppClaims
	err = appClaims.ParseClaims(claims)
	if err != nil {
		return nil, nil, &AuthError{Op: "parse claims", Err: ErrInvalidToken}
	}

	// Get account by ID
	account, err := s.repos.Account.FindByID(ctx, int64(appClaims.ID))
	if err != nil {
		return nil, nil, &AuthError{Op: opGetAccount, Err: ErrAccountNotFound}
	}

	// Ensure account is active
	if !account.Active {
		return nil, nil, &AuthError{Op: "validate token", Err: ErrAccountInactive}
	}

	// Load roles and permissions scoped to the JWT's tenant (D13 revision:
	// never load cross-tenant roles/permissions, even in secondary paths).
	// Use WithTenantTx so that app.current_tenant_id is set for RLS policies.
	// ValidateToken may be called from public routes (e.g. /register) where
	// no tenant middleware exists. Without a tenant-scoped tx, RLS on
	// auth.account_roles and auth.account_permissions returns zero rows.
	if appClaims.TenantID > 0 {
		err = tenant.WithTenantTx(ctx, s.db, appClaims.TenantID, func(ctx context.Context, tx bun.Tx) error {
			txService := s.WithTx(tx).(*Service)
			txService.ensureAccountRolesLoadedForTenant(ctx, account, appClaims.TenantID)
			account.Permissions = txService.loadAccountPermissionsForTenant(ctx, account.ID, appClaims.TenantID)
			return nil
		})
		if err != nil {
			s.getLogger().Warn("failed to load roles/permissions in tenant tx",
				slog.Int64("account_id", account.ID),
				slog.Int64("tenant_id", appClaims.TenantID),
				slog.Any("error", err),
			)
		}
	} else {
		s.ensureAccountRolesLoaded(ctx, account)
		s.ensureAccountPermissionsLoaded(ctx, account)
	}

	return account, &appClaims, nil
}

// RefreshToken generates new token pair from a refresh token
func (s *Service) RefreshToken(ctx context.Context, refreshTokenStr string) (string, string, error) {
	return s.RefreshTokenWithAudit(ctx, refreshTokenStr, "", "")
}

// parseRefreshTokenClaims parses and validates JWT refresh token claims
func (s *Service) parseRefreshTokenClaims(refreshTokenStr string) (*jwt.RefreshClaims, error) {
	jwtToken, err := s.tokenAuth.JwtAuth.Decode(refreshTokenStr)
	if err != nil {
		return nil, &AuthError{Op: "parse refresh token", Err: ErrInvalidToken}
	}

	claims := extractClaims(jwtToken)

	var refreshClaims jwt.RefreshClaims
	err = refreshClaims.ParseClaims(claims)
	if err != nil {
		return nil, &AuthError{Op: "parse refresh claims", Err: ErrInvalidToken}
	}

	return &refreshClaims, nil
}

// refreshTokenInTransaction validates and refreshes token in a transaction.
//
// Uses WithAdminTx (BYPASSRLS) because token refresh is a pre-authentication flow
// with no JWT/tenant context yet. The phoenix_auth connection role cannot pass RLS
// policies on auth.tokens (same reason persistTokenInTransaction uses WithAdminTx).
func (s *Service) refreshTokenInTransaction(ctx context.Context, refreshClaims *jwt.RefreshClaims, ipAddress, userAgent string, tenantID int64) (*auth.Account, *auth.Token, error) {
	var dbToken *auth.Token
	var account *auth.Account
	var newToken *auth.Token

	err := tenant.WithAdminTx(ctx, s.db, func(ctx context.Context, tx bun.Tx) error {
		var err error

		// Fetch and validate token
		dbToken, err = s.fetchAndValidateToken(ctx, refreshClaims.Token, ipAddress, userAgent)
		if err != nil {
			return err
		}

		// Detect potential token theft
		if err := s.detectTokenTheft(ctx, dbToken, ipAddress, userAgent); err != nil {
			return err
		}

		// Fetch and validate account
		account, err = s.fetchAndValidateAccount(ctx, dbToken.AccountID, ipAddress, userAgent)
		if err != nil {
			return err
		}

		// Backward compat: pre-migration tokens have tenantID=0 in their claims.
		// Resolve from account_tenants so the rotated token gets the correct value.
		effectiveTenantID := tenantID
		if effectiveTenantID == 0 {
			effectiveTenantID, _, _ = s.resolveAccountTenant(ctx, account.ID, "")
		}

		// Create and persist new token with resolved tenant
		newToken, err = s.createAndPersistNewToken(ctx, dbToken, account.ID, effectiveTenantID)
		if err != nil {
			return err
		}

		// Update last login
		loginTime := time.Now()
		account.LastLogin = &loginTime
		return s.repos.Account.Update(ctx, account)
	})

	if err != nil {
		return nil, nil, &AuthError{Op: "refresh transaction", Err: err}
	}

	return account, newToken, nil
}

// fetchAndValidateToken retrieves token and checks expiry
func (s *Service) fetchAndValidateToken(ctx context.Context, tokenStr, ipAddress, userAgent string) (*auth.Token, error) {
	dbToken, err := s.repos.Token.FindByTokenForUpdate(ctx, tokenStr)
	if err != nil {
		return nil, &AuthError{Op: "get token", Err: ErrTokenNotFound}
	}

	if time.Now().After(dbToken.Expiry) {
		_ = s.repos.Token.Delete(ctx, dbToken.ID)
		if ipAddress != "" && dbToken.AccountID > 0 {
			s.logAuthEvent(ctx, dbToken.AccountID, audit.EventTypeTokenExpired, false, ipAddress, userAgent, "Token expired")
		}
		return nil, &AuthError{Op: "check token expiry", Err: ErrTokenExpired}
	}

	return dbToken, nil
}

// detectTokenTheft checks for token family theft detection
func (s *Service) detectTokenTheft(ctx context.Context, dbToken *auth.Token, ipAddress, userAgent string) error {
	if dbToken.FamilyID == "" {
		return nil
	}

	latestToken, err := s.repos.Token.GetLatestTokenInFamily(ctx, dbToken.FamilyID)
	if err != nil || latestToken == nil || latestToken.Generation <= dbToken.Generation {
		return nil
	}

	// Token theft detected - invalidate entire family
	_ = s.repos.Token.DeleteByFamilyID(ctx, dbToken.FamilyID)

	if ipAddress != "" {
		s.logAuthEvent(ctx, dbToken.AccountID, audit.EventTypeTokenRefresh, false, ipAddress, userAgent, "Token theft detected - family invalidated")
	}

	return &AuthError{Op: "token theft detection", Err: ErrInvalidToken}
}

// fetchAndValidateAccount retrieves account and checks if active
func (s *Service) fetchAndValidateAccount(ctx context.Context, accountID int64, ipAddress, userAgent string) (*auth.Account, error) {
	account, err := s.repos.Account.FindByID(ctx, accountID)
	if err != nil {
		return nil, &AuthError{Op: opGetAccount, Err: ErrAccountNotFound}
	}

	if !account.Active {
		if ipAddress != "" {
			s.logAuthEvent(ctx, account.ID, audit.EventTypeTokenRefresh, false, ipAddress, userAgent, "Account inactive")
		}
		return nil, &AuthError{Op: "check account status", Err: ErrAccountInactive}
	}

	return account, nil
}

// createAndPersistNewToken creates new token and deletes old one
func (s *Service) createAndPersistNewToken(ctx context.Context, oldToken *auth.Token, accountID int64, tenantID int64) (*auth.Token, error) {
	newToken := &auth.Token{
		Token:      uuid.Must(uuid.NewV4()).String(),
		AccountID:  accountID,
		Expiry:     time.Now().Add(s.jwtRefreshExpiry),
		Mobile:     oldToken.Mobile,
		Identifier: oldToken.Identifier,
		FamilyID:   oldToken.FamilyID,
		Generation: oldToken.Generation + 1,
	}

	if err := s.repos.Token.Delete(ctx, oldToken.ID); err != nil {
		return nil, err
	}

	// Set tenant ID from refresh claims (not from context — refresh is a public route)
	newToken.SetTenantID(tenantID)

	if err := s.repos.Token.Create(ctx, newToken); err != nil {
		return nil, err
	}

	return newToken, nil
}

// refreshResult carries token pair through singleflight
type refreshResult struct {
	accessToken  string
	refreshToken string
}

// RefreshTokenWithAudit generates new token pair from a refresh token with audit logging.
// Concurrent calls with the same refresh token are deduplicated via singleflight.
func (s *Service) RefreshTokenWithAudit(ctx context.Context, refreshTokenStr, ipAddress, userAgent string) (string, string, error) {
	// Use the caller's context so cancellation propagates to the DB transaction.
	// If the first caller disconnects (e.g. frontend 5s timeout), the transaction
	// rolls back and the old refresh token is preserved — callers retry safely.
	// NOT using WithoutCancel: a transient retry for shared callers is far better
	// than completing a token rotation the client never receives (permanent logout).
	sfCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	v, err, shared := s.refreshSF.Do(refreshTokenStr, func() (any, error) {
		return s.doRefreshTokenWithAudit(sfCtx, refreshTokenStr, ipAddress, userAgent)
	})
	if shared {
		s.getLogger().Info("concurrent_refresh_deduplicated", "shared", true)
	}
	if err != nil {
		return "", "", err
	}
	r := v.(*refreshResult)
	return r.accessToken, r.refreshToken, nil
}

// doRefreshTokenWithAudit performs the actual token refresh (called via singleflight)
func (s *Service) doRefreshTokenWithAudit(ctx context.Context, refreshTokenStr, ipAddress, userAgent string) (*refreshResult, error) {
	// Parse and validate refresh token claims
	refreshClaims, err := s.parseRefreshTokenClaims(refreshTokenStr)
	if err != nil {
		return nil, err
	}

	// Re-validate tenant access if tenant_id was in the refresh token
	if refreshClaims.TenantID > 0 {
		if err := s.validateTenantAccess(ctx, refreshClaims); err != nil {
			return nil, err
		}
	}

	// Validate and refresh token in transaction (pass tenant from old JWT for the new token)
	account, newToken, err := s.refreshTokenInTransaction(ctx, refreshClaims, ipAddress, userAgent, refreshClaims.TenantID)
	if err != nil {
		return nil, err
	}

	// Load account metadata (roles, permissions, person info)
	// Refresh flow uses no tenant slug — tenant is carried over from the existing token.
	metadata, err := s.loadAccountMetadata(ctx, account, "")
	if err != nil {
		return nil, err
	}

	// Build JWT claims from account and metadata
	appClaims, newRefreshClaims := s.buildJWTClaims(account, newToken, metadata, account.Email)

	// Generate token pair and log success as token refresh
	accessToken, refreshToken, err := s.generateAndLogTokens(ctx, account.ID, appClaims, newRefreshClaims, ipAddress, userAgent, audit.EventTypeTokenRefresh)
	if err != nil {
		return nil, err
	}
	return &refreshResult{accessToken: accessToken, refreshToken: refreshToken}, nil
}

// validateTenantAccess ensures the account still has active access to the tenant from the refresh token.
func (s *Service) validateTenantAccess(ctx context.Context, claims *jwt.RefreshClaims) error {
	exists, err := s.repos.AccountTenant.ExistsByAccountAndTenant(ctx, int64(claims.ID), claims.TenantID)
	if err != nil {
		s.getLogger().Warn("failed to validate tenant access during refresh",
			slog.Int("account_id", claims.ID),
			slog.Int64("tenant_id", claims.TenantID),
			slog.Any("error", err),
		)
		return &AuthError{Op: "validate tenant access", Err: ErrAccountInactive}
	}
	if !exists {
		s.getLogger().Warn("tenant access revoked, rejecting refresh",
			slog.Int("account_id", claims.ID),
			slog.Int64("tenant_id", claims.TenantID),
		)
		return &AuthError{Op: "validate tenant access", Err: fmt.Errorf("tenant access revoked")}
	}
	return nil
}

// Logout invalidates a refresh token
func (s *Service) Logout(ctx context.Context, refreshTokenStr string) error {
	return s.LogoutWithAudit(ctx, refreshTokenStr, "", "")
}

// LogoutWithAudit invalidates a refresh token with audit logging.
//
// Uses WithAdminTx (BYPASSRLS) because logout is a pre-deauthentication flow.
// auth.tokens has RLS enabled — without setting app.current_tenant_id, the
// tenant isolation policy silently filters out all rows, causing FindByToken
// to return "not found" and tokens to never actually be deleted.
func (s *Service) LogoutWithAudit(ctx context.Context, refreshTokenStr, ipAddress, userAgent string) error {
	// Parse JWT refresh token
	jwtToken, err := s.tokenAuth.JwtAuth.Decode(refreshTokenStr)
	if err != nil {
		return &AuthError{Op: "parse refresh token", Err: ErrInvalidToken}
	}

	// Extract claims
	claims := extractClaims(jwtToken)

	// Parse refresh token claims
	var refreshClaims jwt.RefreshClaims
	err = refreshClaims.ParseClaims(claims)
	if err != nil {
		return &AuthError{Op: "parse refresh claims", Err: ErrInvalidToken}
	}

	// Use WithAdminTx to bypass RLS on auth.tokens (same pattern as refreshTokenInTransaction)
	err = tenant.WithAdminTx(ctx, s.db, func(ctx context.Context, tx bun.Tx) error {
		// Get token from database to find the account ID
		dbToken, err := s.repos.Token.FindByToken(ctx, refreshClaims.Token)
		if err != nil {
			// Token not found, consider logout successful
			return nil
		}

		// Delete ALL tokens for this account to ensure complete logout
		// This ensures that all sessions (access and refresh tokens) are invalidated
		err = s.repos.Token.DeleteByAccountID(ctx, dbToken.AccountID)
		if err != nil {
			// Log the error but don't fail the logout
			s.getLogger().Warn("failed to delete all tokens during logout",
				slog.Int64("account_id", dbToken.AccountID),
				slog.Any("error", err),
			)
			// Still try to delete the specific token
			if deleteErr := s.repos.Token.Delete(ctx, dbToken.ID); deleteErr != nil {
				return &AuthError{Op: "delete token", Err: deleteErr}
			}
		}

		// Log successful logout
		if ipAddress != "" {
			s.logAuthEvent(ctx, dbToken.AccountID, audit.EventTypeLogout, true, ipAddress, userAgent, "")
		}

		return nil
	})

	return err
}

// ChangePassword updates an account's password
func (s *Service) ChangePassword(ctx context.Context, accountID int, currentPassword, newPassword string) error {
	// Get account
	account, err := s.repos.Account.FindByID(ctx, int64(accountID))
	if err != nil {
		return &AuthError{Op: opGetAccount, Err: ErrAccountNotFound}
	}

	// Verify current password
	if account.PasswordHash == nil || *account.PasswordHash == "" {
		return &AuthError{Op: "verify password", Err: ErrInvalidCredentials}
	}

	valid, err := userpass.VerifyPassword(currentPassword, *account.PasswordHash)
	if err != nil || !valid {
		return &AuthError{Op: "verify password", Err: ErrInvalidCredentials}
	}

	// Validate new password
	if err := ValidatePasswordStrength(newPassword); err != nil {
		return &AuthError{Op: "validate password", Err: err}
	}

	// Hash new password
	passwordHash, err := HashPassword(newPassword)
	if err != nil {
		return &AuthError{Op: opHashPassword, Err: err}
	}

	// Update password
	account.PasswordHash = &passwordHash
	if err := s.repos.Account.Update(ctx, account); err != nil {
		return &AuthError{Op: opUpdateAccount, Err: err}
	}

	return nil
}

// GetAccountByID retrieves an account by ID
func (s *Service) GetAccountByID(ctx context.Context, id int) (*auth.Account, error) {
	account, err := s.repos.Account.FindByID(ctx, int64(id))
	if err != nil {
		return nil, &AuthError{Op: opGetAccount, Err: ErrAccountNotFound}
	}
	return account, nil
}

// GetAccountByEmail retrieves an account by email
func (s *Service) GetAccountByEmail(ctx context.Context, email string) (*auth.Account, error) {
	// Normalize email
	email = strings.TrimSpace(strings.ToLower(email))

	account, err := s.repos.Account.FindByEmail(ctx, email)
	if err != nil {
		return nil, &AuthError{Op: "get account by email", Err: ErrAccountNotFound}
	}
	return account, nil
}

// Helper functions

// extractClaims extracts all claims from a jwt token into a map
func extractClaims(token jwx.Token) map[string]interface{} {
	claims := make(map[string]interface{})

	// Extract all claims via Keys() + Get()
	for _, k := range token.Keys() {
		var v interface{}
		if err := token.Get(k, &v); err == nil {
			claims[k] = v
		}
	}

	return claims
}

// getAccountPermissions retrieves all permissions for an account (both direct and role-based)
// Uses the repository's FindByAccountID which combines direct and role-based permissions in a single query
func (s *Service) getAccountPermissions(ctx context.Context, accountID int64) ([]*auth.Permission, error) {
	// FindByAccountID already uses a CTE to combine direct and role-based permissions
	// in a single query, avoiding N+1 queries
	return s.repos.Permission.FindByAccountID(ctx, accountID)
}
