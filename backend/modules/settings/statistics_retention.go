package settings

import (
	"context"
	"fmt"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// StatisticsRetentionSource is the resolver slice the statistics retention
// windows read through.
type StatisticsRetentionSource interface {
	ResolveInt(ctx context.Context, key string) (int, error)
}

// StatisticsRetention are the retention windows that cap the Statistik
// report: room visits are kept as long as the visits of a child without an
// accepted consent (privacy consent retention), course attendance as long as
// the timetable keeps its blocks.
type StatisticsSettingsSource interface {
	StatisticsRetentionSource
	ResolveBool(context.Context, string) (bool, error)
}

type StatisticsRetention struct{ source StatisticsSettingsSource }

// NewStatisticsRetention binds the windows to the resolver.
func NewStatisticsRetention(source StatisticsSettingsSource) StatisticsRetention {
	return StatisticsRetention{source: source}
}

func (r StatisticsRetention) RoomRetentionDays(ctx context.Context) (int, error) {
	enabled, err := r.source.ResolveBool(ctx, configModel.KeyAttendanceLogEnabled)
	if err != nil {
		return 0, err
	}
	if !enabled {
		return 0, nil
	}
	return r.source.ResolveInt(ctx, configModel.KeyPrivacyConsentRetentionDays)
}

func (r StatisticsRetention) CourseRetentionDays(ctx context.Context) (int, error) {
	return r.source.ResolveInt(ctx, configModel.KeyGDPRTimetableRetentionDays)
}

// AttendanceRetentionDays caps the actual values of the occupancy by time of
// day (#4764): daily attendance is kept that long.
func (r StatisticsRetention) AttendanceRetentionDays(ctx context.Context) (int, error) {
	return NewAttendanceRetention(r.source).Days(ctx)
}

// StatusDayRetention is the retention window of the student status days
// (sick and excused days), the tenant's GDPR status day retention.
type StatusDayRetention struct{ source StatisticsRetentionSource }

// NewStatusDayRetention binds the window to the resolver.
func NewStatusDayRetention(source StatisticsRetentionSource) StatusDayRetention {
	return StatusDayRetention{source: source}
}

// Days resolves the window.
func (r StatusDayRetention) Days(ctx context.Context) (int, error) {
	days, err := r.source.ResolveInt(ctx, configModel.KeyGDPRStatusDayRetentionDays)
	if err != nil {
		return 0, r.SettingError(err)
	}
	return days, nil
}

// SettingError names the setting behind a window the deletion refused or
// could not read.
func (r StatusDayRetention) SettingError(err error) error {
	return fmt.Errorf("resolve %s: %w", configModel.KeyGDPRStatusDayRetentionDays, err)
}

// AttendanceRetention is the retention window of the daily attendance, the
// tenant's GDPR attendance retention (#3957).
type AttendanceRetention struct{ source StatisticsRetentionSource }

// NewAttendanceRetention binds the window to the resolver.
func NewAttendanceRetention(source StatisticsRetentionSource) AttendanceRetention {
	return AttendanceRetention{source: source}
}

// Days resolves the window.
func (r AttendanceRetention) Days(ctx context.Context) (int, error) {
	days, err := r.source.ResolveInt(ctx, configModel.KeyGDPRAttendanceRetentionDays)
	if err != nil {
		return 0, r.SettingError(err)
	}
	return days, nil
}

// SettingError names the setting behind a window the deletion refused or
// could not read.
func (r AttendanceRetention) SettingError(err error) error {
	return fmt.Errorf("resolve %s: %w", configModel.KeyGDPRAttendanceRetentionDays, err)
}

// GuardianRetention is the retention window of a parent contact without a
// child, the tenant's GDPR guardian retention (#4043).
type GuardianRetention struct{ source StatisticsRetentionSource }

// NewGuardianRetention binds the window to the resolver.
func NewGuardianRetention(source StatisticsRetentionSource) GuardianRetention {
	return GuardianRetention{source: source}
}

// Days resolves the window.
func (r GuardianRetention) Days(ctx context.Context) (int, error) {
	days, err := r.source.ResolveInt(ctx, configModel.KeyGDPRGuardianRetentionDays)
	if err != nil {
		return 0, r.SettingError(err)
	}
	return days, nil
}

// SettingError names the setting behind a window the deletion refused or
// could not read.
func (r GuardianRetention) SettingError(err error) error {
	return fmt.Errorf("resolve %s: %w", configModel.KeyGDPRGuardianRetentionDays, err)
}
