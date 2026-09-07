// Package repositoryadapter keeps the legacy work-time repository consumers on
// the Workforce capability while they migrate to its Query and Command
// façades. It performs no persistence of its own: it only maps the legacy
// models onto the public capability types and preserves the error values those
// callers still classify on.
package repositoryadapter

import (
	"context"
	"database/sql"
	"errors"
	"time"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// WorkTimeModelRepository serves configModels.WorkTimeModelRepository from the
// Workforce capability.
type WorkTimeModelRepository struct{ workforce workforce.Capability }

// NewWorkTimeModelRepository binds the adapter to the capability.
func NewWorkTimeModelRepository(capability workforce.Capability) configModels.WorkTimeModelRepository {
	if capability == nil {
		panic("work-time model repository adapter: Workforce capability is required")
	}
	return &WorkTimeModelRepository{workforce: capability}
}

func (r *WorkTimeModelRepository) List(ctx context.Context) ([]*configModels.WorkTimeModel, error) {
	models, err := r.workforce.ListWorkTimeModels(ctx)
	if err != nil {
		return nil, err
	}
	return modelsToLegacy(models), nil
}

func (r *WorkTimeModelRepository) FindByID(ctx context.Context, id int64) (*configModels.WorkTimeModel, error) {
	model, err := r.workforce.FindWorkTimeModel(ctx, id)
	if err != nil {
		return nil, missingAsNoRows(err)
	}
	return modelToLegacy(model), nil
}

func (r *WorkTimeModelRepository) FindByIDs(ctx context.Context, ids []int64) ([]*configModels.WorkTimeModel, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	models, err := r.workforce.ListWorkTimeModelsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	return modelsToLegacy(models), nil
}

// Create persists the template and writes the assigned identity back onto the
// caller's structs, which is the contract the legacy repository had.
func (r *WorkTimeModelRepository) Create(ctx context.Context, model *configModels.WorkTimeModel, entries []*configModels.WorkTimeModelEntry) error {
	if model == nil {
		return errors.New("work-time model is required")
	}
	created, err := r.workforce.CreateWorkTimeModel(ctx, workforce.CreateWorkTimeModel{
		WorkTimeModelFields: fieldsFromLegacy(model, entries),
	})
	if err != nil {
		return err
	}
	model.ID = created.ID
	model.TenantID = created.TenantID
	for _, entry := range entries {
		if entry != nil {
			entry.ModelID = created.ID
		}
	}
	return nil
}

func (r *WorkTimeModelRepository) Update(ctx context.Context, model *configModels.WorkTimeModel, entries []*configModels.WorkTimeModelEntry) error {
	if model == nil {
		return errors.New("work-time model is required")
	}
	updated, err := r.workforce.UpdateWorkTimeModel(ctx, workforce.UpdateWorkTimeModel{
		ID:                  model.ID,
		WorkTimeModelFields: fieldsFromLegacy(model, entries),
	})
	if err != nil {
		return missingAsNoRows(err)
	}
	model.TenantID = updated.TenantID
	for _, entry := range entries {
		if entry != nil {
			entry.ModelID = updated.ID
		}
	}
	return nil
}

func (r *WorkTimeModelRepository) RefreshAssignedStaffSchedules(ctx context.Context, modelID int64) error {
	return missingAsNoRows(r.workforce.RefreshAssignedStaffSchedules(ctx, modelID))
}

func (r *WorkTimeModelRepository) Delete(ctx context.Context, id int64) error {
	return missingAsNoRows(r.workforce.DeleteWorkTimeModel(ctx, id))
}

// StaffWorkScheduleRepository serves configModels.StaffWorkScheduleRepository
// from the Workforce capability.
type StaffWorkScheduleRepository struct{ workforce workforce.Capability }

// NewStaffWorkScheduleRepository binds the adapter to the capability.
func NewStaffWorkScheduleRepository(capability workforce.Capability) configModels.StaffWorkScheduleRepository {
	if capability == nil {
		panic("staff work schedule repository adapter: Workforce capability is required")
	}
	return &StaffWorkScheduleRepository{workforce: capability}
}

func (r *StaffWorkScheduleRepository) GetCurrentByStaffID(ctx context.Context, staffID int64) ([]*configModels.StaffWorkSchedule, error) {
	rows, err := r.workforce.CurrentStaffSchedule(ctx, staffID)
	if err != nil {
		return nil, err
	}
	return schedulesToLegacy(rows), nil
}

func (r *StaffWorkScheduleRepository) GetByStaffIDAndDate(ctx context.Context, staffID int64, date configModels.CalendarDate) ([]*configModels.StaffWorkSchedule, error) {
	rows, err := r.workforce.StaffScheduleOn(ctx, staffID, string(date))
	if err != nil {
		return nil, err
	}
	return schedulesToLegacy(rows), nil
}

func (r *StaffWorkScheduleRepository) FindByStaffIDsValidInRange(ctx context.Context, staffIDs []int64, from, to configModels.CalendarDate) ([]*configModels.StaffWorkSchedule, error) {
	if len(staffIDs) == 0 {
		return nil, nil
	}
	rows, err := r.workforce.StaffSchedulesInRange(ctx, staffIDs, string(from), string(to))
	if err != nil {
		return nil, err
	}
	return schedulesToLegacy(rows), nil
}

func (r *StaffWorkScheduleRepository) HasScheduleHistory(ctx context.Context, staffID int64) (bool, error) {
	return r.workforce.HasStaffScheduleHistory(ctx, staffID)
}

func (r *StaffWorkScheduleRepository) FindStaffIDsWithScheduleHistory(ctx context.Context, staffIDs []int64) (map[int64]bool, error) {
	return r.workforce.StaffIDsWithScheduleHistory(ctx, staffIDs)
}

func (r *StaffWorkScheduleRepository) ReplaceSchedule(ctx context.Context, staffID int64, entries []*configModels.StaffWorkSchedule, anchor configModels.CalendarDate) error {
	rows := make([]workforce.StaffWorkScheduleEntry, 0, len(entries))
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		rows = append(rows, workforce.StaffWorkScheduleEntry{
			WeekIndex:      entry.WeekIndex,
			RotationLength: entry.RotationLength,
			DayOfWeek:      entry.DayOfWeek,
			TargetMinutes:  entry.TargetMinutes,
			StartTime:      clockToPublic(entry.StartTime),
		})
	}
	return r.workforce.ReplaceStaffSchedule(ctx, workforce.ReplaceStaffSchedule{
		StaffID: staffID, Entries: rows, RotationAnchorDate: string(anchor),
	})
}

// --- mapping ---

// missingAsNoRows restores the sql.ErrNoRows the legacy repositories returned
// for a template that is not visible to the caller's tenant.
func missingAsNoRows(err error) error {
	if errors.Is(err, workforce.ErrWorkTimeModelNotFound) {
		return sql.ErrNoRows
	}
	return err
}

func fieldsFromLegacy(model *configModels.WorkTimeModel, entries []*configModels.WorkTimeModelEntry) workforce.WorkTimeModelFields {
	fields := workforce.WorkTimeModelFields{
		Name:               model.Name,
		RotationLength:     model.RotationLength,
		RotationAnchorDate: string(model.RotationAnchorDate),
		Entries:            make([]workforce.WorkTimeModelEntry, 0, len(entries)),
	}
	for _, entry := range entries {
		if entry == nil {
			continue
		}
		fields.Entries = append(fields.Entries, workforce.WorkTimeModelEntry{
			WeekIndex:     entry.WeekIndex,
			DayOfWeek:     entry.DayOfWeek,
			TargetMinutes: entry.TargetMinutes,
			StartTime:     clockToPublic(entry.StartTime),
		})
	}
	return fields
}

func modelsToLegacy(models []workforce.WorkTimeModel) []*configModels.WorkTimeModel {
	if models == nil {
		return nil
	}
	result := make([]*configModels.WorkTimeModel, 0, len(models))
	for _, model := range models {
		result = append(result, modelToLegacy(model))
	}
	return result
}

func modelToLegacy(model workforce.WorkTimeModel) *configModels.WorkTimeModel {
	legacy := &configModels.WorkTimeModel{
		ID:                 model.ID,
		TenantID:           model.TenantID,
		Name:               model.Name,
		RotationLength:     model.RotationLength,
		RotationAnchorDate: configModels.CalendarDate(model.RotationAnchorDate),
	}
	if model.Entries == nil {
		return legacy
	}
	legacy.Entries = make([]*configModels.WorkTimeModelEntry, 0, len(model.Entries))
	for _, entry := range model.Entries {
		legacy.Entries = append(legacy.Entries, &configModels.WorkTimeModelEntry{
			ModelID:       model.ID,
			WeekIndex:     entry.WeekIndex,
			DayOfWeek:     entry.DayOfWeek,
			TargetMinutes: entry.TargetMinutes,
			StartTime:     clockFromPublic(entry.StartTime),
		})
	}
	return legacy
}

func schedulesToLegacy(rows []workforce.StaffWorkSchedule) []*configModels.StaffWorkSchedule {
	if rows == nil {
		return nil
	}
	result := make([]*configModels.StaffWorkSchedule, 0, len(rows))
	for _, row := range rows {
		legacy := &configModels.StaffWorkSchedule{
			ID:             row.ID,
			TenantID:       row.TenantID,
			StaffID:        row.StaffID,
			WeekIndex:      row.WeekIndex,
			RotationLength: row.RotationLength,
			DayOfWeek:      row.DayOfWeek,
			TargetMinutes:  row.TargetMinutes,
			StartTime:      clockFromPublic(row.StartTime),
			ValidFrom:      configModels.CalendarDate(row.ValidFrom),
		}
		if row.RotationAnchorDate != "" {
			anchor := configModels.CalendarDate(row.RotationAnchorDate)
			legacy.RotationAnchorDate = &anchor
		}
		if row.ValidUntil != "" {
			until := configModels.CalendarDate(row.ValidUntil)
			legacy.ValidUntil = &until
		}
		result = append(result, legacy)
	}
	return result
}

// clockToPublic renders a legacy wall-clock instant as the capability's
// HH:MM:SS string, reading the time-of-day components only and discarding the
// date anchor the value carries.
func clockToPublic(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format(workforce.ClockLayout)
}

// clockFromPublic rebuilds the legacy wall-clock instant. time.Parse anchors
// it at UTC, which is exactly what the retained TIME columns are bound with.
func clockFromPublic(value string) *time.Time {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(workforce.ClockLayout, value)
	if err != nil {
		return nil
	}
	return &parsed
}
