package active

import (
	"context"
	"fmt"
	"log"
	"time"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
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
	// Parse dates
	dateStart, err := time.Parse("2006-01-02", req.DateStart)
	if err != nil {
		return nil, fmt.Errorf("invalid date_start format, expected YYYY-MM-DD")
	}
	dateEnd, err := time.Parse("2006-01-02", req.DateEnd)
	if err != nil {
		return nil, fmt.Errorf("invalid date_end format, expected YYYY-MM-DD")
	}

	// Check for overlapping absences — merge if same type, reject if different type
	existing, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, dateStart, dateEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing absences: %w", err)
	}
	if len(existing) > 0 {
		// Check if all overlapping absences have the same type
		for _, e := range existing {
			if e.AbsenceType != req.AbsenceType {
				return nil, fmt.Errorf("absence overlaps with existing %s absence from %s to %s",
					e.AbsenceType,
					e.DateStart.Format("2006-01-02"),
					e.DateEnd.Format("2006-01-02"))
			}
		}

		// Same type: merge by expanding date range to cover all overlapping absences
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

		// Update the first existing absence with merged range, delete the rest
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
		for _, e := range existing[1:] {
			if err := s.absenceRepo.Delete(ctx, e.ID); err != nil {
				log.Printf("Warning: failed to delete merged absence %d: %v", e.ID, err)
			}
		}

		return toAbsenceResponse(primary), nil
	}

	// Log warning if work sessions exist in the date range (non-blocking)
	sessions, err := s.workSessionRepo.GetHistoryByStaffID(ctx, staffID, dateStart, dateEnd)
	if err == nil && len(sessions) > 0 {
		log.Printf("Warning: %d work session(s) exist for staff %d in absence range %s to %s",
			len(sessions), staffID, req.DateStart, req.DateEnd)
	}

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

	if req.AbsenceType != nil {
		absence.AbsenceType = *req.AbsenceType
	}
	if req.DateStart != nil {
		dateStart, err := time.Parse("2006-01-02", *req.DateStart)
		if err != nil {
			return nil, fmt.Errorf("invalid date_start format, expected YYYY-MM-DD")
		}
		absence.DateStart = dateStart
	}
	if req.DateEnd != nil {
		dateEnd, err := time.Parse("2006-01-02", *req.DateEnd)
		if err != nil {
			return nil, fmt.Errorf("invalid date_end format, expected YYYY-MM-DD")
		}
		absence.DateEnd = dateEnd
	}
	if req.HalfDay != nil {
		absence.HalfDay = *req.HalfDay
	}
	if req.Note != nil {
		absence.Note = *req.Note
	}

	// Check for overlapping absences (excluding self)
	existing, err := s.absenceRepo.GetByStaffAndDateRange(ctx, staffID, absence.DateStart, absence.DateEnd)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing absences: %w", err)
	}
	for _, e := range existing {
		if e.ID != absenceID {
			return nil, fmt.Errorf("updated dates overlap with existing absence from %s to %s",
				e.DateStart.Format("2006-01-02"),
				e.DateEnd.Format("2006-01-02"))
		}
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
