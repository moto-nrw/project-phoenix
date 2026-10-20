package timetracking

import (
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// --- mapping ---

func StaffBalanceAdjustmentToCapability(entity *StaffBalanceAdjustment) workforce.StaffBalanceAdjustment {
	return workforce.StaffBalanceAdjustment{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, Type: entity.Type, MinutesDelta: entity.MinutesDelta,
		EffectiveDate: entity.EffectiveDate.String(), Note: entity.Note, DecidedBy: entity.DecidedBy, DecidedAt: entity.DecidedAt,
		CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func adjustmentToRecord(value workforce.StaffBalanceAdjustment) *StaffBalanceAdjustment {
	entity := &StaffBalanceAdjustment{}
	applyAdjustmentToRecord(entity, value)
	return entity
}

func applyAdjustmentToRecord(entity *StaffBalanceAdjustment, value workforce.StaffBalanceAdjustment) {
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

func adjustmentsToRecords(values []workforce.StaffBalanceAdjustment) []*StaffBalanceAdjustment {
	result := make([]*StaffBalanceAdjustment, 0, len(values))
	for _, value := range values {
		result = append(result, adjustmentToRecord(value))
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

func openingToRecord(value workforce.StaffVacationOpening) *StaffVacationOpening {
	entity := &StaffVacationOpening{}
	applyOpeningToRecord(entity, value)
	return entity
}

func applyOpeningToRecord(entity *StaffVacationOpening, value workforce.StaffVacationOpening) {
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

func quotaToRecord(value workforce.StaffVacationQuota) *StaffVacationQuota {
	entity := &StaffVacationQuota{}
	applyQuotaToRecord(entity, value)
	return entity
}

func applyQuotaToRecord(entity *StaffVacationQuota, value workforce.StaffVacationQuota) {
	entity.ID = value.ID
	entity.CreatedAt = value.CreatedAt
	entity.UpdatedAt = value.UpdatedAt
	entity.TenantID = value.TenantID
	entity.StaffID = value.StaffID
	entity.Year = value.Year
	entity.EntitledDays = value.EntitledDays
	entity.CarryoverDays = value.CarryoverDays
}

func staffAdjustmentsByID(values []workforce.StaffBalanceAdjustment) map[int64][]*StaffBalanceAdjustment {
	result := make(map[int64][]*StaffBalanceAdjustment)
	for _, value := range values {
		result[value.StaffID] = append(result[value.StaffID], adjustmentToRecord(value))
	}
	return result
}
func firstOpening(values []workforce.StaffVacationOpening) *StaffVacationOpening {
	if len(values) == 0 {
		return nil
	}
	return openingToRecord(values[0])
}
func openingsByStaff(values []workforce.StaffVacationOpening) map[int64]*StaffVacationOpening {
	result := make(map[int64]*StaffVacationOpening)
	for _, value := range values {
		result[value.StaffID] = openingToRecord(value)
	}
	return result
}
func firstQuota(values []workforce.StaffVacationQuota) *StaffVacationQuota {
	if len(values) == 0 {
		return nil
	}
	return quotaToRecord(values[0])
}
func quotasByStaff(values []workforce.StaffVacationQuota) map[int64]*StaffVacationQuota {
	result := make(map[int64]*StaffVacationQuota)
	for _, value := range values {
		result[value.StaffID] = quotaToRecord(value)
	}
	return result
}

func BalanceAdjustmentRecord(value workforce.StaffBalanceAdjustment) *StaffBalanceAdjustment {
	return adjustmentToRecord(value)
}
