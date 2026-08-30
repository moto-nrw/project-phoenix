package config

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/models/config"
)

// errPresenceModeSwitchBlockedMsg is the German user-facing copy returned when
// an operator tries to flip operations.presence_mode while students are still
// checked in for the day.
//
//nolint:staticcheck // ST1005: German user-facing message
const errPresenceModeSwitchBlockedMsg = "Moduswechsel während aktiver Anwesenheit nicht möglich. Bitte zunächst Tagesabschluss durchführen."

// ErrPresenceModeSwitchBlocked is the sentinel returned by
// CheckPresenceModeSwitch so callers can branch on it via errors.Is without
// relying on string equality. Mirrors the SettingsService error pattern
// (DefinitionNotFoundError / InvalidValueError).
//
//nolint:staticcheck // ST1005: German user-facing message
var ErrPresenceModeSwitchBlocked = errors.New(errPresenceModeSwitchBlockedMsg)

// OpenAttendanceChecker reports whether any student is checked in for a given
// calendar day. Implemented by active.Service.
type OpenAttendanceChecker interface {
	HasOpenAttendanceOn(ctx context.Context, day config.CalendarDate) (bool, error)
}

type OperatorSettingsRuntime interface {
	WithinTenant(context.Context, int64, func(context.Context) error) error
	AfterCommit(context.Context, func())
	Today() config.CalendarDate
}

type SettingsChangedNotifier func(context.Context, int64, string)

// OperatorValueSetHook runs in the same tenant transaction as an operator
// setting write. The returned closure runs after commit and is meant for
// non-transactional work such as file deletion or broadcasts. Mirrors the
// tenant-side ValueSetCallback contract so side effects apply uniformly
// regardless of who flipped the value.
type OperatorValueSetHook func(ctx context.Context, tenantID int64, key string, value any) (postCommit func(), err error)

// OperatorSettingsService orchestrates operator-scoped setting writes: the
// presence-mode switch guard, the tenant transaction, the side-effect hook,
// and the cross-portal settings broadcast. Operators bypass per-setting
// permission checks (SetValue/ResetValue are called with nil permissions).
type OperatorSettingsService interface {
	// SetValue writes a setting value for a school. force bypasses the
	// presence-mode switch guard for operational recovery.
	SetValue(ctx context.Context, schoolID int64, key string, value any, changedBy int64, force bool, hook OperatorValueSetHook) error

	// ResetValue removes a school's setting override, replaying the hook only
	// for keys whose registry default carries reset-specific side effects.
	ResetValue(ctx context.Context, schoolID int64, key string, changedBy int64, hook OperatorValueSetHook) error
}

type operatorSettingsService struct {
	settings   SettingsService
	runtime    OperatorSettingsRuntime
	notify     SettingsChangedNotifier
	attendance OpenAttendanceChecker
	logger     *slog.Logger
}

// NewOperatorSettingsService constructs the operator settings orchestrator.
// broadcaster and attendance are optional (nil-safe) — nil broadcaster skips
// the SSE fan-out; attendance is only consulted for the presence-mode guard.
func NewOperatorSettingsService(
	settings SettingsService,
	runtime OperatorSettingsRuntime,
	notify SettingsChangedNotifier,
	attendance OpenAttendanceChecker,
	logger *slog.Logger,
) OperatorSettingsService {
	if logger == nil {
		logger = slog.Default()
	}
	return &operatorSettingsService{
		settings:   settings,
		runtime:    runtime,
		notify:     notify,
		attendance: attendance,
		logger:     logger,
	}
}

// SetValue orchestrates an operator write inside one tenant transaction:
// presence-mode guard, value write, side-effect hook, and broadcast.
func (s *operatorSettingsService) SetValue(ctx context.Context, schoolID int64, key string, value any, changedBy int64, force bool, hook OperatorValueSetHook) error {
	// Audit-log force-bypass writes on the guarded key so we have a trail when
	// an operator overrides the safety check. Warn (not Info) so it surfaces
	// in standard log review.
	if force && key == config.KeyPresenceMode {
		s.logger.Warn("operator_setting_force_bypass",
			slog.Int64("operator_id", changedBy),
			slog.Int64("school_id", schoolID),
			slog.String("setting_key", key),
		)
	}

	if s.runtime == nil {
		return ErrRuntimeUnavailable
	}
	return s.runtime.WithinTenant(ctx, schoolID, func(txCtx context.Context) error {
		if err := CheckPresenceModeSwitch(txCtx, s.attendance, s.runtime.Today(), key, force); err != nil {
			return err
		}
		if err := s.settings.SetValue(txCtx, key, value, &changedBy, nil); err != nil {
			return err
		}
		if hook != nil {
			cb, err := hook(txCtx, schoolID, key, value)
			if err != nil {
				return err
			}
			s.runtime.AfterCommit(txCtx, cb)
		}
		s.scheduleBroadcast(txCtx, schoolID, key)
		return nil
	})
}

// ResetValue removes a school's setting override, replaying the hook only for
// keys that need reset-specific side effects (e.g. purging student photos when
// student_photos_enabled resets to its default-off state).
func (s *operatorSettingsService) ResetValue(ctx context.Context, schoolID int64, key string, changedBy int64, hook OperatorValueSetHook) error {
	if s.runtime == nil {
		return ErrRuntimeUnavailable
	}
	return s.runtime.WithinTenant(ctx, schoolID, func(txCtx context.Context) error {
		if err := s.settings.ResetValue(txCtx, key, &changedBy, nil); err != nil {
			return err
		}
		if hook != nil && resetReplaysHook(key) {
			if def := config.GetDefinition(key); def != nil {
				cb, err := hook(txCtx, schoolID, key, def.Default)
				if err != nil {
					return err
				}
				s.runtime.AfterCommit(txCtx, cb)
			}
		}
		s.scheduleBroadcast(txCtx, schoolID, key)
		return nil
	})
}

// scheduleBroadcast queues the cross-portal tenant_settings_changed SSE event
// to fire after the tenant transaction commits. No-op without a broadcaster.
func (s *operatorSettingsService) scheduleBroadcast(ctx context.Context, tenantID int64, key string) {
	if s.notify == nil || tenantID == 0 {
		return
	}
	s.runtime.AfterCommit(ctx, func() {
		s.notify(ctx, tenantID, key)
	})
}

// resetReplaysHook gates which keys re-run their side-effect hook on reset.
// Limited to keys whose registry default carries semantic meaning (e.g. the
// student-photo purge on disable).
func resetReplaysHook(key string) bool {
	return key == config.KeyStudentPhotosEnabled || key == config.KeyEnrollmentBookingsAuthoritative
}

// CheckPresenceModeSwitch rejects a write to operations.presence_mode when any
// student is currently checked in for the tenant, unless force is set. Must run
// inside the tenant transaction so RLS scopes the attendance query.
//
// Why guard only the switch, not every write: the cascading impact of flipping
// presence mode mid-day (stale open visits, SSE events mis-keyed, device UX
// inconsistent with the tenant it's authenticated for) is far larger than any
// other operator setting.
func CheckPresenceModeSwitch(ctx context.Context, checker OpenAttendanceChecker, today config.CalendarDate, key string, force bool) error {
	if key != config.KeyPresenceMode {
		return nil
	}
	if force {
		return nil
	}
	if checker == nil {
		return nil
	}
	// Bind the Berlin calendar date as a DATE literal (matches how
	// performCheckIn writes active.attendance.date). Using CURRENT_DATE on the
	// postgres side would be wrong: the PG session is UTC, so between
	// 22:00–24:00 UTC (i.e. ~00:00–02:00 Berlin) CURRENT_DATE returns
	// "yesterday" while open rows for the new Berlin day already exist under
	// "today" — the guard would silently let the switch through in exactly the
	// window it's supposed to block.
	exists, err := checker.HasOpenAttendanceOn(ctx, today)
	if err != nil {
		return fmt.Errorf("failed to check active attendance before mode switch: %w", err)
	}
	if exists {
		return ErrPresenceModeSwitchBlocked
	}
	return nil
}
