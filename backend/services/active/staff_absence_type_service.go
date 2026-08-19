package active

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// staffAbsenceTypeNameUniqueIndex is the case-insensitive unique index from
// migration 1.15.308 (active.staff_absence_types on (tenant_id, LOWER(name))).
// Two concurrent creates can both pass the nameTaken pre-check and then race on
// INSERT; the loser must surface as ErrAbsenceTypeNameTaken (409), not a 500.
const staffAbsenceTypeNameUniqueIndex = "uniq_staff_absence_types_tenant_name"

var (
	// ErrAbsenceTypeNotFound signals that the requested absence type does not
	// exist for the current tenant.
	ErrAbsenceTypeNotFound = errors.New("absence type not found")
	// ErrAbsenceTypeInvalid wraps model/input validation failures (maps to 400).
	ErrAbsenceTypeInvalid = errors.New("invalid absence type")
	// ErrAbsenceTypeNameTaken signals a duplicate name within the tenant.
	ErrAbsenceTypeNameTaken = errors.New("eine Abwesenheitsart mit diesem Namen gibt es bereits")
	// ErrAbsenceTypeNameReserved signals a name that collides with one of the
	// five standard types. Allowing it would put two entries reading "Urlaub"
	// in the dropdown, one of which does not touch the Urlaubskontingent.
	ErrAbsenceTypeNameReserved = errors.New("dieser Name ist bereits eine Standard-Abwesenheitsart")
	// ErrAbsenceTypeInactive signals an attempt to file a new absence under a
	// deactivated art.
	ErrAbsenceTypeInactive = errors.New("diese Abwesenheitsart ist deaktiviert")
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
	// UpdateAbsenceType renames and/or (de)activates an existing art. A nil
	// field is left untouched.
	UpdateAbsenceType(ctx context.Context, id int64, name *string, isActive *bool) (*activeModels.StaffAbsenceType, error)
	// ResolveForAbsence validates that the art may carry a NEW absence and
	// returns it: it must exist for the tenant and still be active.
	ResolveForAbsence(ctx context.Context, id int64) (*activeModels.StaffAbsenceType, error)
	// LabelsByID returns id → name for the current tenant, used to stamp the
	// display name onto absence read paths.
	LabelsByID(ctx context.Context) (map[int64]string, error)
}

type staffAbsenceTypeService struct {
	repo   activeModels.StaffAbsenceTypeRepository
	logger *slog.Logger
}

// NewStaffAbsenceTypeService creates a new staff absence type service.
func NewStaffAbsenceTypeService(repo activeModels.StaffAbsenceTypeRepository, logger *slog.Logger) StaffAbsenceTypeService {
	return &staffAbsenceTypeService{repo: repo, logger: logger}
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
	absenceType, err := s.GetAbsenceType(ctx, id)
	if err != nil {
		return nil, err
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
	absenceType := &activeModels.StaffAbsenceType{
		Name: name,
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
	existing, err := s.GetAbsenceType(ctx, id)
	if err != nil {
		return nil, err
	}

	if name != nil {
		existing.Name = *name
		if err := existing.Validate(); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrAbsenceTypeInvalid, err.Error())
		}
		if err := s.checkName(ctx, existing.Name, existing.ID); err != nil {
			return nil, err
		}
	}
	if isActive != nil {
		existing.IsActive = *isActive
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
	labels, err := svc.LabelsByID(ctx)
	if err != nil {
		slog.Default().Warn("resolving absence type labels failed",
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
