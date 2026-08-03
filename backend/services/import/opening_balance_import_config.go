package importpkg

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

// maxOpeningHours mirrors the ledger's carryover bound (10.000 h).
const maxOpeningHours = 10_000.0

// OpeningBalanceImportDeps contains the request-independent dependencies for
// the opening balance import (#2132).
type OpeningBalanceImportDeps struct {
	StaffRepo            userModels.StaffRepository
	AdjustmentRepo       activeModels.StaffBalanceAdjustmentRepository
	VacationOpeningRepo  activeModels.StaffVacationOpeningRepository
	BalanceAdjustService activeSvc.StaffBalanceAdjustmentService
	StaffAbsenceService  activeSvc.StaffAbsenceService
}

// OpeningBalanceImportConfig implements ImportConfig for go-live opening
// balances (#2132). One instance is built PER REQUEST: EffectiveDate, Note,
// and DecidedByStaffID come from the upload form, not from the file, and the
// preload caches are request-scoped.
//
// Duplicate detection happens in Validate (per side, with existing openings
// preloaded) so the dry-run preview reports it; FindExisting always answers
// "new" and Create books only the sides the row carries.
type OpeningBalanceImportConfig struct {
	OpeningBalanceImportDeps

	EffectiveDate    timezone.Date
	Note             string
	DecidedByStaffID int64

	staffByPersonnelNumber map[string]*userModels.Staff
	staffByName            map[string][]*userModels.Staff
	staffWithHoursOpening  map[int64]bool
	staffWithVacOpening    map[int64]bool
}

// OpeningBalanceImportFactory builds a request-scoped import service: the
// Stichtag, note, and acting staff member come from the upload form, so the
// config cannot be a shared singleton like the student/staff ones.
type OpeningBalanceImportFactory func(effectiveDate timezone.Date, note string, decidedByStaffID int64) *ImportService[importModels.OpeningBalanceImportRow]

// NewOpeningBalanceImportConfig creates a request-scoped import configuration.
func NewOpeningBalanceImportConfig(deps OpeningBalanceImportDeps, effectiveDate timezone.Date, note string, decidedByStaffID int64) *OpeningBalanceImportConfig {
	return &OpeningBalanceImportConfig{
		OpeningBalanceImportDeps: deps,
		EffectiveDate:            effectiveDate,
		Note:                     note,
		DecidedByStaffID:         decidedByStaffID,
	}
}

// PreloadReferenceData loads the tenant's staff (for row matching) and the
// already-booked openings on both sides (for duplicate errors in the preview).
func (c *OpeningBalanceImportConfig) PreloadReferenceData(ctx context.Context) error {
	staffMembers, err := c.StaffRepo.ListAllWithPerson(ctx)
	if err != nil {
		return fmt.Errorf("preload staff: %w", err)
	}
	c.staffByPersonnelNumber = make(map[string]*userModels.Staff)
	c.staffByName = make(map[string][]*userModels.Staff)
	for _, staff := range staffMembers {
		if staff.PersonnelNumber != nil {
			if pn := strings.TrimSpace(*staff.PersonnelNumber); pn != "" {
				c.staffByPersonnelNumber[strings.ToLower(pn)] = staff
			}
		}
		if staff.Person != nil {
			key := staffNameKey(staff.Person.FirstName, staff.Person.LastName)
			c.staffByName[key] = append(c.staffByName[key], staff)
		}
	}

	options := modelBase.NewQueryOptions()
	options.Filter.Equal("type", activeModels.BalanceAdjustmentTypeOpening)
	hoursOpenings, err := c.AdjustmentRepo.List(ctx, options)
	if err != nil {
		return fmt.Errorf("preload hours openings: %w", err)
	}
	c.staffWithHoursOpening = make(map[int64]bool, len(hoursOpenings))
	for _, opening := range hoursOpenings {
		c.staffWithHoursOpening[opening.StaffID] = true
	}

	vacOptions := modelBase.NewQueryOptions()
	vacOptions.Filter.Equal("year", c.EffectiveDate.Year)
	vacOpenings, err := c.VacationOpeningRepo.List(ctx, vacOptions)
	if err != nil {
		return fmt.Errorf("preload vacation openings: %w", err)
	}
	c.staffWithVacOpening = make(map[int64]bool, len(vacOpenings))
	for _, opening := range vacOpenings {
		c.staffWithVacOpening[opening.StaffID] = true
	}
	return nil
}

func staffNameKey(firstName, lastName string) string {
	return strings.ToLower(strings.TrimSpace(firstName)) + "|" + strings.ToLower(strings.TrimSpace(lastName))
}

// Validate resolves the row's staff member, parses the German decimal values,
// and reports duplicate takeovers per side.
func (c *OpeningBalanceImportConfig) Validate(_ context.Context, row *importModels.OpeningBalanceImportRow) []importModels.ValidationError {
	var errs []importModels.ValidationError

	row.PersonnelNumber = strings.TrimSpace(row.PersonnelNumber)
	row.FirstName = strings.TrimSpace(row.FirstName)
	row.LastName = strings.TrimSpace(row.LastName)
	row.HoursBalance = strings.TrimSpace(row.HoursBalance)
	row.VacationEntitled = strings.TrimSpace(row.VacationEntitled)
	row.VacationCarryover = strings.TrimSpace(row.VacationCarryover)
	row.VacationRemaining = strings.TrimSpace(row.VacationRemaining)

	errs = append(errs, c.resolveStaff(row)...)
	errs = append(errs, c.parseValues(row)...)

	if row.HoursBalance == "" && row.VacationEntitled == "" && row.VacationCarryover == "" && row.VacationRemaining == "" {
		errs = append(errs, importModels.ValidationError{
			Field:    "values",
			Message:  "Die Zeile enthält keine Werte (Stundensaldo und Urlaubsspalten sind leer).",
			Code:     "no_values",
			Severity: importModels.ErrorSeverityError,
		})
	}

	if row.StaffID > 0 {
		if row.HoursBalanceMinutes != nil && c.staffWithHoursOpening[row.StaffID] {
			errs = append(errs, importModels.ValidationError{
				Field:    "hours_balance",
				Message:  "Für diese Person existiert bereits ein Stundenkonto-Eröffnungssaldo. Zuerst die bestehende Buchung im Stundenkonto löschen.",
				Code:     "hours_opening_exists",
				Severity: importModels.ErrorSeverityError,
			})
		}
		if row.VacationRemainingDays != nil && c.staffWithVacOpening[row.StaffID] {
			errs = append(errs, importModels.ValidationError{
				Field:    "vacation_remaining",
				Message:  fmt.Sprintf("Für diese Person existiert bereits eine Urlaubs-Übernahme für %d. Zuerst die bestehende Übernahme löschen.", c.EffectiveDate.Year),
				Code:     "vacation_opening_exists",
				Severity: importModels.ErrorSeverityError,
			})
		}
	}

	return errs
}

// resolveStaff matches the row to a staff member: exact Personalnummer first,
// then a unique first + last name match.
func (c *OpeningBalanceImportConfig) resolveStaff(row *importModels.OpeningBalanceImportRow) []importModels.ValidationError {
	if row.PersonnelNumber != "" {
		staff, ok := c.staffByPersonnelNumber[strings.ToLower(row.PersonnelNumber)]
		if !ok {
			return []importModels.ValidationError{{
				Field:       "personnel_number",
				Message:     fmt.Sprintf("Personalnummer '%s' wurde keiner Person zugeordnet.", row.PersonnelNumber),
				Code:        "personnel_number_not_found",
				Severity:    importModels.ErrorSeverityError,
				ActualValue: row.PersonnelNumber,
			}}
		}
		row.StaffID = staff.ID
		return nil
	}

	if row.FirstName == "" || row.LastName == "" {
		return []importModels.ValidationError{{
			Field:    "name",
			Message:  "Vorname und Nachname sind erforderlich (oder eine Personalnummer).",
			Code:     "required",
			Severity: importModels.ErrorSeverityError,
		}}
	}
	matches := c.staffByName[staffNameKey(row.FirstName, row.LastName)]
	switch len(matches) {
	case 0:
		return []importModels.ValidationError{{
			Field:       "name",
			Message:     fmt.Sprintf("'%s %s' wurde nicht gefunden. Die Person muss vor dem Import angelegt sein.", row.FirstName, row.LastName),
			Code:        "staff_not_found",
			Severity:    importModels.ErrorSeverityError,
			ActualValue: row.FirstName + " " + row.LastName,
		}}
	case 1:
		row.StaffID = matches[0].ID
		return nil
	default:
		return []importModels.ValidationError{{
			Field:    "name",
			Message:  fmt.Sprintf("'%s %s' ist mehrdeutig (%d Treffer). Bitte die Personalnummer angeben.", row.FirstName, row.LastName, len(matches)),
			Code:     "staff_ambiguous",
			Severity: importModels.ErrorSeverityError,
		}}
	}
}

// parseValues parses the value columns (German decimals, signed where the
// business allows it) into the typed row fields.
func (c *OpeningBalanceImportConfig) parseValues(row *importModels.OpeningBalanceImportRow) []importModels.ValidationError {
	var errs []importModels.ValidationError

	if row.HoursBalance != "" {
		hours, err := parseGermanDecimal(row.HoursBalance)
		switch {
		case err != nil:
			errs = append(errs, decimalError("hours_balance", "Stundensaldo", row.HoursBalance))
		case math.Abs(hours) > maxOpeningHours:
			errs = append(errs, rangeError("hours_balance", fmt.Sprintf("Stundensaldo muss zwischen -%.0f und %.0f Stunden liegen.", maxOpeningHours, maxOpeningHours), row.HoursBalance))
		default:
			minutes := int(math.Round(hours * 60))
			row.HoursBalanceMinutes = &minutes
		}
	}

	if row.VacationEntitled != "" {
		entitled, err := parseGermanDecimal(row.VacationEntitled)
		switch {
		case err != nil:
			errs = append(errs, decimalError("vacation_entitled", "Jahresanspruch", row.VacationEntitled))
		case entitled < 0 || entitled > 366:
			errs = append(errs, rangeError("vacation_entitled", "Jahresanspruch muss zwischen 0 und 366 Tagen liegen.", row.VacationEntitled))
		default:
			row.VacationEntitledDays = &entitled
		}
	}

	if row.VacationCarryover != "" {
		carryover, err := parseGermanDecimal(row.VacationCarryover)
		switch {
		case err != nil:
			errs = append(errs, decimalError("vacation_carryover", "Vorjahresübertrag", row.VacationCarryover))
		case carryover < 0 || carryover > 366:
			errs = append(errs, rangeError("vacation_carryover", "Vorjahresübertrag muss zwischen 0 und 366 Tagen liegen.", row.VacationCarryover))
		default:
			row.VacationCarryoverDays = &carryover
		}
	}

	if row.VacationRemaining != "" {
		remaining, err := parseGermanDecimal(row.VacationRemaining)
		switch {
		case err != nil:
			errs = append(errs, decimalError("vacation_remaining", "Resturlaub", row.VacationRemaining))
		case math.Abs(remaining) > 999:
			errs = append(errs, rangeError("vacation_remaining", "Resturlaub muss zwischen -999 und 999 Tagen liegen.", row.VacationRemaining))
		default:
			row.VacationRemainingDays = &remaining
		}
	}

	return errs
}

func decimalError(field, label, actual string) importModels.ValidationError {
	return importModels.ValidationError{
		Field:       field,
		Message:     fmt.Sprintf("%s '%s' ist keine gültige Zahl (z. B. 12,5 oder -3,25).", label, actual),
		Code:        "invalid_number",
		Severity:    importModels.ErrorSeverityError,
		ActualValue: actual,
	}
}

func rangeError(field, message, actual string) importModels.ValidationError {
	return importModels.ValidationError{
		Field:       field,
		Message:     message,
		Code:        "out_of_range",
		Severity:    importModels.ErrorSeverityError,
		ActualValue: actual,
	}
}

// parseGermanDecimal parses "12,5", "-3,25", or "7.5" into a float64.
func parseGermanDecimal(raw string) (float64, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(raw), ",", ".")
	return strconv.ParseFloat(normalized, 64)
}

// ValidateBatch reports staff members appearing on more than one row.
func (c *OpeningBalanceImportConfig) ValidateBatch(_ context.Context, rows []importModels.OpeningBalanceImportRow) map[int][]importModels.ValidationError {
	seen := make(map[int64]int, len(rows))
	errs := make(map[int][]importModels.ValidationError)
	for i, row := range rows {
		if row.StaffID <= 0 {
			continue
		}
		if firstRow, ok := seen[row.StaffID]; ok {
			errs[i] = append(errs[i], importModels.ValidationError{
				Field:    "name",
				Message:  fmt.Sprintf("'%s %s' ist doppelt in der Importdatei vorhanden (erste Zeile: %d).", row.FirstName, row.LastName, firstRow+2),
				Code:     "duplicate_in_file",
				Severity: importModels.ErrorSeverityError,
			})
			continue
		}
		seen[row.StaffID] = i
	}
	return errs
}

// FindExisting always answers "new": duplicate takeovers are reported per
// side in Validate so the preview shows them, and re-imports for the SAME
// person with a different side (hours booked earlier, vacation now) must not
// be blocked wholesale.
func (c *OpeningBalanceImportConfig) FindExisting(_ context.Context, _ importModels.OpeningBalanceImportRow) (*int64, error) {
	return nil, nil
}

// Create books the sides the row carries: the Stundenkonto opening, the
// vacation quota (Jahresanspruch/Übertrag), and the vacation takeover — in
// that order, because the takeover derives taken_before from the quota.
func (c *OpeningBalanceImportConfig) Create(ctx context.Context, row importModels.OpeningBalanceImportRow) (int64, error) {
	if row.StaffID <= 0 {
		return 0, errors.New("interner Fehler: Zeile ohne aufgelöste Person")
	}
	if row.HoursBalanceMinutes != nil {
		if _, err := c.BalanceAdjustService.CreateOpeningBalance(ctx, row.StaffID, c.DecidedByStaffID, c.EffectiveDate, *row.HoursBalanceMinutes, c.Note); err != nil {
			return 0, translateOpeningBookingError(err)
		}
	}
	if row.VacationEntitledDays != nil || row.VacationCarryoverDays != nil {
		summary, err := c.StaffAbsenceService.GetVacationQuotaSummary(ctx, row.StaffID, c.EffectiveDate.Year)
		if err != nil {
			return 0, fmt.Errorf("urlaubskonto konnte nicht gelesen werden: %w", err)
		}
		entitled, carryover := summary.EntitledDays, summary.CarryoverDays
		if row.VacationEntitledDays != nil {
			entitled = *row.VacationEntitledDays
		}
		if row.VacationCarryoverDays != nil {
			carryover = *row.VacationCarryoverDays
		}
		if err := c.StaffAbsenceService.UpsertVacationQuota(ctx, row.StaffID, c.EffectiveDate.Year, entitled, carryover); err != nil {
			return 0, fmt.Errorf("urlaubsanspruch konnte nicht gespeichert werden: %w", err)
		}
	}
	if row.VacationRemainingDays != nil {
		if _, err := c.StaffAbsenceService.SetVacationOpening(ctx, row.StaffID, c.DecidedByStaffID, activeSvc.SetVacationOpeningRequest{
			EffectiveDate: c.EffectiveDate,
			RemainingDays: *row.VacationRemainingDays,
			Note:          c.Note,
		}); err != nil {
			return 0, translateOpeningBookingError(err)
		}
	}
	return row.StaffID, nil
}

// translateOpeningBookingError maps the booking sentinels to German messages
// — the generic import service embeds the error text verbatim in the
// per-row "Fehler beim Erstellen" message.
func translateOpeningBookingError(err error) error {
	switch {
	case errors.Is(err, activeSvc.ErrOpeningAlreadyExists):
		return errors.New("für diese Person existiert bereits ein Stundenkonto-Eröffnungssaldo")
	case errors.Is(err, activeSvc.ErrVacationOpeningExists):
		return errors.New("für diese Person existiert bereits eine Urlaubs-Übernahme in diesem Jahr")
	case errors.Is(err, activeSvc.ErrVacationOpeningAbsencesBeforeCutoff):
		return errors.New("es existieren bereits Urlaubs-Abwesenheiten vor dem Stichtag — Übernahme würde doppelt zählen")
	case errors.Is(err, activeSvc.ErrAdjustmentInClosedMonth):
		return errors.New("der Monat des Stichtags ist bereits abgeschlossen")
	case errors.Is(err, activeSvc.ErrAdjustmentHasDependentReset):
		return errors.New("es existieren bereits spätere Stundenkonto-Buchungen (Reset), die vom Stichtag abhängen")
	default:
		return err
	}
}

// Update is a no-op: the import is create-only; corrections go through the
// per-person UI (delete + re-create with tombstone).
func (c *OpeningBalanceImportConfig) Update(_ context.Context, _ int64, _ importModels.OpeningBalanceImportRow) error {
	return nil
}

// EntityName returns the entity type name for logging and error messages.
func (c *OpeningBalanceImportConfig) EntityName() string {
	return "Eröffnungssaldo"
}
