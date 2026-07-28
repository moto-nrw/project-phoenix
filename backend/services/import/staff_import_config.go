package importpkg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	authsvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// systemRoleDisplayNames maps system role raw names to their German display
// names. This mirrors SYSTEM_ROLE_TRANSLATIONS in the frontend
// (frontend/src/lib/auth-helpers.ts) so the staff import accepts the same
// names a user sees on the roles page. Tenant-specific custom roles have no
// translation — their raw name is also their display name.
var systemRoleDisplayNames = map[string]string{
	"admin":    "Administrator",
	"user":     "Betreuer",
	"guest":    "Gast",
	"guardian": "Erziehungsberechtigter",
}

// displayNameToRawRole is the reverse lookup (lowercased display name → raw name).
var displayNameToRawRole = func() map[string]string {
	m := make(map[string]string, len(systemRoleDisplayNames))
	for raw, display := range systemRoleDisplayNames {
		m[strings.ToLower(display)] = raw
	}
	return m
}()

// roleDisplayName returns the German display name for a role's raw name, or the
// raw name itself for roles without a translation (e.g. custom tenant roles).
func roleDisplayName(rawName string) string {
	if display, ok := systemRoleDisplayNames[strings.ToLower(rawName)]; ok {
		return display
	}
	return rawName
}

// StaffImportDeps contains the dependencies for StaffImportConfig.
type StaffImportDeps struct {
	InvitationService authsvc.InvitationService
	AccountRepo       authModels.AccountRepository
	AccountTenantRepo authModels.AccountTenantRepository
	RoleRepo          authModels.RoleRepository
	PermissionRepo    authModels.PermissionRepository
	SchoolRepo        platformModels.SchoolRepository
}

// importerPermissionsKey keeps the authenticated importer's permissions
// available while the generic import service processes individual rows.
type importerPermissionsKey struct{}

// ContextWithImporterPermissions stores the authenticated importer's
// permissions for staff invitation authorization.
func ContextWithImporterPermissions(ctx context.Context, permissions []string) context.Context {
	return context.WithValue(ctx, importerPermissionsKey{}, permissions)
}

// ImporterPermissionsFromContext returns the authenticated importer's
// permissions, or nil when no authenticated importer was supplied.
func ImporterPermissionsFromContext(ctx context.Context) []string {
	permissions, _ := ctx.Value(importerPermissionsKey{}).([]string)
	return permissions
}

// StaffImportConfig implements ImportConfig for staff (Mitarbeiter) imports.
//
// Each row is turned into an invitation via the invitation service — the
// import never reads a password or PIN from the CSV. The Person/Account/Staff/
// Teacher records are created when the invitee accepts the invitation and sets
// their own password.
type StaffImportConfig struct {
	StaffImportDeps

	// roleDisplayNames is the pool of role display names used for fuzzy
	// suggestions when a row's role cannot be resolved. Loaded in
	// PreloadReferenceData. Display names are shown so suggestions match the
	// roles page and the import page.
	roleDisplayNames []string
	// schoolName is the tenant's display name, shown in invitation emails.
	schoolName string
}

// NewStaffImportConfig creates a new staff import configuration.
func NewStaffImportConfig(deps StaffImportDeps) *StaffImportConfig {
	return &StaffImportConfig{StaffImportDeps: deps}
}

// PreloadReferenceData loads the tenant's role names (for fuzzy suggestions on
// unresolved roles) and the school display name (for the invitation email).
func (c *StaffImportConfig) PreloadReferenceData(ctx context.Context) error {
	roles, err := c.RoleRepo.List(ctx, map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("preload roles: %w", err)
	}
	c.roleDisplayNames = make([]string, 0, len(roles))
	for _, role := range roles {
		c.roleDisplayNames = append(c.roleDisplayNames, roleDisplayName(role.Name))
	}

	// School name is best-effort: a missing name only degrades the email text,
	// it must not abort the import.
	if c.SchoolRepo != nil {
		if school, err := c.SchoolRepo.FindByID(ctx, tenant.FromContext(ctx)); err == nil && school != nil {
			c.schoolName = school.Name
		}
	}
	return nil
}

// Validate validates a single staff row and resolves its role.
func (c *StaffImportConfig) Validate(ctx context.Context, row *importModels.StaffImportRow) []importModels.ValidationError {
	var errs []importModels.ValidationError

	row.FirstName = strings.TrimSpace(row.FirstName)
	row.LastName = strings.TrimSpace(row.LastName)
	row.Email = strings.TrimSpace(row.Email)
	row.Position = strings.TrimSpace(row.Position)

	if row.FirstName == "" {
		errs = append(errs, requiredFieldError("first_name", "Vorname ist erforderlich"))
	}
	if row.LastName == "" {
		errs = append(errs, requiredFieldError("last_name", "Nachname ist erforderlich"))
	}
	if row.Email == "" {
		errs = append(errs, requiredFieldError("email", "E-Mail ist erforderlich"))
	} else if normalized, err := normalizeStaffEmail(row.Email); err != nil {
		errs = append(errs, importModels.ValidationError{
			Field:    "email",
			Message:  fmt.Sprintf("Ungültige E-Mail-Adresse: %s", row.Email),
			Code:     "invalid_email",
			Severity: importModels.ErrorSeverityError,
		})
	} else {
		row.Email = normalized
	}

	errs = append(errs, c.validateRole(ctx, row)...)

	return errs
}

// ValidateBatch validates invariants that can only be checked across the full
// uploaded file.
func (c *StaffImportConfig) ValidateBatch(_ context.Context, rows []importModels.StaffImportRow) map[int][]importModels.ValidationError {
	seen := make(map[string]int, len(rows))
	errs := make(map[int][]importModels.ValidationError)
	for i, row := range rows {
		email, err := normalizeStaffEmail(row.Email)
		if err != nil || email == "" {
			continue
		}
		key := strings.ToLower(email)
		if firstRow, ok := seen[key]; ok {
			errs[i] = append(errs[i], importModels.ValidationError{
				Field:       "email",
				Message:     fmt.Sprintf("E-Mail '%s' ist doppelt in der Importdatei vorhanden (erste Zeile: %d).", email, firstRow+2),
				Code:        "duplicate_in_file",
				Severity:    importModels.ErrorSeverityError,
				ActualValue: email,
			})
			continue
		}
		seen[key] = i
	}
	return errs
}

func normalizeStaffEmail(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	addr, err := mail.ParseAddress(trimmed)
	if err != nil {
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(addr.Address)), nil
}

// validateRole resolves the row's role name (tenant-aware) and reports a
// blocking error with fuzzy suggestions when it cannot be resolved.
func (c *StaffImportConfig) validateRole(ctx context.Context, row *importModels.StaffImportRow) []importModels.ValidationError {
	name := strings.TrimSpace(row.RoleName)
	row.RoleName = name
	if name == "" {
		return []importModels.ValidationError{requiredFieldError("role", "Rolle ist erforderlich")}
	}

	// Accept either the raw role name (e.g. "user") or the German display name
	// shown on the roles page (e.g. "Betreuer"). FindByName matches the raw
	// name case-insensitively, so translate a display name back first.
	lookup := name
	if raw, ok := displayNameToRawRole[strings.ToLower(name)]; ok {
		lookup = raw
	}

	role, err := c.RoleRepo.FindByName(ctx, lookup)
	if err == nil && role != nil {
		row.RoleID = role.ID
		if c.PermissionRepo != nil {
			role.Permissions, err = c.PermissionRepo.FindByRoleID(ctx, role.ID)
			if err != nil {
				return []importModels.ValidationError{{
					Field:    "role",
					Message:  fmt.Sprintf("Rolle konnte nicht geprüft werden: %s", err.Error()),
					Code:     "role_lookup_failed",
					Severity: importModels.ErrorSeverityError,
				}}
			}
		}
		if !authorize.CanGrantRole(role, ImporterPermissionsFromContext(ctx)) {
			return []importModels.ValidationError{{
				Field:    "role",
				Message:  "Du darfst diese Rolle nicht vergeben",
				Code:     "role_grant_not_permitted",
				Severity: importModels.ErrorSeverityError,
			}}
		}
		return nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return []importModels.ValidationError{{
			Field:    "role",
			Message:  fmt.Sprintf("Rolle konnte nicht geprüft werden: %s", err.Error()),
			Code:     "role_lookup_failed",
			Severity: importModels.ErrorSeverityError,
		}}
	}

	// Not found — offer the closest known role display names as suggestions.
	suggestions := findSimilar(name, 3, func() []string { return c.roleDisplayNames })
	verr := importModels.ValidationError{
		Field:       "role",
		Code:        "role_not_found",
		Severity:    importModels.ErrorSeverityError,
		ActualValue: name,
	}
	if len(suggestions) > 0 {
		verr.Message = fmt.Sprintf("Rolle '%s' nicht gefunden. Meinten Sie: %s?", name, strings.Join(suggestions, ", "))
		verr.Suggestions = suggestions
		verr.AutoFix = &importModels.AutoFix{
			Action:      "replace",
			Replacement: suggestions[0],
			Description: fmt.Sprintf("Automatisch zu '%s' ändern", suggestions[0]),
		}
	} else {
		verr.Message = fmt.Sprintf("Rolle '%s' existiert nicht.", name)
	}
	return []importModels.ValidationError{verr}
}

// FindExisting reports whether the row's email already has access to this
// tenant. A globally existing account in another tenant is not a duplicate:
// CreateInvitation supports inviting that account into the current tenant.
func (c *StaffImportConfig) FindExisting(ctx context.Context, row importModels.StaffImportRow) (*int64, error) {
	email, err := normalizeStaffEmail(row.Email)
	if err != nil || email == "" {
		return nil, nil
	}

	account, err := c.AccountRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No existing account for this email.
		}
		return nil, err
	}

	exists, err := c.AccountTenantRepo.ExistsByAccountAndTenant(ctx, account.ID, tenant.FromContext(ctx))
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, nil
	}

	id := account.ID
	return &id, nil
}

// Create issues an invitation for the staff member. The invitation carries the
// name, role and (optional) position; Person/Account/Staff/Teacher are created
// when the invitee accepts and sets a password.
func (c *StaffImportConfig) Create(ctx context.Context, row importModels.StaffImportRow) (int64, error) {
	email, err := normalizeStaffEmail(row.Email)
	if err != nil {
		return 0, err
	}
	req := authsvc.InvitationRequest{
		Email:            email,
		RoleID:           row.RoleID,
		TenantID:         tenant.FromContext(ctx),
		FirstName:        strutil.TrimToNil(row.FirstName),
		LastName:         strutil.TrimToNil(row.LastName),
		Position:         strutil.TrimToNil(row.Position),
		CreatedBy:        ImporterIDFromContext(ctx),
		SchoolName:       c.schoolName,
		ActorPermissions: ImporterPermissionsFromContext(ctx),
	}

	invitation, err := c.InvitationService.CreateInvitation(ctx, req)
	if err != nil {
		return 0, err
	}
	return invitation.ID, nil
}

// Update is a no-op: existing accounts are skipped (create-only import).
func (c *StaffImportConfig) Update(_ context.Context, _ int64, _ importModels.StaffImportRow) error {
	return nil
}

// EntityName returns the entity type name for logging and error messages.
func (c *StaffImportConfig) EntityName() string {
	return "Mitarbeiter"
}

// requiredFieldError builds a blocking "required field" validation error.
func requiredFieldError(field, message string) importModels.ValidationError {
	return importModels.ValidationError{
		Field:    field,
		Message:  message,
		Code:     "required",
		Severity: importModels.ErrorSeverityError,
	}
}
