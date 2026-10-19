package settings

import (
	"context"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// RetentionWindowKeys are the retention windows the nightly cleanup reads per
// school; the cleanup preloads them before it runs.
func RetentionWindowKeys() []string {
	return []string{
		configModel.KeyGDPRStatusDayRetentionDays,
		configModel.KeyGDPRAttendanceRetentionDays,
		configModel.KeyGDPRParentMessageRetentionDays,
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

// StaffRetentionWindow resolves how many days after the exit a staff member
// is deleted automatically: the school's time-tracking window (#3963).
func StaffRetentionWindow(source StatisticsRetentionSource) func(context.Context) (int, error) {
	return func(ctx context.Context) (int, error) {
		return source.ResolveInt(ctx, configModel.KeyGDPRTimeTrackingRetentionDays)
	}
}
