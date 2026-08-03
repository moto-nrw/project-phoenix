package active

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/realtime"
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
	// Deliberately partial: only ADJUSTMENT writes are blocked. Work sessions
	// and comp_time absences dated before a reset stay allowed — late
	// corrections remain visible in the ledger and shift the post-reset
	// balance while the stored reset delta stays fixed (see
	// TestWTMAdjustments_LateHistoricalCorrectionRemainsVisibleAfterReset).
	ErrAdjustmentHasDependentReset = errors.New("later balance reset depends on this adjustment")
	// ErrAdjustmentExceedsBalance prevents payouts and lump-sum comp-time
	// grants from reducing the account below zero — HTTP 409.
	ErrAdjustmentExceedsBalance = errors.New("balance adjustment exceeds accrued balance")
	// ErrAdjustmentInClosedMonth marks a booking whose effective month is
	// frozen by a Monatsabschluss (#1417) — HTTP 400 with a stable code so
	// the frontend can point at the month close instead of a generic
	// validation message. Wraps ErrAdjustmentInvalid so existing errors.Is
	// call sites keep matching.
	ErrAdjustmentInClosedMonth = fmt.Errorf("%w: effective month is closed", ErrAdjustmentInvalid)
	// ErrOpeningAlreadyExists marks a second opening balance for the same
	// staff member (#2132) — HTTP 409. A takeover is a one-time event;
	// corrections go through delete + re-create so the deletion tombstone
	// keeps the history.
	ErrOpeningAlreadyExists = errors.New("opening balance already exists for this staff member")
)

const (
	// resetUniqueConstraintName is the partial unique index guarding one reset
	// per staff and effective date (migration 1.15.219).
	resetUniqueConstraintName = "uq_sba_reset_per_day"

	// openingUniqueConstraintName is the partial unique index guarding one
	// opening balance per staff member (migration 1.15.261).
	openingUniqueConstraintName = "uq_sba_opening_per_staff"

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
	// DeleteAdjustment removes a booking and writes an append-only tombstone
	// (audit.time_tracking_deletions, #1417) in the same tenant transaction —
	// a deleted ledger row must stay visible in the audit log. deletedBy is
	// the acting staff member.
	DeleteAdjustment(ctx context.Context, staffID, adjustmentID, deletedBy int64) error
	// ResetBalance writes a 'reset' transaction whose delta turns the closing
	// balance as of effectiveDate into carryoverMinutes (#1420 5c). It must
	// run inside the ambient tenant transaction (advisory lock + unique
	// index serialize concurrent resets).
	ResetBalance(ctx context.Context, staffID, decidedBy int64, effectiveDate timezone.Date, carryoverMinutes int, note string) (*activeModels.StaffBalanceAdjustment, error)
	// CreateOpeningBalance writes an 'opening' transaction whose delta turns
	// the closing balance as of effectiveDate into balanceMinutes (#2132).
	// balanceMinutes is SIGNED — a migrated account may start negative. One
	// opening per staff member, ever; corrections are delete + re-create.
	CreateOpeningBalance(ctx context.Context, staffID, decidedBy int64, effectiveDate timezone.Date, balanceMinutes int, note string) (*activeModels.StaffBalanceAdjustment, error)
	// SetSnapshotReader injects the frozen-month reader (#1417) after
	// construction; nil means no month counts as frozen, so existing unit
	// fixtures stay valid.
	SetSnapshotReader(reader adjustmentFreezeReader)
}

// adjustmentFreezeReader is implemented by
// active.StaffMonthBalanceSnapshotRepository (#1417).
type adjustmentFreezeReader interface {
	GetLatestClosedThrough(ctx context.Context, staffID int64, year, month int) (*activeModels.StaffMonthBalanceSnapshot, error)
}

type staffBalanceAdjustmentService struct {
	adjustmentRepo activeModels.StaffBalanceAdjustmentRepository
	monthService   WorkTimeMonthService
	settings       monthSettingsResolver
	snapshotRepo   adjustmentFreezeReader
	deletionRepo   auditModels.TimeTrackingDeletionRepository
	broadcaster    realtime.Broadcaster
	logger         *slog.Logger
}

// SetSnapshotReader wires the frozen-month reader (#1417). Setter injection
// like the month service's: nil means no month counts as frozen, so existing
// unit fixtures stay valid.
func (s *staffBalanceAdjustmentService) SetSnapshotReader(reader adjustmentFreezeReader) {
	s.snapshotRepo = reader
}

// SetDeletionAudit wires the deletion tombstone writer (#1417). Setter
// injection like SetBroadcaster so existing API-layer mocks stay unchanged;
// a nil repository makes DeleteAdjustment fail — deletes without a trace are
// exactly the hole this closes.
func (s *staffBalanceAdjustmentService) SetDeletionAudit(repo auditModels.TimeTrackingDeletionRepository) {
	s.deletionRepo = repo
}

// SetBroadcaster injects the tenant-wide SSE broadcaster. It stays outside
// StaffBalanceAdjustmentService so existing API-layer mocks stay unchanged.
func (s *staffBalanceAdjustmentService) SetBroadcaster(broadcaster realtime.Broadcaster) {
	s.broadcaster = broadcaster
}

func (s *staffBalanceAdjustmentService) broadcastTimeTrackingChanged(ctx context.Context) {
	queueStaffTimeTrackingChanged(ctx, s.broadcaster, s.getLogger())
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

// rejectFrozenMonth fails a booking whose effective month is closed (#1417).
// A frozen month's closing balance no longer comes from its own aggregation,
// so a booking inside it would appear in the history while moving nothing.
//
// Deliberately partial, exactly like ErrAdjustmentHasDependentReset: only
// ADJUSTMENT writes are blocked. Work sessions and comp_time absences inside a
// frozen month stay editable — the divergence they cause is reported as the
// month's drift instead of being suppressed. Reopening the month lifts this.
func (s *staffBalanceAdjustmentService) rejectFrozenMonth(ctx context.Context, staffID int64, effectiveDate timezone.Date) error {
	if s.snapshotRepo == nil {
		return nil
	}
	year, month := effectiveDate.Year, int(effectiveDate.Month)
	snapshot, err := s.snapshotRepo.GetLatestClosedThrough(ctx, staffID, year, month)
	if err != nil {
		return fmt.Errorf("failed to check month close state for adjustment: %w", err)
	}
	if snapshot == nil || snapshot.Year != year || snapshot.Month != month {
		return nil
	}
	return fmt.Errorf(
		"%w: effective_date %s lies in the closed month %04d-%02d",
		ErrAdjustmentInClosedMonth, effectiveDate.String(), year, month,
	)
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
	// Mirror the Monatskarte's future bound: beyond it the closing-balance
	// read has no defined result (and far enough out it fails), so an
	// unbounded date must be a 400, not a 500 (#1420 review).
	if horizon := monthOf(timezone.TodayDate()).addMonths(maxFutureMonths); horizon.before(monthOf(req.EffectiveDate)) {
		return nil, fmt.Errorf(
			"%w: effective_date %s is more than %d months ahead",
			ErrAdjustmentInvalid, req.EffectiveDate.String(), maxFutureMonths,
		)
	}
	if err := s.rejectPreAccountDate(ctx, req.EffectiveDate); err != nil {
		return nil, err
	}
	if err := s.adjustmentRepo.LockStaffBalanceWrites(ctx, staffID); err != nil {
		return nil, fmt.Errorf("failed to lock staff balance writes: %w", err)
	}
	if err := s.rejectFrozenMonth(ctx, staffID, req.EffectiveDate); err != nil {
		return nil, err
	}
	resets, err := s.listRebaselinesOnOrAfter(ctx, staffID, req.EffectiveDate)
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

	reductionCapacity, err := s.monthService.GetBalanceReductionCapacity(ctx, staffID, req.EffectiveDate)
	if err != nil {
		return nil, fmt.Errorf("failed to compute reduction capacity for adjustment: %w", err)
	}
	requestedMinutes := -req.MinutesDelta
	if requestedMinutes > reductionCapacity {
		return nil, fmt.Errorf(
			"%w: requested reduction of %d minutes exceeds available capacity of %d minutes from %s onward",
			ErrAdjustmentExceedsBalance,
			requestedMinutes,
			reductionCapacity,
			req.EffectiveDate.String(),
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
	s.broadcastTimeTrackingChanged(ctx)
	return adjustment, nil
}

func (s *staffBalanceAdjustmentService) DeleteAdjustment(ctx context.Context, staffID, adjustmentID, deletedBy int64) error {
	if staffID <= 0 || adjustmentID <= 0 {
		return fmt.Errorf("%w: staff id and adjustment id are required", ErrAdjustmentInvalid)
	}
	if deletedBy <= 0 {
		return fmt.Errorf("%w: deleting staff id is required", ErrAdjustmentInvalid)
	}
	if err := s.adjustmentRepo.LockStaffBalanceWrites(ctx, staffID); err != nil {
		return fmt.Errorf("failed to lock staff balance writes: %w", err)
	}
	adjustment, err := s.adjustmentRepo.FindByID(ctx, adjustmentID)
	if err != nil {
		// Only a genuine missing row is a 404; a database or transaction
		// failure must stay a 500 (#1420 review).
		if modelBase.IsNoRows(err) {
			return fmt.Errorf("%w: id %d", ErrAdjustmentNotFound, adjustmentID)
		}
		return fmt.Errorf("failed to load balance adjustment: %w", err)
	}
	if adjustment == nil || adjustment.StaffID != staffID {
		return fmt.Errorf("%w: id %d", ErrAdjustmentNotFound, adjustmentID)
	}
	// Checked here rather than up front: the effective date only exists once
	// the row is loaded. Deleting out of a frozen month is the same violation
	// as inserting into one.
	if err := s.rejectFrozenMonth(ctx, staffID, adjustment.EffectiveDate); err != nil {
		return err
	}
	dependent, err := s.hasDependentReset(ctx, adjustment)
	if err != nil {
		return err
	}
	if dependent {
		return fmt.Errorf("%w: adjustment id %d", ErrAdjustmentHasDependentReset, adjustmentID)
	}
	// An opening balance is a migration rebaseline and may legitimately be
	// replaced by a negative opening. Its documented correction workflow is
	// delete + re-create, so do not reject the transient deletion solely
	// because later consumption would make the account negative.
	if adjustment.Type != activeModels.BalanceAdjustmentTypeOpening {
		if err := s.validatePositiveAdjustmentDeletion(ctx, adjustment); err != nil {
			return err
		}
	}
	// Tombstone first, delete second, same transaction: a deleted booking
	// must stay visible in the audit log (#1417).
	if s.deletionRepo == nil {
		return fmt.Errorf("time tracking deletion audit repository is not configured")
	}
	payload, err := json.Marshal(adjustment)
	if err != nil {
		return fmt.Errorf("failed to snapshot adjustment for deletion audit: %w", err)
	}
	if err := s.deletionRepo.Create(ctx, &auditModels.TimeTrackingDeletion{
		StaffID:   staffID,
		Source:    auditModels.TimeTrackingDeletionSourceBalanceAdjustment,
		SourceID:  adjustment.ID,
		DeletedBy: deletedBy,
		Payload:   payload,
		Note:      adjustment.Note,
	}); err != nil {
		return fmt.Errorf("failed to write deletion audit: %w", err)
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
	s.broadcastTimeTrackingChanged(ctx)
	return nil
}

func (s *staffBalanceAdjustmentService) validatePositiveAdjustmentDeletion(ctx context.Context, adjustment *activeModels.StaffBalanceAdjustment) error {
	if adjustment.MinutesDelta <= 0 {
		return nil
	}
	reductionCapacity, err := s.monthService.GetBalanceReductionCapacity(ctx, adjustment.StaffID, adjustment.EffectiveDate)
	if err != nil {
		return fmt.Errorf("failed to compute reduction capacity for adjustment deletion: %w", err)
	}
	// The target adjustment is still present in every capacity checkpoint.
	// hasDependentReset above guarantees that its delta carries through the
	// whole checked timeline, so subtracting it yields the exact capacity
	// after deletion.
	capacityAfterDeletion := reductionCapacity - adjustment.MinutesDelta
	if capacityAfterDeletion >= 0 {
		return nil
	}
	return fmt.Errorf(
		"%w: deleting adjustment %d removes %d minutes and would leave capacity of %d minutes from %s onward",
		ErrAdjustmentExceedsBalance,
		adjustment.ID,
		adjustment.MinutesDelta,
		capacityAfterDeletion,
		adjustment.EffectiveDate.String(),
	)
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
	if err := s.rejectFrozenMonth(ctx, staffID, effectiveDate); err != nil {
		return nil, err
	}
	resets, err := s.listRebaselinesOnOrAfter(ctx, staffID, effectiveDate)
	if err != nil {
		return nil, err
	}
	if len(resets) > 0 {
		if resets[0].EffectiveDate == effectiveDate && resets[0].Type == activeModels.BalanceAdjustmentTypeReset {
			return nil, fmt.Errorf("%w: %s", ErrBalanceAlreadyReset, effectiveDate.String())
		}
		return nil, fmt.Errorf(
			"%w: reset date %s is not after existing %s booking %s",
			ErrAdjustmentHasDependentReset,
			effectiveDate.String(),
			resets[0].Type,
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
	if delta < 0 {
		reductionCapacity, err := s.monthService.GetBalanceReductionCapacity(ctx, staffID, effectiveDate)
		if err != nil {
			return nil, fmt.Errorf("failed to compute reduction capacity for balance reset: %w", err)
		}
		requestedMinutes := int(-delta)
		if requestedMinutes > reductionCapacity {
			return nil, fmt.Errorf(
				"%w: reset reduction of %d minutes exceeds available capacity of %d minutes from %s onward",
				ErrAdjustmentExceedsBalance,
				requestedMinutes,
				reductionCapacity,
				effectiveDate.String(),
			)
		}
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
	s.broadcastTimeTrackingChanged(ctx)
	return adjustment, nil
}

// CreateOpeningBalance writes the go-live opening booking (#2132): a signed
// rebaseline that turns the closing balance as of effectiveDate into
// balanceMinutes. Unlike ResetBalance there is no lower bound and no
// reduction-capacity check — a migrated account legitimately starts negative
// when the staff member owed hours in the previous system.
func (s *staffBalanceAdjustmentService) CreateOpeningBalance(ctx context.Context, staffID, decidedBy int64, effectiveDate timezone.Date, balanceMinutes int, note string) (*activeModels.StaffBalanceAdjustment, error) {
	delta, err := s.validateOpeningBalance(ctx, staffID, decidedBy, effectiveDate, balanceMinutes, note, true)
	if err != nil {
		return nil, err
	}

	adjustment := &activeModels.StaffBalanceAdjustment{
		StaffID:       staffID,
		Type:          activeModels.BalanceAdjustmentTypeOpening,
		MinutesDelta:  delta,
		EffectiveDate: effectiveDate,
		Note:          note,
		DecidedBy:     decidedBy,
		DecidedAt:     time.Now(),
	}
	if err := s.adjustmentRepo.Create(ctx, adjustment); err != nil {
		if modelBase.IsUniqueViolationOn(err, openingUniqueConstraintName) {
			return nil, fmt.Errorf("%w: %s", ErrOpeningAlreadyExists, effectiveDate.String())
		}
		return nil, fmt.Errorf("failed to create opening balance: %w", err)
	}
	s.getLogger().Info("opening balance created",
		"staff_id", staffID,
		"effective_date", effectiveDate.String(),
		"balance_minutes", balanceMinutes,
		"minutes_delta", adjustment.MinutesDelta,
		"decided_by", decidedBy,
	)
	s.broadcastTimeTrackingChanged(ctx)
	return adjustment, nil
}

// ValidateOpeningBalance runs the read-only booking guards used by
// CreateOpeningBalance. It deliberately does not take the transaction-scoped
// staff lock: bulk imports validate every uploaded row before their sorted
// creation pass, and file-order locks here could deadlock concurrent imports.
func (s *staffBalanceAdjustmentService) ValidateOpeningBalance(ctx context.Context, staffID, decidedBy int64, effectiveDate timezone.Date, balanceMinutes int, note string) error {
	_, err := s.validateOpeningBalance(ctx, staffID, decidedBy, effectiveDate, balanceMinutes, note, false)
	return err
}

func (s *staffBalanceAdjustmentService) validateOpeningBalance(ctx context.Context, staffID, decidedBy int64, effectiveDate timezone.Date, balanceMinutes int, note string, lockWrites bool) (int, error) {
	if err := validateAdjustmentCommon(staffID, decidedBy, effectiveDate, note); err != nil {
		return 0, err
	}
	if balanceMinutes < -maxBalanceCarryoverMinutes || balanceMinutes > maxBalanceCarryoverMinutes {
		return 0, fmt.Errorf(
			"%w: balance_minutes must be between %d and %d",
			ErrAdjustmentInvalid,
			-maxBalanceCarryoverMinutes,
			maxBalanceCarryoverMinutes,
		)
	}
	// Same reasoning as the reset: the Stichtag needs a closed cutoff so the
	// stored delta cannot go stale before the day ends.
	if !effectiveDate.Before(timezone.TodayDate()) {
		return 0, fmt.Errorf("%w: effective_date must be before today", ErrAdjustmentInvalid)
	}
	if err := s.rejectPreAccountDate(ctx, effectiveDate); err != nil {
		return 0, err
	}
	if lockWrites {
		if err := s.adjustmentRepo.LockStaffBalanceWrites(ctx, staffID); err != nil {
			return 0, fmt.Errorf("failed to lock staff balance writes: %w", err)
		}
	}
	if err := s.rejectFrozenMonth(ctx, staffID, effectiveDate); err != nil {
		return 0, err
	}
	existing, err := s.listOpenings(ctx, staffID)
	if err != nil {
		return 0, err
	}
	if len(existing) > 0 {
		return 0, fmt.Errorf("%w: booked %s", ErrOpeningAlreadyExists, existing[0].EffectiveDate.String())
	}
	rebaselines, err := s.listRebaselinesOnOrAfter(ctx, staffID, effectiveDate)
	if err != nil {
		return 0, err
	}
	if len(rebaselines) > 0 {
		return 0, fmt.Errorf(
			"%w: opening date %s is not after existing %s booking %s",
			ErrAdjustmentHasDependentReset,
			effectiveDate.String(),
			rebaselines[0].Type,
			rebaselines[0].EffectiveDate.String(),
		)
	}

	previousBalance, err := s.monthService.GetClosingBalanceAsOf(ctx, staffID, effectiveDate)
	if err != nil {
		return 0, fmt.Errorf("failed to compute balance for opening: %w", err)
	}
	delta := int64(balanceMinutes) - int64(previousBalance)
	if delta < minPostgresInteger || delta > maxPostgresInteger {
		return 0, fmt.Errorf("%w: calculated opening delta is outside the supported range", ErrAdjustmentInvalid)
	}
	return int(delta), nil
}

// listOpenings lists all opening bookings for a staff member regardless of
// date — the one-opening-ever guard (#2132).
func (s *staffBalanceAdjustmentService) listOpenings(ctx context.Context, staffID int64) ([]*activeModels.StaffBalanceAdjustment, error) {
	options := modelBase.NewQueryOptions()
	options.Filter.
		Equal("staff_id", staffID).
		Equal("type", activeModels.BalanceAdjustmentTypeOpening)
	openings, err := s.adjustmentRepo.List(ctx, options)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing opening balances: %w", err)
	}
	return openings, nil
}

func (s *staffBalanceAdjustmentService) hasDependentReset(ctx context.Context, adjustment *activeModels.StaffBalanceAdjustment) (bool, error) {
	resets, err := s.listRebaselinesOnOrAfter(ctx, adjustment.StaffID, adjustment.EffectiveDate)
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

// listRebaselinesOnOrAfter lists reset AND opening bookings from
// effectiveDate onward. Both types persisted a delta computed from the
// history before their Stichtag, so any write dated on/before them would
// silently shift the balance they pinned — the shared dependent-booking
// guard treats them identically (#1420, #2132).
func (s *staffBalanceAdjustmentService) listRebaselinesOnOrAfter(ctx context.Context, staffID int64, effectiveDate timezone.Date) ([]*activeModels.StaffBalanceAdjustment, error) {
	options := modelBase.NewQueryOptions()
	options.Filter.
		Equal("staff_id", staffID).
		In("type", activeModels.BalanceAdjustmentTypeReset, activeModels.BalanceAdjustmentTypeOpening).
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
