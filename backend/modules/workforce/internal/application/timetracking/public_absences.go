package timetracking

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

type staffAbsenceCapability struct {
	absences StaffAbsenceService
}

// StaffAbsenceCapability serves workforce.StaffAbsences from the retained
// absence service.
func StaffAbsenceCapability(absences StaffAbsenceService) workforce.StaffAbsences {
	if absences == nil {
		panic("staff absence capability: absence service is required")
	}
	return staffAbsenceCapability{absences: absences}
}

func publicStaffAbsence(entity *StaffAbsence) *workforce.StaffAbsence {
	if entity == nil {
		return nil
	}
	return &workforce.StaffAbsence{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, AbsenceType: entity.AbsenceType, AbsenceTypeID: entity.AbsenceTypeID,
		AbsenceTypeLabel: entity.AbsenceTypeLabel, DateStart: entity.DateStart.String(), DateEnd: entity.DateEnd.String(), HalfDay: entity.HalfDay,
		StartHalfDay: entity.StartHalfDay, EndHalfDay: entity.EndHalfDay, Note: entity.Note, Status: entity.Status, ApprovedBy: entity.ApprovedBy,
		ApprovedAt: entity.ApprovedAt, CreatedBy: entity.CreatedBy, WorkingDays: entity.WorkingDays, DecisionNote: entity.DecisionNote,
		RequestedAt: entity.RequestedAt, SubstituteStaffID: entity.SubstituteStaffID, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func publicAbsenceResponse(value *StaffAbsenceResponse) *workforce.StaffAbsenceResponse {
	if value == nil {
		return nil
	}
	return &workforce.StaffAbsenceResponse{
		StaffAbsence: publicStaffAbsence(value.StaffAbsence), DurationDays: value.DurationDays, AbsenceTypeID: value.AbsenceTypeID,
	}
}

func publicAbsenceResponses(values []*StaffAbsenceResponse) []*workforce.StaffAbsenceResponse {
	if values == nil {
		return nil
	}
	result := make([]*workforce.StaffAbsenceResponse, 0, len(values))
	for _, value := range values {
		result = append(result, publicAbsenceResponse(value))
	}
	return result
}

func publicVacationOpening(entity *StaffVacationOpening) *workforce.StaffVacationOpening {
	if entity == nil {
		return nil
	}
	return &workforce.StaffVacationOpening{
		ID: entity.ID, TenantID: entity.TenantID, StaffID: entity.StaffID, Year: entity.Year, EffectiveDate: entity.EffectiveDate.String(),
		TakenBeforeDays: entity.TakenBeforeDays, EnteredRemainingDays: entity.EnteredRemainingDays, Note: entity.Note,
		DecidedBy: entity.DecidedBy, DecidedAt: entity.DecidedAt, CreatedAt: entity.CreatedAt, UpdatedAt: entity.UpdatedAt,
	}
}

func (c staffAbsenceCapability) absence(value *StaffAbsenceResponse, err error) (*workforce.StaffAbsenceResponse, error) {
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicAbsenceResponse(value), nil
}

func (c staffAbsenceCapability) ListAbsences(ctx context.Context, staffID int64, filter workforce.StaffAbsenceListFilter) ([]*workforce.StaffAbsenceResponse, error) {
	from, err := parseOptionalCapabilityDate(filter.From, "from")
	if err != nil {
		return nil, err
	}
	to, err := parseOptionalCapabilityDate(filter.To, "to")
	if err != nil {
		return nil, err
	}
	values, err := c.absences.ListAbsences(ctx, staffID, StaffAbsenceListFilter{From: from, To: to, Status: filter.Status})
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicAbsenceResponses(values), nil
}

func (c staffAbsenceCapability) AbsencesForRange(ctx context.Context, staffID int64, from, to string) ([]*workforce.StaffAbsenceResponse, error) {
	fromDate, toDate, err := parseCapabilityRange(from, to)
	if err != nil {
		return nil, err
	}
	values, err := c.absences.GetAbsencesForRange(ctx, staffID, fromDate, toDate)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicAbsenceResponses(values), nil
}

// The sick-report writes run with a planner and return the plan changes they
// asked for; the shift-plan-sync workflow applies them (#4825).

func (c staffAbsenceCapability) CreateOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, request workforce.CreateAbsenceRequest) (*workforce.StaffAbsenceResponse, workforce.SickCascadePlan, error) {
	absences, plan, err := planSickCascade(c.absences)
	if err != nil {
		return nil, nil, err
	}
	return plannedAbsence(plan)(c.absence(absences.CreateOwnAbsence(ctx, staffID, actorAccountID, CreateAbsenceRequest(request))))
}

func (c staffAbsenceCapability) CreateAbsenceFor(ctx context.Context, subjectStaffID, createdByStaffID int64, actorAccountID *int64, request workforce.CreateAbsenceRequest) (*workforce.StaffAbsenceResponse, workforce.SickCascadePlan, error) {
	absences, plan, err := planSickCascade(c.absences)
	if err != nil {
		return nil, nil, err
	}
	return plannedAbsence(plan)(c.absence(absences.CreateAbsenceFor(ctx, subjectStaffID, createdByStaffID, actorAccountID, CreateAbsenceRequest(request))))
}

func (c staffAbsenceCapability) UpdateAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64, request workforce.UpdateAbsenceRequest) (*workforce.StaffAbsenceResponse, workforce.SickCascadePlan, error) {
	absences, plan, err := planSickCascade(c.absences)
	if err != nil {
		return nil, nil, err
	}
	return plannedAbsence(plan)(c.absence(absences.UpdateAbsence(ctx, staffID, actorAccountID, absenceID, UpdateAbsenceRequest(request))))
}

func (c staffAbsenceCapability) DeleteOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64) (workforce.SickCascadePlan, error) {
	absences, plan, err := planSickCascade(c.absences)
	if err != nil {
		return nil, err
	}
	return plannedDeletion(plan, mapTimeTrackingFailure(absences.DeleteOwnAbsence(ctx, staffID, actorAccountID, absenceID)))
}

func (c staffAbsenceCapability) DeleteAbsenceFor(ctx context.Context, subjectStaffID, actorStaffID int64, actorAccountID *int64, absenceID int64) (workforce.SickCascadePlan, error) {
	absences, plan, err := planSickCascade(c.absences)
	if err != nil {
		return nil, err
	}
	return plannedDeletion(plan, mapTimeTrackingFailure(absences.DeleteAbsenceFor(ctx, subjectStaffID, actorStaffID, actorAccountID, absenceID)))
}

// plannedAbsence attaches the recorded plan to a successful write.
func plannedAbsence(plan *workforce.SickCascadePlan) func(*workforce.StaffAbsenceResponse, error) (*workforce.StaffAbsenceResponse, workforce.SickCascadePlan, error) {
	return func(value *workforce.StaffAbsenceResponse, err error) (*workforce.StaffAbsenceResponse, workforce.SickCascadePlan, error) {
		if err != nil {
			return nil, nil, err
		}
		return value, *plan, nil
	}
}

// plannedDeletion attaches the recorded plan to a successful deletion.
func plannedDeletion(plan *workforce.SickCascadePlan, err error) (workforce.SickCascadePlan, error) {
	if err != nil {
		return nil, err
	}
	return *plan, nil
}

func (c staffAbsenceCapability) PreviewCompTimeBalance(ctx context.Context, staffID int64, start, end string, halfDay bool) (*workforce.CompTimeBalancePreview, error) {
	startDate, err := parseCapabilityDate(start, "start")
	if err != nil {
		return nil, err
	}
	endDate, err := parseCapabilityDate(end, "end")
	if err != nil {
		return nil, err
	}
	preview, err := c.absences.PreviewCompTimeBalance(ctx, staffID, startDate, endDate, halfDay)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if preview == nil {
		return nil, nil
	}
	return new(workforce.CompTimeBalancePreview(*preview)), nil
}

func (c staffAbsenceCapability) RebookAbsences(ctx context.Context, staffID, actorAccountID int64, request workforce.RebookAbsencesRequest) (*workforce.AbsenceRebookingResult, error) {
	value, err := c.absences.RebookAbsences(ctx, RebookAbsencesRequest{
		StaffID: staffID, ActorAccountID: actorAccountID, AbsenceIDs: request.AbsenceIDs,
		AbsenceType: request.AbsenceType, AbsenceTypeID: request.AbsenceTypeID,
		Reason: request.Reason, DryRun: request.DryRun,
	})
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	result := &workforce.AbsenceRebookingResult{
		Absences: publicAbsenceResponses(value.Absences), Days: value.Days,
		BalanceDeltaMinutes: value.BalanceDeltaMinutes, AllowanceExceeded: value.AllowanceExceeded,
		VacationExceeded: value.VacationExceeded, Applied: value.Applied,
	}
	for _, allowance := range value.Allowances {
		result.Allowances = append(result.Allowances, workforce.RebookingAllowanceYear{
			Year: allowance.Year, EntitledDays: allowance.EntitledDays, TakenDays: allowance.TakenDays,
			ReservedDays: allowance.ReservedDays, RemainingDays: allowance.RemainingDays, BookingDays: allowance.BookingDays,
		})
	}
	for _, year := range value.Vacation {
		result.Vacation = append(result.Vacation, workforce.RebookingVacationYear(year))
	}
	return result, nil
}

func (c staffAbsenceCapability) RequestVacation(ctx context.Context, staffID int64, request workforce.RequestVacationRequest) (*workforce.StaffAbsenceResponse, error) {
	return c.absence(c.absences.RequestVacation(ctx, staffID, RequestVacationRequest(request)))
}

func (c staffAbsenceCapability) CancelAbsence(ctx context.Context, staffID, actorAccountID, absenceID int64) error {
	return mapTimeTrackingFailure(c.absences.CancelAbsence(ctx, staffID, actorAccountID, absenceID))
}

func (c staffAbsenceCapability) ResubmitAbsence(ctx context.Context, staffID, actorAccountID, absenceID int64, note string) (*workforce.StaffAbsenceResponse, error) {
	return c.absence(c.absences.ResubmitAbsence(ctx, staffID, actorAccountID, absenceID, note))
}

func (c staffAbsenceCapability) ApproveAbsence(ctx context.Context, absenceID, actorAccountID, decidedByStaffID int64, note string) (*workforce.StaffAbsenceResponse, error) {
	return c.absence(c.absences.ApproveAbsence(ctx, absenceID, actorAccountID, decidedByStaffID, note))
}

func (c staffAbsenceCapability) DenyAbsence(ctx context.Context, absenceID, actorAccountID, decidedByStaffID int64, reason string) (*workforce.StaffAbsenceResponse, error) {
	return c.absence(c.absences.DenyAbsence(ctx, absenceID, actorAccountID, decidedByStaffID, reason))
}

func (c staffAbsenceCapability) QuestionAbsence(ctx context.Context, absenceID, actorAccountID int64, note string) (*workforce.StaffAbsenceResponse, error) {
	return c.absence(c.absences.QuestionAbsence(ctx, absenceID, actorAccountID, note))
}

func (c staffAbsenceCapability) ListPendingRequests(ctx context.Context) ([]*workforce.StaffAbsenceResponse, error) {
	values, err := c.absences.ListPendingRequests(ctx)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicAbsenceResponses(values), nil
}

func (c staffAbsenceCapability) ListAbsenceRequests(ctx context.Context, query workforce.AbsenceRequestListQuery) ([]*workforce.StaffAbsenceRequestItem, error) {
	values, err := c.absences.ListAbsenceRequests(ctx, AbsenceRequestListQuery(query))
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if values == nil {
		return nil, nil
	}
	result := make([]*workforce.StaffAbsenceRequestItem, 0, len(values))
	for _, value := range values {
		if value == nil {
			result = append(result, nil)
			continue
		}
		result = append(result, &workforce.StaffAbsenceRequestItem{
			StaffAbsenceResponse: publicAbsenceResponse(value.StaffAbsenceResponse), StaffName: value.StaffName, DecidedByName: value.DecidedByName,
		})
	}
	return result, nil
}

func (c staffAbsenceCapability) VacationQuotaSummary(ctx context.Context, staffID int64, year int) (*workforce.VacationQuotaSummary, error) {
	summary, err := c.absences.GetVacationQuotaSummary(ctx, staffID, year)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	if summary == nil {
		return nil, nil
	}
	return &workforce.VacationQuotaSummary{
		StaffID: summary.StaffID, Year: summary.Year, EntitledDays: summary.EntitledDays, CarryoverDays: summary.CarryoverDays,
		TakenBeforeDays: summary.TakenBeforeDays, TakenDays: summary.TakenDays, ReservedDays: summary.ReservedDays,
		RemainingDays: summary.RemainingDays, Opening: publicVacationOpening(summary.Opening),
	}, nil
}

func (c staffAbsenceCapability) UpsertVacationQuota(ctx context.Context, staffID int64, year int, entitled, carryover float64) error {
	return mapTimeTrackingFailure(c.absences.UpsertVacationQuota(ctx, staffID, year, entitled, carryover))
}

func (c staffAbsenceCapability) SetVacationQuota(ctx context.Context, change workforce.VacationQuotaChange) error {
	return mapTimeTrackingFailure(c.absences.SetVacationQuota(ctx, VacationQuotaChange(change)))
}

// VacationTakeoverCapability binds the import to the public Workforce contract.
func VacationTakeoverCapability(absences StaffAbsenceService) workforce.VacationTakeovers {
	if absences == nil {
		return nil
	}
	return staffAbsenceCapability{absences: absences}
}

func (c staffAbsenceCapability) ValidateVacationOpeningAbsencesBefore(ctx context.Context, staffID int64, effectiveDate string) error {
	effective, err := parseCapabilityDate(effectiveDate, "effective_date")
	if err != nil {
		return err
	}
	return mapTimeTrackingFailure(c.absences.ValidateVacationOpeningAbsencesBefore(ctx, staffID, effective))
}

func (c staffAbsenceCapability) SetVacationOpening(ctx context.Context, staffID, decidedBy int64, request workforce.SetVacationOpeningRequest) (*workforce.StaffVacationOpening, error) {
	effective, err := parseCapabilityDate(request.EffectiveDate, "effective_date")
	if err != nil {
		return nil, err
	}
	opening, err := c.absences.SetVacationOpening(ctx, staffID, decidedBy, SetVacationOpeningRequest{
		EffectiveDate: effective, RemainingDays: request.RemainingDays, Note: request.Note,
	})
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return publicVacationOpening(opening), nil
}

func (c staffAbsenceCapability) DeleteVacationOpening(ctx context.Context, staffID, deletedBy int64, year int) error {
	return mapTimeTrackingFailure(c.absences.DeleteVacationOpening(ctx, staffID, deletedBy, year))
}
