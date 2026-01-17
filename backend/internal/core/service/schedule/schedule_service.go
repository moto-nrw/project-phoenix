package schedule

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/schedule"
	schedulePort "github.com/moto-nrw/project-phoenix/internal/core/port/schedule"
	"github.com/uptrace/bun"
)

// service implements the schedule.Service interface
// It composes handlers for different aspects of schedule management
type service struct {
	dateframeRepo      schedulePort.DateframeRepository
	timeframeRepo      schedulePort.TimeframeRepository
	recurrenceRuleRepo schedulePort.RecurrenceRuleRepository
	db                 *bun.DB
	txHandler          *base.TxHandler

	// Composed handlers
	timeframeHandler   *timeframeHandler
	recurrenceHandler  *recurrenceHandler
	analysisHandler    *analysisHandler
}

// NewService creates a new schedule service with composed handlers
func NewService(
	dateframeRepo schedulePort.DateframeRepository,
	timeframeRepo schedulePort.TimeframeRepository,
	recurrenceRuleRepo schedulePort.RecurrenceRuleRepository,
	db *bun.DB,
) Service {
	return &service{
		dateframeRepo:      dateframeRepo,
		timeframeRepo:      timeframeRepo,
		recurrenceRuleRepo: recurrenceRuleRepo,
		db:                 db,
		txHandler:          base.NewTxHandler(db),
		// Initialize composed handlers
		timeframeHandler:  newTimeframeHandler(timeframeRepo),
		recurrenceHandler: newRecurrenceHandler(recurrenceRuleRepo),
		analysisHandler:   newAnalysisHandler(timeframeRepo, recurrenceRuleRepo),
	}
}

// WithTx returns a new service that uses the provided transaction
func (s *service) WithTx(tx bun.Tx) any {
	// Get repositories with transaction if they implement the TransactionalRepository interface
	var dateframeRepo = s.dateframeRepo
	var timeframeRepo = s.timeframeRepo
	var recurrenceRuleRepo = s.recurrenceRuleRepo

	// Try to cast repositories to TransactionalRepository and apply the transaction
	if txRepo, ok := s.dateframeRepo.(base.TransactionalRepository); ok {
		dateframeRepo = txRepo.WithTx(tx).(schedulePort.DateframeRepository)
	}
	if txRepo, ok := s.timeframeRepo.(base.TransactionalRepository); ok {
		timeframeRepo = txRepo.WithTx(tx).(schedulePort.TimeframeRepository)
	}
	if txRepo, ok := s.recurrenceRuleRepo.(base.TransactionalRepository); ok {
		recurrenceRuleRepo = txRepo.WithTx(tx).(schedulePort.RecurrenceRuleRepository)
	}

	// Return a new service with the transaction
	return &service{
		dateframeRepo:      dateframeRepo,
		timeframeRepo:      timeframeRepo,
		recurrenceRuleRepo: recurrenceRuleRepo,
		db:                 s.db,
		txHandler:          s.txHandler.WithTx(tx),
		// Re-initialize composed handlers with transactional repositories
		timeframeHandler:  newTimeframeHandler(timeframeRepo),
		recurrenceHandler: newRecurrenceHandler(recurrenceRuleRepo),
		analysisHandler:   newAnalysisHandler(timeframeRepo, recurrenceRuleRepo),
	}
}

// ========== Timeframe Operations (delegated to timeframeHandler) ==========

// GetTimeframe retrieves a timeframe by its ID
func (s *service) GetTimeframe(ctx context.Context, id int64) (*schedule.Timeframe, error) {
	return s.timeframeHandler.GetTimeframe(ctx, id)
}

// CreateTimeframe creates a new timeframe
func (s *service) CreateTimeframe(ctx context.Context, timeframe *schedule.Timeframe) error {
	return s.timeframeHandler.CreateTimeframe(ctx, timeframe)
}

// UpdateTimeframe updates an existing timeframe
func (s *service) UpdateTimeframe(ctx context.Context, timeframe *schedule.Timeframe) error {
	return s.timeframeHandler.UpdateTimeframe(ctx, timeframe)
}

// DeleteTimeframe deletes a timeframe by its ID
func (s *service) DeleteTimeframe(ctx context.Context, id int64) error {
	return s.timeframeHandler.DeleteTimeframe(ctx, id)
}

// ListTimeframes retrieves all timeframes matching the provided filters
func (s *service) ListTimeframes(ctx context.Context, options *base.QueryOptions) ([]*schedule.Timeframe, error) {
	return s.timeframeHandler.ListTimeframes(ctx, options)
}

// FindActiveTimeframes finds all active timeframes
func (s *service) FindActiveTimeframes(ctx context.Context) ([]*schedule.Timeframe, error) {
	return s.timeframeHandler.FindActiveTimeframes(ctx)
}

// FindTimeframesByTimeRange finds all timeframes that overlap with the given time range
func (s *service) FindTimeframesByTimeRange(ctx context.Context, startTime, endTime time.Time) ([]*schedule.Timeframe, error) {
	return s.timeframeHandler.FindTimeframesByTimeRange(ctx, startTime, endTime)
}

// ========== RecurrenceRule Operations (delegated to recurrenceHandler) ==========

// GetRecurrenceRule retrieves a recurrence rule by its ID
func (s *service) GetRecurrenceRule(ctx context.Context, id int64) (*schedule.RecurrenceRule, error) {
	return s.recurrenceHandler.GetRecurrenceRule(ctx, id)
}

// CreateRecurrenceRule creates a new recurrence rule
func (s *service) CreateRecurrenceRule(ctx context.Context, rule *schedule.RecurrenceRule) error {
	return s.recurrenceHandler.CreateRecurrenceRule(ctx, rule)
}

// UpdateRecurrenceRule updates an existing recurrence rule
func (s *service) UpdateRecurrenceRule(ctx context.Context, rule *schedule.RecurrenceRule) error {
	return s.recurrenceHandler.UpdateRecurrenceRule(ctx, rule)
}

// DeleteRecurrenceRule deletes a recurrence rule by its ID
func (s *service) DeleteRecurrenceRule(ctx context.Context, id int64) error {
	return s.recurrenceHandler.DeleteRecurrenceRule(ctx, id)
}

// ListRecurrenceRules retrieves all recurrence rules matching the provided filters
func (s *service) ListRecurrenceRules(ctx context.Context, options *base.QueryOptions) ([]*schedule.RecurrenceRule, error) {
	return s.recurrenceHandler.ListRecurrenceRules(ctx, options)
}

// FindRecurrenceRulesByFrequency finds all recurrence rules with the specified frequency
func (s *service) FindRecurrenceRulesByFrequency(ctx context.Context, frequency string) ([]*schedule.RecurrenceRule, error) {
	return s.recurrenceHandler.FindRecurrenceRulesByFrequency(ctx, frequency)
}

// FindRecurrenceRulesByWeekday finds all recurrence rules that include the specified weekday
func (s *service) FindRecurrenceRulesByWeekday(ctx context.Context, weekday string) ([]*schedule.RecurrenceRule, error) {
	return s.recurrenceHandler.FindRecurrenceRulesByWeekday(ctx, weekday)
}

// ========== Schedule Analysis (delegated to analysisHandler) ==========

// GenerateEvents generates events based on a recurrence rule within a date range
func (s *service) GenerateEvents(ctx context.Context, ruleID int64, startDate, endDate time.Time) ([]time.Time, error) {
	return s.analysisHandler.GenerateEvents(ctx, ruleID, startDate, endDate)
}

// CheckConflict checks if there are any conflicts for the given time range
func (s *service) CheckConflict(ctx context.Context, startTime, endTime time.Time) (bool, []*schedule.Timeframe, error) {
	return s.analysisHandler.CheckConflict(ctx, startTime, endTime)
}

// FindAvailableSlots finds available time slots within a date range
func (s *service) FindAvailableSlots(ctx context.Context, startDate, endDate time.Time, duration time.Duration) ([]*schedule.Timeframe, error) {
	return s.analysisHandler.FindAvailableSlots(ctx, startDate, endDate, duration)
}
