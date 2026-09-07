package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// LockStaffAbsence serializes overlap-sensitive absence writes of one staff
// member. It uses the same key the legacy repository did, so every writer of
// the same rows keeps queuing on one lock.
func (t transaction) LockStaffAbsence(ctx context.Context, staffID int64) error {
	if staffID <= 0 {
		return errors.New("workforce compose: staff id is required")
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 {
		return errors.New("workforce compose: tenant id is required")
	}
	if err := t.acquireXactLock(ctx, fmt.Sprintf("staff-absence:%d:%d", tenantID, staffID)); err != nil {
		return fmt.Errorf("lock staff absence writes: %w", err)
	}
	return nil
}

// --- staff absences ---

func (e engine) FindStaffAbsence(ctx context.Context, id int64) (workforce.StaffAbsence, error) {
	value, err := e.service.FindStaffAbsence(ctx, id)
	return absenceToPublic(value), mapError(err)
}

func (e engine) ListStaffAbsences(ctx context.Context, filter workforce.StaffAbsenceFilter) ([]workforce.StaffAbsence, error) {
	values, err := e.service.ListStaffAbsences(ctx, absenceFilterToDomain(filter))
	return absencesToPublic(values), mapError(err)
}

func (e engine) CountStaffAbsences(ctx context.Context, filter workforce.StaffAbsenceFilter) (int, error) {
	value, err := e.service.CountStaffAbsences(ctx, absenceFilterToDomain(filter))
	return value, mapError(err)
}

func (e engine) ListStaffAbsenceRequests(ctx context.Context, filter workforce.StaffAbsenceRequestFilter) ([]workforce.StaffAbsence, error) {
	values, err := e.service.ListStaffAbsenceRequests(ctx, domain.StaffAbsenceRequestFilter{
		Statuses: filter.Statuses, Types: filter.Types, FilterSubjects: filter.FilterSubjects,
		SubjectStaffIDs: filter.SubjectStaffIDs, Limit: filter.Limit, Decided: filter.Decided,
	})
	return absencesToPublic(values), mapError(err)
}

func (e engine) StaffAbsenceMapForDate(ctx context.Context, date string) (map[int64]string, error) {
	value, err := e.service.StaffAbsenceMapForDate(ctx, date)
	return value, mapError(err)
}

func (e engine) StaffAbsenceTypeIDMapForDate(ctx context.Context, date string) (map[int64]int64, error) {
	value, err := e.service.StaffAbsenceTypeIDMapForDate(ctx, date)
	return value, mapError(err)
}

func (e engine) OldestStaffAbsenceDate(ctx context.Context, column, before string) (string, error) {
	value, err := e.service.OldestStaffAbsenceDate(ctx, column, before)
	return value, mapError(err)
}

func (e engine) LockStaffAbsenceWrites(ctx context.Context, staffID int64) error {
	return mapError(e.service.LockStaffAbsenceWrites(ctx, staffID))
}

func (e engine) CreateStaffAbsence(ctx context.Context, value workforce.StaffAbsence) (workforce.StaffAbsence, error) {
	created, err := e.service.CreateStaffAbsence(ctx, absenceToDomain(value))
	return absenceToPublic(created), mapError(err)
}

func (e engine) UpdateStaffAbsence(ctx context.Context, value workforce.StaffAbsence) (workforce.StaffAbsence, error) {
	updated, err := e.service.UpdateStaffAbsence(ctx, absenceToDomain(value))
	return absenceToPublic(updated), mapError(err)
}

func (e engine) DeleteStaffAbsence(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteStaffAbsence(ctx, id))
}

func (e engine) DeleteNonHistoricalStaffAbsences(ctx context.Context, staffID int64, from string) (int64, error) {
	value, err := e.service.DeleteNonHistoricalStaffAbsences(ctx, staffID, from)
	return value, mapError(err)
}

func (e engine) DeleteStaffAbsencesOlderThan(ctx context.Context, column, cutoff string) (int64, error) {
	value, err := e.service.DeleteStaffAbsencesOlderThan(ctx, column, cutoff)
	return value, mapError(err)
}

// --- absence types ---

func (e engine) ListStaffAbsenceTypes(ctx context.Context) ([]workforce.StaffAbsenceType, error) {
	values, err := e.service.ListStaffAbsenceTypes(ctx)
	if values == nil {
		return nil, mapError(err)
	}
	result := make([]workforce.StaffAbsenceType, 0, len(values))
	for _, value := range values {
		result = append(result, absenceTypeToPublic(value))
	}
	return result, mapError(err)
}

func (e engine) FindStaffAbsenceType(ctx context.Context, id int64) (workforce.StaffAbsenceType, error) {
	value, err := e.service.FindStaffAbsenceType(ctx, id)
	return absenceTypeToPublic(value), mapError(err)
}

func (e engine) LockStaffAbsenceType(ctx context.Context, id int64) (workforce.StaffAbsenceType, error) {
	value, err := e.service.LockStaffAbsenceType(ctx, id)
	return absenceTypeToPublic(value), mapError(err)
}

func (e engine) StaffAbsenceTypeInUse(ctx context.Context, id int64) (bool, error) {
	value, err := e.service.StaffAbsenceTypeInUse(ctx, id)
	return value, mapError(err)
}

func (e engine) CreateStaffAbsenceType(ctx context.Context, fields workforce.StaffAbsenceTypeFields) (workforce.StaffAbsenceType, error) {
	value, err := e.service.CreateStaffAbsenceType(ctx, domain.StaffAbsenceTypeFields{
		Name: fields.Name, BaseType: fields.BaseType, IsActive: fields.IsActive,
		AllowanceEnabled: fields.AllowanceEnabled, OverrunPolicy: fields.OverrunPolicy,
	})
	return absenceTypeToPublic(value), mapError(err)
}

func (e engine) UpdateStaffAbsenceType(ctx context.Context, value workforce.StaffAbsenceType) (workforce.StaffAbsenceType, error) {
	updated, err := e.service.UpdateStaffAbsenceType(ctx, domain.StaffAbsenceType{
		ID: value.ID, TenantID: value.TenantID, Name: value.Name, BaseType: value.BaseType, IsActive: value.IsActive,
		AllowanceEnabled: value.AllowanceEnabled, OverrunPolicy: value.OverrunPolicy,
	})
	return absenceTypeToPublic(updated), mapError(err)
}

// --- audit ---

func (e engine) RecordStaffAbsenceAudit(ctx context.Context, value workforce.StaffAbsenceAudit) (workforce.StaffAbsenceAudit, error) {
	recorded, err := e.service.RecordStaffAbsenceAudit(ctx, domain.StaffAbsenceAudit{
		ID: value.ID, TenantID: value.TenantID, AbsenceID: value.AbsenceID, FromStatus: value.FromStatus,
		ToStatus: value.ToStatus, ActorID: value.ActorID, Note: value.Note, ChangedAt: value.ChangedAt,
	})
	return workforce.StaffAbsenceAudit{
		ID: recorded.ID, TenantID: recorded.TenantID, AbsenceID: recorded.AbsenceID, FromStatus: recorded.FromStatus,
		ToStatus: recorded.ToStatus, ActorID: recorded.ActorID, Note: recorded.Note, ChangedAt: recorded.ChangedAt,
	}, mapError(err)
}

// --- group substitutions ---

func (e engine) FindGroupSubstitution(ctx context.Context, id int64) (workforce.GroupSubstitution, error) {
	value, err := e.service.FindGroupSubstitution(ctx, id)
	return substitutionToPublic(value), mapError(err)
}

func (e engine) LockGroupSubstitution(ctx context.Context, id int64) (workforce.GroupSubstitution, error) {
	value, err := e.service.LockGroupSubstitution(ctx, id)
	return substitutionToPublic(value), mapError(err)
}

func (e engine) ListGroupSubstitutions(ctx context.Context, filter workforce.GroupSubstitutionFilter) ([]workforce.GroupSubstitution, error) {
	values, err := e.service.ListGroupSubstitutions(ctx, domain.GroupSubstitutionFilter{
		TenantID: filter.TenantID, GroupID: filter.GroupID, GroupIDs: filter.GroupIDs, SubstituteStaffID: filter.SubstituteStaffID,
		RegularStaffID: filter.RegularStaffID, StaffID: filter.StaffID, TargetType: filter.TargetType, On: filter.On,
		OverlapFrom: filter.OverlapFrom, OverlapTo: filter.OverlapTo, EndsOnOrAfter: filter.EndsOnOrAfter,
		ReasonContains: filter.ReasonContains, Limit: filter.Limit, Offset: filter.Offset,
	})
	if values == nil {
		return nil, mapError(err)
	}
	result := make([]workforce.GroupSubstitution, 0, len(values))
	for _, value := range values {
		result = append(result, substitutionToPublic(value))
	}
	return result, mapError(err)
}

func (e engine) CreateGroupSubstitution(ctx context.Context, value workforce.GroupSubstitution) (workforce.GroupSubstitution, error) {
	created, err := e.service.CreateGroupSubstitution(ctx, substitutionToDomain(value))
	return substitutionToPublic(created), mapError(err)
}

func (e engine) UpdateGroupSubstitution(ctx context.Context, value workforce.GroupSubstitution) (workforce.GroupSubstitution, error) {
	updated, err := e.service.UpdateGroupSubstitution(ctx, substitutionToDomain(value))
	return substitutionToPublic(updated), mapError(err)
}

func (e engine) DeleteGroupSubstitution(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteGroupSubstitution(ctx, id))
}

func (e engine) DeleteGroupSubstitutionsForStaff(ctx context.Context, staffID int64, from string) (int64, error) {
	value, err := e.service.DeleteGroupSubstitutionsForStaff(ctx, staffID, from)
	return value, mapError(err)
}

// --- mapping ---

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

func absenceToPublic(value domain.StaffAbsence) workforce.StaffAbsence {
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
		result = append(result, absenceToPublic(value))
	}
	return result
}

func absenceTypeToPublic(value domain.StaffAbsenceType) workforce.StaffAbsenceType {
	return workforce.StaffAbsenceType{
		ID: value.ID, TenantID: value.TenantID, Name: value.Name, BaseType: value.BaseType, IsActive: value.IsActive,
		AllowanceEnabled: value.AllowanceEnabled, OverrunPolicy: value.OverrunPolicy, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
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
