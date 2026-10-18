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

const (
	attendanceVisibleTooLongReason    = "Die Anwesenheit ist höchstens so lange sichtbar, wie moto sie aufbewahrt. Wählen Sie höchstens %d Tage."
	attendanceRetentionTooShortReason = "Das Team sieht die Anwesenheit %d Tage zurück. Die Aufbewahrung muss mindestens so lang sein. Verkürzen Sie zuerst die Sichtbarkeit."
)

func isAttendanceWindowKey(key string) bool {
	return key == config.KeyAttendanceVisibleDays || key == config.KeyGDPRAttendanceRetentionDays
}

// lockAttendanceWindows serializes both write directions until commit, like
// lockOverviewScopeDependents: two writers starting from a valid pair would
// otherwise each read the other's old value and commit an inverted pair.
func (s *settingsService) lockAttendanceWindows(ctx context.Context) error {
	s.flushRequestCacheForLock(ctx)
	if s.runtime == nil || !s.runtime.HasTransaction(ctx) {
		return ErrRuntimeUnavailable
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
	if key == config.KeyAttendanceVisibleDays {
		return &InvalidValueError{Key: key, Reason: fmt.Sprintf(attendanceVisibleTooLongReason, retention)}
	}
	return &InvalidValueError{Key: key, Reason: fmt.Sprintf(attendanceRetentionTooShortReason, visible)}
}
