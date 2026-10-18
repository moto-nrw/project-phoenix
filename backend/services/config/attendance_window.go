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

// AttendanceWindowError rejects a write that would let the visible window
// exceed the retention window. Key is the setting written; the HTTP layer
// names the conflicting window to the person through its own error code.
type AttendanceWindowError struct {
	Key           string
	VisibleDays   int
	RetentionDays int
}

func (e *AttendanceWindowError) Error() string {
	return fmt.Sprintf("invalid value for setting %s: attendance visible for %d days exceeds its retention of %d days",
		e.Key, e.VisibleDays, e.RetentionDays)
}

// VisibleWritten reports whether the write lengthened the visible window;
// otherwise it shortened the retention window.
func (e *AttendanceWindowError) VisibleWritten() bool {
	return e.Key == config.KeyAttendanceVisibleDays
}

// Unwrap keeps the rejection an invalid value (400) for every caller.
func (e *AttendanceWindowError) Unwrap() error {
	return ErrInvalidValue
}

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
	return &AttendanceWindowError{Key: key, VisibleDays: visible, RetentionDays: retention}
}
