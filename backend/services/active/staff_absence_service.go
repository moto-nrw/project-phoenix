package active

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// CreateAbsenceRequest defines the request for creating an absence
type CreateAbsenceRequest struct {
	AbsenceType string `json:"absence_type"`
	DateStart   string `json:"date_start"`
	DateEnd     string `json:"date_end"`
	HalfDay     bool   `json:"half_day"`
	Note        string `json:"note"`
}

// UpdateAbsenceRequest defines the request for updating an absence
type UpdateAbsenceRequest struct {
	AbsenceType *string `json:"absence_type"`
	DateStart   *string `json:"date_start"`
	DateEnd     *string `json:"date_end"`
	HalfDay     *bool   `json:"half_day"`
	Note        *string `json:"note"`
}

// StaffAbsenceResponse wraps an absence with calculated fields
type StaffAbsenceResponse struct {
	*activeModels.StaffAbsence
	DurationDays int `json:"duration_days"`
}

// RequestVacationRequest is what the MA submits via "Urlaub beantragen".
// StartHalfDay/EndHalfDay are Personio-style boundary half-days: 0.5 day
// off the first / last day of the range, full days for everything between.
type RequestVacationRequest struct {
	DateStart         string `json:"date_start"`
	DateEnd           string `json:"date_end"`
	StartHalfDay      bool   `json:"start_half_day"`
	EndHalfDay        bool   `json:"end_half_day"`
	Note              string `json:"note"`
	SubstituteStaffID *int64 `json:"substitute_staff_id,omitempty"`
}

// VacationDecisionRequest is the admin's approve/deny payload
type VacationDecisionRequest struct {
	DecisionNote string `json:"decision_note"`
}

// VacationQuotaSummary aggregates entitled/taken/reserved for a staff/year
type VacationQuotaSummary struct {
	StaffID       int64   `json:"staff_id"`
	Year          int     `json:"year"`
	EntitledDays  float64 `json:"entitled_days"`
	CarryoverDays float64 `json:"carryover_days"`
	TakenDays     float64 `json:"taken_days"`
	ReservedDays  float64 `json:"reserved_days"`
	RemainingDays float64 `json:"remaining_days"`
}

// StaffAbsenceService defines operations for staff absence management
type StaffAbsenceService interface {
	CreateAbsence(ctx context.Context, staffID int64, req CreateAbsenceRequest) (*StaffAbsenceResponse, error)
	UpdateAbsence(ctx context.Context, staffID int64, absenceID int64, req UpdateAbsenceRequest) (*StaffAbsenceResponse, error)
	DeleteAbsence(ctx context.Context, staffID int64, absenceID int64) error
	GetAbsencesForRange(ctx context.Context, staffID int64, from, to timezone.Date) ([]*StaffAbsenceResponse, error)
	HasAbsenceOnDate(ctx context.Context, staffID int64, date timezone.Date) (bool, *activeModels.StaffAbsence, error)

	// Vacation workflow (Tranche 4)
	RequestVacation(ctx context.Context, staffID int64, req RequestVacationRequest) (*StaffAbsenceResponse, error)
	ApproveAbsence(ctx context.Context, absenceID int64, actorAccountID int64, decidedByStaffID int64, note string) (*StaffAbsenceResponse, error)
	DenyAbsence(ctx context.Context, absenceID int64, actorAccountID int64, decidedByStaffID int64, reason string) (*StaffAbsenceResponse, error)
	CancelAbsence(ctx context.Context, staffID int64, actorAccountID int64, absenceID int64) error
	GetVacationQuotaSummary(ctx context.Context, staffID int64, year int) (*VacationQuotaSummary, error)
	UpsertVacationQuota(ctx context.Context, staffID int64, year int, entitled, carryover float64) error
	ListPendingRequests(ctx context.Context) ([]*StaffAbsenceResponse, error)
}

// staffAbsenceService implements StaffAbsenceService
type staffAbsenceService struct {
	absenceRepo     activeModels.StaffAbsenceRepository
	workSessionRepo activeModels.WorkSessionRepository
	quotaRepo       activeModels.StaffVacationQuotaRepository
	auditRepo       activeModels.StaffAbsenceAuditRepository
}

// NewStaffAbsenceService creates a new staff absence service
func NewStaffAbsenceService(
	absenceRepo activeModels.StaffAbsenceRepository,
	workSessionRepo activeModels.WorkSessionRepository,
	quotaRepo activeModels.StaffVacationQuotaRepository,
	auditRepo activeModels.StaffAbsenceAuditRepository,
) StaffAbsenceService {
	return &staffAbsenceService{
		absenceRepo:     absenceRepo,
		workSessionRepo: workSessionRepo,
		quotaRepo:       quotaRepo,
		auditRepo:       auditRepo,
	}
}

// defaultEntitledDays is the fallback when no per-staff quota row exists.
// 30 working days = TVöD-Allgemein standard. Pro-Tenant override via settings
// will come in a follow-up, see Issue #1375 Tranche 4a.
const defaultEntitledDays = 30.0

// CreateAbsence creates a new absence record
func (s *staffAbsenceService) CreateAbsence(ctx context.Context, staffID int64, req CreateAbsenceRequest) (*StaffAbsenceResponse, error) {
	if req.AbsenceType == activeModels.AbsenceTypeVacation {
		return nil, fmt.Errorf("vacation absences must be requested through the vacation flow")
	}

	dateStart, dateEnd, err := parseDateRange(req.DateStart, req.DateEnd)
	if err != nil {
		return nil, err
	}

	// Check for overlapping absences, merge if same type, reject if different type.
	existing, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, dateStart, dateEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing absences: %w", err)
	}

	if len(existing) > 0 {
		blocking := filterBlockingAbsences(existing)
		if len(blocking) > 0 {
			return s.mergeOverlappingAbsences(ctx, blocking, dateStart, dateEnd, req)
		}
	}

	s.warnIfWorkSessionsExist(ctx, staffID, dateStart, dateEnd)

	return s.createNewAbsence(ctx, staffID, dateStart, dateEnd, req)
}

// parseDateRange parses start and end date strings in ISO format.
func parseDateRange(startStr, endStr string) (timezone.Date, timezone.Date, error) {
	dateStart, err := timezone.ParseDate(startStr)
	if err != nil {
		return timezone.Date{}, timezone.Date{}, fmt.Errorf("invalid date_start format, expected YYYY-MM-DD")
	}
	dateEnd, err := timezone.ParseDate(endStr)
	if err != nil {
		return timezone.Date{}, timezone.Date{}, fmt.Errorf("invalid date_end format, expected YYYY-MM-DD")
	}
	return dateStart, dateEnd, nil
}

// mergeOverlappingAbsences handles overlapping absences: rejects if different type, merges if same type.
func (s *staffAbsenceService) mergeOverlappingAbsences(
	ctx context.Context,
	existing []*activeModels.StaffAbsence,
	dateStart, dateEnd timezone.Date,
	req CreateAbsenceRequest,
) (*StaffAbsenceResponse, error) {
	// Check if all overlapping absences have the same type
	if err := validateSameAbsenceType(existing, req.AbsenceType); err != nil {
		return nil, err
	}

	// Calculate merged date range
	mergedStart, mergedEnd := calculateMergedDateRange(existing, dateStart, dateEnd)

	// Update the primary absence with merged range
	primary := existing[0]
	primary.DateStart = mergedStart
	primary.DateEnd = mergedEnd
	if req.Note != "" && primary.Note == "" {
		primary.Note = req.Note
	}
	primary.UpdatedAt = time.Now()

	if err := s.absenceRepo.Update(ctx, primary); err != nil {
		return nil, fmt.Errorf("failed to merge absence: %w", err)
	}

	// Delete remaining overlapping absences
	s.deleteRemainingAbsences(ctx, existing[1:])

	return toAbsenceResponse(primary), nil
}

// validateSameAbsenceType checks that all existing absences match the requested type.
func validateSameAbsenceType(existing []*activeModels.StaffAbsence, absenceType string) error {
	for _, e := range existing {
		if e.AbsenceType != absenceType {
			return fmt.Errorf("absence overlaps with existing %s absence from %s to %s",
				e.AbsenceType,
				e.DateStart,
				e.DateEnd)
		}
	}
	return nil
}

// calculateMergedDateRange expands the date range to cover all overlapping absences.
func calculateMergedDateRange(existing []*activeModels.StaffAbsence, dateStart, dateEnd timezone.Date) (timezone.Date, timezone.Date) {
	mergedStart := dateStart
	mergedEnd := dateEnd
	for _, e := range existing {
		if e.DateStart.Before(mergedStart) {
			mergedStart = e.DateStart
		}
		if e.DateEnd.After(mergedEnd) {
			mergedEnd = e.DateEnd
		}
	}
	return mergedStart, mergedEnd
}

// deleteRemainingAbsences deletes absences that were merged into the primary.
func (s *staffAbsenceService) deleteRemainingAbsences(ctx context.Context, absences []*activeModels.StaffAbsence) {
	for _, e := range absences {
		if err := s.absenceRepo.Delete(ctx, e.ID); err != nil {
			slog.Default().WarnContext(ctx, "failed to delete merged absence",
				slog.Int64("absence_id", e.ID),
				slog.String("error", err.Error()))
		}
	}
}

// warnIfWorkSessionsExist logs a warning if work sessions exist in the date range.
func (s *staffAbsenceService) warnIfWorkSessionsExist(ctx context.Context, staffID int64, dateStart, dateEnd timezone.Date) {
	sessions, err := s.workSessionRepo.GetHistoryByStaffID(ctx, staffID, dateStart, dateEnd)
	if err == nil && len(sessions) > 0 {
		slog.Default().WarnContext(ctx, "work sessions exist in absence range",
			slog.Int("session_count", len(sessions)),
			slog.Int64("staff_id", staffID),
			slog.String("date_start", dateStart.String()),
			slog.String("date_end", dateEnd.String()))
	}
}

// createNewAbsence creates a new absence record in the database.
func (s *staffAbsenceService) createNewAbsence(
	ctx context.Context,
	staffID int64,
	dateStart, dateEnd timezone.Date,
	req CreateAbsenceRequest,
) (*StaffAbsenceResponse, error) {
	now := time.Now()
	absence := &activeModels.StaffAbsence{
		StaffID:     staffID,
		AbsenceType: req.AbsenceType,
		DateStart:   dateStart,
		DateEnd:     dateEnd,
		HalfDay:     req.HalfDay,
		Note:        req.Note,
		Status:      activeModels.AbsenceStatusReported,
		CreatedBy:   staffID,
	}
	absence.CreatedAt = now
	absence.UpdatedAt = now

	absence.SetTenantID(tenant.FromContext(ctx))
	if err := s.absenceRepo.Create(ctx, absence); err != nil {
		return nil, fmt.Errorf("failed to create absence: %w", err)
	}

	return toAbsenceResponse(absence), nil
}

// UpdateAbsence updates an existing absence record
func (s *staffAbsenceService) UpdateAbsence(ctx context.Context, staffID int64, absenceID int64, req UpdateAbsenceRequest) (*StaffAbsenceResponse, error) {
	absence, err := s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return nil, fmt.Errorf("absence not found")
	}

	// Verify ownership
	if absence.StaffID != staffID {
		return nil, fmt.Errorf("can only update own absences")
	}
	if isVacationWorkflowAbsence(absence) {
		return nil, fmt.Errorf("vacation workflow absences must be changed through the vacation flow")
	}
	if req.AbsenceType != nil && *req.AbsenceType == activeModels.AbsenceTypeVacation {
		return nil, fmt.Errorf("vacation absences must be requested through the vacation flow")
	}

	// Apply updates from request
	if err := applyAbsenceUpdates(absence, req); err != nil {
		return nil, err
	}

	// Check for overlapping absences (excluding self)
	if err := s.checkOverlapExcludingSelf(ctx, staffID, absenceID, absence.DateStart, absence.DateEnd); err != nil {
		return nil, err
	}

	absence.UpdatedAt = time.Now()

	if err := absence.Validate(); err != nil {
		return nil, fmt.Errorf("invalid absence data: %w", err)
	}

	if err := s.absenceRepo.Update(ctx, absence); err != nil {
		return nil, fmt.Errorf("failed to update absence: %w", err)
	}

	return toAbsenceResponse(absence), nil
}

// applyAbsenceUpdates applies partial updates from the request to the absence.
func applyAbsenceUpdates(absence *activeModels.StaffAbsence, req UpdateAbsenceRequest) error {
	if req.AbsenceType != nil {
		absence.AbsenceType = *req.AbsenceType
	}
	if req.DateStart != nil {
		dateStart, err := timezone.ParseDate(*req.DateStart)
		if err != nil {
			return fmt.Errorf("invalid date_start format, expected YYYY-MM-DD")
		}
		absence.DateStart = dateStart
	}
	if req.DateEnd != nil {
		dateEnd, err := timezone.ParseDate(*req.DateEnd)
		if err != nil {
			return fmt.Errorf("invalid date_end format, expected YYYY-MM-DD")
		}
		absence.DateEnd = dateEnd
	}
	if req.HalfDay != nil {
		absence.HalfDay = *req.HalfDay
	}
	if req.Note != nil {
		absence.Note = *req.Note
	}
	return nil
}

// checkOverlapExcludingSelf checks for overlapping absences, excluding the given absence ID.
func (s *staffAbsenceService) checkOverlapExcludingSelf(ctx context.Context, staffID, absenceID int64, dateStart, dateEnd timezone.Date) error {
	existing, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, dateStart, dateEnd)
	if err != nil {
		return fmt.Errorf("failed to check existing absences: %w", err)
	}
	for _, e := range existing {
		if e.ID != absenceID && blocksAbsenceRange(e.Status) {
			return fmt.Errorf("updated dates overlap with existing absence from %s to %s",
				e.DateStart,
				e.DateEnd)
		}
	}
	return nil
}

// DeleteAbsence deletes an absence record
func (s *staffAbsenceService) DeleteAbsence(ctx context.Context, staffID int64, absenceID int64) error {
	absence, err := s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return fmt.Errorf("absence not found")
	}

	// Verify ownership
	if absence.StaffID != staffID {
		return fmt.Errorf("can only delete own absences")
	}
	if isVacationWorkflowAbsence(absence) {
		return fmt.Errorf("vacation workflow absences must be canceled through the vacation flow")
	}

	if err := s.absenceRepo.Delete(ctx, absenceID); err != nil {
		return fmt.Errorf("failed to delete absence: %w", err)
	}

	return nil
}

// GetAbsencesForRange returns absences for a staff member in a date range
func (s *staffAbsenceService) GetAbsencesForRange(ctx context.Context, staffID int64, from, to timezone.Date) ([]*StaffAbsenceResponse, error) {
	absences, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get absences: %w", err)
	}

	responses := make([]*StaffAbsenceResponse, len(absences))
	for i, a := range absences {
		responses[i] = toAbsenceResponse(a)
	}

	return responses, nil
}

// HasAbsenceOnDate checks if a staff member has an absence on a specific date
func (s *staffAbsenceService) HasAbsenceOnDate(ctx context.Context, staffID int64, date timezone.Date) (bool, *activeModels.StaffAbsence, error) {
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

func toAbsenceResponse(a *activeModels.StaffAbsence) *StaffAbsenceResponse {
	return &StaffAbsenceResponse{
		StaffAbsence: a,
		DurationDays: a.DurationDays(),
	}
}

func isEffectiveAbsenceStatus(status string) bool {
	return status == activeModels.AbsenceStatusReported ||
		status == activeModels.AbsenceStatusApproved
}

func blocksAbsenceRange(status string) bool {
	return status == activeModels.AbsenceStatusReported ||
		status == activeModels.AbsenceStatusRequested ||
		status == activeModels.AbsenceStatusApproved
}

func isVacationWorkflowAbsence(absence *activeModels.StaffAbsence) bool {
	if absence.AbsenceType != activeModels.AbsenceTypeVacation {
		return false
	}
	return absence.Status == activeModels.AbsenceStatusRequested ||
		absence.Status == activeModels.AbsenceStatusApproved ||
		absence.Status == activeModels.AbsenceStatusDeclined ||
		absence.Status == activeModels.AbsenceStatusCanceled
}

func filterBlockingAbsences(rows []*activeModels.StaffAbsence) []*activeModels.StaffAbsence {
	filtered := make([]*activeModels.StaffAbsence, 0, len(rows))
	for _, row := range rows {
		if blocksAbsenceRange(row.Status) {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func effectiveBoundaryHalfDays(a *activeModels.StaffAbsence) (bool, bool) {
	if a.HalfDay && !a.StartHalfDay && !a.EndHalfDay {
		return true, true
	}
	return a.StartHalfDay, a.EndHalfDay
}

func isWorkingDay(d timezone.Date) bool {
	w := d.Weekday()
	return w != time.Saturday && w != time.Sunday
}

// Vacation workflow (Tranche 4)

// countWorkingDays returns Mon-Fri count between two dates inclusive,
// minus 0.5 for each boundary half-day. Personio-style: first day half
// and last day half are independent flags. Feiertage are NOT excluded
// yet. That lives in Tranche 3 (Issue #1231).
func countWorkingDays(from, to timezone.Date, startHalf, endHalf bool) float64 {
	if to.Before(from) {
		return 0
	}
	count := 0
	for d := from; !d.After(to); d = d.AddDays(1) {
		if isWorkingDay(d) {
			count++
		}
	}
	result := float64(count)
	// Single-day range: only one boundary exists, so a single half flag halves it.
	if from == to {
		if isWorkingDay(from) && (startHalf || endHalf) {
			result -= 0.5
		}
		return result
	}
	if startHalf && isWorkingDay(from) {
		result -= 0.5
	}
	if endHalf && isWorkingDay(to) {
		result -= 0.5
	}
	return result
}

func (s *staffAbsenceService) RequestVacation(ctx context.Context, staffID int64, req RequestVacationRequest) (*StaffAbsenceResponse, error) {
	dateStart, dateEnd, err := parseDateRange(req.DateStart, req.DateEnd)
	if err != nil {
		return nil, err
	}
	if isBeforeLocalToday(dateStart, time.Now()) {
		return nil, fmt.Errorf("vacation request must start today or in the future")
	}

	existing, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, dateStart, dateEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing absences: %w", err)
	}
	// Declined and canceled rows do not block the time slot, the staff
	// member is free to request the same dates again. Only requested,
	// approved and reported absences hold the range.
	for _, e := range existing {
		if blocksAbsenceRange(e.Status) {
			return nil, fmt.Errorf("dates overlap with an existing absence")
		}
	}

	workingDays := countWorkingDays(dateStart, dateEnd, req.StartHalfDay, req.EndHalfDay)
	if workingDays <= 0 {
		return nil, fmt.Errorf("vacation range contains no working days")
	}

	now := time.Now()
	// HalfDay legacy flag stays in sync: true iff either boundary is half.
	halfDay := req.StartHalfDay || req.EndHalfDay
	absence := &activeModels.StaffAbsence{
		StaffID:           staffID,
		AbsenceType:       activeModels.AbsenceTypeVacation,
		DateStart:         dateStart,
		DateEnd:           dateEnd,
		HalfDay:           halfDay,
		StartHalfDay:      req.StartHalfDay,
		EndHalfDay:        req.EndHalfDay,
		Note:              req.Note,
		Status:            activeModels.AbsenceStatusRequested,
		CreatedBy:         staffID,
		WorkingDays:       &workingDays,
		RequestedAt:       now,
		SubstituteStaffID: req.SubstituteStaffID,
	}
	absence.CreatedAt = now
	absence.UpdatedAt = now
	absence.SetTenantID(tenant.FromContext(ctx))

	if err := s.absenceRepo.Create(ctx, absence); err != nil {
		return nil, fmt.Errorf("failed to create vacation request: %w", err)
	}
	return toAbsenceResponse(absence), nil
}

func (s *staffAbsenceService) ApproveAbsence(ctx context.Context, absenceID int64, actorAccountID int64, decidedByStaffID int64, note string) (*StaffAbsenceResponse, error) {
	absence, err := s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return nil, fmt.Errorf("absence not found")
	}
	if absence.Status != activeModels.AbsenceStatusRequested {
		return nil, fmt.Errorf("only requested absences can be approved")
	}
	fromStatus := absence.Status
	now := time.Now()
	absence.Status = activeModels.AbsenceStatusApproved
	absence.ApprovedBy = &decidedByStaffID
	absence.ApprovedAt = &now
	absence.DecisionNote = note
	absence.UpdatedAt = now

	if err := s.absenceRepo.Update(ctx, absence); err != nil {
		return nil, fmt.Errorf("failed to approve absence: %w", err)
	}
	if err := s.createAudit(ctx, absence.ID, actorAccountID, fromStatus, absence.Status, note); err != nil {
		return nil, fmt.Errorf("failed to audit absence approval: %w", err)
	}
	return toAbsenceResponse(absence), nil
}

func (s *staffAbsenceService) DenyAbsence(ctx context.Context, absenceID int64, actorAccountID int64, decidedByStaffID int64, reason string) (*StaffAbsenceResponse, error) {
	if reason == "" {
		return nil, fmt.Errorf("decline reason is required")
	}
	absence, err := s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return nil, fmt.Errorf("absence not found")
	}
	if absence.Status != activeModels.AbsenceStatusRequested {
		return nil, fmt.Errorf("only requested absences can be declined")
	}
	fromStatus := absence.Status
	now := time.Now()
	absence.Status = activeModels.AbsenceStatusDeclined
	absence.ApprovedBy = &decidedByStaffID
	absence.ApprovedAt = &now
	absence.DecisionNote = reason
	absence.UpdatedAt = now

	if err := s.absenceRepo.Update(ctx, absence); err != nil {
		return nil, fmt.Errorf("failed to decline absence: %w", err)
	}
	if err := s.createAudit(ctx, absence.ID, actorAccountID, fromStatus, absence.Status, reason); err != nil {
		return nil, fmt.Errorf("failed to audit absence decline: %w", err)
	}
	return toAbsenceResponse(absence), nil
}

func (s *staffAbsenceService) CancelAbsence(ctx context.Context, staffID int64, actorAccountID int64, absenceID int64) error {
	absence, err := s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return fmt.Errorf("absence not found")
	}
	if absence.StaffID != staffID {
		return fmt.Errorf("can only cancel own absences")
	}
	// MA can cancel requested or approved future vacation. Past approved
	// absences become historical record and are not cancelable from the UI.
	if absence.Status != activeModels.AbsenceStatusRequested &&
		absence.Status != activeModels.AbsenceStatusApproved {
		return fmt.Errorf("only pending or approved absences can be canceled")
	}
	if absence.Status == activeModels.AbsenceStatusApproved &&
		isBeforeLocalToday(absence.DateStart, time.Now()) {
		return fmt.Errorf("past absences cannot be canceled")
	}
	fromStatus := absence.Status
	absence.Status = activeModels.AbsenceStatusCanceled
	absence.UpdatedAt = time.Now()
	if err := s.absenceRepo.Update(ctx, absence); err != nil {
		return fmt.Errorf("failed to cancel absence: %w", err)
	}
	if err := s.createAudit(ctx, absence.ID, actorAccountID, fromStatus, absence.Status, ""); err != nil {
		return fmt.Errorf("failed to audit absence cancellation: %w", err)
	}
	return nil
}

func (s *staffAbsenceService) createAudit(ctx context.Context, absenceID int64, actorAccountID int64, fromStatus string, toStatus string, note string) error {
	if s.auditRepo == nil {
		return fmt.Errorf("staff absence audit repository is not configured")
	}
	audit := &activeModels.StaffAbsenceAudit{
		AbsenceID:  absenceID,
		FromStatus: &fromStatus,
		ToStatus:   toStatus,
		ActorID:    actorAccountID,
		Note:       note,
	}
	return s.auditRepo.Create(ctx, audit)
}

func isBeforeLocalToday(date timezone.Date, now time.Time) bool {
	return date.Before(timezone.DateFromTime(now))
}

func (s *staffAbsenceService) GetVacationQuotaSummary(ctx context.Context, staffID int64, year int) (*VacationQuotaSummary, error) {
	quota, err := s.quotaRepo.GetByStaffAndYear(ctx, staffID, year)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch quota: %w", err)
	}
	entitled := defaultEntitledDays
	carryover := 0.0
	if quota != nil {
		entitled = quota.EntitledDays
		carryover = quota.CarryoverDays
	}

	yearStart := timezone.NewDate(year, time.January, 1)
	yearEnd := timezone.NewDate(year, time.December, 31)
	absences, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, yearStart, yearEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch year absences: %w", err)
	}

	taken, reserved := 0.0, 0.0
	for _, a := range absences {
		if a.AbsenceType != activeModels.AbsenceTypeVacation {
			continue
		}
		days := vacationDaysInYear(a, yearStart, yearEnd)
		if a.WorkingDays != nil && dateWithinRange(a.DateStart, yearStart, yearEnd) && dateWithinRange(a.DateEnd, yearStart, yearEnd) {
			days = *a.WorkingDays
		}
		switch a.Status {
		case activeModels.AbsenceStatusApproved, activeModels.AbsenceStatusReported:
			taken += days
		case activeModels.AbsenceStatusRequested:
			reserved += days
		}
	}

	return &VacationQuotaSummary{
		StaffID:       staffID,
		Year:          year,
		EntitledDays:  entitled,
		CarryoverDays: carryover,
		TakenDays:     taken,
		ReservedDays:  reserved,
		RemainingDays: entitled + carryover - taken - reserved,
	}, nil
}

func dateWithinRange(d, from, to timezone.Date) bool {
	return !d.Before(from) && !d.After(to)
}

func vacationDaysInYear(a *activeModels.StaffAbsence, yearStart, yearEnd timezone.Date) float64 {
	from := a.DateStart
	if from.Before(yearStart) {
		from = yearStart
	}
	to := a.DateEnd
	if to.After(yearEnd) {
		to = yearEnd
	}
	if to.Before(from) {
		return 0
	}

	startHalf, endHalf := effectiveBoundaryHalfDays(a)
	if from != a.DateStart {
		startHalf = false
	}
	if to != a.DateEnd {
		endHalf = false
	}
	return countWorkingDays(from, to, startHalf, endHalf)
}

func (s *staffAbsenceService) UpsertVacationQuota(ctx context.Context, staffID int64, year int, entitled, carryover float64) error {
	now := time.Now()
	quota := &activeModels.StaffVacationQuota{
		StaffID:       staffID,
		Year:          year,
		EntitledDays:  entitled,
		CarryoverDays: carryover,
	}
	quota.CreatedAt = now
	quota.UpdatedAt = now
	quota.SetTenantID(tenant.FromContext(ctx))
	return s.quotaRepo.Upsert(ctx, quota)
}

func (s *staffAbsenceService) ListPendingRequests(ctx context.Context) ([]*StaffAbsenceResponse, error) {
	rows, err := s.absenceRepo.ListByStatus(ctx, activeModels.AbsenceStatusRequested)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending requests: %w", err)
	}
	responses := make([]*StaffAbsenceResponse, len(rows))
	for i, r := range rows {
		responses[i] = toAbsenceResponse(r)
	}
	return responses, nil
}
