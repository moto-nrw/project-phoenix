package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// ErrCareOfferingNotFound is the sentinel returned by GetByID when the
// row doesn't exist (or the tenant can't see it via RLS).
var ErrCareOfferingNotFound = errors.New("care offering not found")

// CareOfferingService manages the per-tenant care-offering catalog.
// Admin endpoints (PR 6) read + write all offerings; the public endpoint
// (PR 7 consumer) calls ListPublicOpenWindow.
type CareOfferingService interface {
	List(ctx context.Context) ([]*enrollmentModels.CareOffering, error)
	ListByCalendarPeriod(ctx context.Context, calendarPeriodID int64) ([]*enrollmentModels.CareOffering, error)
	ListPublic(ctx context.Context, now time.Time) ([]*enrollmentModels.CareOffering, error)
	GetByID(ctx context.Context, id int64) (*enrollmentModels.CareOffering, error)
	Create(ctx context.Context, offering *enrollmentModels.CareOffering) (*enrollmentModels.CareOffering, error)
	Update(ctx context.Context, offering *enrollmentModels.CareOffering) error
	Delete(ctx context.Context, id int64) error

	// Clone copies an existing offering into a new row scoped to the
	// target calendar period. All fields except id / tenant_id are
	// preserved; date fields (application window, service dates) are
	// shifted by daysOffset (use 0 for an exact copy). The new
	// offering's calendar_period_id is set to the target.
	//
	// Use case: end of school year, admin clones the prior year's
	// catalog with daysOffset=365 to spin up next year's offerings
	// quickly. Admin then trims / edits the result.
	Clone(ctx context.Context, sourceID int64, targetCalendarPeriodID int64, daysOffset int) (*enrollmentModels.CareOffering, error)
}

// CareOfferingServiceConfig is the dep-injection bundle.
type CareOfferingServiceConfig struct {
	Repo   enrollmentModels.CareOfferingRepository
	Logger *slog.Logger
}

type careOfferingService struct {
	repo   enrollmentModels.CareOfferingRepository
	logger *slog.Logger
}

// NewCareOfferingService builds the service.
func NewCareOfferingService(cfg CareOfferingServiceConfig) CareOfferingService {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &careOfferingService{
		repo:   cfg.Repo,
		logger: logger,
	}
}

func (s *careOfferingService) List(ctx context.Context) ([]*enrollmentModels.CareOffering, error) {
	return s.repo.ListByTenant(ctx)
}

func (s *careOfferingService) ListByCalendarPeriod(ctx context.Context, calendarPeriodID int64) ([]*enrollmentModels.CareOffering, error) {
	if calendarPeriodID <= 0 {
		return nil, fmt.Errorf("calendar_period_id must be positive")
	}
	return s.repo.ListByCalendarPeriod(ctx, calendarPeriodID)
}

func (s *careOfferingService) ListPublic(ctx context.Context, now time.Time) ([]*enrollmentModels.CareOffering, error) {
	if now.IsZero() {
		now = time.Now()
	}
	return s.repo.ListPublicOpenWindow(ctx, now)
}

func (s *careOfferingService) GetByID(ctx context.Context, id int64) (*enrollmentModels.CareOffering, error) {
	if id <= 0 {
		return nil, ErrCareOfferingNotFound
	}
	offering, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, ErrCareOfferingNotFound
	}
	return offering, nil
}

func (s *careOfferingService) Create(ctx context.Context, offering *enrollmentModels.CareOffering) (*enrollmentModels.CareOffering, error) {
	if offering == nil {
		return nil, fmt.Errorf("offering is required")
	}
	if err := s.repo.Create(ctx, offering); err != nil {
		return nil, err
	}
	s.logger.Info("care offering created",
		slog.Int64("offering_id", offering.ID),
		slog.String("name", offering.Name))
	return offering, nil
}

func (s *careOfferingService) Update(ctx context.Context, offering *enrollmentModels.CareOffering) error {
	if offering == nil || offering.ID <= 0 {
		return fmt.Errorf("offering with valid id is required")
	}
	if err := s.repo.Update(ctx, offering); err != nil {
		return err
	}
	s.logger.Info("care offering updated", slog.Int64("offering_id", offering.ID))
	return nil
}

func (s *careOfferingService) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("id must be positive")
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.logger.Info("care offering deleted", slog.Int64("offering_id", id))
	return nil
}

// Clone implements the next-year duplication helper. Date shifting uses
// AddDate(0, 0, daysOffset) so DST boundaries are handled consistently.
// daysOffset == 0 produces an exact copy (still useful when admin wants
// to pivot an offering across periods without shifting dates).
func (s *careOfferingService) Clone(ctx context.Context, sourceID int64, targetCalendarPeriodID int64, daysOffset int) (*enrollmentModels.CareOffering, error) {
	if sourceID <= 0 {
		return nil, fmt.Errorf("source id must be positive")
	}
	if targetCalendarPeriodID <= 0 {
		return nil, fmt.Errorf("target calendar period id must be positive")
	}

	source, err := s.repo.FindByID(ctx, sourceID)
	if err != nil {
		return nil, fmt.Errorf("clone: source lookup: %w", err)
	}

	clone := *source
	clone.ID = 0 // BIGSERIAL — let the DB assign
	clone.SortOrder = source.SortOrder
	clone.CalendarPeriodID = &targetCalendarPeriodID

	if daysOffset != 0 {
		shiftDay := func(t *time.Time) *time.Time {
			if t == nil {
				return nil
			}
			out := t.AddDate(0, 0, daysOffset)
			return &out
		}
		clone.ApplicationWindowStart = shiftDay(clone.ApplicationWindowStart)
		clone.ApplicationWindowEnd = shiftDay(clone.ApplicationWindowEnd)
		clone.ServiceStartDate = shiftDay(clone.ServiceStartDate)
		clone.ServiceEndDate = shiftDay(clone.ServiceEndDate)
	}

	if err := s.repo.Create(ctx, &clone); err != nil {
		return nil, fmt.Errorf("clone: create: %w", err)
	}
	s.logger.Info("care offering cloned",
		slog.Int64("source_id", sourceID),
		slog.Int64("clone_id", clone.ID),
		slog.Int64("target_period_id", targetCalendarPeriodID),
		slog.Int("days_offset", daysOffset))
	return &clone, nil
}
