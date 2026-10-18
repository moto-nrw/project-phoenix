package scheduler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
)

var errSchedulerSettingsBatchUnsupported = errors.New("scheduler settings resolver does not support batch snapshots")

type schedulerMinuteSnapshot struct {
	tenantIDs []int64
	settings  map[int64]ports.SettingsSnapshot
}

type schedulerMinuteSnapshotLoad struct {
	bucket   time.Time
	ready    chan struct{}
	snapshot *schedulerMinuteSnapshot
	err      error
}

// schedulerPollingSettingKeys lists settings read by idle polling paths, the
// reminder/notification services they invoke every minute, and nightly
// retention owners. Bound settings snapshots reject undeclared keys, so every
// setting a fired daily job reads belongs here too.
var schedulerPollingSettingKeys = []string{
	settingDataCleanupEnabled,
	settingDataCleanupTime,
	settingDataCleanupTimeoutMinutes,
	settingStatusDayRetentionDays,
	settingAttendanceRetentionDays,
	settingGuardianRetentionDays,
	settingParentMessageRetentionDays,
	settingWithdrawnEnrollmentRetentionDays,
	settingEnrollmentRejectedRetentionDays,
	settingStaffSickNoteRetentionDays,
	settingStudentRetentionDays,
	settingTimeTrackingRetentionDays,
	settingFeedbackDataRetentionDays,
	settingSessionEndEnabled,
	settingSessionEndTime,
	settingSessionEndTimeoutMinutes,
	settingSessionCleanupEnabled,
	settingSessionCleanupIntervalMinutes,
	settingSessionAbandonedThresholdMin,
	settingTrackingAutoCheckoutEnabled,
	settingTrackingAutoCheckoutGraceMinutes,
	settingSickClearMode,
	settingExcusedClearMode,
	settingTimetableMaterializationEnabled,
	settingTimetableMaterializationWeekday,
	settingTimetableMaterializationWeeksAhead,
	settingTimetableEnabled,
	settingTimetableAutoStartPlanned,
	settingTimetableAutoEndEnabled,
	settingTimetableAutoEndGraceMinutes,
	settingTimetableOverdueThresholdMinutes,
	settingNotificationsDispatchEnabled,
	settingNotificationsOnDutyOnly,
	settingRemindersPickupUpcomingEnabled,
	settingRemindersPickupOverdueEnabled,
	settingRemindersActivityStartEnabled,
	settingRemindersActivityOverdueEnabled,
	settingRemindersPickupUpcomingLeadMinutes,
	settingRemindersActivityStartLeadMinutes,
	settingCalendarAppointmentReminderEnabled,
	settingCalendarAppointmentReminderLeadHours,
	settingPresenceMode,
	// Read for weekend dates by every job that touches care days (#3921).
	settingWeekendFollowsFriday,
	settingEnrollmentWaitlistEnabled,
	settingEnrollmentAutoInviteGuardianOnApprove,
	settingEnrollmentCareOfferingsEnabled,
	settingEnrollmentDefaultActivationMode,
	settingEnrollmentNotifyPerDecision,
}

// getMinuteSnapshot coalesces concurrent scheduler goroutines into one active
// tenant query and one config.setting_values batch query per wall-clock minute.
func (s *Scheduler) getMinuteSnapshot(ctx context.Context) (*schedulerMinuteSnapshot, error) {
	ctx = s.withUnitOfWork(ctx)
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
	ctx = s.withUnitOfWork(ctx)
	result := &schedulerMinuteSnapshot{}
	err := s.tenantRuntime.WithinAdmin(ctx, func(txCtx context.Context) error {
		tenantIDs, listErr := s.schoolRepo.ListActiveTenantIDs(txCtx)
		if listErr != nil {
			return listErr
		}
		result.tenantIDs = tenantIDs
		return nil
	})
	if err != nil {
		return result, fmt.Errorf("list active tenants: %w", err)
	}

	batch, ok := s.settings.(ports.SettingsSnapshotSource)
	if !ok {
		return result, errSchedulerSettingsBatchUnsupported
	}
	result.settings, err = batch.ResolveSettingsSnapshots(ctx, result.tenantIDs, schedulerPollingSettingKeys)
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
) []int64 {
	result := s.runTenantBatches(ctx, tenantIDs, opName, adaptTenantCommand(fn))
	return result.CompletedTenantIDs()
}
