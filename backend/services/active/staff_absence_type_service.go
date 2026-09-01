package active

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// staffAbsenceTypeNameUniqueIndex is the case-insensitive unique index from
// migration 1.15.312 (active.staff_absence_types on (tenant_id, LOWER(name))).
// Two concurrent creates can both pass the nameTaken pre-check and then race on
// INSERT; the loser must surface as ErrAbsenceTypeNameTaken (409), not a 500.
const staffAbsenceTypeNameUniqueIndex = "uniq_staff_absence_types_tenant_name"

var (
	// ErrAbsenceTypeNotFound signals that the requested absence type does not
	// exist for the current tenant.
	ErrAbsenceTypeNotFound = errors.New("diese Abwesenheitsart gibt es nicht")
	// ErrAbsenceTypeInvalid wraps model/input validation failures (maps to 400).
	ErrAbsenceTypeInvalid = errors.New("ungültige Abwesenheitsart")
	// ErrAbsenceTypeNameTaken signals a duplicate name within the tenant.
	ErrAbsenceTypeNameTaken = errors.New("eine Abwesenheitsart mit diesem Namen gibt es bereits")
	// ErrAbsenceTypeNameReserved signals a name that collides with one of the
	// five standard types. Allowing it would put two entries reading "Urlaub"
	// in the dropdown, one of which does not touch the Urlaubskontingent.
	ErrAbsenceTypeNameReserved = errors.New("dieser Name ist bereits eine Standard-Abwesenheitsart")
	// ErrAbsenceTypeInactive signals an attempt to file a new absence under a
	// deactivated art.
	ErrAbsenceTypeInactive = errors.New("diese Abwesenheitsart ist deaktiviert")
	// ErrAbsenceTypeInUse prevents historical absences from changing their
	// display name retroactively through a rename of the referenced art.
	ErrAbsenceTypeInUse = errors.New("eine verwendete Abwesenheitsart kann nicht umbenannt werden")
)

// standardAbsenceTypeNames are the German labels of the five code-owned
// standard types. They are not rows, so the uniqueness index cannot protect
// them — this list does.
var standardAbsenceTypeNames = map[string]struct{}{
	"urlaub":               {},
	"krank":                {},
	"krankmeldung":         {},
	"fortbildung":          {},
	"sonstige":             {},
	"sonstiges":            {},
	"freizeitausgleich":    {},
	"sonstige abwesenheit": {},
}

// StaffAbsenceTypeService manages school-defined absence names (#2403).
//
// Scope of this version: an entry is a named subtype of AbsenceTypeOther and
// inherits its calculation exactly. The service therefore never lets a caller
// pick a base type — that is what keeps a freely typed name from silently
// changing Urlaubskontingent, Sollzeit or Stundenkonto.
type StaffAbsenceTypeService interface {
	// ListAbsenceTypes returns all absence types of the current tenant (active
	// and inactive), ordered by name.
	ListAbsenceTypes(ctx context.Context) ([]*activeModels.StaffAbsenceType, error)
	// GetAbsenceType returns a single absence type by ID.
	GetAbsenceType(ctx context.Context, id int64) (*activeModels.StaffAbsenceType, error)
	// CreateAbsenceType persists a new named art for the current tenant.
	CreateAbsenceType(ctx context.Context, name string) (*activeModels.StaffAbsenceType, error)
	CreateAbsenceTypeWithConfig(ctx context.Context, name string, allowanceEnabled bool, overrunPolicy string) (*activeModels.StaffAbsenceType, error)
	// UpdateAbsenceType renames and/or (de)activates an existing art. A nil
	// field is left untouched.
	UpdateAbsenceType(ctx context.Context, id int64, name *string, isActive *bool) (*activeModels.StaffAbsenceType, error)
	// UpdateAbsenceTypeWithConfig applies identity, lifecycle, and allowance
	// configuration in one repository write so validation cannot leave a
	// partially updated type behind.
	UpdateAbsenceTypeWithConfig(ctx context.Context, id int64, name *string, isActive *bool, allowanceEnabled *bool, overrunPolicy *string) (*activeModels.StaffAbsenceType, error)
	// ConfigureAllowance turns the type's own yearly account on or off and
	// chooses whether an overrun warns or blocks.
	ConfigureAllowance(ctx context.Context, id int64, enabled bool, overrunPolicy string) error
	// SetAllowance creates or corrects one person's yearly claim. Every write
	// requires a reason and appends an audit row.
	SetAllowance(ctx context.Context, req SetAbsenceTypeAllowanceRequest) (*AbsenceTypeAllowanceSummary, error)
	GetAllowanceSummary(ctx context.Context, staffID, absenceTypeID int64, year int) (*AbsenceTypeAllowanceSummary, error)
	PreviewAllowanceBooking(ctx context.Context, staffID, absenceTypeID int64, start, end timezone.Date, halfDay bool) ([]*AbsenceTypeAllowanceSummary, error)
	// ResolveForAbsence validates that the art may carry a NEW absence and
	// returns it: it must exist for the tenant and still be active.
	ResolveForAbsence(ctx context.Context, id int64) (*activeModels.StaffAbsenceType, error)
	// LabelsByID returns id → name for the current tenant, used to stamp the
	// display name onto absence read paths.
	LabelsByID(ctx context.Context) (map[int64]string, error)
	// StampLabels fills AbsenceTypeLabel on every absence carrying a
	// school-defined art. Callers normally go through the nil-safe
	// StampAbsenceTypeLabels wrapper.
	StampLabels(ctx context.Context, absences []*activeModels.StaffAbsence)
}

type staffAbsenceTypeService struct {
	repo          activeModels.StaffAbsenceTypeRepository
	allowanceRepo activeModels.StaffAbsenceTypeAllowanceRepository
	changeRepo    activeModels.StaffAbsenceTypeAllowanceChangeRepository
	absenceRepo   activeModels.StaffAbsenceRepository
	logger        *slog.Logger
}

var (
	ErrAbsenceTypeAllowanceInvalid  = errors.New("ungültiger Anspruch")
	ErrAbsenceTypeAllowanceExceeded = errors.New("kontingent dieser Abwesenheitsart überschritten")
)

type SetAbsenceTypeAllowanceRequest struct {
	StaffID       int64
	AbsenceTypeID int64
	Year          int
	EntitledDays  float64
	Reason        string
	ChangedBy     int64
}

type AbsenceTypeAllowanceSummary struct {
	StaffID       int64   `json:"staff_id"`
	AbsenceTypeID int64   `json:"absence_type_id"`
	Year          int     `json:"year"`
	EntitledDays  float64 `json:"entitled_days"`
	TakenDays     float64 `json:"taken_days"`
	ReservedDays  float64 `json:"reserved_days"`
	RemainingDays float64 `json:"remaining_days"`
}

// NewStaffAbsenceTypeService creates a new staff absence type service.
func NewStaffAbsenceTypeService(repo activeModels.StaffAbsenceTypeRepository, logger *slog.Logger) StaffAbsenceTypeService {
	return &staffAbsenceTypeService{repo: repo, logger: logger}
}

func (s *staffAbsenceTypeService) SetAllowanceRepositories(
	allowanceRepo activeModels.StaffAbsenceTypeAllowanceRepository,
	changeRepo activeModels.StaffAbsenceTypeAllowanceChangeRepository,
	absenceRepo activeModels.StaffAbsenceRepository,
) {
	s.allowanceRepo = allowanceRepo
	s.changeRepo = changeRepo
	s.absenceRepo = absenceRepo
}

func (s *staffAbsenceTypeService) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

func (s *staffAbsenceTypeService) ListAbsenceTypes(ctx context.Context) ([]*activeModels.StaffAbsenceType, error) {
	return s.repo.ListAll(ctx)
}

func (s *staffAbsenceTypeService) GetAbsenceType(ctx context.Context, id int64) (*activeModels.StaffAbsenceType, error) {
	if id <= 0 {
		return nil, ErrAbsenceTypeNotFound
	}
	absenceType, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAbsenceTypeNotFound
		}
		return nil, fmt.Errorf("find absence type: %w", err)
	}
	if absenceType == nil {
		return nil, ErrAbsenceTypeNotFound
	}
	return absenceType, nil
}

func (s *staffAbsenceTypeService) ResolveForAbsence(ctx context.Context, id int64) (*activeModels.StaffAbsenceType, error) {
	if id <= 0 {
		return nil, ErrAbsenceTypeNotFound
	}
	absenceType, err := s.repo.LockByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("lock absence type: %w", err)
	}
	if absenceType == nil {
		return nil, ErrAbsenceTypeNotFound
	}
	if !absenceType.IsActive {
		return nil, ErrAbsenceTypeInactive
	}
	return absenceType, nil
}

func (s *staffAbsenceTypeService) LabelsByID(ctx context.Context) (map[int64]string, error) {
	types, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	labels := make(map[int64]string, len(types))
	for _, t := range types {
		labels[t.ID] = t.Name
	}
	return labels, nil
}

// normalizeName is the comparison key for both the reserved-name check and the
// duplicate check: trimmed and case-folded, matching the LOWER(name) index.
func normalizeAbsenceTypeName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// nameTaken reports whether another art (id != excludeID) already uses the
// given name within the tenant.
func (s *staffAbsenceTypeService) nameTaken(ctx context.Context, name string, excludeID int64) (bool, error) {
	existing, err := s.repo.ListAll(ctx)
	if err != nil {
		return false, err
	}
	target := normalizeAbsenceTypeName(name)
	for _, t := range existing {
		if t.ID == excludeID {
			continue
		}
		if normalizeAbsenceTypeName(t.Name) == target {
			return true, nil
		}
	}
	return false, nil
}

// isAbsenceTypeNameConflict reports whether err is the Postgres unique
// violation on the per-tenant case-insensitive name index. errors.As inside the
// helper traverses the *base.DatabaseError wrapper the repository adds.
func isAbsenceTypeNameConflict(err error) bool {
	return base.IsUniqueViolationOn(err, staffAbsenceTypeNameUniqueIndex)
}

// checkName runs both name gates shared by create and rename.
func (s *staffAbsenceTypeService) checkName(ctx context.Context, name string, excludeID int64) error {
	if _, reserved := standardAbsenceTypeNames[normalizeAbsenceTypeName(name)]; reserved {
		return ErrAbsenceTypeNameReserved
	}
	taken, err := s.nameTaken(ctx, name, excludeID)
	if err != nil {
		return err
	}
	if taken {
		return ErrAbsenceTypeNameTaken
	}
	return nil
}

func (s *staffAbsenceTypeService) CreateAbsenceType(ctx context.Context, name string) (*activeModels.StaffAbsenceType, error) {
	return s.CreateAbsenceTypeWithConfig(ctx, name, false, activeModels.AbsenceTypeOverrunWarn)
}

func (s *staffAbsenceTypeService) CreateAbsenceTypeWithConfig(ctx context.Context, name string, allowanceEnabled bool, overrunPolicy string) (*activeModels.StaffAbsenceType, error) {
	absenceType := &activeModels.StaffAbsenceType{
		Name:             name,
		AllowanceEnabled: allowanceEnabled,
		OverrunPolicy:    overrunPolicy,
		// v1: every school-defined art is a named subtype of "Sonstige" and
		// inherits its calculation. Not caller-controlled on purpose.
		BaseType: activeModels.AbsenceTypeOther,
		IsActive: true,
	}
	if err := absenceType.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrAbsenceTypeInvalid, err.Error())
	}
	if err := s.checkName(ctx, absenceType.Name, 0); err != nil {
		return nil, err
	}
	absenceType.SetTenantID(tenant.FromContext(ctx))
	if err := s.repo.Create(ctx, absenceType); err != nil {
		if isAbsenceTypeNameConflict(err) {
			return nil, ErrAbsenceTypeNameTaken
		}
		return nil, err
	}
	s.getLogger().Info("staff absence type created", "absence_type_id", absenceType.ID)
	return absenceType, nil
}

func (s *staffAbsenceTypeService) UpdateAbsenceType(ctx context.Context, id int64, name *string, isActive *bool) (*activeModels.StaffAbsenceType, error) {
	return s.UpdateAbsenceTypeWithConfig(ctx, id, name, isActive, nil, nil)
}

func (s *staffAbsenceTypeService) UpdateAbsenceTypeWithConfig(ctx context.Context, id int64, name *string, isActive *bool, allowanceEnabled *bool, overrunPolicy *string) (*activeModels.StaffAbsenceType, error) {
	if id <= 0 {
		return nil, ErrAbsenceTypeNotFound
	}
	existing, err := s.repo.LockByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("lock absence type: %w", err)
	}
	if existing == nil {
		return nil, ErrAbsenceTypeNotFound
	}

	if name != nil {
		previousName := existing.Name
		existing.Name = *name
		if err := existing.Validate(); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrAbsenceTypeInvalid, err.Error())
		}
		if err := s.checkName(ctx, existing.Name, existing.ID); err != nil {
			return nil, err
		}
		if existing.Name != previousName {
			inUse, err := s.repo.IsInUse(ctx, existing.ID)
			if err != nil {
				return nil, fmt.Errorf("check absence type usage: %w", err)
			}
			if inUse {
				return nil, ErrAbsenceTypeInUse
			}
		}
	}
	if isActive != nil {
		existing.IsActive = *isActive
	}
	if allowanceEnabled != nil {
		existing.AllowanceEnabled = *allowanceEnabled
	}
	if overrunPolicy != nil {
		existing.OverrunPolicy = *overrunPolicy
	}
	if err := existing.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrAbsenceTypeInvalid, err.Error())
	}

	if err := s.repo.Update(ctx, existing); err != nil {
		if isAbsenceTypeNameConflict(err) {
			return nil, ErrAbsenceTypeNameTaken
		}
		return nil, err
	}
	s.getLogger().Info("staff absence type updated",
		"absence_type_id", existing.ID,
		"is_active", existing.IsActive,
	)
	return existing, nil
}

func (s *staffAbsenceTypeService) ConfigureAllowance(ctx context.Context, id int64, enabled bool, overrunPolicy string) error {
	absenceType, err := s.repo.LockByID(ctx, id)
	if err != nil {
		return fmt.Errorf("lock absence type: %w", err)
	}
	if absenceType == nil {
		return ErrAbsenceTypeNotFound
	}
	absenceType.AllowanceEnabled = enabled
	absenceType.OverrunPolicy = overrunPolicy
	if err := absenceType.Validate(); err != nil {
		return fmt.Errorf("%w: %s", ErrAbsenceTypeInvalid, err.Error())
	}
	if err := s.repo.Update(ctx, absenceType); err != nil {
		return fmt.Errorf("update absence type allowance configuration: %w", err)
	}
	return nil
}

func (s *staffAbsenceTypeService) requireAllowanceRepositories() error {
	if s.allowanceRepo == nil || s.changeRepo == nil || s.absenceRepo == nil {
		return errors.New("absence type allowance repositories are not configured")
	}
	return nil
}

func (s *staffAbsenceTypeService) SetAllowance(ctx context.Context, req SetAbsenceTypeAllowanceRequest) (*AbsenceTypeAllowanceSummary, error) {
	if err := s.requireAllowanceRepositories(); err != nil {
		return nil, err
	}
	absenceType, err := s.GetAbsenceType(ctx, req.AbsenceTypeID)
	if err != nil {
		return nil, err
	}
	if !absenceType.AllowanceEnabled {
		return nil, fmt.Errorf("%w: diese Abwesenheitsart hat kein Kontingent", ErrAbsenceTypeAllowanceInvalid)
	}
	allowance := &activeModels.StaffAbsenceTypeAllowance{
		StaffID: req.StaffID, AbsenceTypeID: req.AbsenceTypeID,
		Year: req.Year, EntitledDays: req.EntitledDays,
	}
	allowance.SetTenantID(tenant.FromContext(ctx))
	if err := allowance.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrAbsenceTypeAllowanceInvalid, err.Error())
	}
	if strings.TrimSpace(req.Reason) == "" || req.ChangedBy <= 0 {
		return nil, fmt.Errorf("%w: Begründung und ändernde Person sind erforderlich", ErrAbsenceTypeAllowanceInvalid)
	}
	if err := s.absenceRepo.LockStaffAbsenceWrites(ctx, req.StaffID); err != nil {
		return nil, fmt.Errorf("lock staff allowance writes: %w", err)
	}
	previous, err := s.findAllowance(ctx, req.StaffID, req.AbsenceTypeID, req.Year)
	if err != nil {
		return nil, fmt.Errorf("load existing absence type allowance: %w", err)
	}
	var oldDays *float64
	if previous != nil {
		value := previous.EntitledDays
		oldDays = &value
	}
	if err := s.allowanceRepo.Upsert(ctx, allowance); err != nil {
		return nil, fmt.Errorf("save absence type allowance: %w", err)
	}
	change := &activeModels.StaffAbsenceTypeAllowanceChange{
		StaffID: req.StaffID, AbsenceTypeID: req.AbsenceTypeID, Year: req.Year,
		OldEntitledDays: oldDays, NewEntitledDays: req.EntitledDays,
		Reason: strings.TrimSpace(req.Reason), ChangedBy: req.ChangedBy,
	}
	change.SetTenantID(tenant.FromContext(ctx))
	if err := s.changeRepo.Create(ctx, change); err != nil {
		return nil, fmt.Errorf("audit absence type allowance: %w", err)
	}
	return s.GetAllowanceSummary(ctx, req.StaffID, req.AbsenceTypeID, req.Year)
}

func (s *staffAbsenceTypeService) findAllowance(ctx context.Context, staffID, absenceTypeID int64, year int) (*activeModels.StaffAbsenceTypeAllowance, error) {
	options := base.NewQueryOptions()
	options.Filter.Equal("staff_id", staffID).Equal("absence_type_id", absenceTypeID).Equal("year", year)
	rows, err := s.allowanceRepo.List(ctx, options)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return rows[0], nil
}

func (s *staffAbsenceTypeService) GetAllowanceSummary(ctx context.Context, staffID, absenceTypeID int64, year int) (*AbsenceTypeAllowanceSummary, error) {
	if err := s.requireAllowanceRepositories(); err != nil {
		return nil, err
	}
	if staffID <= 0 || year < 2000 || year > 2100 {
		return nil, ErrAbsenceTypeAllowanceInvalid
	}
	if _, err := s.GetAbsenceType(ctx, absenceTypeID); err != nil {
		return nil, err
	}
	allowance, err := s.findAllowance(ctx, staffID, absenceTypeID, year)
	if err != nil {
		return nil, fmt.Errorf("load absence type allowance: %w", err)
	}
	yearStart := timezone.NewDate(year, time.January, 1)
	yearEnd := timezone.NewDate(year, time.December, 31)
	absences, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, yearStart, yearEnd)
	if err != nil {
		return nil, fmt.Errorf("load absences for allowance: %w", err)
	}
	entitled, taken, reserved := 0.0, 0.0, 0.0
	if allowance != nil {
		entitled = allowance.EntitledDays
	}
	for _, absence := range absences {
		if absence.AbsenceTypeID == nil || *absence.AbsenceTypeID != absenceTypeID {
			continue
		}
		days := vacationDaysInYear(absence, yearStart, yearEnd)
		switch absence.Status {
		case activeModels.AbsenceStatusReported, activeModels.AbsenceStatusApproved:
			taken += days
		case activeModels.AbsenceStatusRequested, activeModels.AbsenceStatusQuestion:
			reserved += days
		}
	}
	return &AbsenceTypeAllowanceSummary{
		StaffID: staffID, AbsenceTypeID: absenceTypeID, Year: year,
		EntitledDays: entitled, TakenDays: taken, ReservedDays: reserved,
		RemainingDays: entitled - taken - reserved,
	}, nil
}

func (s *staffAbsenceTypeService) PreviewAllowanceBooking(
	ctx context.Context,
	staffID, absenceTypeID int64,
	start, end timezone.Date,
	halfDay bool,
) ([]*AbsenceTypeAllowanceSummary, error) {
	if start.IsZero() || end.IsZero() || end.Before(start) {
		return nil, ErrAbsenceTypeAllowanceInvalid
	}
	absenceType, err := s.GetAbsenceType(ctx, absenceTypeID)
	if err != nil {
		return nil, err
	}
	if !absenceType.AllowanceEnabled {
		return nil, nil
	}
	previews := make([]*AbsenceTypeAllowanceSummary, 0, end.Year()-start.Year()+1)
	blocked := false
	for year := start.Year(); year <= end.Year(); year++ {
		yearStart := timezone.NewDate(year, time.January, 1)
		yearEnd := timezone.NewDate(year, time.December, 31)
		summary, err := s.GetAllowanceSummary(ctx, staffID, absenceTypeID, year)
		if err != nil {
			return nil, err
		}
		candidate := &activeModels.StaffAbsence{
			DateStart: start, DateEnd: end, HalfDay: halfDay,
			StartHalfDay: halfDay, EndHalfDay: halfDay,
		}
		existing, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, yearStart, yearEnd)
		if err != nil {
			return nil, fmt.Errorf("load overlapping absences for allowance preview: %w", err)
		}
		days := additionalAllowanceDays(candidate, existing, absenceTypeID, yearStart, yearEnd)
		summary.TakenDays += days
		summary.RemainingDays -= days
		previews = append(previews, summary)
		if summary.RemainingDays < 0 && absenceType.OverrunPolicy == activeModels.AbsenceTypeOverrunBlock {
			blocked = true
		}
	}
	if blocked {
		return previews, ErrAbsenceTypeAllowanceExceeded
	}
	return previews, nil
}

// additionalAllowanceDays counts only coverage the new booking adds. The
// absence service merges overlapping entries; charging the complete candidate
// here would reject a harmless extension or a full-day correction of an
// existing half day.
func additionalAllowanceDays(candidate *activeModels.StaffAbsence, existing []*activeModels.StaffAbsence, absenceTypeID int64, from, to timezone.Date) float64 {
	existingCoverage := make(map[timezone.Date]float64)
	for _, absence := range existing {
		if absence.AbsenceTypeID == nil || *absence.AbsenceTypeID != absenceTypeID || !blocksAbsenceRange(absence.Status) {
			continue
		}
		for day := from; !day.After(to); day = day.AddDays(1) {
			coverage := absenceCoverageOn(absence, day)
			if coverage > existingCoverage[day] {
				existingCoverage[day] = coverage
			}
		}
	}

	additional := 0.0
	for day := from; !day.After(to); day = day.AddDays(1) {
		coverage := absenceCoverageOn(candidate, day)
		if coverage > existingCoverage[day] {
			additional += coverage - existingCoverage[day]
		}
	}
	return additional
}

func absenceCoverageOn(absence *activeModels.StaffAbsence, day timezone.Date) float64 {
	if !isWorkingDay(day) || day.Before(absence.DateStart) || day.After(absence.DateEnd) {
		return 0
	}
	startHalf, endHalf := effectiveBoundaryHalfDays(absence)
	if absence.DateStart == absence.DateEnd {
		if startHalf || endHalf {
			return 0.5
		}
		return 1
	}
	if day == absence.DateStart && startHalf || day == absence.DateEnd && endHalf {
		return 0.5
	}
	return 1
}

// StampAbsenceTypeLabels fills AbsenceTypeLabel on every absence that carries a
// school-defined art (#2403), so downstream renderers — list views, Monatskarte,
// audit display, exports — show the school's own wording without each of them
// having to know the lookup table exists.
//
// Nil-safe and free when nothing needs it: a nil service (bare-constructed
// services in unit tests) or a batch without a single custom art returns
// without touching the database. A label that can no longer be resolved (the
// art was deleted out from under a row, which the FK forbids) is left empty so
// the caller falls back to the standard label rather than rendering a blank.
func StampAbsenceTypeLabels(ctx context.Context, svc StaffAbsenceTypeService, absences []*activeModels.StaffAbsence) {
	if svc == nil {
		return
	}
	svc.StampLabels(ctx, absences)
}

// StampLabels carries the work of StampAbsenceTypeLabels. It sits on the
// service rather than beside it so a failed lookup reaches the logger this
// service was constructed with, instead of bypassing the configured handler
// through slog.Default().
func (s *staffAbsenceTypeService) StampLabels(ctx context.Context, absences []*activeModels.StaffAbsence) {
	needed := false
	for _, a := range absences {
		if a != nil && a.AbsenceTypeID != nil {
			needed = true
			break
		}
	}
	if !needed {
		return
	}
	labels, err := s.LabelsByID(ctx)
	if err != nil {
		s.getLogger().WarnContext(ctx, "resolving absence type labels failed",
			slog.String("error", err.Error()),
		)
		return
	}
	for _, a := range absences {
		if a == nil || a.AbsenceTypeID == nil {
			continue
		}
		if name, ok := labels[*a.AbsenceTypeID]; ok {
			a.AbsenceTypeLabel = name
		}
	}
}
