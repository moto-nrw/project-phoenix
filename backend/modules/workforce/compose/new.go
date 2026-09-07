// Package compose wires the Workforce work-time module over the shared tenant
// runtime, the Bun database, and the School Membership capability that owns
// the staff rows a template is assigned to.
package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

// Dependencies are the collaborators the Workforce work-time module cannot
// own. AssignedStaffIDs and RebaseStaffAnchor reach the School Membership
// rows; both are required so a missing wiring fails at startup instead of
// silently skipping the assigned-staff refresh.
type Dependencies struct {
	DB                *bun.DB
	AssignedStaffIDs  func(ctx context.Context, workTimeModelID int64) ([]int64, error)
	RebaseStaffAnchor func(ctx context.Context, workTimeModelID int64, anchorDate string) ([]int64, error)
	Observe           func(Observation)
}

// New composes the Workforce work-time module. Every operation runs on the
// caller's ambient tenant transaction when one exists and otherwise opens one,
// so row-level security decides visibility exactly as it did for the legacy
// repositories.
func New(dependencies Dependencies) (*workforce.Module, error) {
	if dependencies.DB == nil || dependencies.AssignedStaffIDs == nil ||
		dependencies.RebaseStaffAnchor == nil || dependencies.Observe == nil {
		return nil, errors.New("workforce compose: all dependencies are required")
	}
	store := postgres.New(databaseRuntime(dependencies.DB))
	observe := func(observation Observation) {
		observation.Err = mapError(observation.Err)
		dependencies.Observe(observation)
	}
	service := application.New(
		store,
		transaction{},
		assignments{ids: dependencies.AssignedStaffIDs, rebase: dependencies.RebaseStaffAnchor},
		clock{},
		observe,
	)
	return workforce.NewModule(engine{service: service}), nil
}

func databaseRuntime(db *bun.DB) postgres.Database {
	return func(ctx context.Context) (bun.IDB, int64, error) {
		tenantID := tenant.FromContext(ctx)
		transaction, hasTransaction := tenant.TransactionFromContext(ctx)
		if !hasTransaction {
			return db, tenantID, nil
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, tenantID, nil
		case *bun.Tx:
			if tx != nil {
				return tx, tenantID, nil
			}
			return db, tenantID, nil
		default:
			return nil, 0, fmt.Errorf("workforce postgres: unsupported transaction %T", transaction)
		}
	}
}

type transaction struct{}

func (transaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	return tenant.WithinCurrentTenant(ctx, callback)
}

// LockStaffBalance serializes every writer that changes one staff member's
// target working time, so a template refresh and a manual schedule change
// cannot interleave into a half-closed version history.
func (transaction) LockStaffBalance(ctx context.Context, staffID int64) error {
	if staffID <= 0 {
		return errors.New("workforce compose: staff id is required")
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return errors.New("workforce compose: tenant id is required")
	}
	if err := tenant.AcquireLock(ctx, fmt.Sprintf("staff-balance:%d:%d", tenantID, staffID), false); err != nil {
		return fmt.Errorf("lock staff balance writes: %w", err)
	}
	return nil
}

type clock struct{}

func (clock) Today() string { return timezone.TodayDate().String() }

type assignments struct {
	ids    func(context.Context, int64) ([]int64, error)
	rebase func(context.Context, int64, string) ([]int64, error)
}

func (a assignments) AssignedStaffIDs(ctx context.Context, workTimeModelID int64) ([]int64, error) {
	return a.ids(ctx, workTimeModelID)
}

func (a assignments) RebaseAnchor(ctx context.Context, workTimeModelID int64, anchorDate string) ([]int64, error) {
	return a.rebase(ctx, workTimeModelID, anchorDate)
}

type engine struct{ service *application.Service }

func (e engine) ListWorkTimeModels(ctx context.Context) ([]workforce.WorkTimeModel, error) {
	values, err := e.service.ListWorkTimeModels(ctx)
	return modelsToPublic(values), mapError(err)
}

func (e engine) FindWorkTimeModel(ctx context.Context, id int64) (workforce.WorkTimeModel, error) {
	value, err := e.service.FindWorkTimeModel(ctx, id)
	return modelToPublic(value), mapError(err)
}

func (e engine) ListWorkTimeModelsByIDs(ctx context.Context, ids []int64) ([]workforce.WorkTimeModel, error) {
	values, err := e.service.ListWorkTimeModelsByIDs(ctx, ids)
	return modelsToPublic(values), mapError(err)
}

func (e engine) CreateWorkTimeModel(ctx context.Context, input workforce.CreateWorkTimeModel) (workforce.WorkTimeModel, error) {
	value, err := e.service.CreateWorkTimeModel(ctx, modelFieldsToDomain(input.WorkTimeModelFields))
	return modelToPublic(value), mapError(err)
}

func (e engine) UpdateWorkTimeModel(ctx context.Context, input workforce.UpdateWorkTimeModel) (workforce.WorkTimeModel, error) {
	value, err := e.service.UpdateWorkTimeModel(ctx, input.ID, modelFieldsToDomain(input.WorkTimeModelFields))
	return modelToPublic(value), mapError(err)
}

func (e engine) DeleteWorkTimeModel(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteWorkTimeModel(ctx, id))
}

func (e engine) ReplaceStaffSchedule(ctx context.Context, input workforce.ReplaceStaffSchedule) error {
	entries := make([]domain.StaffWorkScheduleFields, 0, len(input.Entries))
	for _, entry := range input.Entries {
		entries = append(entries, domain.StaffWorkScheduleFields{
			WeekIndex: entry.WeekIndex, RotationLength: entry.RotationLength,
			DayOfWeek: entry.DayOfWeek, TargetMinutes: entry.TargetMinutes, StartTime: entry.StartTime,
		})
	}
	return mapError(e.service.ReplaceStaffSchedule(ctx, input.StaffID, entries, input.RotationAnchorDate))
}

func (e engine) CurrentStaffSchedule(ctx context.Context, staffID int64) ([]workforce.StaffWorkSchedule, error) {
	values, err := e.service.CurrentStaffSchedule(ctx, staffID)
	return schedulesToPublic(values), mapError(err)
}

func (e engine) StaffScheduleOn(ctx context.Context, staffID int64, date string) ([]workforce.StaffWorkSchedule, error) {
	values, err := e.service.StaffScheduleOn(ctx, staffID, date)
	return schedulesToPublic(values), mapError(err)
}

func (e engine) StaffSchedulesInRange(ctx context.Context, staffIDs []int64, from, to string) ([]workforce.StaffWorkSchedule, error) {
	values, err := e.service.StaffSchedulesInRange(ctx, staffIDs, from, to)
	return schedulesToPublic(values), mapError(err)
}

func (e engine) HasStaffScheduleHistory(ctx context.Context, staffID int64) (bool, error) {
	value, err := e.service.HasStaffScheduleHistory(ctx, staffID)
	return value, mapError(err)
}

func (e engine) StaffIDsWithScheduleHistory(ctx context.Context, staffIDs []int64) (map[int64]bool, error) {
	value, err := e.service.StaffIDsWithScheduleHistory(ctx, staffIDs)
	return value, mapError(err)
}

func modelFieldsToDomain(fields workforce.WorkTimeModelFields) domain.WorkTimeModelFields {
	entries := make([]domain.WorkTimeModelEntryFields, 0, len(fields.Entries))
	for _, entry := range fields.Entries {
		entries = append(entries, domain.WorkTimeModelEntryFields{
			WeekIndex: entry.WeekIndex, DayOfWeek: entry.DayOfWeek,
			TargetMinutes: entry.TargetMinutes, StartTime: entry.StartTime,
		})
	}
	return domain.WorkTimeModelFields{
		Name: fields.Name, RotationLength: fields.RotationLength,
		RotationAnchorDate: fields.RotationAnchorDate, Entries: entries,
	}
}

func modelsToPublic(values []domain.WorkTimeModel) []workforce.WorkTimeModel {
	if values == nil {
		return nil
	}
	result := make([]workforce.WorkTimeModel, 0, len(values))
	for _, value := range values {
		result = append(result, modelToPublic(value))
	}
	return result
}

func modelToPublic(value domain.WorkTimeModel) workforce.WorkTimeModel {
	model := workforce.WorkTimeModel{
		ID: value.ID, TenantID: value.TenantID, Name: value.Name,
		RotationLength: value.RotationLength, RotationAnchorDate: value.RotationAnchorDate,
	}
	if value.Entries == nil {
		return model
	}
	model.Entries = make([]workforce.WorkTimeModelEntry, 0, len(value.Entries))
	for _, entry := range value.Entries {
		model.Entries = append(model.Entries, workforce.WorkTimeModelEntry{
			WeekIndex: entry.WeekIndex, DayOfWeek: entry.DayOfWeek,
			TargetMinutes: entry.TargetMinutes, StartTime: entry.StartTime,
		})
	}
	return model
}

func schedulesToPublic(values []domain.StaffWorkSchedule) []workforce.StaffWorkSchedule {
	if values == nil {
		return nil
	}
	result := make([]workforce.StaffWorkSchedule, 0, len(values))
	for _, value := range values {
		result = append(result, workforce.StaffWorkSchedule{
			ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID,
			WeekIndex: value.WeekIndex, RotationLength: value.RotationLength,
			DayOfWeek: value.DayOfWeek, TargetMinutes: value.TargetMinutes,
			StartTime: value.StartTime, RotationAnchorDate: value.RotationAnchorDate,
			ValidFrom: value.ValidFrom, ValidUntil: value.ValidUntil,
		})
	}
	return result
}

func mapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrWorkTimeModelNotFound):
		return workforce.ErrWorkTimeModelNotFound
	case errors.Is(err, domain.ErrWorkTimeModelAssigned):
		return workforce.ErrWorkTimeModelAssigned
	case errors.Is(err, domain.ErrInvalidWorkTime):
		return &workforce.InvalidWorkTimeError{Reason: err.Error()}
	default:
		return err
	}
}
