package importpkg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	authsvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// StaffImportDeps contains the dependencies for StaffImportConfig.
type StaffImportDeps struct {
	InvitationService authsvc.InvitationService
	AccountRepo       authModels.AccountRepository
	RoleRepo          authModels.RoleRepository
	SchoolRepo        platformModels.SchoolRepository
}

// StaffImportConfig implements ImportConfig for staff (Mitarbeiter) imports.
//
// Each row is turned into an invitation via the invitation service — the
// import never reads a password or PIN from the CSV. The Person/Account/Staff/
// Teacher records are created when the invitee accepts the invitation and sets
// their own password.
type StaffImportConfig struct {
	invitationService authsvc.InvitationService
	accountRepo       authModels.AccountRepository
	roleRepo          authModels.RoleRepository
	schoolRepo        platformModels.SchoolRepository

	// roleNames is the pool of valid role names used for fuzzy suggestions
	// when a row's role cannot be resolved. Loaded in PreloadReferenceData.
	roleNames []string
	// schoolName is the tenant's display name, shown in invitation emails.
	schoolName string
}

// NewStaffImportConfig creates a new staff import configuration.
func NewStaffImportConfig(deps StaffImportDeps) *StaffImportConfig {
	return &StaffImportConfig{
		invitationService: deps.InvitationService,
		accountRepo:       deps.AccountRepo,
		roleRepo:          deps.RoleRepo,
		schoolRepo:        deps.SchoolRepo,
	}
}

// PreloadReferenceData loads the tenant's role names (for fuzzy suggestions on
// unresolved roles) and the school display name (for the invitation email).
func (c *StaffImportConfig) PreloadReferenceData(ctx context.Context) error {
	roles, err := c.roleRepo.List(ctx, map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("preload roles: %w", err)
	}
	c.roleNames = make([]string, 0, len(roles))
	for _, role := range roles {
		c.roleNames = append(c.roleNames, role.Name)
	}

	// School name is best-effort: a missing name only degrades the email text,
	// it must not abort the import.
	if c.schoolRepo != nil {
		if school, err := c.schoolRepo.FindByID(ctx, tenant.FromContext(ctx)); err == nil && school != nil {
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
	} else if _, err := mail.ParseAddress(row.Email); err != nil {
		errs = append(errs, importModels.ValidationError{
			Field:    "email",
			Message:  fmt.Sprintf("Ungültige E-Mail-Adresse: %s", row.Email),
			Code:     "invalid_email",
			Severity: importModels.ErrorSeverityError,
		})
	}

	errs = append(errs, c.validateRole(ctx, row)...)

	return errs
}

// validateRole resolves the row's role name (tenant-aware) and reports a
// blocking error with fuzzy suggestions when it cannot be resolved.
func (c *StaffImportConfig) validateRole(ctx context.Context, row *importModels.StaffImportRow) []importModels.ValidationError {
	name := strings.TrimSpace(row.RoleName)
	row.RoleName = name
	if name == "" {
		return []importModels.ValidationError{requiredFieldError("role", "Rolle ist erforderlich")}
	}

	role, err := c.roleRepo.FindByName(ctx, name)
	if err == nil && role != nil {
		row.RoleID = role.ID
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

	// Not found — offer the closest known role names as suggestions.
	suggestions := findSimilar(name, 3, func() []string { return c.roleNames })
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

// FindExisting reports whether an account already exists for the row's email.
// In create mode an existing account is reported as a duplicate so the staff
// member is not invited twice.
func (c *StaffImportConfig) FindExisting(ctx context.Context, row importModels.StaffImportRow) (*int64, error) {
	email := strings.TrimSpace(row.Email)
	if email == "" {
		return nil, nil
	}

	account, err := c.accountRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // No existing account for this email.
		}
		return nil, err
	}

	id := account.ID
	return &id, nil
}

// Create issues an invitation for the staff member. The invitation carries the
// name, role and (optional) position; Person/Account/Staff/Teacher are created
// when the invitee accepts and sets a password.
func (c *StaffImportConfig) Create(ctx context.Context, row importModels.StaffImportRow) (int64, error) {
	req := authsvc.InvitationRequest{
		Email:      row.Email,
		RoleID:     row.RoleID,
		TenantID:   tenant.FromContext(ctx),
		FirstName:  stringPtr(row.FirstName),
		LastName:   stringPtr(row.LastName),
		Position:   stringPtr(row.Position),
		CreatedBy:  ImporterIDFromContext(ctx),
		SchoolName: c.schoolName,
	}

	invitation, err := c.invitationService.CreateInvitation(ctx, req)
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
