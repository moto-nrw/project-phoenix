package importpkg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
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

// genderAliases maps the German (and English) spellings accepted in the
// "Geschlecht" column to the stored values.
var genderAliases = map[string]string{
	"w": userModels.GenderFemale, "weiblich": userModels.GenderFemale, "female": userModels.GenderFemale, "f": userModels.GenderFemale,
	"m": userModels.GenderMale, "männlich": userModels.GenderMale, "maennlich": userModels.GenderMale, "male": userModels.GenderMale,
	"d": userModels.GenderDiverse, "divers": userModels.GenderDiverse, "diverse": userModels.GenderDiverse,
}

// employmentTypeAliases maps the "Beschäftigungsart" column to users.staff.employment_type.
var employmentTypeAliases = map[string]string{
	"vollzeit": userModels.EmploymentTypeFullTime, "full_time": userModels.EmploymentTypeFullTime, "fulltime": userModels.EmploymentTypeFullTime,
	"teilzeit": userModels.EmploymentTypePartTime, "part_time": userModels.EmploymentTypePartTime, "parttime": userModels.EmploymentTypePartTime,
	"minijob": userModels.EmploymentTypeMinijob, "geringfügig": userModels.EmploymentTypeMinijob, "geringfuegig": userModels.EmploymentTypeMinijob,
}

// StaffImportDeps contains the dependencies for StaffImportConfig.
type StaffImportDeps struct {
	InvitationService authsvc.InvitationService
	InvitationRepo    authModels.InvitationTokenRepository
	AccountRepo       authModels.AccountRepository
	AccountTenantRepo authModels.AccountTenantRepository
	RoleRepo          authModels.RoleRepository
	PermissionRepo    authModels.PermissionRepository
	SchoolRepo        platformModels.SchoolRepository

	// Stammdaten targets (#2600)
	PersonRepo        userModels.PersonRepository
	StaffRepo         userModels.StaffRepository
	TeacherRepo       userModels.TeacherRepository
	MasterDataRepo    userModels.StaffMasterDataRepository
	QualificationRepo userModels.StaffQualificationRepository
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
// Each row is a full Stammdatensatz (#2600): Create files Person, Staff, the
// caregiver profile the role calls for, the master data and the
// qualifications right away, so the person is in the staff list before
// anyone has clicked an e-mail. A row with an e-mail address additionally
// issues a portal invitation that remembers the person; accepting it links
// the new account to that person instead of filing a second one. The import
// never reads a password or PIN from the file.
type StaffImportConfig struct {
	StaffImportDeps

	// roleDisplayNames is the pool of role display names used for fuzzy
	// suggestions when a row's role cannot be resolved. Loaded in
	// PreloadReferenceData. Display names are shown so suggestions match the
	// roles page and the import page.
	roleDisplayNames []string
	// rolesByID keeps the resolved roles so Create can decide whether the row
	// needs a caregiver profile without a second lookup.
	rolesByID map[int64]*authModels.Role
	// schoolName is the tenant's display name, shown in invitation emails.
	schoolName string

	// Existing staff of the tenant, keyed for FindExisting: by lowercased
	// personnel number and by lowercased "vorname|nachname".
	staffByPersonnelNumber map[string]*userModels.Staff
	staffByName            map[string][]*userModels.Staff
}

// NewStaffImportConfig creates a new staff import configuration.
func NewStaffImportConfig(deps StaffImportDeps) *StaffImportConfig {
	return &StaffImportConfig{StaffImportDeps: deps}
}

// PreloadReferenceData loads the tenant's role names (for fuzzy suggestions on
// unresolved roles), the school display name (for the invitation email) and the
// existing staff (for duplicate detection and update matching).
func (c *StaffImportConfig) PreloadReferenceData(ctx context.Context) error {
	roles, err := c.RoleRepo.List(ctx, map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("preload roles: %w", err)
	}
	c.roleDisplayNames = make([]string, 0, len(roles))
	c.rolesByID = make(map[int64]*authModels.Role, len(roles))
	for _, role := range roles {
		c.roleDisplayNames = append(c.roleDisplayNames, roleDisplayName(role.Name))
		c.rolesByID[role.ID] = role
	}

	// School name is best-effort: a missing name only degrades the email text,
	// it must not abort the import.
	if c.SchoolRepo != nil {
		if school, err := c.SchoolRepo.FindByID(ctx, tenant.FromContext(ctx)); err == nil && school != nil {
			c.schoolName = school.Name
		}
	}

	c.staffByPersonnelNumber = make(map[string]*userModels.Staff)
	c.staffByName = make(map[string][]*userModels.Staff)
	if c.StaffRepo == nil {
		return nil
	}
	existing, err := c.StaffRepo.ListAllWithPerson(ctx)
	if err != nil {
		return fmt.Errorf("preload staff: %w", err)
	}
	for _, staff := range existing {
		if staff.PersonnelNumber != nil {
			if pn := strings.ToLower(strings.TrimSpace(*staff.PersonnelNumber)); pn != "" {
				c.staffByPersonnelNumber[pn] = staff
			}
		}
		if staff.Person != nil {
			key := staffNameKey(staff.Person.FirstName, staff.Person.LastName)
			c.staffByName[key] = append(c.staffByName[key], staff)
		}
	}
	return nil
}

// Validate validates a single staff row, resolves its role and normalizes the
// enumerated and date columns to their stored spelling.
func (c *StaffImportConfig) Validate(ctx context.Context, row *importModels.StaffImportRow) []importModels.ValidationError {
	var errs []importModels.ValidationError

	trimStaffRow(row)
	requiresCreateFields := true
	if mode := importModeFromContext(ctx); mode == importModels.ImportModeUpdate || mode == importModels.ImportModeUpsert {
		existing, findErr := c.FindExisting(ctx, *row)
		if findErr != nil {
			return []importModels.ValidationError{{Field: "staff", Message: fmt.Sprintf("Mitarbeiter konnte nicht geprüft werden: %s", findErr.Error()), Code: "existing_lookup_failed", Severity: importModels.ErrorSeverityError}}
		}
		requiresCreateFields = existing == nil
	}

	if requiresCreateFields && row.FirstName == "" {
		errs = append(errs, requiredFieldError("first_name", "Vorname ist erforderlich"))
	}
	if requiresCreateFields && row.LastName == "" {
		errs = append(errs, requiredFieldError("last_name", "Nachname ist erforderlich"))
	}
	if row.Email != "" {
		if normalized, err := normalizeStaffEmail(row.Email); err != nil {
			errs = append(errs, importModels.ValidationError{
				Field:    "email",
				Message:  fmt.Sprintf("Ungültige E-Mail-Adresse: %s", row.Email),
				Code:     "invalid_email",
				Severity: importModels.ErrorSeverityError,
			})
		} else {
			row.Email = normalized
		}
	} else {
		errs = append(errs, importModels.ValidationError{
			Field:    "email",
			Message:  "Keine E-Mail-Adresse: Die Person wird ohne Zugang angelegt. Eine Einladung kann später über die Personalverwaltung verschickt werden.",
			Code:     "no_login",
			Severity: importModels.ErrorSeverityWarning,
		})
	}

	if requiresCreateFields {
		errs = append(errs, c.validateRole(ctx, row)...)
	}
	errs = append(errs, validateStaffMasterFields(row)...)

	return errs
}

func trimStaffRow(row *importModels.StaffImportRow) {
	row.FirstName = strings.TrimSpace(row.FirstName)
	row.LastName = strings.TrimSpace(row.LastName)
	row.Email = strings.TrimSpace(row.Email)
	row.Position = strings.TrimSpace(row.Position)
	row.Birthday = strings.TrimSpace(row.Birthday)
	row.Gender = strings.TrimSpace(row.Gender)
	row.StaffNotes = strings.TrimSpace(row.StaffNotes)
	row.PersonnelNumber = strings.TrimSpace(row.PersonnelNumber)
	row.EmploymentType = strings.TrimSpace(row.EmploymentType)
	row.EntryDate = strings.TrimSpace(row.EntryDate)
	row.ContractEndDate = strings.TrimSpace(row.ContractEndDate)
	row.ProbationEndDate = strings.TrimSpace(row.ProbationEndDate)
	row.WeeklyHours = strings.TrimSpace(row.WeeklyHours)
	row.AddressStreet = strings.TrimSpace(row.AddressStreet)
	row.AddressPostalCode = strings.TrimSpace(row.AddressPostalCode)
	row.AddressCity = strings.TrimSpace(row.AddressCity)
	row.Phone = strings.TrimSpace(row.Phone)
	row.ContactEmail = strings.TrimSpace(row.ContactEmail)
	row.EmergencyContactName = strings.TrimSpace(row.EmergencyContactName)
	row.EmergencyContactPhone = strings.TrimSpace(row.EmergencyContactPhone)
	row.Qualifications = strings.TrimSpace(row.Qualifications)
}

// validateStaffMasterFields checks the Stammdaten columns and rewrites them to
// their canonical spelling (ISO dates, stored enum values).
func validateStaffMasterFields(row *importModels.StaffImportRow) []importModels.ValidationError {
	var errs []importModels.ValidationError

	if row.Gender != "" {
		if mapped, ok := genderAliases[strings.ToLower(row.Gender)]; ok {
			row.Gender = mapped
		} else {
			errs = append(errs, importModels.ValidationError{
				Field:       "gender",
				Message:     fmt.Sprintf("Unbekanntes Geschlecht '%s'. Erlaubt: w, m, d (weiblich, männlich, divers).", row.Gender),
				Code:        "invalid_gender",
				Severity:    importModels.ErrorSeverityError,
				ActualValue: row.Gender,
			})
		}
	}

	if row.EmploymentType != "" {
		if mapped, ok := employmentTypeAliases[strings.ToLower(row.EmploymentType)]; ok {
			row.EmploymentType = mapped
		} else {
			errs = append(errs, importModels.ValidationError{
				Field:       "employment_type",
				Message:     fmt.Sprintf("Unbekannte Beschäftigungsart '%s'. Erlaubt: Vollzeit, Teilzeit, Minijob.", row.EmploymentType),
				Code:        "invalid_employment_type",
				Severity:    importModels.ErrorSeverityError,
				ActualValue: row.EmploymentType,
			})
		}
	}

	dateField := func(value *string, field, label string) *timezone.Date {
		if *value == "" {
			return nil
		}
		parsed, err := parseDateFormats(*value)
		if err != nil {
			errs = append(errs, importModels.ValidationError{
				Field:    field,
				Message:  fmt.Sprintf("Ungültiges Datumsformat für '%s'. Bitte verwenden Sie JJJJ-MM-TT, TT.MM.JJJJ oder TT.MM.JJ.", label),
				Code:     "invalid_date_format",
				Severity: importModels.ErrorSeverityError,
			})
			return nil
		}
		*value = parsed.Format("2006-01-02")
		d := timezone.DateFromTime(parsed)
		return &d
	}

	if row.Birthday != "" {
		if _, err := parseSupportedDate(row.Birthday); err != nil {
			message := "Ungültiges Datumsformat für 'Geburtstag'. Bitte verwenden Sie JJJJ-MM-TT, TT.MM.JJJJ oder TT.MM.JJ."
			if errors.Is(err, errFutureBirthday) {
				message = "Ungültiges Geburtsdatum. Geburtstage in der Zukunft sind nicht erlaubt."
			}
			errs = append(errs, importModels.ValidationError{
				Field:    "birthday",
				Message:  message,
				Code:     "invalid_date",
				Severity: importModels.ErrorSeverityError,
			})
		} else {
			dateField(&row.Birthday, "birthday", "Geburtstag")
		}
	}

	entry := dateField(&row.EntryDate, "entry_date", "Eintritt")
	contractEnd := dateField(&row.ContractEndDate, "contract_end_date", "Vertragsende")
	probationEnd := dateField(&row.ProbationEndDate, "probation_end_date", "Probezeit bis")
	if entry != nil && contractEnd != nil && contractEnd.Before(*entry) {
		errs = append(errs, importModels.ValidationError{
			Field:    "contract_end_date",
			Message:  "'Vertragsende' darf nicht vor 'Eintritt' liegen.",
			Code:     "invalid_date_range",
			Severity: importModels.ErrorSeverityError,
		})
	}
	if entry != nil && probationEnd != nil && probationEnd.Before(*entry) {
		errs = append(errs, importModels.ValidationError{
			Field:    "probation_end_date",
			Message:  "'Probezeit bis' darf nicht vor 'Eintritt' liegen.",
			Code:     "invalid_date_range",
			Severity: importModels.ErrorSeverityError,
		})
	}

	if row.WeeklyHours != "" {
		hours, err := parseDecimalHours(row.WeeklyHours)
		if err != nil || hours < 0 || hours > 80 {
			errs = append(errs, importModels.ValidationError{
				Field:       "weekly_hours",
				Message:     fmt.Sprintf("Ungültige Wochenstunden '%s'. Bitte eine Zahl zwischen 0 und 80 angeben (z.B. 39 oder 19,5).", row.WeeklyHours),
				Code:        "invalid_weekly_hours",
				Severity:    importModels.ErrorSeverityError,
				ActualValue: row.WeeklyHours,
			})
		} else {
			row.WeeklyHours = strconv.FormatFloat(hours, 'f', -1, 64)
		}
	}

	for _, phone := range []struct {
		value string
		field string
		label string
	}{
		{row.Phone, "phone", "Telefon"},
		{row.EmergencyContactPhone, "emergency_contact_phone", "Notfallkontakt Telefon"},
	} {
		if phone.value != "" && userModels.ValidateOptionalPhone(phone.value) != nil {
			errs = append(errs, importModels.ValidationError{
				Field:    phone.field,
				Message:  fmt.Sprintf("Ungültiges Telefon-Format für '%s': %s", phone.label, phone.value),
				Code:     "invalid_phone",
				Severity: importModels.ErrorSeverityError,
			})
		}
	}

	if row.ContactEmail != "" && !userModels.IsValidEmailFormat(row.ContactEmail) {
		errs = append(errs, importModels.ValidationError{
			Field:    "contact_email",
			Message:  fmt.Sprintf("Ungültige Kontakt-E-Mail: %s", row.ContactEmail),
			Code:     "invalid_email",
			Severity: importModels.ErrorSeverityError,
		})
	}

	if row.Qualifications != "" {
		if _, err := ParseStaffQualifications(row.Qualifications); err != nil {
			errs = append(errs, importModels.ValidationError{
				Field:    "qualifications",
				Message:  err.Error(),
				Code:     "invalid_qualifications",
				Severity: importModels.ErrorSeverityError,
			})
		}
	}

	return errs
}

// parseDecimalHours accepts "39", "19.5" and the German "19,5".
func parseDecimalHours(raw string) (float64, error) {
	return strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(raw), ",", "."), 64)
}

// StaffQualificationEntry is one parsed entry of the "Qualifikationen" column.
type StaffQualificationEntry struct {
	Name       string
	AcquiredOn *timezone.Date
	ExpiresOn  *timezone.Date
}

// ParseStaffQualifications parses the "Qualifikationen" column:
//
//	Erste Hilfe (01.03.2024 bis 01.03.2026); Schwimmschein (01.05.2023); Fortbildung Inklusion
//
// Entries are separated by ";". An optional parenthesised suffix carries the
// acquisition date and, after "bis", the expiry date.
func ParseStaffQualifications(raw string) ([]StaffQualificationEntry, error) {
	var entries []StaffQualificationEntry
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		entry := StaffQualificationEntry{Name: part}
		if open := strings.LastIndex(part, "("); open >= 0 && strings.HasSuffix(part, ")") {
			entry.Name = strings.TrimSpace(part[:open])
			inner := strings.TrimSpace(part[open+1 : len(part)-1])
			if entry.Name == "" {
				return nil, fmt.Errorf("Qualifikation ohne Namen: '%s'", part) //nolint:staticcheck // ST1005: user-facing German message
			}
			from, until, ok := strings.Cut(inner, " bis ")
			if !ok {
				from, until, ok = strings.Cut(inner, " - ")
			}
			if !ok {
				from, until = inner, ""
			}
			acquired, err := parseOptionalQualificationDate(from)
			if err != nil {
				return nil, fmt.Errorf("Qualifikation '%s': ungültiges Datum '%s'. Format: Name (TT.MM.JJJJ bis TT.MM.JJJJ)", entry.Name, strings.TrimSpace(from)) //nolint:staticcheck // ST1005: user-facing German message
			}
			expires, err := parseOptionalQualificationDate(until)
			if err != nil {
				return nil, fmt.Errorf("Qualifikation '%s': ungültiges Datum '%s'. Format: Name (TT.MM.JJJJ bis TT.MM.JJJJ)", entry.Name, strings.TrimSpace(until)) //nolint:staticcheck // ST1005: user-facing German message
			}
			if acquired != nil && expires != nil && expires.Before(*acquired) {
				return nil, fmt.Errorf("Qualifikation '%s': Ablaufdatum liegt vor dem Erwerbsdatum", entry.Name) //nolint:staticcheck // ST1005: user-facing German message
			}
			entry.AcquiredOn = acquired
			entry.ExpiresOn = expires
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func parseOptionalQualificationDate(raw string) (*timezone.Date, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parsed, err := parseDateFormats(raw)
	if err != nil {
		return nil, err
	}
	d := timezone.DateFromTime(parsed)
	return &d, nil
}

// ValidateBatch validates invariants that can only be checked across the full
// uploaded file: duplicate e-mails and duplicate personnel numbers.
func (c *StaffImportConfig) ValidateBatch(_ context.Context, rows []importModels.StaffImportRow) map[int][]importModels.ValidationError {
	seenEmail := make(map[string]int, len(rows))
	seenPN := make(map[string]int, len(rows))
	errs := make(map[int][]importModels.ValidationError)
	for i, row := range rows {
		if email, err := normalizeStaffEmail(row.Email); err == nil && email != "" {
			if firstRow, ok := seenEmail[email]; ok {
				errs[i] = append(errs[i], importModels.ValidationError{
					Field:       "email",
					Message:     fmt.Sprintf("E-Mail '%s' ist doppelt in der Importdatei vorhanden (erste Zeile: %d).", email, firstRow+2),
					Code:        "duplicate_in_file",
					Severity:    importModels.ErrorSeverityError,
					ActualValue: email,
				})
			} else {
				seenEmail[email] = i
			}
		}
		if pn := strings.ToLower(strings.TrimSpace(row.PersonnelNumber)); pn != "" {
			if firstRow, ok := seenPN[pn]; ok {
				errs[i] = append(errs[i], importModels.ValidationError{
					Field:       "personnel_number",
					Message:     fmt.Sprintf("Personalnummer '%s' ist doppelt in der Importdatei vorhanden (erste Zeile: %d).", row.PersonnelNumber, firstRow+2),
					Code:        "duplicate_in_file",
					Severity:    importModels.ErrorSeverityError,
					ActualValue: row.PersonnelNumber,
				})
			} else {
				seenPN[pn] = i
			}
		}
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
		role, err = authsvc.ValidateResolvedAssignableSchoolRole(role, tenant.FromContext(ctx))
		if err != nil {
			return []importModels.ValidationError{{
				Field:    "role",
				Message:  err.Error(),
				Code:     "role_not_assignable",
				Severity: importModels.ErrorSeverityError,
			}}
		}
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
		skipGrantCheck := importModeFromContext(ctx) == importModels.ImportModeUpdate
		if importModeFromContext(ctx) == importModels.ImportModeUpsert {
			existing, findErr := c.FindExisting(ctx, *row)
			if findErr != nil {
				return []importModels.ValidationError{{Field: "role", Message: fmt.Sprintf("Bestehende Person konnte nicht geprüft werden: %s", findErr.Error()), Code: "existing_lookup_failed", Severity: importModels.ErrorSeverityError}}
			}
			skipGrantCheck = existing != nil
		}
		if !skipGrantCheck && !authorize.CanGrantRole(role, ImporterPermissionsFromContext(ctx)) {
			return []importModels.ValidationError{{
				Field:    "role",
				Message:  "Du darfst diese Rolle nicht vergeben",
				Code:     "role_grant_not_permitted",
				Severity: importModels.ErrorSeverityError,
			}}
		}
		if c.rolesByID == nil {
			c.rolesByID = make(map[int64]*authModels.Role)
		}
		c.rolesByID[role.ID] = role
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

// FindExisting resolves the row to an existing staff member of this tenant.
// Keys, in order: personnel number, login e-mail (account with access to this
// tenant), and finally an unambiguous first+last name. The returned id is the
// users.staff id — the row Update receives.
//
// A globally existing account in another tenant is not a duplicate:
// CreateInvitation supports inviting that account into the current tenant.
func (c *StaffImportConfig) FindExisting(ctx context.Context, row importModels.StaffImportRow) (*int64, error) {
	if pn := strings.ToLower(strings.TrimSpace(row.PersonnelNumber)); pn != "" {
		if staff, ok := c.staffByPersonnelNumber[pn]; ok {
			id := staff.ID
			return &id, nil
		}
	}

	if id, err := c.findStaffByLoginEmail(ctx, row.Email); err != nil || id != nil {
		return id, err
	}

	candidates := c.staffByName[staffNameKey(row.FirstName, row.LastName)]
	rowPN := strings.ToLower(strings.TrimSpace(row.PersonnelNumber))
	var matches []*userModels.Staff
	for _, staff := range candidates {
		// A different personnel number on either side means a different person
		// who happens to share the name.
		if rowPN != "" && (staff.PersonnelNumber == nil || strings.ToLower(strings.TrimSpace(*staff.PersonnelNumber)) != rowPN) {
			continue
		}
		matches = append(matches, staff)
	}
	switch len(matches) {
	case 0:
		return nil, nil
	case 1:
		id := matches[0].ID
		return &id, nil
	default:
		return nil, fmt.Errorf("mehrere Personen mit dem Namen '%s %s' vorhanden. Bitte Personalnummer oder E-Mail zur Unterscheidung angeben", row.FirstName, row.LastName)
	}
}

// findStaffByLoginEmail resolves email → account → person → staff within the
// current tenant. Returns (nil, nil) at every "not here" step.
func (c *StaffImportConfig) findStaffByLoginEmail(ctx context.Context, rawEmail string) (*int64, error) {
	email, err := normalizeStaffEmail(rawEmail)
	if err != nil || email == "" || c.AccountRepo == nil {
		return nil, nil
	}

	account, err := c.AccountRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.findStaffByPendingInvitation(ctx, email)
		}
		return nil, err
	}
	if account == nil {
		return c.findStaffByPendingInvitation(ctx, email)
	}

	exists, err := c.AccountTenantRepo.ExistsByAccountAndTenant(ctx, account.ID, tenant.FromContext(ctx))
	if err != nil {
		return nil, err
	}
	if !exists {
		return c.findStaffByPendingInvitation(ctx, email)
	}

	if c.PersonRepo == nil || c.StaffRepo == nil {
		// Legacy wiring without the Stammdaten repos: the account itself is
		// the duplicate marker, as before #2600.
		id := account.ID
		return &id, nil
	}
	person, err := c.PersonRepo.FindByAccountID(ctx, account.ID)
	if err != nil {
		return nil, err
	}
	if person == nil {
		return nil, nil
	}
	staff, err := c.StaffRepo.FindByPersonID(ctx, person.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if staff == nil || staff.DeletedAt != nil {
		return nil, nil
	}
	id := staff.ID
	return &id, nil
}

// findStaffByPendingInvitation resolves staff created by an import but not yet
// linked to an account. The invitation stores the imported person ID until it
// is accepted, so e-mail remains a stable update key in that interval.
func (c *StaffImportConfig) findStaffByPendingInvitation(ctx context.Context, email string) (*int64, error) {
	if c.InvitationRepo == nil || c.StaffRepo == nil {
		return nil, nil
	}
	invitations, err := c.InvitationRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	var match *int64
	for _, invitation := range invitations {
		if invitation.UsedAt != nil || invitation.PersonID == nil {
			continue
		}
		staff, err := c.StaffRepo.FindByPersonID(ctx, *invitation.PersonID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}
			return nil, err
		}
		if staff == nil || staff.DeletedAt != nil {
			continue
		}
		if match != nil && *match != staff.ID {
			return nil, fmt.Errorf("mehrere offene Einladungen für E-Mail-Adresse '%s' verweisen auf unterschiedliche Personen", email)
		}
		id := staff.ID
		match = &id
	}
	return match, nil
}

// Create files the Stammdatensatz (Person, Staff, caregiver profile when the
// role needs one, master data, qualifications) and, when the row carries an
// e-mail, an invitation that points at the new person. Runs in a savepoint so
// a failing row leaves nothing behind while the surrounding import
// transaction keeps the other rows.
func (c *StaffImportConfig) Create(ctx context.Context, row importModels.StaffImportRow) (int64, error) {
	if c.PersonRepo == nil || c.StaffRepo == nil {
		return 0, errors.New("staff import: Stammdaten repositories are not wired")
	}

	var staffID int64
	err := tenant.WithSavepoint(ctx, func(ctx context.Context) error {
		id, err := c.createRecords(ctx, row)
		if err != nil {
			return err
		}
		staffID = id
		return nil
	})
	if err != nil {
		return 0, err
	}
	return staffID, nil
}

func (c *StaffImportConfig) createRecords(ctx context.Context, row importModels.StaffImportRow) (int64, error) {
	tenantID := tenant.FromContext(ctx)

	person := &userModels.Person{
		FirstName: row.FirstName,
		LastName:  row.LastName,
		Birthday:  optionalImportDate(row.Birthday),
	}
	person.SetTenantID(tenantID)
	if err := c.PersonRepo.Create(ctx, person); err != nil {
		return 0, fmt.Errorf("Person anlegen: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}

	staff := &userModels.Staff{
		PersonID:        person.ID,
		StaffNotes:      row.StaffNotes,
		EmploymentType:  strutil.TrimToNil(row.EmploymentType),
		PersonnelNumber: strutil.TrimToNil(row.PersonnelNumber),
	}
	staff.SetTenantID(tenantID)
	if err := c.StaffRepo.Create(ctx, staff); err != nil {
		return 0, fmt.Errorf("Mitarbeiter anlegen: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}

	if role := c.rolesByID[row.RoleID]; role != nil && authsvc.RoleNeedsCaregiverProfile(role) && c.TeacherRepo != nil {
		teacher := &userModels.Teacher{StaffID: staff.ID, Role: row.Position}
		teacher.SetTenantID(tenantID)
		if err := c.TeacherRepo.Create(ctx, teacher); err != nil {
			return 0, fmt.Errorf("Betreuungsprofil anlegen: %w", err) //nolint:staticcheck // ST1005: user-facing German message
		}
	}

	if err := c.writeMasterData(ctx, staff.ID, nil, row); err != nil {
		return 0, err
	}
	if err := c.writeQualifications(ctx, staff.ID, row); err != nil {
		return 0, err
	}

	if row.Email != "" && c.InvitationService != nil {
		if err := c.invite(ctx, person.ID, row); err != nil {
			return 0, err
		}
	}

	// Keep the in-memory indexes current so a later row of the same file
	// resolves to this record instead of creating a twin.
	staff.Person = person
	c.indexStaff(staff)

	return staff.ID, nil
}

func (c *StaffImportConfig) indexStaff(staff *userModels.Staff) {
	if c.staffByPersonnelNumber == nil {
		c.staffByPersonnelNumber = make(map[string]*userModels.Staff)
	}
	if c.staffByName == nil {
		c.staffByName = make(map[string][]*userModels.Staff)
	}
	if staff.PersonnelNumber != nil {
		if pn := strings.ToLower(strings.TrimSpace(*staff.PersonnelNumber)); pn != "" {
			c.staffByPersonnelNumber[pn] = staff
		}
	}
	if staff.Person != nil {
		key := staffNameKey(staff.Person.FirstName, staff.Person.LastName)
		c.staffByName[key] = append(c.staffByName[key], staff)
	}
}

func (c *StaffImportConfig) unindexStaff(staff *userModels.Staff) {
	if staff.PersonnelNumber != nil {
		delete(c.staffByPersonnelNumber, strings.ToLower(strings.TrimSpace(*staff.PersonnelNumber)))
	}
	if staff.Person != nil {
		key := staffNameKey(staff.Person.FirstName, staff.Person.LastName)
		candidates := c.staffByName[key]
		for i, candidate := range candidates {
			if candidate.ID == staff.ID {
				c.staffByName[key] = append(candidates[:i], candidates[i+1:]...)
				break
			}
		}
		if len(c.staffByName[key]) == 0 {
			delete(c.staffByName, key)
		}
	}
}

// invite issues the portal invitation for a freshly imported person.
func (c *StaffImportConfig) invite(ctx context.Context, personID int64, row importModels.StaffImportRow) error {
	email, err := normalizeStaffEmail(row.Email)
	if err != nil {
		return err
	}
	pid := personID
	req := authsvc.InvitationRequest{
		Email:            email,
		RoleID:           row.RoleID,
		TenantID:         tenant.FromContext(ctx),
		FirstName:        strutil.TrimToNil(row.FirstName),
		LastName:         strutil.TrimToNil(row.LastName),
		Position:         strutil.TrimToNil(row.Position),
		PersonID:         &pid,
		CreatedBy:        ImporterIDFromContext(ctx),
		SchoolName:       c.schoolName,
		ActorPermissions: ImporterPermissionsFromContext(ctx),
	}
	if _, err := c.InvitationService.CreateInvitation(ctx, req); err != nil {
		return fmt.Errorf("Einladung anlegen: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}
	return nil
}

// writeMasterData creates or patches users.staff_master_data. Only columns
// with a value are written; an empty cell never clears a stored value, so a
// partial file can be re-imported without losing data.
func (c *StaffImportConfig) writeMasterData(ctx context.Context, staffID int64, existing *userModels.StaffMasterData, row importModels.StaffImportRow) error {
	if c.MasterDataRepo == nil || !rowHasMasterData(row) {
		return nil
	}

	data := existing
	created := false
	if data == nil {
		data = &userModels.StaffMasterData{StaffID: staffID}
		data.SetTenantID(tenant.FromContext(ctx))
		created = true
	}

	setStr := func(dst **string, v string) {
		if v != "" {
			*dst = strutil.TrimToNil(v)
		}
	}
	setStr(&data.Gender, row.Gender)
	setStr(&data.AddressStreet, row.AddressStreet)
	setStr(&data.AddressPostalCode, row.AddressPostalCode)
	setStr(&data.AddressCity, row.AddressCity)
	setStr(&data.Phone, row.Phone)
	setStr(&data.Email, row.ContactEmail)
	setStr(&data.EmergencyContactName, row.EmergencyContactName)
	setStr(&data.EmergencyContactPhone, row.EmergencyContactPhone)
	if d := optionalImportDate(row.EntryDate); d != nil {
		data.EntryDate = d
	}
	if d := optionalImportDate(row.ContractEndDate); d != nil {
		data.ContractEndDate = d
	}
	if d := optionalImportDate(row.ProbationEndDate); d != nil {
		data.ProbationEndDate = d
	}
	if row.WeeklyHours != "" {
		if hours, err := parseDecimalHours(row.WeeklyHours); err == nil {
			data.WeeklyHours = &hours
		}
	}

	if err := data.Validate(); err != nil {
		return fmt.Errorf("Stammdaten ungültig: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}
	if created {
		if err := c.MasterDataRepo.Create(ctx, data); err != nil {
			return fmt.Errorf("Stammdaten anlegen: %w", err) //nolint:staticcheck // ST1005: user-facing German message
		}
		return nil
	}
	if err := c.MasterDataRepo.Update(ctx, data); err != nil {
		return fmt.Errorf("Stammdaten aktualisieren: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}
	return nil
}

func rowHasMasterData(row importModels.StaffImportRow) bool {
	return row.Gender != "" || row.AddressStreet != "" || row.AddressPostalCode != "" || row.AddressCity != "" ||
		row.Phone != "" || row.ContactEmail != "" || row.EmergencyContactName != "" || row.EmergencyContactPhone != "" ||
		row.EntryDate != "" || row.ContractEndDate != "" || row.ProbationEndDate != "" || row.WeeklyHours != ""
}

// writeQualifications replaces the qualification list when the row carries
// one. An empty cell keeps the stored list.
func (c *StaffImportConfig) writeQualifications(ctx context.Context, staffID int64, row importModels.StaffImportRow) error {
	if c.QualificationRepo == nil || row.Qualifications == "" {
		return nil
	}
	entries, err := ParseStaffQualifications(row.Qualifications)
	if err != nil {
		return err
	}
	tenantID := tenant.FromContext(ctx)
	rows := make([]*userModels.StaffQualification, 0, len(entries))
	for _, entry := range entries {
		q := &userModels.StaffQualification{
			StaffID:    staffID,
			Name:       entry.Name,
			AcquiredOn: entry.AcquiredOn,
			ExpiresOn:  entry.ExpiresOn,
		}
		q.SetTenantID(tenantID)
		rows = append(rows, q)
	}
	if err := c.QualificationRepo.ReplaceForStaff(ctx, staffID, rows); err != nil {
		return fmt.Errorf("Qualifikationen speichern: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}
	return nil
}

// Update patches an existing staff member: names, birthday, notes, personnel
// number, employment type and the master data. Empty cells leave the stored
// value untouched. The role and the login are not touched — role changes and
// invitations for existing people belong to the Personalverwaltung.
func (c *StaffImportConfig) Update(ctx context.Context, staffID int64, row importModels.StaffImportRow) error {
	if c.PersonRepo == nil || c.StaffRepo == nil {
		return errors.New("staff import: Stammdaten repositories are not wired")
	}
	return tenant.WithSavepoint(ctx, func(ctx context.Context) error {
		return c.updateRecords(ctx, staffID, row)
	})
}

func (c *StaffImportConfig) updateRecords(ctx context.Context, staffID int64, row importModels.StaffImportRow) error {
	staff, err := c.StaffRepo.FindByID(ctx, staffID)
	if err != nil {
		return fmt.Errorf("Mitarbeiter laden: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}
	if staff == nil {
		return errors.New("Mitarbeiter nicht gefunden") //nolint:staticcheck // ST1005: user-facing German message
	}
	person, err := c.PersonRepo.FindByID(ctx, staff.PersonID)
	if err != nil {
		return fmt.Errorf("Person laden: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}
	if person == nil {
		return errors.New("Person nicht gefunden") //nolint:staticcheck // ST1005: user-facing German message
	}
	staff.Person = person
	c.unindexStaff(staff)

	if row.PersonnelNumber != "" {
		pn := strings.ToLower(row.PersonnelNumber)
		if other, ok := c.staffByPersonnelNumber[pn]; ok && other.ID != staff.ID {
			return fmt.Errorf("Personalnummer '%s' ist bereits einer anderen Person zugeordnet", row.PersonnelNumber) //nolint:staticcheck // ST1005: user-facing German message
		}
	}

	personChanged := false
	if row.FirstName != "" && row.FirstName != person.FirstName {
		person.FirstName = row.FirstName
		personChanged = true
	}
	if row.LastName != "" && row.LastName != person.LastName {
		person.LastName = row.LastName
		personChanged = true
	}
	if d := optionalImportDate(row.Birthday); d != nil {
		person.Birthday = d
		personChanged = true
	}
	if personChanged {
		if err := c.PersonRepo.Update(ctx, person); err != nil {
			return fmt.Errorf("Person aktualisieren: %w", err) //nolint:staticcheck // ST1005: user-facing German message
		}
	}

	staffChanged := false
	if row.StaffNotes != "" && row.StaffNotes != staff.StaffNotes {
		staff.StaffNotes = row.StaffNotes
		staffChanged = true
	}
	if row.EmploymentType != "" && !ptrEquals(staff.EmploymentType, row.EmploymentType) {
		staff.EmploymentType = strutil.TrimToNil(row.EmploymentType)
		staffChanged = true
	}
	if row.PersonnelNumber != "" && !ptrEquals(staff.PersonnelNumber, row.PersonnelNumber) {
		staff.PersonnelNumber = strutil.TrimToNil(row.PersonnelNumber)
		staffChanged = true
	}
	if staffChanged {
		if err := c.StaffRepo.Update(ctx, staff); err != nil {
			return fmt.Errorf("Mitarbeiter aktualisieren: %w", err) //nolint:staticcheck // ST1005: user-facing German message
		}
	}

	if row.Position != "" && c.TeacherRepo != nil {
		teacher, err := c.TeacherRepo.FindByStaffID(ctx, staff.ID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("Betreuungsprofil laden: %w", err) //nolint:staticcheck // ST1005: user-facing German message
		}
		if teacher != nil && teacher.DeletedAt == nil && teacher.Role != row.Position {
			teacher.Role = row.Position
			if err := c.TeacherRepo.Update(ctx, teacher); err != nil {
				return fmt.Errorf("Betreuungsprofil aktualisieren: %w", err) //nolint:staticcheck // ST1005: user-facing German message
			}
		}
	}

	var existing *userModels.StaffMasterData
	if c.MasterDataRepo != nil {
		existing, err = c.MasterDataRepo.FindByStaffID(ctx, staff.ID)
		if err != nil {
			return fmt.Errorf("Stammdaten laden: %w", err) //nolint:staticcheck // ST1005: user-facing German message
		}
	}
	if err := c.writeMasterData(ctx, staff.ID, existing, row); err != nil {
		return err
	}
	if err := c.writeQualifications(ctx, staff.ID, row); err != nil {
		return err
	}

	c.indexStaff(staff)
	return nil
}

// optionalImportDate parses an already normalized (ISO) or raw date cell into
// a calendar date; nil for empty or unparseable input (validation reported the
// latter before Create/Update run).
func optionalImportDate(raw string) *timezone.Date {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parsed, err := parseDateFormats(raw)
	if err != nil {
		return nil
	}
	d := timezone.DateFromTime(parsed)
	return &d
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
