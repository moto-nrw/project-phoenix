package importpkg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strconv"
	"strings"
	"sync"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	authsvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/import/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// systemRoleDisplayNames maps system role raw names to their German display
// names. This mirrors SYSTEM_ROLE_TRANSLATIONS in the frontend
// (frontend/src/lib/auth-helpers.ts) so the staff import accepts the same
// names a user sees on the roles page. Tenant-specific custom roles have no
// translation — their raw name is also their display name.
var systemRoleDisplayNames = map[string]string{
	"admin":     "Administrator",
	"user":      "Betreuer",
	"guest":     "Gast",
	"guardian":  "Erziehungsberechtigter",
	"lehrkraft": "Lehrkraft",
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

	// Stammdaten owners (#2600, #2708): People Directory files the person,
	// School Membership the staff and caregiver rows, Workforce the master
	// data and qualifications.
	Persons    ports.PersonDirectory
	Membership ports.StaffMembership
	Records    ports.StaffRecords
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

// ErrImportModeForbidden reports that the importer lacks a permission the
// requested import mode needs. Handlers map it to 403.
var ErrImportModeForbidden = errors.New("import mode not permitted")

// staffUpdateImportPermissions are the permissions an importer needs before
// the staff import may change existing records (#2906). Update and upsert
// mode write the same fields as the personnel screens: notes, employment
// type, personnel number and position belong to staff:manage (PUT
// /api/staff/{id}); names, birthday, contact, contract and qualifications to
// staff:stammdaten (PUT /api/staff/{id}/stammdaten/*). users:create alone
// files new records only. The literals mirror permissions.StaffManage and
// permissions.StaffStammdaten; the architecture policy keeps that package
// out of the import services, as auth/authorize/role_grant.go does it too.
var staffUpdateImportPermissions = []string{"staff:manage", "staff:stammdaten"}

// AuthorizeImportMode refuses update-capable modes unless the importer holds
// every permission the personnel screens demand for the same writes. The
// import service calls it before any row is validated, so a create-only
// caller cannot even preview an update.
func (c *StaffImportConfig) AuthorizeImportMode(ctx context.Context, mode importModels.ImportMode) error {
	if mode == importModels.ImportModeCreate {
		return nil
	}
	importerPermissions := ImporterPermissionsFromContext(ctx)
	for _, required := range staffUpdateImportPermissions {
		if !authorize.HasPermission(required, importerPermissions) {
			return fmt.Errorf("%w: mode %q requires %s", ErrImportModeForbidden, mode, required)
		}
	}
	return nil
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
	importMu *sync.Mutex

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
	staffByPersonnelNumber map[string]*indexedStaff
	staffByName            map[string][]*indexedStaff
}

// indexedStaff is the in-memory match key of one existing staff member:
// the membership row id, its personnel number and the person's name.
type indexedStaff struct {
	ID              int64
	PersonID        int64
	PersonnelNumber *string
	FirstName       string
	LastName        string
}

// NewStaffImportConfig creates a new staff import configuration.
func NewStaffImportConfig(deps StaffImportDeps) *StaffImportConfig {
	return &StaffImportConfig{StaffImportDeps: deps, importMu: &sync.Mutex{}}
}

func (c *StaffImportConfig) NewRequestScoped() importModels.ImportConfig[importModels.StaffImportRow] {
	return &StaffImportConfig{StaffImportDeps: c.StaffImportDeps, importMu: c.importMu}
}

func (c *StaffImportConfig) ImportLock() *sync.Mutex { return c.importMu }

// PreloadReferenceData loads the tenant's role names (for fuzzy suggestions on
// unresolved roles), the school display name (for the invitation email) and the
// existing staff (for duplicate detection and update matching).
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
	var matches []*indexedStaff
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
