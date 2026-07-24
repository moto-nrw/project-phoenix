package active

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
)

// Sentinel errors for handler mapping (#1420).
var (
	// ErrAdjustmentInvalid marks a caller mistake in an adjustment request
	// (bad type, zero delta, missing note, future reset date) — HTTP 400.
	ErrAdjustmentInvalid = errors.New("invalid balance adjustment")
	// ErrAdjustmentNotFound marks a delete of an adjustment that does not
	// exist for this staff member — HTTP 404.
	ErrAdjustmentNotFound = errors.New("balance adjustment not found")
	// ErrBalanceAlreadyReset marks a second reset for the same staff member
	// and effective date (double-click, concurrent admins) — HTTP 409.
	ErrBalanceAlreadyReset = errors.New("balance already reset for this date")
	// ErrAdjustmentHasDependentReset prevents a write from changing history
	// that an existing reset used to calculate its persisted delta — HTTP 409.
	ErrAdjustmentHasDependentReset = errors.New("later balance reset depends on this adjustment")
)

const (
	// resetUniqueConstraintName is the partial unique index guarding one reset
	// per staff and effective date (migration 1.15.218).
	resetUniqueConstraintName = "uq_sba_reset_per_day"

	// These bounds mirror the privileged admin UI. They are business limits,
	// not presentation hints: direct API callers must not bypass them.
	maxBalanceAdjustmentMinutes = 1_000 * 60
	maxBalanceCarryoverMinutes  = 10_000 * 60

	minPostgresInteger = -1 << 31
	maxPostgresInteger = 1<<31 - 1
)

// CreateBalanceAdjustmentRequest carries a payout or lump-sum comp-time
// grant (#1420 5a). MinutesDelta is signed and must be negative — both types
// only ever reduce the Stundenkonto.
type CreateBalanceAdjustmentRequest struct {
	Type          string
	MinutesDelta  int
	EffectiveDate timezone.Date
	Note          string
}

// StaffBalanceAdjustmentService owns the Stundenkonto lifecycle transactions
// (#1420): payout, lump-sum Freizeitausgleich grants, and the school-year
// reset. All writes require time_tracking:manage at the route level.
type StaffBalanceAdjustmentService interface {
	ListAdjustments(ctx context.Context, staffID int64, from, to timezone.Date) ([]*activeModels.StaffBalanceAdjustment, error)
	CreateAdjustment(ctx context.Context, staffID, decidedBy int64, req CreateBalanceAdjustmentRequest) (*activeModels.StaffBalanceAdjustment, error)
	DeleteAdjustment(ctx context.Context, staffID, adjustmentID int64) error
	// ResetBalance writes a 'reset' transaction whose delta turns the closing
	// balance as of effectiveDate into carryoverMinutes (#1420 5c). It must
	// run inside the ambient tenant transaction (advisory lock + unique
	// index serialize concurrent resets).
	ResetBalance(ctx context.Context, staffID, decidedBy int64, effectiveDate timezone.Date, carryoverMinutes int, note string) (*activeModels.StaffBalanceAdjustment, error)
}

type staffBalanceAdjustmentService struct {
	adjustmentRepo activeModels.StaffBalanceAdjustmentRepository
	monthService   WorkTimeMonthService
	settings       monthSettingsResolver
	logger         *slog.Logger
}

func NewStaffBalanceAdjustmentService(
	adjustmentRepo activeModels.StaffBalanceAdjustmentRepository,
	monthService WorkTimeMonthService,
	settings monthSettingsResolver,
	logger *slog.Logger,
) StaffBalanceAdjustmentService {
	return &staffBalanceAdjustmentService{
		adjustmentRepo: adjustmentRepo,
		monthService:   monthService,
		settings:       settings,
		logger:         logger,
	}
}

// rejectPreAccountDate fails a booking dated before the account start: such
// a row would sit outside every carry chain and silently never move the
// Stundenkonto, while still showing up in the history list. The anchor is
// resolved against TODAY's chain (the account the KPI shows), mirroring
// chainAnchor's empty-setting fallback (January 1st of the current year).
func (s *staffBalanceAdjustmentService) rejectPreAccountDate(ctx context.Context, effectiveDate timezone.Date) error {
	anchor, err := resolveAccountAnchor(ctx, s.settings, s.getLogger(), monthOf(timezone.TodayDate()))
	if err != nil {
		return fmt.Errorf("failed to resolve account start for adjustment: %w", err)
	}
	if effectiveDate.Before(anchor) {
		return fmt.Errorf("%w: effective_date %s lies before the account start %s", ErrAdjustmentInvalid, effectiveDate.String(), anchor.String())
	}
	return nil
}

func (s *staffBalanceAdjustmentService) ListAdjustments(ctx context.Context, staffID int64, from, to timezone.Date) ([]*activeModels.StaffBalanceAdjustment, error) {
	if staffID <= 0 {
		return nil, fmt.Errorf("%w: staff id is required", ErrAdjustmentInvalid)
	}
	if from.IsZero() || to.IsZero() || to.Before(from) {
		return nil, fmt.Errorf("%w: from and to must form a valid range", ErrAdjustmentInvalid)
	}
	return s.adjustmentRepo.GetByStaffAndDateRange(ctx, staffID, from, to)
}

func (s *staffBalanceAdjustmentService) CreateAdjustment(ctx context.Context, staffID, decidedBy int64, req CreateBalanceAdjustmentRequest) (*activeModels.StaffBalanceAdjustment, error) {
	if req.Type != activeModels.BalanceAdjustmentTypePayout && req.Type != activeModels.BalanceAdjustmentTypeCompTime {
		return nil, fmt.Errorf("%w: type must be payout or comp_time", ErrAdjustmentInvalid)
	}
	if req.MinutesDelta >= 0 {
		return nil, fmt.Errorf("%w: minutes_delta must be negative", ErrAdjustmentInvalid)
	}
	if req.MinutesDelta < -maxBalanceAdjustmentMinutes {
		return nil, fmt.Errorf("%w: minutes_delta must not exceed %d minutes", ErrAdjustmentInvalid, maxBalanceAdjustmentMinutes)
	}
	if err := validateAdjustmentCommon(staffID, decidedBy, req.EffectiveDate, req.Note); err != nil {
		return nil, err
	}
	if err := s.rejectPreAccountDate(ctx, req.EffectiveDate); err != nil {
		return nil, err
	}
	if err := s.adjustmentRepo.LockStaffBalanceWrites(ctx, staffID); err != nil {
		return nil, fmt.Errorf("failed to lock staff balance writes: %w", err)
	}
	resets, err := s.listResetsOnOrAfter(ctx, staffID, req.EffectiveDate)
	if err != nil {
		return nil, err
	}
	if len(resets) > 0 {
		return nil, fmt.Errorf(
			"%w: effective_date %s is not after reset %s",
			ErrAdjustmentHasDependentReset,
			req.EffectiveDate.String(),
			resets[0].EffectiveDate.String(),
		)
	}

	adjustment := &activeModels.StaffBalanceAdjustment{
		StaffID:       staffID,
		Type:          req.Type,
		MinutesDelta:  req.MinutesDelta,
		EffectiveDate: req.EffectiveDate,
		Note:          req.Note,
		DecidedBy:     decidedBy,
		DecidedAt:     time.Now(),
	}
	if err := s.adjustmentRepo.Create(ctx, adjustment); err != nil {
		return nil, fmt.Errorf("failed to create balance adjustment: %w", err)
	}
	s.getLogger().Info("balance adjustment created",
		"staff_id", staffID,
		"type", req.Type,
		"minutes_delta", req.MinutesDelta,
		"effective_date", req.EffectiveDate.String(),
		"decided_by", decidedBy,
	)
	return adjustment, nil
}

func (s *staffBalanceAdjustmentService) DeleteAdjustment(ctx context.Context, staffID, adjustmentID int64) error {
	if staffID <= 0 || adjustmentID <= 0 {
		return fmt.Errorf("%w: staff id and adjustment id are required", ErrAdjustmentInvalid)
	}
	if err := s.adjustmentRepo.LockStaffBalanceWrites(ctx, staffID); err != nil {
		return fmt.Errorf("failed to lock staff balance writes: %w", err)
	}
	adjustment, err := s.adjustmentRepo.FindByID(ctx, adjustmentID)
	if err != nil {
		return fmt.Errorf("%w: id %d", ErrAdjustmentNotFound, adjustmentID)
	}
	if adjustment == nil || adjustment.StaffID != staffID {
		return fmt.Errorf("%w: id %d", ErrAdjustmentNotFound, adjustmentID)
	}
	dependent, err := s.hasDependentReset(ctx, adjustment)
	if err != nil {
		return err
	}
	if dependent {
		return fmt.Errorf("%w: adjustment id %d", ErrAdjustmentHasDependentReset, adjustmentID)
	}
	if err := s.adjustmentRepo.Delete(ctx, adjustmentID); err != nil {
		return fmt.Errorf("failed to delete balance adjustment: %w", err)
	}
	s.getLogger().Info("balance adjustment deleted",
		"staff_id", staffID,
		"adjustment_id", adjustmentID,
		"type", adjustment.Type,
		"minutes_delta", adjustment.MinutesDelta,
	)
	return nil
}

func (s *staffBalanceAdjustmentService) ResetBalance(ctx context.Context, staffID, decidedBy int64, effectiveDate timezone.Date, carryoverMinutes int, note string) (*activeModels.StaffBalanceAdjustment, error) {
	if err := validateAdjustmentCommon(staffID, decidedBy, effectiveDate, note); err != nil {
		return nil, err
	}
	if carryoverMinutes < 0 || carryoverMinutes > maxBalanceCarryoverMinutes {
		return nil, fmt.Errorf(
			"%w: carryover_minutes must be between 0 and %d",
			ErrAdjustmentInvalid,
			maxBalanceCarryoverMinutes,
		)
	}
	// A reset needs a closed cutoff. Today's sessions, absences, and targets
	// can still change after this request, which would make the stored delta
	// stale before the day ends.
	if !effectiveDate.Before(timezone.TodayDate()) {
		return nil, fmt.Errorf("%w: effective_date must be before today", ErrAdjustmentInvalid)
	}
	if err := s.rejectPreAccountDate(ctx, effectiveDate); err != nil {
		return nil, err
	}

	// Serialize concurrent resets: the advisory lock covers the
	// read-compute-insert sequence, the partial unique index is the backstop.
	if err := s.adjustmentRepo.LockStaffBalanceWrites(ctx, staffID); err != nil {
		return nil, fmt.Errorf("failed to lock staff balance writes: %w", err)
	}
	resets, err := s.listResetsOnOrAfter(ctx, staffID, effectiveDate)
	if err != nil {
		return nil, err
	}
	if len(resets) > 0 {
		if resets[0].EffectiveDate == effectiveDate {
			return nil, fmt.Errorf("%w: %s", ErrBalanceAlreadyReset, effectiveDate.String())
		}
		return nil, fmt.Errorf(
			"%w: reset date %s is before existing reset %s",
			ErrAdjustmentHasDependentReset,
			effectiveDate.String(),
			resets[0].EffectiveDate.String(),
		)
	}

	previousBalance, err := s.monthService.GetClosingBalanceAsOf(ctx, staffID, effectiveDate)
	if err != nil {
		return nil, fmt.Errorf("failed to compute balance for reset: %w", err)
	}
	delta := int64(carryoverMinutes) - int64(previousBalance)
	if delta < minPostgresInteger || delta > maxPostgresInteger {
		return nil, fmt.Errorf("%w: calculated reset delta is outside the supported range", ErrAdjustmentInvalid)
	}

	adjustment := &activeModels.StaffBalanceAdjustment{
		StaffID:       staffID,
		Type:          activeModels.BalanceAdjustmentTypeReset,
		MinutesDelta:  int(delta),
		EffectiveDate: effectiveDate,
		Note:          note,
		DecidedBy:     decidedBy,
		DecidedAt:     time.Now(),
	}
	if err := s.adjustmentRepo.Create(ctx, adjustment); err != nil {
		if modelBase.IsUniqueViolationOn(err, resetUniqueConstraintName) {
			return nil, fmt.Errorf("%w: %s", ErrBalanceAlreadyReset, effectiveDate.String())
		}
		return nil, fmt.Errorf("failed to create balance reset: %w", err)
	}
	s.getLogger().Info("balance reset",
		"staff_id", staffID,
		"effective_date", effectiveDate.String(),
		"previous_balance_minutes", previousBalance,
		"carryover_minutes", carryoverMinutes,
		"minutes_delta", adjustment.MinutesDelta,
		"decided_by", decidedBy,
	)
	return adjustment, nil
}

func (s *staffBalanceAdjustmentService) hasDependentReset(ctx context.Context, adjustment *activeModels.StaffBalanceAdjustment) (bool, error) {
	resets, err := s.listResetsOnOrAfter(ctx, adjustment.StaffID, adjustment.EffectiveDate)
	if err != nil {
		return false, err
	}
	for _, reset := range resets {
		if reset.EffectiveDate.After(adjustment.EffectiveDate) ||
			(reset.EffectiveDate == adjustment.EffectiveDate && reset.ID > adjustment.ID) {
			return true, nil
		}
	}
	return false, nil
}

func (s *staffBalanceAdjustmentService) listResetsOnOrAfter(ctx context.Context, staffID int64, effectiveDate timezone.Date) ([]*activeModels.StaffBalanceAdjustment, error) {
	options := modelBase.NewQueryOptions()
	options.Filter.
		Equal("staff_id", staffID).
		Equal("type", activeModels.BalanceAdjustmentTypeReset).
		GreaterThanOrEqual("effective_date", effectiveDate)
	sorting := &modelBase.Sorting{}
	sorting.AddField("effective_date", modelBase.SortAsc)
	sorting.AddField("id", modelBase.SortAsc)
	options.Sorting = sorting

	resets, err := s.adjustmentRepo.List(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to check dependent balance resets: %w", err)
	}
	return resets, nil
}

func validateAdjustmentCommon(staffID, decidedBy int64, effectiveDate timezone.Date, note string) error {
	if staffID <= 0 {
		return fmt.Errorf("%w: staff id is required", ErrAdjustmentInvalid)
	}
	if decidedBy <= 0 {
		return fmt.Errorf("%w: decided_by is required", ErrAdjustmentInvalid)
	}
	if effectiveDate.IsZero() {
		return fmt.Errorf("%w: effective_date is required", ErrAdjustmentInvalid)
	}
	if strings.TrimSpace(note) == "" {
		return fmt.Errorf("%w: note is required", ErrAdjustmentInvalid)
	}
	return nil
}

func (s *staffBalanceAdjustmentService) getLogger() *slog.Logger {
	if s.logger == nil {
		return slog.Default()
	}
	return s.logger
}
