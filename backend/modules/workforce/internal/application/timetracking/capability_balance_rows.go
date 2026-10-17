package timetracking

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The row adapters in this file serve the balance adjustment, vacation
// opening and vacation quota row contracts from the Workforce capability,
// like capability_work_session_rows.go does for the work sessions and their
// breaks. They perform no persistence of their own.

// --- staff balance adjustments ---

type staffBalanceAdjustmentRepository struct {
	workforce workforce.Capability
	errs      recordErrors
}

// NewBalanceAdjustmentRows serves
// StaffBalanceAdjustmentRepository from the Workforce capability.
func NewBalanceAdjustmentRows(capability workforce.Capability) StaffBalanceAdjustmentRepository {
	if capability == nil {
		panic("staff balance adjustment repository adapter: Workforce capability is required")
	}
	return staffBalanceAdjustmentRepository{workforce: capability}
}

func (r staffBalanceAdjustmentRepository) Create(ctx context.Context, entity *StaffBalanceAdjustment) error {
	if entity == nil {
		return errors.New("StaffBalanceAdjustment cannot be nil or zero value")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	created, err := r.workforce.CreateStaffBalanceAdjustment(ctx, adjustmentToCapability(entity))
	if err != nil {
		return r.errs.workSessionWrite("create", err)
	}
	applyAdjustmentToLegacy(entity, created)
	return nil
}

func (r staffBalanceAdjustmentRepository) FindByID(ctx context.Context, id any) (*StaffBalanceAdjustment, error) {
	adjustmentID, err := legacyID(id)
	if err != nil {
		return nil, r.errs.WrapRecordError("find by id", err)
	}
	value, err := r.workforce.FindStaffBalanceAdjustment(ctx, adjustmentID)
	if err != nil {
		return nil, r.errs.read("find by id", err, workforce.ErrStaffBalanceAdjustmentNotFound)
	}
	return adjustmentToLegacy(value), nil
}

func (r staffBalanceAdjustmentRepository) Update(ctx context.Context, entity *StaffBalanceAdjustment) error {
	if entity == nil {
		return errors.New("StaffBalanceAdjustment cannot be nil or zero value")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	updated, err := r.workforce.UpdateStaffBalanceAdjustment(ctx, adjustmentToCapability(entity))
	if err != nil {
		if errors.Is(err, workforce.ErrStaffBalanceAdjustmentNotFound) {
			return r.errs.rowsAffected("update StaffBalanceAdjustment")
		}
		return r.errs.workSessionWrite("update", err)
	}
	applyAdjustmentToLegacy(entity, updated)
	return nil
}

func (r staffBalanceAdjustmentRepository) Delete(ctx context.Context, id any) error {
	adjustmentID, err := legacyID(id)
	if err != nil {
		return r.errs.WrapRecordError("delete", err)
	}
	if err := r.workforce.DeleteStaffBalanceAdjustment(ctx, adjustmentID); err != nil {
		return r.errs.workSessionWrite("delete", err)
	}
	return nil
}

func (r staffBalanceAdjustmentRepository) ListWithFilter(ctx context.Context, filter workforce.StaffBalanceAdjustmentFilter) ([]*StaffBalanceAdjustment, error) {
	return r.list(ctx, "list with options", filter)
}

func (r staffBalanceAdjustmentRepository) list(ctx context.Context, op string, filter workforce.StaffBalanceAdjustmentFilter) ([]*StaffBalanceAdjustment, error) {
	values, err := r.workforce.ListStaffBalanceAdjustments(ctx, filter)
	if err != nil {
		return nil, r.errs.read(op, err, nil)
	}
	return adjustmentsToLegacy(values), nil
}

func (r staffBalanceAdjustmentRepository) LockStaffBalanceWrites(ctx context.Context, staffID int64) error {
	return r.workforce.LockStaffBalanceWrites(ctx, staffID)
}

func (r staffBalanceAdjustmentRepository) GetByStaffAndDateRange(ctx context.Context, staffID int64, from, to calendar.Date) ([]*StaffBalanceAdjustment, error) {
	return r.list(ctx, "get balance adjustments by staff+range", workforce.StaffBalanceAdjustmentFilter{
		StaffID: staffID, EffectiveFrom: from.String(), EffectiveTo: to.String(),
	})
}

func (r staffBalanceAdjustmentRepository) GetByStaffIDsAndDateRange(ctx context.Context, staffIDs []int64, from, to calendar.Date) (map[int64][]*StaffBalanceAdjustment, error) {
	result := make(map[int64][]*StaffBalanceAdjustment, len(staffIDs))
	if len(staffIDs) == 0 {
		return result, nil
	}
	values, err := r.workforce.ListStaffBalanceAdjustments(ctx, workforce.StaffBalanceAdjustmentFilter{
		StaffIDs: staffIDs, EffectiveFrom: from.String(), EffectiveTo: to.String(),
	})
	if err != nil {
		return nil, r.errs.read("get balance adjustments by staff IDs+range", err, nil)
	}
	for _, value := range values {
		result[value.StaffID] = append(result[value.StaffID], adjustmentToLegacy(value))
	}
	return result, nil
}

// --- staff vacation openings ---

type staffVacationOpeningRepository struct {
	workforce workforce.Capability
	errs      recordErrors
}

// NewVacationOpeningRows serves
// StaffVacationOpeningRepository from the Workforce capability.
func NewVacationOpeningRows(capability workforce.Capability) StaffVacationOpeningRepository {
	if capability == nil {
		panic("staff vacation opening repository adapter: Workforce capability is required")
	}
	return staffVacationOpeningRepository{workforce: capability}
}

func (r staffVacationOpeningRepository) Create(ctx context.Context, entity *StaffVacationOpening) error {
	if entity == nil {
		return errors.New("StaffVacationOpening cannot be nil or zero value")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	created, err := r.workforce.CreateStaffVacationOpening(ctx, openingToCapability(entity))
	if err != nil {
		return r.errs.workSessionWrite("create", err)
	}
	applyOpeningToLegacy(entity, created)
	return nil
}

func (r staffVacationOpeningRepository) FindByID(ctx context.Context, id any) (*StaffVacationOpening, error) {
	openingID, err := legacyID(id)
	if err != nil {
		return nil, r.errs.WrapRecordError("find by id", err)
	}
	value, err := r.workforce.FindStaffVacationOpening(ctx, openingID)
	if err != nil {
		return nil, r.errs.read("find by id", err, workforce.ErrStaffVacationOpeningNotFound)
	}
	return openingToLegacy(value), nil
}

func (r staffVacationOpeningRepository) Update(ctx context.Context, entity *StaffVacationOpening) error {
	if entity == nil {
		return errors.New("StaffVacationOpening cannot be nil or zero value")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	updated, err := r.workforce.UpdateStaffVacationOpening(ctx, openingToCapability(entity))
	if err != nil {
		if errors.Is(err, workforce.ErrStaffVacationOpeningNotFound) {
			return r.errs.rowsAffected("update StaffVacationOpening")
		}
		return r.errs.workSessionWrite("update", err)
	}
	applyOpeningToLegacy(entity, updated)
	return nil
}

func (r staffVacationOpeningRepository) Delete(ctx context.Context, id any) error {
	openingID, err := legacyID(id)
	if err != nil {
		return r.errs.WrapRecordError("delete", err)
	}
	if err := r.workforce.DeleteStaffVacationOpening(ctx, openingID); err != nil {
		return r.errs.workSessionWrite("delete", err)
	}
	return nil
}

func (r staffVacationOpeningRepository) GetByStaffAndYear(ctx context.Context, staffID int64, year int) (*StaffVacationOpening, error) {
	values, err := r.workforce.ListStaffVacationOpenings(ctx, workforce.StaffVacationFilter{StaffID: staffID, Year: year, Limit: 1})
	if err != nil {
		return nil, r.errs.read("get vacation opening by staff+year", err, nil)
	}
	if len(values) == 0 {
		return nil, nil
	}
	return openingToLegacy(values[0]), nil
}

func (r staffVacationOpeningRepository) GetByStaffIDsAndYear(ctx context.Context, staffIDs []int64, year int) (map[int64]*StaffVacationOpening, error) {
	result := make(map[int64]*StaffVacationOpening, len(staffIDs))
	if len(staffIDs) == 0 {
		return result, nil
	}
	values, err := r.workforce.ListStaffVacationOpenings(ctx, workforce.StaffVacationFilter{StaffIDs: staffIDs, Year: year})
	if err != nil {
		return nil, r.errs.read("get vacation openings by staff IDs+year", err, nil)
	}
	for _, value := range values {
		result[value.StaffID] = openingToLegacy(value)
	}
	return result, nil
}

// --- staff vacation quota ---

type staffVacationQuotaRepository struct {
	workforce workforce.Capability
	errs      recordErrors
}

// NewVacationQuotaRows serves
// StaffVacationQuotaRepository from the Workforce capability.
func NewVacationQuotaRows(capability workforce.Capability) StaffVacationQuotaRepository {
	if capability == nil {
		panic("staff vacation quota repository adapter: Workforce capability is required")
	}
	return staffVacationQuotaRepository{workforce: capability}
}

func (r staffVacationQuotaRepository) Create(ctx context.Context, entity *StaffVacationQuota) error {
	if entity == nil {
		return errors.New("StaffVacationQuota cannot be nil or zero value")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	created, err := r.workforce.CreateStaffVacationQuota(ctx, quotaToCapability(entity))
	if err != nil {
		return r.errs.workSessionWrite("create", err)
	}
	applyQuotaToLegacy(entity, created)
	return nil
}

func (r staffVacationQuotaRepository) FindByID(ctx context.Context, id any) (*StaffVacationQuota, error) {
	quotaID, err := legacyID(id)
	if err != nil {
		return nil, r.errs.WrapRecordError("find by id", err)
	}
	value, err := r.workforce.FindStaffVacationQuota(ctx, quotaID)
	if err != nil {
		return nil, r.errs.read("find by id", err, workforce.ErrStaffVacationQuotaNotFound)
	}
	return quotaToLegacy(value), nil
}

func (r staffVacationQuotaRepository) Update(ctx context.Context, entity *StaffVacationQuota) error {
	if entity == nil {
		return errors.New("StaffVacationQuota cannot be nil or zero value")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	updated, err := r.workforce.UpdateStaffVacationQuota(ctx, quotaToCapability(entity))
	if err != nil {
		if errors.Is(err, workforce.ErrStaffVacationQuotaNotFound) {
			return r.errs.rowsAffected("update StaffVacationQuota")
		}
		return r.errs.workSessionWrite("update", err)
	}
	applyQuotaToLegacy(entity, updated)
	return nil
}

func (r staffVacationQuotaRepository) Delete(ctx context.Context, id any) error {
	quotaID, err := legacyID(id)
	if err != nil {
		return r.errs.WrapRecordError("delete", err)
	}
	if err := r.workforce.DeleteStaffVacationQuota(ctx, quotaID); err != nil {
		return r.errs.workSessionWrite("delete", err)
	}
	return nil
}

func (r staffVacationQuotaRepository) GetByStaffAndYear(ctx context.Context, staffID int64, year int) (*StaffVacationQuota, error) {
	values, err := r.workforce.ListStaffVacationQuotas(ctx, workforce.StaffVacationFilter{StaffID: staffID, Year: year, Limit: 1})
	if err != nil {
		return nil, r.errs.read("get vacation quota by staff+year", err, nil)
	}
	if len(values) == 0 {
		return nil, nil
	}
	return quotaToLegacy(values[0]), nil
}

func (r staffVacationQuotaRepository) GetByStaffIDsAndYear(ctx context.Context, staffIDs []int64, year int) (map[int64]*StaffVacationQuota, error) {
	result := make(map[int64]*StaffVacationQuota, len(staffIDs))
	if len(staffIDs) == 0 {
		return result, nil
	}
	values, err := r.workforce.ListStaffVacationQuotas(ctx, workforce.StaffVacationFilter{StaffIDs: staffIDs, Year: year})
	if err != nil {
		return nil, r.errs.read("get vacation quotas by staff IDs+year", err, nil)
	}
	for _, value := range values {
		result[value.StaffID] = quotaToLegacy(value)
	}
	return result, nil
}

func (r staffVacationQuotaRepository) Upsert(ctx context.Context, entity *StaffVacationQuota) error {
	if entity == nil {
		return fmt.Errorf("quota cannot be nil")
	}
	if err := entity.Validate(); err != nil {
		return err
	}
	if err := r.workforce.UpsertStaffVacationQuota(ctx, quotaToCapability(entity)); err != nil {
		return r.errs.workSessionWrite("upsert vacation quota", err)
	}
	return nil
}

// --- mapping ---

func adjustmentToCapability(entity *StaffBalanceAdjustment) workforce.StaffBalanceAdjustment {
	return workforce.StaffBalanceAdjustment{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, Type: entity.Type, MinutesDelta: entity.MinutesDelta,
		EffectiveDate: entity.EffectiveDate.String(), Note: entity.Note, DecidedBy: entity.DecidedBy, DecidedAt: entity.DecidedAt,
		CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func adjustmentToLegacy(value workforce.StaffBalanceAdjustment) *StaffBalanceAdjustment {
	entity := &StaffBalanceAdjustment{}
	applyAdjustmentToLegacy(entity, value)
	return entity
}

func applyAdjustmentToLegacy(entity *StaffBalanceAdjustment, value workforce.StaffBalanceAdjustment) {
	entity.ID = value.ID
	entity.CreatedAt = value.CreatedAt
	entity.UpdatedAt = value.UpdatedAt
	entity.TenantID = value.TenantID
	entity.StaffID = value.StaffID
	entity.Type = value.Type
	entity.MinutesDelta = value.MinutesDelta
	entity.EffectiveDate = calendar.Date(value.EffectiveDate)
	entity.Note = value.Note
	entity.DecidedBy = value.DecidedBy
	entity.DecidedAt = value.DecidedAt
}

func adjustmentsToLegacy(values []workforce.StaffBalanceAdjustment) []*StaffBalanceAdjustment {
	result := make([]*StaffBalanceAdjustment, 0, len(values))
	for _, value := range values {
		result = append(result, adjustmentToLegacy(value))
	}
	return result
}

func openingToCapability(entity *StaffVacationOpening) workforce.StaffVacationOpening {
	return workforce.StaffVacationOpening{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, Year: entity.Year, EffectiveDate: entity.EffectiveDate.String(),
		TakenBeforeDays: entity.TakenBeforeDays, EnteredRemainingDays: entity.EnteredRemainingDays, Note: entity.Note,
		DecidedBy: entity.DecidedBy, DecidedAt: entity.DecidedAt, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func openingToLegacy(value workforce.StaffVacationOpening) *StaffVacationOpening {
	entity := &StaffVacationOpening{}
	applyOpeningToLegacy(entity, value)
	return entity
}

func applyOpeningToLegacy(entity *StaffVacationOpening, value workforce.StaffVacationOpening) {
	entity.ID = value.ID
	entity.CreatedAt = value.CreatedAt
	entity.UpdatedAt = value.UpdatedAt
	entity.TenantID = value.TenantID
	entity.StaffID = value.StaffID
	entity.Year = value.Year
	entity.EffectiveDate = calendar.Date(value.EffectiveDate)
	entity.TakenBeforeDays = value.TakenBeforeDays
	entity.EnteredRemainingDays = value.EnteredRemainingDays
	entity.Note = value.Note
	entity.DecidedBy = value.DecidedBy
	entity.DecidedAt = value.DecidedAt
}

func quotaToCapability(entity *StaffVacationQuota) workforce.StaffVacationQuota {
	return workforce.StaffVacationQuota{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, Year: entity.Year,
		EntitledDays: entity.EntitledDays, CarryoverDays: entity.CarryoverDays, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
		ChangeReason: entity.ChangeReason, ChangedBy: entity.ChangedBy,
	}
}

func quotaToLegacy(value workforce.StaffVacationQuota) *StaffVacationQuota {
	entity := &StaffVacationQuota{}
	applyQuotaToLegacy(entity, value)
	return entity
}

func applyQuotaToLegacy(entity *StaffVacationQuota, value workforce.StaffVacationQuota) {
	entity.ID = value.ID
	entity.CreatedAt = value.CreatedAt
	entity.UpdatedAt = value.UpdatedAt
	entity.TenantID = value.TenantID
	entity.StaffID = value.StaffID
	entity.Year = value.Year
	entity.EntitledDays = value.EntitledDays
	entity.CarryoverDays = value.CarryoverDays
}
