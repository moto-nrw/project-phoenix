package config

import (
	"context"
	"fmt"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// The window the team sees attendance (gdpr.attendance_visible_days) never
// exceeds the attendance retention window (gdpr.attendance_retention_days,
// #3957): the school would otherwise promise a history the nightly cleanup
// has already deleted. The school writes the first, the moto team the second.

func isAttendanceWindowKey(key string) bool {
	return key == config.KeyAttendanceVisibleDays || key == config.KeyGDPRAttendanceRetentionDays
}

// lockAttendanceWindows serializes both write directions until commit, like
// lockOverviewScopeDependents: two writers starting from a valid pair would
// otherwise each read the other's old value and commit an inverted pair.
func (s *settingsService) lockAttendanceWindows(ctx context.Context) error {
	s.flushRequestCacheForLock(ctx)
	if s.runtime == nil || !s.runtime.HasTransaction(ctx) {
		return config.ErrRuntimeUnavailable
	}
	if err := s.runtime.AcquireLock(ctx, fmt.Sprintf("attendance-windows:%d", s.tenantID(ctx)), false); err != nil {
		return fmt.Errorf("lock attendance windows: %w", err)
	}
	return nil
}

func (s *settingsService) validateAttendanceWindows(ctx context.Context, key string, value any) error {
	num, ok := toFloat64(value)
	if !ok {
		// validateValue already rejects a non-numeric value.
		return nil
	}
	if err := s.lockAttendanceWindows(ctx); err != nil {
		return err
	}
	sibling := config.KeyGDPRAttendanceRetentionDays
	if key == config.KeyGDPRAttendanceRetentionDays {
		sibling = config.KeyAttendanceVisibleDays
	}
	snapshot, err := s.resolveForTenantInTx(ctx, s.tenantID(ctx), sibling)
	if err != nil {
		return fmt.Errorf("resolve paired attendance window: %w", err)
	}
	current, err := snapshot.Int(sibling)
	if err != nil {
		return fmt.Errorf("resolve paired attendance window: %w", err)
	}
	visible, retention := int(num), current
	if key == config.KeyGDPRAttendanceRetentionDays {
		visible, retention = current, int(num)
	}
	if visible <= retention {
		return nil
	}
	return &config.AttendanceWindowError{Key: key, VisibleDays: visible, RetentionDays: retention}
}
