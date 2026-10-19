package timetracking

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ListAbsences returns a staff member's absences for a date range, status, or
// both. The generic repository filter keeps tenant scoping in one place.
func (s *staffAbsenceService) ListAbsences(ctx context.Context, staffID int64, filter StaffAbsenceListFilter) ([]*StaffAbsenceResponse, error) {
	if (filter.From == nil) != (filter.To == nil) {
		return nil, fmt.Errorf("from and to must be provided together")
	}
	if filter.From == nil && filter.Status == "" {
		return nil, fmt.Errorf("absence list filter is required")
	}
	if filter.Status != "" && !slices.Contains(ValidAbsenceStatuses, filter.Status) {
		return nil, &workforce.TimeTrackingError{Kind: workforce.ErrAbsenceInvalid, Cause: fmt.Errorf("invalid absence status")}
	}

	query := workforce.StaffAbsenceFilter{StaffID: staffID}
	if filter.From != nil && filter.To != nil {
		query.OverlapFrom, query.OverlapTo = filter.From.String(), filter.To.String()
	}
	if filter.Status != "" {
		query.Statuses = []string{filter.Status}
	}
	if filter.From != nil {
		query.Order = []workforce.StaffAbsenceOrder{{Field: workforce.StaffAbsenceOrderDateStart}}
	} else {
		query.Order = []workforce.StaffAbsenceOrder{{Field: workforce.StaffAbsenceOrderRequestedAt, Descending: true}}
	}

	absences, err := s.absenceRepo.ListWithFilter(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get absences: %w", err)
	}

	responses := make([]*StaffAbsenceResponse, len(absences))
	for i, a := range absences {
		responses[i] = toAbsenceResponse(a)
	}

	return s.withLabels(ctx, responses...), nil
}

// GetAbsencesForRange preserves the range-only service contract used by staff
// detail views.
func (s *staffAbsenceService) GetAbsencesForRange(ctx context.Context, staffID int64, from, to calendar.Date) ([]*StaffAbsenceResponse, error) {
	absences, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get absences: %w", err)
	}

	responses := make([]*StaffAbsenceResponse, len(absences))
	for i, a := range absences {
		responses[i] = toAbsenceResponse(a)
	}

	return s.withLabels(ctx, responses...), nil
}

// HasAbsenceOnDate checks if a staff member has an absence on a specific date
func (s *staffAbsenceService) HasAbsenceOnDate(ctx context.Context, staffID int64, date calendar.Date) (bool, *StaffAbsence, error) {
	absence, err := s.absenceRepo.GetByStaffAndDate(ctx, staffID, date)
	if err != nil {
		return false, nil, fmt.Errorf("failed to check absence: %w", err)
	}
	if absence == nil {
		return false, nil, nil
	}
	if !isEffectiveAbsenceStatus(absence.Status) {
		return false, nil, nil
	}
	return true, absence, nil
}

func toAbsenceResponse(a *StaffAbsence) *StaffAbsenceResponse {
	var absenceTypeID *string
	if a.AbsenceTypeID != nil {
		value := strconv.FormatInt(*a.AbsenceTypeID, 10)
		absenceTypeID = &value
	}
	return &StaffAbsenceResponse{
		StaffAbsence:  a,
		DurationDays:  absenceDurationDays(a),
		AbsenceTypeID: absenceTypeID,
	}
}

func isEffectiveAbsenceStatus(status string) bool {
	return status == AbsenceStatusReported ||
		status == AbsenceStatusApproved
}

func blocksAbsenceRange(status string) bool {
	return status == AbsenceStatusReported ||
		status == AbsenceStatusRequested ||
		status == AbsenceStatusQuestion ||
		status == AbsenceStatusApproved
}

func isVacationWorkflowAbsence(absence *StaffAbsence) bool {
	if absence.AbsenceType != AbsenceTypeVacation {
		return false
	}
	return absence.Status == AbsenceStatusRequested ||
		absence.Status == AbsenceStatusQuestion ||
		absence.Status == AbsenceStatusApproved ||
		absence.Status == AbsenceStatusDeclined ||
		absence.Status == AbsenceStatusCanceled
}

// isDirectVacation reports vacation the Leitung entered without a request. It
// spends the Urlaubskontingent, so only a manager may change or delete it.
func isDirectVacation(absence *StaffAbsence) bool {
	return absence.AbsenceType == AbsenceTypeVacation &&
		absence.Status == AbsenceStatusReported
}

func filterBlockingAbsences(rows []*StaffAbsence) []*StaffAbsence {
	filtered := make([]*StaffAbsence, 0, len(rows))
	for _, row := range rows {
		if blocksAbsenceRange(row.Status) {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func (s *staffAbsenceService) ListAbsenceRequests(ctx context.Context, req AbsenceRequestListQuery) ([]*StaffAbsenceRequestItem, error) {
	filter := AbsenceRequestFilter{
		Types:   req.Types,
		Search:  req.Search,
		Decided: req.History,
	}
	if req.History {
		// Only requests that went through the approval flow have a history.
		// Admin-direct entries (status "reported") were never requested.
		filter.Statuses = []string{
			AbsenceStatusApproved,
			AbsenceStatusDeclined,
			AbsenceStatusCanceled,
		}
	} else {
		filter.Statuses = []string{
			AbsenceStatusRequested,
			AbsenceStatusQuestion,
		}
	}

	rows, err := s.absenceRepo.ListRequests(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list absence requests: %w", err)
	}
	responses := make([]*StaffAbsenceResponse, len(rows))
	for i, row := range rows {
		responses[i] = toAbsenceResponse(row.StaffAbsence)
	}
	s.withLabels(ctx, responses...)

	items := make([]*StaffAbsenceRequestItem, len(rows))
	for i, row := range rows {
		items[i] = &StaffAbsenceRequestItem{
			StaffAbsenceResponse: responses[i],
			StaffName:            row.StaffName,
			DecidedByName:        row.DecidedByName,
		}
	}
	return items, nil
}

func (s *staffAbsenceService) ListPendingRequests(ctx context.Context) ([]*StaffAbsenceResponse, error) {
	rows, err := s.absenceRepo.ListByStatuses(ctx, []string{
		AbsenceStatusRequested,
		AbsenceStatusQuestion,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pending requests: %w", err)
	}
	responses := make([]*StaffAbsenceResponse, len(rows))
	for i, r := range rows {
		responses[i] = toAbsenceResponse(r)
	}
	return s.withLabels(ctx, responses...), nil
}
