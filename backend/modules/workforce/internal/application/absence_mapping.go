package application

import (
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

func absenceFilterToDomain(filter workforce.StaffAbsenceFilter) domain.StaffAbsenceFilter {
	order := make([]domain.StaffAbsenceOrder, 0, len(filter.Order))
	for _, entry := range filter.Order {
		order = append(order, domain.StaffAbsenceOrder{Field: domain.StaffAbsenceOrderField(entry.Field), Descending: entry.Descending})
	}
	return domain.StaffAbsenceFilter{
		StaffID: filter.StaffID, StaffIDs: filter.StaffIDs, Statuses: filter.Statuses, Types: filter.Types,
		OverlapFrom: filter.OverlapFrom, OverlapTo: filter.OverlapTo, DateEndBefore: filter.DateEndBefore,
		DateStartBefore: filter.DateStartBefore, NonHistoricalFrom: filter.NonHistoricalFrom,
		Order: order, Limit: filter.Limit, Offset: filter.Offset,
	}
}

func absenceToDomain(value workforce.StaffAbsence) domain.StaffAbsence {
	return domain.StaffAbsence{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, AbsenceType: value.AbsenceType,
		AbsenceTypeID: value.AbsenceTypeID, DateStart: value.DateStart, DateEnd: value.DateEnd,
		HalfDay: value.HalfDay, StartHalfDay: value.StartHalfDay, EndHalfDay: value.EndHalfDay, Note: value.Note,
		Status: value.Status, ApprovedBy: value.ApprovedBy, ApprovedAt: value.ApprovedAt, CreatedBy: value.CreatedBy,
		WorkingDays: value.WorkingDays, DecisionNote: value.DecisionNote, RequestedAt: value.RequestedAt,
		SubstituteStaffID: value.SubstituteStaffID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

// AbsenceToPublic maps an absence onto the contract.
func AbsenceToPublic(value domain.StaffAbsence) workforce.StaffAbsence {
	return workforce.StaffAbsence{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, AbsenceType: value.AbsenceType,
		AbsenceTypeID: value.AbsenceTypeID, DateStart: value.DateStart, DateEnd: value.DateEnd,
		HalfDay: value.HalfDay, StartHalfDay: value.StartHalfDay, EndHalfDay: value.EndHalfDay, Note: value.Note,
		Status: value.Status, ApprovedBy: value.ApprovedBy, ApprovedAt: value.ApprovedAt, CreatedBy: value.CreatedBy,
		WorkingDays: value.WorkingDays, DecisionNote: value.DecisionNote, RequestedAt: value.RequestedAt,
		SubstituteStaffID: value.SubstituteStaffID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func absencesToPublic(values []domain.StaffAbsence) []workforce.StaffAbsence {
	if values == nil {
		return nil
	}
	result := make([]workforce.StaffAbsence, 0, len(values))
	for _, value := range values {
		result = append(result, AbsenceToPublic(value))
	}
	return result
}

func absenceTypeToPublic(value domain.StaffAbsenceType) workforce.StaffAbsenceType {
	return workforce.StaffAbsenceType{
		ID: value.ID, TenantID: value.TenantID, Name: value.Name, BaseType: value.BaseType, IsActive: value.IsActive,
		AllowanceEnabled: value.AllowanceEnabled, CarryoverUntil: value.CarryoverUntil,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func substitutionToDomain(value workforce.GroupSubstitution) domain.GroupSubstitution {
	return domain.GroupSubstitution{
		ID: value.ID, TenantID: value.TenantID, TargetType: value.TargetType, GroupID: value.GroupID,
		RegularStaffID: value.RegularStaffID, SubstituteStaffID: value.SubstituteStaffID,
		StartDate: value.StartDate, EndDate: value.EndDate, Reason: value.Reason,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func substitutionToPublic(value domain.GroupSubstitution) workforce.GroupSubstitution {
	return workforce.GroupSubstitution{
		ID: value.ID, TenantID: value.TenantID, TargetType: value.TargetType, GroupID: value.GroupID,
		RegularStaffID: value.RegularStaffID, SubstituteStaffID: value.SubstituteStaffID,
		StartDate: value.StartDate, EndDate: value.EndDate, Reason: value.Reason,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}
