package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// --- work sessions ---

func (e engine) FindWorkSession(ctx context.Context, id int64) (workforce.WorkSession, error) {
	value, err := e.service.FindWorkSession(ctx, id)
	return workSessionToPublic(value), mapWorkSessionError(err)
}

func (e engine) LockOpenWorkSession(ctx context.Context, id int64) (workforce.WorkSession, error) {
	value, err := e.service.LockOpenWorkSession(ctx, id)
	return workSessionToPublic(value), mapWorkSessionError(err)
}

func (e engine) OpenWorkSessionOn(ctx context.Context, staffID int64, date string, lock bool) (workforce.WorkSession, error) {
	value, err := e.service.OpenWorkSessionOn(ctx, staffID, date, lock)
	return workSessionToPublic(value), mapWorkSessionError(err)
}

func (e engine) TodayOpenWorkSession(ctx context.Context, staffID int64) (workforce.WorkSession, error) {
	value, err := e.service.TodayOpenWorkSession(ctx, staffID)
	return workSessionToPublic(value), mapWorkSessionError(err)
}

func (e engine) LatestOpenWorkSession(ctx context.Context, staffID int64) (workforce.WorkSession, error) {
	value, err := e.service.LatestOpenWorkSession(ctx, staffID)
	return workSessionToPublic(value), mapWorkSessionError(err)
}

func (e engine) ListWorkSessions(ctx context.Context, filter workforce.WorkSessionFilter) ([]workforce.WorkSession, error) {
	order := make([]domain.WorkSessionOrder, 0, len(filter.Order))
	for _, entry := range filter.Order {
		order = append(order, domain.WorkSessionOrder{Field: domain.WorkSessionOrderField(entry.Field), Descending: entry.Descending})
	}
	values, err := e.service.ListWorkSessions(ctx, domain.WorkSessionFilter{
		IDs: filter.IDs, StaffID: filter.StaffID, StaffIDs: filter.StaffIDs, Date: filter.Date,
		DateFrom: filter.DateFrom, DateTo: filter.DateTo, DateBefore: filter.DateBefore, Open: filter.Open,
		Order: order, Limit: filter.Limit, Offset: filter.Offset,
	})
	return workSessionsToPublic(values), mapWorkSessionError(err)
}

func (e engine) ListOverlappingWorkSessions(ctx context.Context, staffIDs []int64, from time.Time, to *time.Time) ([]workforce.WorkSession, error) {
	values, err := e.service.ListOverlappingWorkSessions(ctx, staffIDs, from, to)
	return workSessionsToPublic(values), mapWorkSessionError(err)
}

func (e engine) CountWorkSessions(ctx context.Context, filter workforce.WorkSessionFilter) (int, error) {
	value, err := e.service.CountWorkSessions(ctx, domain.WorkSessionFilter{
		IDs: filter.IDs, StaffID: filter.StaffID, StaffIDs: filter.StaffIDs, Date: filter.Date,
		DateFrom: filter.DateFrom, DateTo: filter.DateTo, DateBefore: filter.DateBefore, Open: filter.Open,
	})
	return value, mapWorkSessionError(err)
}

func (e engine) OldestWorkSessionDate(ctx context.Context, column, before string) (string, error) {
	value, err := e.service.OldestWorkSessionDate(ctx, column, before)
	return value, mapWorkSessionError(err)
}

func (e engine) DeleteWorkSessionsOlderThan(ctx context.Context, column, cutoff string) (int64, error) {
	value, err := e.service.DeleteWorkSessionsOlderThan(ctx, column, cutoff)
	return value, mapWorkSessionError(err)
}

func (e engine) WorkPresenceMap(ctx context.Context) (map[int64]string, error) {
	value, err := e.service.WorkPresenceMap(ctx)
	return value, mapWorkSessionError(err)
}

func (e engine) CreateWorkSession(ctx context.Context, value workforce.WorkSession) (workforce.WorkSession, error) {
	created, err := e.service.CreateWorkSession(ctx, workSessionToDomain(value))
	return workSessionToPublic(created), mapWorkSessionError(err)
}

func (e engine) UpdateWorkSession(ctx context.Context, value workforce.WorkSession) (workforce.WorkSession, error) {
	updated, err := e.service.UpdateWorkSession(ctx, workSessionToDomain(value))
	return workSessionToPublic(updated), mapWorkSessionError(err)
}

func (e engine) DeleteWorkSession(ctx context.Context, id int64) error {
	return mapWorkSessionError(e.service.DeleteWorkSession(ctx, id))
}

func (e engine) SetWorkSessionBreakMinutes(ctx context.Context, id int64, minutes int) (int64, error) {
	value, err := e.service.SetWorkSessionBreakMinutes(ctx, id, minutes)
	return value, mapWorkSessionError(err)
}

func (e engine) CloseWorkSession(ctx context.Context, id int64, checkOut time.Time, autoCheckedOut bool) (bool, error) {
	value, err := e.service.CloseWorkSession(ctx, id, checkOut, autoCheckedOut)
	return value, mapWorkSessionError(err)
}

func (e engine) LockStaffBalanceWrites(ctx context.Context, staffID int64) error {
	return mapWorkSessionError(e.service.LockStaffBalanceWrites(ctx, staffID))
}

// --- breaks ---

func (e engine) FindWorkSessionBreak(ctx context.Context, id int64) (workforce.WorkSessionBreak, error) {
	value, err := e.service.FindWorkSessionBreak(ctx, id)
	return workSessionBreakToPublic(value), mapWorkSessionError(err)
}

func (e engine) ListWorkSessionBreaks(ctx context.Context, filter workforce.WorkSessionBreakFilter) ([]workforce.WorkSessionBreak, error) {
	values, err := e.service.ListWorkSessionBreaks(ctx, domain.WorkSessionBreakFilter{
		SessionID: filter.SessionID, SessionIDs: filter.SessionIDs, Active: filter.Active, Limit: filter.Limit, Offset: filter.Offset,
	})
	return workSessionBreaksToPublic(values), mapWorkSessionError(err)
}

func (e engine) ExpiredWorkSessionBreaks(ctx context.Context, before time.Time) ([]workforce.WorkSessionBreak, error) {
	values, err := e.service.ExpiredWorkSessionBreaks(ctx, before)
	return workSessionBreaksToPublic(values), mapWorkSessionError(err)
}

func (e engine) CreateWorkSessionBreak(ctx context.Context, value workforce.WorkSessionBreak) (workforce.WorkSessionBreak, error) {
	created, err := e.service.CreateWorkSessionBreak(ctx, workSessionBreakToDomain(value))
	return workSessionBreakToPublic(created), mapWorkSessionError(err)
}

func (e engine) UpdateWorkSessionBreak(ctx context.Context, value workforce.WorkSessionBreak) (workforce.WorkSessionBreak, error) {
	updated, err := e.service.UpdateWorkSessionBreak(ctx, workSessionBreakToDomain(value))
	return workSessionBreakToPublic(updated), mapWorkSessionError(err)
}

func (e engine) DeleteWorkSessionBreak(ctx context.Context, id int64) error {
	return mapWorkSessionError(e.service.DeleteWorkSessionBreak(ctx, id))
}

func (e engine) EndWorkSessionBreak(ctx context.Context, id int64, endedAt time.Time, durationMinutes int) (int64, error) {
	value, err := e.service.EndWorkSessionBreak(ctx, id, endedAt, durationMinutes)
	return value, mapWorkSessionError(err)
}

func (e engine) SetWorkSessionBreakDuration(ctx context.Context, id int64, durationMinutes int, endedAt time.Time) (int64, error) {
	value, err := e.service.SetWorkSessionBreakDuration(ctx, id, durationMinutes, endedAt)
	return value, mapWorkSessionError(err)
}

// --- staff balance adjustments ---

func (e engine) FindStaffBalanceAdjustment(ctx context.Context, id int64) (workforce.StaffBalanceAdjustment, error) {
	value, err := e.service.FindStaffBalanceAdjustment(ctx, id)
	return adjustmentToPublic(value), mapWorkSessionError(err)
}

func (e engine) ListStaffBalanceAdjustments(ctx context.Context, filter workforce.StaffBalanceAdjustmentFilter) ([]workforce.StaffBalanceAdjustment, error) {
	values, err := e.service.ListStaffBalanceAdjustments(ctx, domain.StaffBalanceAdjustmentFilter{
		StaffID: filter.StaffID, StaffIDs: filter.StaffIDs, Types: filter.Types,
		EffectiveFrom: filter.EffectiveFrom, EffectiveTo: filter.EffectiveTo, Limit: filter.Limit, Offset: filter.Offset,
	})
	if values == nil {
		return nil, mapWorkSessionError(err)
	}
	result := make([]workforce.StaffBalanceAdjustment, 0, len(values))
	for _, value := range values {
		result = append(result, adjustmentToPublic(value))
	}
	return result, mapWorkSessionError(err)
}

func (e engine) CreateStaffBalanceAdjustment(ctx context.Context, value workforce.StaffBalanceAdjustment) (workforce.StaffBalanceAdjustment, error) {
	created, err := e.service.CreateStaffBalanceAdjustment(ctx, adjustmentToDomain(value))
	return adjustmentToPublic(created), mapWorkSessionError(err)
}

func (e engine) UpdateStaffBalanceAdjustment(ctx context.Context, value workforce.StaffBalanceAdjustment) (workforce.StaffBalanceAdjustment, error) {
	updated, err := e.service.UpdateStaffBalanceAdjustment(ctx, adjustmentToDomain(value))
	return adjustmentToPublic(updated), mapWorkSessionError(err)
}

func (e engine) DeleteStaffBalanceAdjustment(ctx context.Context, id int64) error {
	return mapWorkSessionError(e.service.DeleteStaffBalanceAdjustment(ctx, id))
}

// --- staff vacation openings ---

func (e engine) FindStaffVacationOpening(ctx context.Context, id int64) (workforce.StaffVacationOpening, error) {
	value, err := e.service.FindStaffVacationOpening(ctx, id)
	return openingToPublic(value), mapWorkSessionError(err)
}

func (e engine) ListStaffVacationOpenings(ctx context.Context, filter workforce.StaffVacationFilter) ([]workforce.StaffVacationOpening, error) {
	values, err := e.service.ListStaffVacationOpenings(ctx, vacationFilterToDomain(filter))
	if values == nil {
		return nil, mapWorkSessionError(err)
	}
	result := make([]workforce.StaffVacationOpening, 0, len(values))
	for _, value := range values {
		result = append(result, openingToPublic(value))
	}
	return result, mapWorkSessionError(err)
}

func (e engine) CreateStaffVacationOpening(ctx context.Context, value workforce.StaffVacationOpening) (workforce.StaffVacationOpening, error) {
	created, err := e.service.CreateStaffVacationOpening(ctx, openingToDomain(value))
	return openingToPublic(created), mapWorkSessionError(err)
}

func (e engine) UpdateStaffVacationOpening(ctx context.Context, value workforce.StaffVacationOpening) (workforce.StaffVacationOpening, error) {
	updated, err := e.service.UpdateStaffVacationOpening(ctx, openingToDomain(value))
	return openingToPublic(updated), mapWorkSessionError(err)
}

func (e engine) DeleteStaffVacationOpening(ctx context.Context, id int64) error {
	return mapWorkSessionError(e.service.DeleteStaffVacationOpening(ctx, id))
}

// --- staff vacation quota ---

func (e engine) FindStaffVacationQuota(ctx context.Context, id int64) (workforce.StaffVacationQuota, error) {
	value, err := e.service.FindStaffVacationQuota(ctx, id)
	return quotaToPublic(value), mapWorkSessionError(err)
}

func (e engine) ListStaffVacationQuotas(ctx context.Context, filter workforce.StaffVacationFilter) ([]workforce.StaffVacationQuota, error) {
	values, err := e.service.ListStaffVacationQuotas(ctx, vacationFilterToDomain(filter))
	if values == nil {
		return nil, mapWorkSessionError(err)
	}
	result := make([]workforce.StaffVacationQuota, 0, len(values))
	for _, value := range values {
		result = append(result, quotaToPublic(value))
	}
	return result, mapWorkSessionError(err)
}

func (e engine) CreateStaffVacationQuota(ctx context.Context, value workforce.StaffVacationQuota) (workforce.StaffVacationQuota, error) {
	created, err := e.service.CreateStaffVacationQuota(ctx, quotaToDomain(value))
	return quotaToPublic(created), mapWorkSessionError(err)
}

func (e engine) UpdateStaffVacationQuota(ctx context.Context, value workforce.StaffVacationQuota) (workforce.StaffVacationQuota, error) {
	updated, err := e.service.UpdateStaffVacationQuota(ctx, quotaToDomain(value))
	return quotaToPublic(updated), mapWorkSessionError(err)
}

func (e engine) DeleteStaffVacationQuota(ctx context.Context, id int64) error {
	return mapWorkSessionError(e.service.DeleteStaffVacationQuota(ctx, id))
}

func (e engine) UpsertStaffVacationQuota(ctx context.Context, value workforce.StaffVacationQuota) error {
	return mapWorkSessionError(e.service.UpsertStaffVacationQuota(ctx, quotaToDomain(value)))
}

// --- mapping ---

func workSessionToDomain(value workforce.WorkSession) domain.WorkSession {
	return domain.WorkSession{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Date: value.Date, Status: value.Status,
		Source: value.Source, CheckInTime: value.CheckInTime, CheckOutTime: value.CheckOutTime, ReopenedAt: value.ReopenedAt,
		BreakMinutes: value.BreakMinutes, Notes: value.Notes, AutoCheckedOut: value.AutoCheckedOut, CreatedBy: value.CreatedBy,
		UpdatedBy: value.UpdatedBy, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func workSessionToPublic(value domain.WorkSession) workforce.WorkSession {
	return workforce.WorkSession{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Date: value.Date, Status: value.Status,
		Source: value.Source, CheckInTime: value.CheckInTime, CheckOutTime: value.CheckOutTime, ReopenedAt: value.ReopenedAt,
		BreakMinutes: value.BreakMinutes, Notes: value.Notes, AutoCheckedOut: value.AutoCheckedOut, CreatedBy: value.CreatedBy,
		UpdatedBy: value.UpdatedBy, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func workSessionsToPublic(values []domain.WorkSession) []workforce.WorkSession {
	if values == nil {
		return nil
	}
	result := make([]workforce.WorkSession, 0, len(values))
	for _, value := range values {
		result = append(result, workSessionToPublic(value))
	}
	return result
}

func workSessionBreakToDomain(value workforce.WorkSessionBreak) domain.WorkSessionBreak {
	return domain.WorkSessionBreak{
		ID: value.ID, TenantID: value.TenantID, SessionID: value.SessionID, StartedAt: value.StartedAt, EndedAt: value.EndedAt,
		DurationMinutes: value.DurationMinutes, PlannedEndTime: value.PlannedEndTime, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func workSessionBreakToPublic(value domain.WorkSessionBreak) workforce.WorkSessionBreak {
	return workforce.WorkSessionBreak{
		ID: value.ID, TenantID: value.TenantID, SessionID: value.SessionID, StartedAt: value.StartedAt, EndedAt: value.EndedAt,
		DurationMinutes: value.DurationMinutes, PlannedEndTime: value.PlannedEndTime, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func workSessionBreaksToPublic(values []domain.WorkSessionBreak) []workforce.WorkSessionBreak {
	if values == nil {
		return nil
	}
	result := make([]workforce.WorkSessionBreak, 0, len(values))
	for _, value := range values {
		result = append(result, workSessionBreakToPublic(value))
	}
	return result
}

func adjustmentToDomain(value workforce.StaffBalanceAdjustment) domain.StaffBalanceAdjustment {
	return domain.StaffBalanceAdjustment{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Type: value.Type, MinutesDelta: value.MinutesDelta,
		EffectiveDate: value.EffectiveDate, Note: value.Note, DecidedBy: value.DecidedBy, DecidedAt: value.DecidedAt,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func adjustmentToPublic(value domain.StaffBalanceAdjustment) workforce.StaffBalanceAdjustment {
	return workforce.StaffBalanceAdjustment{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Type: value.Type, MinutesDelta: value.MinutesDelta,
		EffectiveDate: value.EffectiveDate, Note: value.Note, DecidedBy: value.DecidedBy, DecidedAt: value.DecidedAt,
		CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func openingToDomain(value workforce.StaffVacationOpening) domain.StaffVacationOpening {
	return domain.StaffVacationOpening{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Year: value.Year, EffectiveDate: value.EffectiveDate,
		TakenBeforeDays: value.TakenBeforeDays, EnteredRemainingDays: value.EnteredRemainingDays, Note: value.Note,
		DecidedBy: value.DecidedBy, DecidedAt: value.DecidedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func openingToPublic(value domain.StaffVacationOpening) workforce.StaffVacationOpening {
	return workforce.StaffVacationOpening{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Year: value.Year, EffectiveDate: value.EffectiveDate,
		TakenBeforeDays: value.TakenBeforeDays, EnteredRemainingDays: value.EnteredRemainingDays, Note: value.Note,
		DecidedBy: value.DecidedBy, DecidedAt: value.DecidedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func quotaToDomain(value workforce.StaffVacationQuota) domain.StaffVacationQuota {
	return domain.StaffVacationQuota{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Year: value.Year,
		EntitledDays: value.EntitledDays, CarryoverDays: value.CarryoverDays, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func quotaToPublic(value domain.StaffVacationQuota) workforce.StaffVacationQuota {
	return workforce.StaffVacationQuota{
		ID: value.ID, TenantID: value.TenantID, StaffID: value.StaffID, Year: value.Year,
		EntitledDays: value.EntitledDays, CarryoverDays: value.CarryoverDays, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
	}
}

func vacationFilterToDomain(filter workforce.StaffVacationFilter) domain.StaffVacationFilter {
	order := make([]domain.StaffVacationOrder, 0, len(filter.Order))
	for _, entry := range filter.Order {
		order = append(order, domain.StaffVacationOrder{Field: domain.StaffVacationOrderField(entry.Field), Descending: entry.Descending})
	}
	return domain.StaffVacationFilter{
		StaffID: filter.StaffID, StaffIDs: filter.StaffIDs, Year: filter.Year, Order: order, Limit: filter.Limit, Offset: filter.Offset,
	}
}

// mapWorkSessionError translates the work-session family errors before the
// shared mapping handles the rest.
func mapWorkSessionError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrWorkSessionNotFound):
		return workforce.ErrWorkSessionNotFound
	case errors.Is(err, domain.ErrWorkSessionBreakNotFound):
		return workforce.ErrWorkSessionBreakNotFound
	case errors.Is(err, domain.ErrStaffBalanceAdjustmentNotFound):
		return workforce.ErrStaffBalanceAdjustmentNotFound
	case errors.Is(err, domain.ErrStaffVacationOpeningNotFound):
		return workforce.ErrStaffVacationOpeningNotFound
	case errors.Is(err, domain.ErrStaffVacationQuotaNotFound):
		return workforce.ErrStaffVacationQuotaNotFound
	case errors.Is(err, domain.ErrInvalidWorkSession):
		return &workforce.InvalidWorkSessionError{Reason: err.Error()}
	case errors.Is(err, domain.ErrWorkSessionAlreadyOpen):
		return &workforce.ConflictError{Kind: workforce.ErrWorkSessionAlreadyOpen, Cause: conflictCause(err)}
	default:
		return mapError(err)
	}
}
