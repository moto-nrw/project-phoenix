package schedule

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// shiftTypeNameUniqueIndex is the case-insensitive unique index from migration
// 1.15.170 (schedule.shift_types on (tenant_id, LOWER(name))). Two concurrent
// creates can both pass the nameTaken pre-check and then race on INSERT; the
// loser hits this index and must surface as ErrShiftTypeNameTaken (409), not a
// raw 500.
const shiftTypeNameUniqueIndex = "uniq_shift_types_tenant_name"

var (
	// ErrShiftTypeNotFound signals that the requested shift type does not exist.
	ErrShiftTypeNotFound = errors.New("shift type not found")
	// ErrShiftTypeInvalid wraps model/input validation failures (maps to 400).
	ErrShiftTypeInvalid = errors.New("invalid shift type")
	// ErrShiftTypeNameTaken signals a duplicate name within the tenant.
	ErrShiftTypeNameTaken = errors.New("a shift type with this name already exists")
)

// defaultShiftType is one example a school gets when it clicks
// "Beispiele hinzufügen". These are examples, not a hard-wired list — every
// row is editable and deletable afterwards. Colors mirror LOCATION_COLORS.
type defaultShiftType struct {
	name  string
	color string
}

var defaultShiftTypes = []defaultShiftType{
	{name: "Betreuung / Zeit am Kind", color: "#83CD2D"},
	{name: "Vorbereitung / Verfügungszeit", color: "#5080D8"},
	{name: "Vertretungsunterricht", color: "#F78C10"},
	{name: "Pause", color: "#6B7280"},
	{name: "Sonstiges", color: "#D946EF"},
}

// ShiftTypeService manages tenant-defined shift types (Schichtarten, #1836).
type ShiftTypeService interface {
	// ListShiftTypes returns all shift types for the current tenant (active and
	// inactive), ordered by name.
	ListShiftTypes(ctx context.Context) ([]*scheduleModels.ShiftType, error)
	// GetShiftType returns a single shift type by ID.
	GetShiftType(ctx context.Context, id int64) (*scheduleModels.ShiftType, error)
	// CreateShiftType validates and persists a new shift type.
	CreateShiftType(ctx context.Context, shiftType *scheduleModels.ShiftType) (*scheduleModels.ShiftType, error)
	// UpdateShiftType validates and persists changes to an existing shift type.
	UpdateShiftType(ctx context.Context, shiftType *scheduleModels.ShiftType) (*scheduleModels.ShiftType, error)
	// DeleteShiftType removes a shift type. Shifts referencing it keep the shift
	// but lose the type (ON DELETE SET NULL).
	DeleteShiftType(ctx context.Context, id int64) error
	// CreateDefaultShiftTypes seeds example shift types for the current tenant,
	// skipping any whose name already exists (idempotent). Returns the full
	// list afterwards.
	CreateDefaultShiftTypes(ctx context.Context) ([]*scheduleModels.ShiftType, error)
}

type shiftTypeService struct {
	repo   scheduleModels.ShiftTypeRepository
	logger *slog.Logger
}

// NewShiftTypeService creates a new shift type service.
func NewShiftTypeService(repo scheduleModels.ShiftTypeRepository, logger *slog.Logger) ShiftTypeService {
	return &shiftTypeService{repo: repo, logger: logger}
}

func (s *shiftTypeService) getLogger() *slog.Logger {
	return cmp.Or(s.logger, slog.Default())
}

func (s *shiftTypeService) ListShiftTypes(ctx context.Context) ([]*scheduleModels.ShiftType, error) {
	return s.repo.ListAll(ctx)
}

func (s *shiftTypeService) GetShiftType(ctx context.Context, id int64) (*scheduleModels.ShiftType, error) {
	if id <= 0 {
		return nil, ErrShiftTypeNotFound
	}
	shiftType, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrShiftTypeNotFound
		}
		return nil, fmt.Errorf("find shift type: %w", err)
	}
	if shiftType == nil {
		return nil, ErrShiftTypeNotFound
	}
	return shiftType, nil
}

// nameTaken reports whether another shift type (id != excludeID) already uses
// the given name (case-insensitive) within the tenant.
func (s *shiftTypeService) nameTaken(ctx context.Context, name string, excludeID int64) (bool, error) {
	existing, err := s.repo.ListAll(ctx)
	if err != nil {
		return false, err
	}
	target := strings.ToLower(strings.TrimSpace(name))
	for _, t := range existing {
		if t.ID == excludeID {
			continue
		}
		if strings.ToLower(strings.TrimSpace(t.Name)) == target {
			return true, nil
		}
	}
	return false, nil
}

// isShiftTypeNameConflict reports whether err is a Postgres unique violation
// on the per-tenant case-insensitive name index. errors.As inside the helper
// traverses the *base.DatabaseError wrapper the repository adds.
func isShiftTypeNameConflict(err error) bool {
	return base.IsUniqueViolationOn(err, shiftTypeNameUniqueIndex)
}

func (s *shiftTypeService) CreateShiftType(ctx context.Context, shiftType *scheduleModels.ShiftType) (*scheduleModels.ShiftType, error) {
	if err := shiftType.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrShiftTypeInvalid, err.Error())
	}
	taken, err := s.nameTaken(ctx, shiftType.Name, 0)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrShiftTypeNameTaken
	}
	shiftType.SetTenantID(tenant.FromContext(ctx))
	if err := s.repo.Create(ctx, shiftType); err != nil {
		if isShiftTypeNameConflict(err) {
			return nil, ErrShiftTypeNameTaken
		}
		return nil, err
	}
	s.getLogger().Info("shift type created", "shift_type_id", shiftType.ID)
	return shiftType, nil
}

func (s *shiftTypeService) UpdateShiftType(ctx context.Context, shiftType *scheduleModels.ShiftType) (*scheduleModels.ShiftType, error) {
	if shiftType.ID <= 0 {
		return nil, ErrShiftTypeNotFound
	}
	existing, err := s.repo.FindByID(ctx, shiftType.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrShiftTypeNotFound
		}
		return nil, fmt.Errorf("find shift type: %w", err)
	}
	if existing == nil {
		return nil, ErrShiftTypeNotFound
	}
	if err := shiftType.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", ErrShiftTypeInvalid, err.Error())
	}
	taken, err := s.nameTaken(ctx, shiftType.Name, shiftType.ID)
	if err != nil {
		return nil, err
	}
	if taken {
		return nil, ErrShiftTypeNameTaken
	}
	shiftType.TenantID = existing.TenantID
	if err := s.repo.Update(ctx, shiftType); err != nil {
		if isShiftTypeNameConflict(err) {
			return nil, ErrShiftTypeNameTaken
		}
		return nil, err
	}
	s.getLogger().Info("shift type updated", "shift_type_id", shiftType.ID)
	return shiftType, nil
}

func (s *shiftTypeService) DeleteShiftType(ctx context.Context, id int64) error {
	if id <= 0 {
		return ErrShiftTypeNotFound
	}
	existing, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrShiftTypeNotFound
		}
		return fmt.Errorf("find shift type: %w", err)
	}
	if existing == nil {
		return ErrShiftTypeNotFound
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.getLogger().Info("shift type deleted", "shift_type_id", id)
	return nil
}

func (s *shiftTypeService) CreateDefaultShiftTypes(ctx context.Context) ([]*scheduleModels.ShiftType, error) {
	existing, err := s.repo.ListAll(ctx)
	if err != nil {
		return nil, err
	}
	present := make(map[string]struct{}, len(existing))
	for _, t := range existing {
		present[strings.ToLower(strings.TrimSpace(t.Name))] = struct{}{}
	}

	tenantID := tenant.FromContext(ctx)
	for _, def := range defaultShiftTypes {
		if _, ok := present[strings.ToLower(def.name)]; ok {
			continue
		}
		shiftType := &scheduleModels.ShiftType{
			Name:     def.name,
			Color:    def.color,
			IsActive: true,
		}
		if err := shiftType.Validate(); err != nil {
			return nil, fmt.Errorf("%w: %s", ErrShiftTypeInvalid, err.Error())
		}
		shiftType.SetTenantID(tenantID)
		// CreateIfAbsent (ON CONFLICT DO NOTHING) makes the seed race-safe: a
		// concurrent seed for the same tenant that inserted this default between
		// our ListAll snapshot and here is a clean no-op, not a unique violation.
		// Catching the violation after the fact instead would abort the request's
		// tenant transaction and break the later inserts and the final ListAll.
		if _, err := s.repo.CreateIfAbsent(ctx, shiftType); err != nil {
			return nil, err
		}
	}
	s.getLogger().Info("default shift types seeded")
	return s.repo.ListAll(ctx)
}
