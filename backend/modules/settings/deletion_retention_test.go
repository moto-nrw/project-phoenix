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
	assert.Equal(t, []string{
		configModel.KeyGDPRStatusDayRetentionDays,
		configModel.KeyGDPRAttendanceRetentionDays,
		configModel.KeyGDPRParentMessageRetentionDays,
		configModel.KeyGDPRWithdrawnEnrollmentRetentionDays,
		configModel.KeyGDPRStaffSickNoteRetentionDays,
		configModel.KeyGDPRStudentRetentionDays,
		configModel.KeyGDPRTimeTrackingRetentionDays,
	}, RetentionWindowKeys())
}
