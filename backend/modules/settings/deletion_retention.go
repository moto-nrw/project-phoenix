package settings

import (
	"context"
	"fmt"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// RetentionWindowKeys are the retention windows the nightly cleanup reads per
// school; the cleanup preloads them before it runs.
func RetentionWindowKeys() []string {
	return []string{
		configModel.KeyGDPRStatusDayRetentionDays,
		configModel.KeyGDPRAttendanceRetentionDays,
		configModel.KeyGDPRMorningAttendanceRetentionDays,
		configModel.KeyGDPRParentMessageRetentionDays,
		configModel.KeyGDPRStaffMessageRetentionDays,
		configModel.KeyGDPRTimetableRetentionDays,
		configModel.KeyGDPRWithdrawnEnrollmentRetentionDays,
		configModel.KeyGDPRStaffSickNoteRetentionDays,
		configModel.KeyGDPRStudentRetentionDays,
		configModel.KeyGDPRGuardianRetentionDays,
		configModel.KeyGDPRTimeTrackingRetentionDays,
	}
}

// StudentRetentionWindow resolves how many days after the last care day a
// child is deleted automatically (#3962).
func StudentRetentionWindow(source StatisticsRetentionSource) func(context.Context) (int, error) {
	return func(ctx context.Context) (int, error) {
		return source.ResolveInt(ctx, configModel.KeyGDPRStudentRetentionDays)
	}
}

// MorningAttendanceRetentionWindow resolves how many days after the school
// day the morning attendance a Lehrkraft marked is deleted (#4790). A failed
// read names the setting.
func MorningAttendanceRetentionWindow(source StatisticsRetentionSource) func(context.Context) (int, error) {
	return func(ctx context.Context) (int, error) {
		days, err := source.ResolveInt(ctx, configModel.KeyGDPRMorningAttendanceRetentionDays)
		if err != nil {
			return 0, fmt.Errorf("resolve %s: %w", configModel.KeyGDPRMorningAttendanceRetentionDays, err)
		}
		return days, nil
	}
}

// StaffRetentionWindow resolves how many days after the exit a staff member
// is deleted automatically: the school's time-tracking window (#3963).
func StaffRetentionWindow(source StatisticsRetentionSource) func(context.Context) (int, error) {
	return func(ctx context.Context) (int, error) {
		return source.ResolveInt(ctx, configModel.KeyGDPRTimeTrackingRetentionDays)
	}
}

// RetentionWindow is one retention window of the school in context that a
// nightly step reads: the content a school creates itself follows the window
// of the data kind it belongs to (#4045).
type RetentionWindow struct {
	source StatisticsRetentionSource
	key    string
}

// ParentAnnouncementRetentionWindow is the window of the parent
// announcements: the school's parent message window.
func ParentAnnouncementRetentionWindow(source StatisticsRetentionSource) RetentionWindow {
	return RetentionWindow{source: source, key: configModel.KeyGDPRParentMessageRetentionDays}
}

// StaffNoticeRetentionWindow is the window of the staff notices: the
// school's team chat window.
func StaffNoticeRetentionWindow(source StatisticsRetentionSource) RetentionWindow {
	return RetentionWindow{source: source, key: configModel.KeyGDPRStaffMessageRetentionDays}
}

// AppointmentRetentionWindow is the window of the ended appointments: the
// school's timetable window.
func AppointmentRetentionWindow(source StatisticsRetentionSource) RetentionWindow {
	return RetentionWindow{source: source, key: configModel.KeyGDPRTimetableRetentionDays}
}

// GradeTransitionRetentionWindow is the window of the grade transitions,
// whose history names children: the school's window for children after
// their last care day.
func GradeTransitionRetentionWindow(source StatisticsRetentionSource) RetentionWindow {
	return RetentionWindow{source: source, key: configModel.KeyGDPRStudentRetentionDays}
}

// Days resolves the window.
func (w RetentionWindow) Days(ctx context.Context) (int, error) {
	days, err := w.source.ResolveInt(ctx, w.key)
	if err != nil {
		return 0, w.SettingError(err)
	}
	return days, nil
}

// SettingError names the setting behind a window the deletion refused or
// could not read.
func (w RetentionWindow) SettingError(err error) error {
	return fmt.Errorf("resolve %s: %w", w.key, err)
}
