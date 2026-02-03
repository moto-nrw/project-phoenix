package active

import (
	"context"
	"fmt"
	"log"
	"time"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
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

// StaffAbsenceService defines operations for staff absence management
type StaffAbsenceService interface {
	CreateAbsence(ctx context.Context, staffID int64, req CreateAbsenceRequest) (*StaffAbsenceResponse, error)
	UpdateAbsence(ctx context.Context, staffID int64, absenceID int64, req UpdateAbsenceRequest) (*StaffAbsenceResponse, error)
	DeleteAbsence(ctx context.Context, staffID int64, absenceID int64) error
	GetAbsencesForRange(ctx context.Context, staffID int64, from, to time.Time) ([]*StaffAbsenceResponse, error)
	HasAbsenceOnDate(ctx context.Context, staffID int64, date time.Time) (bool, *activeModels.StaffAbsence, error)
}

// staffAbsenceService implements StaffAbsenceService
type staffAbsenceService struct {
	absenceRepo     activeModels.StaffAbsenceRepository
	workSessionRepo activeModels.WorkSessionRepository
}

// NewStaffAbsenceService creates a new staff absence service
func NewStaffAbsenceService(absenceRepo activeModels.StaffAbsenceRepository, workSessionRepo activeModels.WorkSessionRepository) StaffAbsenceService {
	return &staffAbsenceService{
		absenceRepo:     absenceRepo,
		workSessionRepo: workSessionRepo,
	}
}

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
			log.Printf("Warning: failed to delete merged absence %d: %v", e.ID, err)
		}
	}
}

// warnIfWorkSessionsExist logs a warning if work sessions exist in the date range.
func (s *staffAbsenceService) warnIfWorkSessionsExist(ctx context.Context, staffID int64, dateStart, dateEnd time.Time) {
	sessions, err := s.workSessionRepo.GetHistoryByStaffID(ctx, staffID, dateStart, dateEnd)
	if err == nil && len(sessions) > 0 {
		log.Printf("Warning: %d work session(s) exist for staff %d in absence range %s to %s",
			len(sessions), staffID, dateStart.Format(dateFormatISO), dateEnd.Format(dateFormatISO))
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
