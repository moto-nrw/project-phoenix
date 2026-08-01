package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

var errSchedulerSettingsBatchUnsupported = errors.New("scheduler settings resolver does not support batch snapshots")

type schedulerSettingsBatchResolver interface {
	ResolveManyForTenants(ctx context.Context, tenantIDs []int64, keys []string) (map[int64]*configService.SettingsSnapshot, error)
}

type schedulerMinuteSnapshot struct {
	tenantIDs []int64
	settings  map[int64]*configService.SettingsSnapshot
}

type schedulerMinuteSnapshotLoad struct {
	bucket   time.Time
	ready    chan struct{}
	snapshot *schedulerMinuteSnapshot
	err      error
}

// schedulerPollingSettingKeys lists settings read by idle polling paths and
// the reminder/notification services they invoke every minute. Job-specific
// values used only when a daily job actually fires stay demand-loaded.
var schedulerPollingSettingKeys = []string{
	configModel.KeyDataCleanupEnabled,
	configModel.KeyDataCleanupTime,
	configModel.KeyDataCleanupTimeoutMinutes,
	configModel.KeyFeedbackDataRetentionDays,
	configModel.KeySessionEndEnabled,
	configModel.KeySessionEndTime,
	configModel.KeySessionEndTimeoutMinutes,
	configModel.KeySessionCleanupEnabled,
	configModel.KeySessionCleanupIntervalMinutes,
	configModel.KeySessionAbandonedThresholdMin,
	configModel.KeyTrackingAutoCheckoutEnabled,
	configModel.KeyTrackingAutoCheckoutGraceMinutes,
	configModel.KeyStatusFlagClearTime,
	configModel.KeySickClearMode,
	configModel.KeyExcusedClearMode,
	configModel.KeyTimetableMaterializationEnabled,
	configModel.KeyTimetableMaterializationWeekday,
	configModel.KeyTimetableMaterializationWeeksAhead,
	configModel.KeyTimetableEnabled,
	configModel.KeyTimetableAutoStartPlanned,
	configModel.KeyTimetableOverdueThresholdMinutes,
	configModel.KeyNotificationsDispatchEnabled,
	configModel.KeyNotificationsOnDutyOnly,
	configModel.KeyRemindersPickupUpcomingEnabled,
	configModel.KeyRemindersPickupOverdueEnabled,
	configModel.KeyRemindersActivityStartEnabled,
	configModel.KeyRemindersActivityOverdueEnabled,
	configModel.KeyRemindersPickupUpcomingLeadMinutes,
	configModel.KeyRemindersActivityStartLeadMinutes,
	configModel.KeyCalendarAppointmentReminderEnabled,
	configModel.KeyCalendarAppointmentReminderLeadHours,
	configModel.KeyPresenceMode,
	configModel.KeyStudentDataScope,
	configModel.KeyEnrollmentWaitlistEnabled,
	configModel.KeyEnrollmentAutoInviteGuardianOnApprove,
	configModel.KeyEnrollmentCareOfferingsEnabled,
	configModel.KeyEnrollmentDefaultActivationMode,
	configModel.KeyEnrollmentNotifyPerDecision,
}

// getMinuteSnapshot coalesces concurrent scheduler goroutines into one active
// tenant query and one config.setting_values batch query per wall-clock minute.
func (s *Scheduler) getMinuteSnapshot(ctx context.Context) (*schedulerMinuteSnapshot, error) {
	now := time.Now
	if s.minuteSnapshotNow != nil {
		now = s.minuteSnapshotNow
	}
	// This bucket only coalesces work within one minute; it does not model a
	// Berlin calendar date or wall-clock business time.
	bucket := now().Truncate(time.Minute) //nolint:forbidigo

	s.minuteSnapshotMu.Lock()
	if load := s.minuteSnapshotLoad; load != nil && load.bucket.Equal(bucket) {
		s.minuteSnapshotMu.Unlock()
		select {
		case <-load.ready:
			return load.snapshot, load.err
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-s.done:
			return nil, context.Canceled
		}
	}

	load := &schedulerMinuteSnapshotLoad{
		bucket: bucket,
		ready:  make(chan struct{}),
	}
	s.minuteSnapshotLoad = load
	s.minuteSnapshotMu.Unlock()

	loader := s.loadMinuteSnapshot
	if s.minuteSnapshotLoader != nil {
		loader = s.minuteSnapshotLoader
	}
	load.snapshot, load.err = loader(ctx)

	s.minuteSnapshotMu.Lock()
	close(load.ready)
	s.minuteSnapshotMu.Unlock()
	return load.snapshot, load.err
}

func (s *Scheduler) loadMinuteSnapshot(ctx context.Context) (*schedulerMinuteSnapshot, error) {
	result := &schedulerMinuteSnapshot{}
	err := tenant.WithAdminTx(ctx, s.db, func(txCtx context.Context, _ bun.Tx) error {
		schools, listErr := s.schoolRepo.ListActive(txCtx)
		if listErr != nil {
			return listErr
		}
		result.tenantIDs = make([]int64, 0, len(schools))
		for _, school := range schools {
			result.tenantIDs = append(result.tenantIDs, school.ID)
		}
		return nil
	})
	if err != nil {
		return result, fmt.Errorf("list active tenants: %w", err)
	}

	batchResolver, ok := s.settings.(schedulerSettingsBatchResolver)
	if !ok {
		return result, errSchedulerSettingsBatchUnsupported
	}
	result.settings, err = batchResolver.ResolveManyForTenants(ctx, result.tenantIDs, schedulerPollingSettingKeys)
	if err != nil {
		return result, fmt.Errorf("load tenant settings: %w", err)
	}
	return result, nil
}

func (s *Scheduler) forEachKnownTenant(
	ctx context.Context,
	tenantIDs []int64,
	opName string,
	fn func(context.Context, int64) error,
) {
	for _, tenantID := range tenantIDs {
		err := tenant.WithTenantTx(ctx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
			return fn(txCtx, tenantID)
		})
		if err != nil {
			s.getLogger().Error("tenant operation failed, continuing to next tenant",
				slog.String("operation", opName),
				slog.Int64("tenant_id", tenantID),
				slog.String("error", err.Error()),
			)
		}
	}
}
