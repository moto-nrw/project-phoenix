package active

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// dateFormatISO is the standard date format for parsing and formatting
const dateFormatISO = "2006-01-02"

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
	GetAbsencesForRange(ctx context.Context, staffID int64, from, to time.Time) ([]*StaffAbsenceResponse, error)
	HasAbsenceOnDate(ctx context.Context, staffID int64, date time.Time) (bool, *activeModels.StaffAbsence, error)

	// Vacation workflow (Tranche 4)
	RequestVacation(ctx context.Context, staffID int64, req RequestVacationRequest) (*StaffAbsenceResponse, error)
	ApproveAbsence(ctx context.Context, absenceID int64, decidedBy int64, note string) (*StaffAbsenceResponse, error)
	DenyAbsence(ctx context.Context, absenceID int64, decidedBy int64, reason string) (*StaffAbsenceResponse, error)
	CancelAbsence(ctx context.Context, staffID int64, absenceID int64) error
	GetVacationQuotaSummary(ctx context.Context, staffID int64, year int) (*VacationQuotaSummary, error)
	UpsertVacationQuota(ctx context.Context, staffID int64, year int, entitled, carryover float64) error
	ListPendingRequests(ctx context.Context) ([]*StaffAbsenceResponse, error)
}

// staffAbsenceService implements StaffAbsenceService
type staffAbsenceService struct {
	absenceRepo     activeModels.StaffAbsenceRepository
	workSessionRepo activeModels.WorkSessionRepository
	quotaRepo       activeModels.StaffVacationQuotaRepository
}

// NewStaffAbsenceService creates a new staff absence service
func NewStaffAbsenceService(
	absenceRepo activeModels.StaffAbsenceRepository,
	workSessionRepo activeModels.WorkSessionRepository,
	quotaRepo activeModels.StaffVacationQuotaRepository,
) StaffAbsenceService {
	return &staffAbsenceService{
		absenceRepo:     absenceRepo,
		workSessionRepo: workSessionRepo,
		quotaRepo:       quotaRepo,
	}
}

// defaultEntitledDays is the fallback when no per-staff quota row exists.
// 30 working days = TVöD-Allgemein standard. Pro-Tenant override via settings
// will come in a follow-up, see Issue #1375 Tranche 4a.
const defaultEntitledDays = 30.0

// CreateAbsence creates a new absence record
func (s *staffAbsenceService) CreateAbsence(ctx context.Context, staffID int64, req CreateAbsenceRequest) (*StaffAbsenceResponse, error) {
	dateStart, dateEnd, err := parseDateRange(req.DateStart, req.DateEnd)
	if err != nil {
		return nil, err
	}

	// Check for overlapping absences — merge if same type, reject if different type
	existing, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, dateStart, dateEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing absences: %w", err)
	}

	if len(existing) > 0 {
		return s.mergeOverlappingAbsences(ctx, existing, dateStart, dateEnd, req)
	}

	s.warnIfWorkSessionsExist(ctx, staffID, dateStart, dateEnd)

	return s.createNewAbsence(ctx, staffID, dateStart, dateEnd, req)
}

// parseDateRange parses start and end date strings in ISO format.
func parseDateRange(startStr, endStr string) (time.Time, time.Time, error) {
	dateStart, err := time.Parse(dateFormatISO, startStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid date_start format, expected YYYY-MM-DD")
	}
	dateEnd, err := time.Parse(dateFormatISO, endStr)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid date_end format, expected YYYY-MM-DD")
	}
	return dateStart, dateEnd, nil
}

// mergeOverlappingAbsences handles overlapping absences: rejects if different type, merges if same type.
func (s *staffAbsenceService) mergeOverlappingAbsences(
	ctx context.Context,
	existing []*activeModels.StaffAbsence,
	dateStart, dateEnd time.Time,
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
				e.DateStart.Format(dateFormatISO),
				e.DateEnd.Format(dateFormatISO))
		}
	}
	return nil
}

// calculateMergedDateRange expands the date range to cover all overlapping absences.
func calculateMergedDateRange(existing []*activeModels.StaffAbsence, dateStart, dateEnd time.Time) (time.Time, time.Time) {
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
func (s *staffAbsenceService) warnIfWorkSessionsExist(ctx context.Context, staffID int64, dateStart, dateEnd time.Time) {
	sessions, err := s.workSessionRepo.GetHistoryByStaffID(ctx, staffID, dateStart, dateEnd)
	if err == nil && len(sessions) > 0 {
		slog.Default().WarnContext(ctx, "work sessions exist in absence range",
			slog.Int("session_count", len(sessions)),
			slog.Int64("staff_id", staffID),
			slog.String("date_start", dateStart.Format(dateFormatISO)),
			slog.String("date_end", dateEnd.Format(dateFormatISO)))
	}
}

// createNewAbsence creates a new absence record in the database.
func (s *staffAbsenceService) createNewAbsence(
	ctx context.Context,
	staffID int64,
	dateStart, dateEnd time.Time,
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
		dateStart, err := time.Parse(dateFormatISO, *req.DateStart)
		if err != nil {
			return fmt.Errorf("invalid date_start format, expected YYYY-MM-DD")
		}
		absence.DateStart = dateStart
	}
	if req.DateEnd != nil {
		dateEnd, err := time.Parse(dateFormatISO, *req.DateEnd)
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
func (s *staffAbsenceService) checkOverlapExcludingSelf(ctx context.Context, staffID, absenceID int64, dateStart, dateEnd time.Time) error {
	existing, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, dateStart, dateEnd)
	if err != nil {
		return fmt.Errorf("failed to check existing absences: %w", err)
	}
	for _, e := range existing {
		if e.ID != absenceID {
			return fmt.Errorf("updated dates overlap with existing absence from %s to %s",
				e.DateStart.Format(dateFormatISO),
				e.DateEnd.Format(dateFormatISO))
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

	if err := s.absenceRepo.Delete(ctx, absenceID); err != nil {
		return fmt.Errorf("failed to delete absence: %w", err)
	}

	return nil
}

// GetAbsencesForRange returns absences for a staff member in a date range
func (s *staffAbsenceService) GetAbsencesForRange(ctx context.Context, staffID int64, from, to time.Time) ([]*StaffAbsenceResponse, error) {
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
func (s *staffAbsenceService) HasAbsenceOnDate(ctx context.Context, staffID int64, date time.Time) (bool, *activeModels.StaffAbsence, error) {
	absence, err := s.absenceRepo.GetByStaffAndDate(ctx, staffID, date)
	if err != nil {
		return false, nil, fmt.Errorf("failed to check absence: %w", err)
	}
	if absence == nil {
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

// ─── Vacation workflow (Tranche 4) ───────────────────────────────────────────

// countWorkingDays returns Mon-Fri count between two dates inclusive,
// minus 0.5 for each boundary half-day. Personio-style: first day half
// and last day half are independent flags. Feiertage are NOT excluded
// yet — that lives in Tranche 3 (Issue #1231).
func countWorkingDays(from, to time.Time, startHalf, endHalf bool) float64 {
	if to.Before(from) {
		return 0
	}
	count := 0
	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		w := d.Weekday()
		if w != time.Saturday && w != time.Sunday {
			count++
		}
	}
	result := float64(count)
	// Single-day range: only one boundary exists, so a single half flag
	// halves it. Treat startHalf and endHalf identically in that case.
	if from.Equal(to) {
		if startHalf || endHalf {
			result -= 0.5
		}
		return result
	}
	if startHalf {
		result -= 0.5
	}
	if endHalf {
		result -= 0.5
	}
	return result
}

func (s *staffAbsenceService) RequestVacation(ctx context.Context, staffID int64, req RequestVacationRequest) (*StaffAbsenceResponse, error) {
	dateStart, dateEnd, err := parseDateRange(req.DateStart, req.DateEnd)
	if err != nil {
		return nil, err
	}
	if dateStart.Before(time.Now().Truncate(24 * time.Hour)) {
		return nil, fmt.Errorf("vacation request must start today or in the future")
	}

	existing, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, dateStart, dateEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing absences: %w", err)
	}
	if len(existing) > 0 {
		return nil, fmt.Errorf("dates overlap with an existing absence")
	}

	workingDays := countWorkingDays(dateStart, dateEnd, req.StartHalfDay, req.EndHalfDay)
	if workingDays == 0 {
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

func (s *staffAbsenceService) ApproveAbsence(ctx context.Context, absenceID int64, decidedBy int64, note string) (*StaffAbsenceResponse, error) {
	absence, err := s.absenceRepo.FindByID(ctx, absenceID)
	if err != nil {
		return nil, fmt.Errorf("absence not found")
	}
	if absence.Status != activeModels.AbsenceStatusRequested {
		return nil, fmt.Errorf("only requested absences can be approved")
	}
	now := time.Now()
	absence.Status = activeModels.AbsenceStatusApproved
	absence.ApprovedBy = &decidedBy
	absence.ApprovedAt = &now
	absence.DecisionNote = note
	absence.UpdatedAt = now

	if err := s.absenceRepo.Update(ctx, absence); err != nil {
		return nil, fmt.Errorf("failed to approve absence: %w", err)
	}
	return toAbsenceResponse(absence), nil
}

func (s *staffAbsenceService) DenyAbsence(ctx context.Context, absenceID int64, decidedBy int64, reason string) (*StaffAbsenceResponse, error) {
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
	now := time.Now()
	absence.Status = activeModels.AbsenceStatusDeclined
	absence.ApprovedBy = &decidedBy
	absence.ApprovedAt = &now
	absence.DecisionNote = reason
	absence.UpdatedAt = now

	if err := s.absenceRepo.Update(ctx, absence); err != nil {
		return nil, fmt.Errorf("failed to decline absence: %w", err)
	}
	return toAbsenceResponse(absence), nil
}

func (s *staffAbsenceService) CancelAbsence(ctx context.Context, staffID int64, absenceID int64) error {
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
		absence.DateStart.Before(time.Now().Truncate(24*time.Hour)) {
		return fmt.Errorf("past absences cannot be canceled")
	}
	absence.Status = activeModels.AbsenceStatusCanceled
	absence.UpdatedAt = time.Now()
	if err := s.absenceRepo.Update(ctx, absence); err != nil {
		return fmt.Errorf("failed to cancel absence: %w", err)
	}
	return nil
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

	yearStart := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	yearEnd := time.Date(year, 12, 31, 0, 0, 0, 0, time.UTC)
	absences, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, yearStart, yearEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch year absences: %w", err)
	}

	taken, reserved := 0.0, 0.0
	for _, a := range absences {
		if a.AbsenceType != activeModels.AbsenceTypeVacation {
			continue
		}
		days := 0.0
		if a.WorkingDays != nil {
			days = *a.WorkingDays
		} else {
			// Legacy rows pre-1.15.59 only had HalfDay; treat that flag as
			// "both ends halfed" to mirror the migration backfill.
			days = countWorkingDays(a.DateStart, a.DateEnd, a.HalfDay || a.StartHalfDay, a.HalfDay || a.EndHalfDay)
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
