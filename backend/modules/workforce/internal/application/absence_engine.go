package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func (e Engine) FindStaffAbsence(ctx context.Context, id int64) (workforce.StaffAbsence, error) {
	value, err := e.service.FindStaffAbsence(ctx, id)
	return AbsenceToPublic(value), MapError(err)
}

func (e Engine) ListStaffAbsences(ctx context.Context, filter workforce.StaffAbsenceFilter) ([]workforce.StaffAbsence, error) {
	values, err := e.service.ListStaffAbsences(ctx, absenceFilterToDomain(filter))
	return absencesToPublic(values), MapError(err)
}

func (e Engine) CountStaffAbsences(ctx context.Context, filter workforce.StaffAbsenceFilter) (int, error) {
	value, err := e.service.CountStaffAbsences(ctx, absenceFilterToDomain(filter))
	return value, MapError(err)
}

func (e Engine) ListStaffAbsenceRequests(ctx context.Context, filter workforce.StaffAbsenceRequestFilter) ([]workforce.StaffAbsence, error) {
	values, err := e.service.ListStaffAbsenceRequests(ctx, domain.StaffAbsenceRequestFilter{
		Statuses: filter.Statuses, Types: filter.Types, FilterSubjects: filter.FilterSubjects,
		SubjectStaffIDs: filter.SubjectStaffIDs, Limit: filter.Limit, Decided: filter.Decided,
	})
	return absencesToPublic(values), MapError(err)
}

func (e Engine) StaffAbsenceMapForDate(ctx context.Context, date string) (map[int64]string, error) {
	value, err := e.service.StaffAbsenceMapForDate(ctx, date)
	return value, MapError(err)
}

func (e Engine) StaffAbsenceTypeIDMapForDate(ctx context.Context, date string) (map[int64]int64, error) {
	value, err := e.service.StaffAbsenceTypeIDMapForDate(ctx, date)
	return value, MapError(err)
}

func (e Engine) OldestStaffAbsenceDate(ctx context.Context, column, before string) (string, error) {
	value, err := e.service.OldestStaffAbsenceDate(ctx, column, before)
	return value, MapError(err)
}

func (e Engine) LockStaffAbsenceWrites(ctx context.Context, staffID int64) error {
	return MapError(e.service.LockStaffAbsenceWrites(ctx, staffID))
}

func (e Engine) CreateStaffAbsence(ctx context.Context, value *workforce.StaffAbsence) (workforce.StaffAbsence, error) {
	created, err := e.service.CreateStaffAbsence(ctx, absenceToDomain(*value))
	return AbsenceToPublic(created), MapError(err)
}

func (e Engine) UpdateStaffAbsence(ctx context.Context, value workforce.StaffAbsence) (workforce.StaffAbsence, error) {
	updated, err := e.service.UpdateStaffAbsence(ctx, absenceToDomain(value))
	return AbsenceToPublic(updated), MapError(err)
}

func (e Engine) DeleteStaffAbsence(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteStaffAbsence(ctx, id))
}

func (e Engine) DeleteNonHistoricalStaffAbsences(ctx context.Context, staffID int64, from string) (int64, error) {
	value, err := e.service.DeleteNonHistoricalStaffAbsences(ctx, staffID, from)
	return value, MapError(err)
}

func (e Engine) DeleteStaffAbsencesOlderThan(ctx context.Context, column, cutoff string) (int64, error) {
	value, err := e.service.DeleteStaffAbsencesOlderThan(ctx, column, cutoff)
	return value, MapError(err)
}

func (e Engine) ListStaffAbsenceTypes(ctx context.Context) ([]workforce.StaffAbsenceType, error) {
	values, err := e.service.ListStaffAbsenceTypes(ctx)
	if values == nil {
		return nil, MapError(err)
	}
	result := make([]workforce.StaffAbsenceType, 0, len(values))
	for _, value := range values {
		result = append(result, absenceTypeToPublic(value))
	}
	return result, MapError(err)
}

func (e Engine) FindStaffAbsenceType(ctx context.Context, id int64) (workforce.StaffAbsenceType, error) {
	value, err := e.service.FindStaffAbsenceType(ctx, id)
	return absenceTypeToPublic(value), MapError(err)
}

func (e Engine) LockStaffAbsenceType(ctx context.Context, id int64) (workforce.StaffAbsenceType, error) {
	value, err := e.service.LockStaffAbsenceType(ctx, id)
	return absenceTypeToPublic(value), MapError(err)
}

func (e Engine) StaffAbsenceTypeInUse(ctx context.Context, id int64) (bool, error) {
	value, err := e.service.StaffAbsenceTypeInUse(ctx, id)
	return value, MapError(err)
}

func (e Engine) SetAllowance(ctx context.Context, input workforce.SetAbsenceTypeAllowance) (workforce.AbsenceTypeAllowanceSummary, error) {
	value, err := e.service.SetAllowance(ctx, domain.SetAbsenceTypeAllowance{
		StaffID: input.StaffID, AbsenceTypeID: input.AbsenceTypeID, Year: input.Year,
		EntitledDays: input.EntitledDays, Reason: input.Reason, ChangedBy: input.ChangedBy,
	})
	return allowanceSummaryToPublic(value), MapError(err)
}

func (e Engine) AllowanceSummary(ctx context.Context, staffID, absenceTypeID int64, year int) (workforce.AbsenceTypeAllowanceSummary, error) {
	value, err := e.service.AllowanceSummary(ctx, staffID, absenceTypeID, year)
	return allowanceSummaryToPublic(value), MapError(err)
}

func (e Engine) CreateAbsenceType(ctx context.Context, input workforce.CreateAbsenceType) (workforce.StaffAbsenceType, error) {
	value, err := e.service.CreateAbsenceType(ctx, domain.StaffAbsenceTypeFields{
		Name: input.Name, AllowanceEnabled: input.AllowanceEnabled, CarryoverUntil: input.CarryoverUntil,
	})
	return absenceTypeToPublic(value), MapError(err)
}

func (e Engine) UpdateAbsenceType(ctx context.Context, input workforce.UpdateAbsenceType) (workforce.StaffAbsenceType, error) {
	value, err := e.service.UpdateAbsenceType(ctx, input.ID, input.Name, input.IsActive, input.AllowanceEnabled, input.CarryoverUntil)
	return absenceTypeToPublic(value), MapError(err)
}

func (e Engine) CreateStaffAbsenceType(ctx context.Context, fields workforce.StaffAbsenceTypeFields) (workforce.StaffAbsenceType, error) {
	value, err := e.service.CreateStaffAbsenceType(ctx, domain.StaffAbsenceTypeFields{
		Name: fields.Name, BaseType: fields.BaseType, IsActive: fields.IsActive,
		AllowanceEnabled: fields.AllowanceEnabled, CarryoverUntil: fields.CarryoverUntil,
	})
	return absenceTypeToPublic(value), MapError(err)
}

func (e Engine) UpdateStaffAbsenceType(ctx context.Context, value workforce.StaffAbsenceType) (workforce.StaffAbsenceType, error) {
	updated, err := e.service.UpdateStaffAbsenceType(ctx, domain.StaffAbsenceType{
		ID: value.ID, TenantID: value.TenantID, Name: value.Name, BaseType: value.BaseType, IsActive: value.IsActive,
		AllowanceEnabled: value.AllowanceEnabled, CarryoverUntil: value.CarryoverUntil,
	})
	return absenceTypeToPublic(updated), MapError(err)
}

func (e Engine) RecordStaffAbsenceAudit(ctx context.Context, value workforce.StaffAbsenceAudit) (workforce.StaffAbsenceAudit, error) {
	recorded, err := e.service.RecordStaffAbsenceAudit(ctx, domain.StaffAbsenceAudit{
		ID: value.ID, TenantID: value.TenantID, AbsenceID: value.AbsenceID, FromStatus: value.FromStatus,
		ToStatus: value.ToStatus, ActorID: value.ActorID, Note: value.Note, ChangedAt: value.ChangedAt,
		TypeChange: (*domain.AbsenceTypeChange)(value.TypeChange),
	})
	return workforce.StaffAbsenceAudit{
		ID: recorded.ID, TenantID: recorded.TenantID, AbsenceID: recorded.AbsenceID, FromStatus: recorded.FromStatus,
		ToStatus: recorded.ToStatus, ActorID: recorded.ActorID, Note: recorded.Note, ChangedAt: recorded.ChangedAt,
		TypeChange: (*workforce.AbsenceTypeChange)(recorded.TypeChange),
	}, MapError(err)
}

func (e Engine) FindGroupSubstitution(ctx context.Context, id int64) (workforce.GroupSubstitution, error) {
	value, err := e.service.FindGroupSubstitution(ctx, id)
	return substitutionToPublic(value), MapError(err)
}

func (e Engine) LockGroupSubstitution(ctx context.Context, id int64) (workforce.GroupSubstitution, error) {
	value, err := e.service.LockGroupSubstitution(ctx, id)
	return substitutionToPublic(value), MapError(err)
}

func (e Engine) ListGroupSubstitutions(ctx context.Context, filter workforce.GroupSubstitutionFilter) ([]workforce.GroupSubstitution, error) {
	values, err := e.service.ListGroupSubstitutions(ctx, domain.GroupSubstitutionFilter{
		TenantID: filter.TenantID, GroupID: filter.GroupID, GroupIDs: filter.GroupIDs, SubstituteStaffID: filter.SubstituteStaffID,
		RegularStaffID: filter.RegularStaffID, StaffID: filter.StaffID, TargetType: filter.TargetType, On: filter.On,
		OverlapFrom: filter.OverlapFrom, OverlapTo: filter.OverlapTo, EndsOnOrAfter: filter.EndsOnOrAfter,
		ReasonContains: filter.ReasonContains, Limit: filter.Limit, Offset: filter.Offset,
	})
	if values == nil {
		return nil, MapError(err)
	}
	result := make([]workforce.GroupSubstitution, 0, len(values))
	for _, value := range values {
		result = append(result, substitutionToPublic(value))
	}
	return result, MapError(err)
}

func (e Engine) CreateGroupSubstitution(ctx context.Context, value workforce.GroupSubstitution) (workforce.GroupSubstitution, error) {
	created, err := e.service.CreateGroupSubstitution(ctx, substitutionToDomain(value))
	return substitutionToPublic(created), MapError(err)
}

func (e Engine) UpdateGroupSubstitution(ctx context.Context, value workforce.GroupSubstitution) (workforce.GroupSubstitution, error) {
	updated, err := e.service.UpdateGroupSubstitution(ctx, substitutionToDomain(value))
	return substitutionToPublic(updated), MapError(err)
}

func (e Engine) DeleteGroupSubstitution(ctx context.Context, id int64) error {
	return MapError(e.service.DeleteGroupSubstitution(ctx, id))
}

func (e Engine) DeleteGroupSubstitutionsForStaff(ctx context.Context, staffID int64, from string) (int64, error) {
	value, err := e.service.DeleteGroupSubstitutionsForStaff(ctx, staffID, from)
	return value, MapError(err)
}

func (e Engine) LockGroupSubstitutions(ctx context.Context) error {
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return errors.New("workforce: group substitution locks require a transaction")
	}
	return MapError(e.service.LockGroupSubstitutions(ctx))
}

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
