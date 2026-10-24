package settings

import (
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeletionRetentionWindowsReadTheirSettings(t *testing.T) {
	t.Parallel()
	source := retentionSource{
		configModel.KeyGDPRStudentRetentionDays:      400,
		configModel.KeyGDPRTimeTrackingRetentionDays: 800,
	}
	students, err := StudentRetentionWindow(source)(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 400, students)
	staff, err := StaffRetentionWindow(source)(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 800, staff)
	morning, err := MorningAttendanceRetentionWindow(retentionSource{configModel.KeyGDPRMorningAttendanceRetentionDays: 60})(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 60, morning)
	_, err = MorningAttendanceRetentionWindow(retentionSource{})(t.Context())
	require.ErrorContains(t, err, "resolve "+configModel.KeyGDPRMorningAttendanceRetentionDays+": ")
	assert.Equal(t, []string{
		configModel.KeyGDPRStatusDayRetentionDays,
		configModel.KeyGDPRAttendanceRetentionDays,
		configModel.KeyGDPRMorningAttendanceRetentionDays,
		configModel.KeyGDPRHomeworkRetentionDays,
		configModel.KeyGDPRParentMessageRetentionDays,
		configModel.KeyGDPRStaffMessageRetentionDays,
		configModel.KeyGDPRTimetableRetentionDays,
		configModel.KeyGDPRWithdrawnEnrollmentRetentionDays,
		configModel.KeyGDPRStaffSickNoteRetentionDays,
		configModel.KeyGDPRStudentRetentionDays,
		configModel.KeyGDPRGuardianRetentionDays,
		configModel.KeyGDPRTimeTrackingRetentionDays,
	}, RetentionWindowKeys())
}

func TestSchoolContentFollowsTheWindowOfItsDataKind(t *testing.T) {
	t.Parallel()
	source := retentionSource{
		configModel.KeyGDPRParentMessageRetentionDays: 100,
		configModel.KeyGDPRStaffMessageRetentionDays:  200,
		configModel.KeyGDPRTimetableRetentionDays:     300,
		configModel.KeyGDPRStudentRetentionDays:       400,
	}
	for window, want := range map[string]struct {
		window RetentionWindow
		days   int
	}{
		"parent announcements": {ParentAnnouncementRetentionWindow(source), 100},
		"staff notices":        {StaffNoticeRetentionWindow(source), 200},
		"appointments":         {AppointmentRetentionWindow(source), 300},
		"grade transitions":    {GradeTransitionRetentionWindow(source), 400},
	} {
		days, err := want.window.Days(t.Context())
		require.NoError(t, err, window)
		assert.Equal(t, want.days, days, window)
	}
	assert.ErrorContains(t, StaffNoticeRetentionWindow(source).SettingError(assert.AnError), configModel.KeyGDPRStaffMessageRetentionDays)
}
