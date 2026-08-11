package authorize

// Pure-logic tests for student_absence.go (#2232). Reuses the stubs declared
// in student_access_test.go (stubUserCtx, stubSettings, studentInGroup, ptr).

import (
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func openCareSettings() *stubSettings {
	return &stubSettings{value: configModel.GroupModeOpenCare}
}

func fixedGroupSettings() *stubSettings {
	return &stubSettings{value: configModel.GroupModeFixedGroups}
}

func staffCtx() *stubUserCtx {
	return &stubUserCtx{staff: &users.Staff{}}
}

func groupless() *users.Student {
	return &users.Student{}
}

// --- CanManageStudentAbsence -------------------------------------------

func TestCanManageStudentAbsence_OpenCareGrouplessStudent(t *testing.T) {
	ok, err := CanManageStudentAbsence(
		t.Context(),
		[]string{"users:read", "users:update", "users:absence"},
		groupless(),
		staffCtx(),
		openCareSettings(),
		nil,
	)
	require.NoError(t, err)
	assert.True(t, ok, "open care + users:absence + staff must be allowed on a child without a group")
}

func TestCanManageStudentAbsence_OpenCareStudentInForeignGroup(t *testing.T) {
	// A tenant that switched to open care keeps historical group assignments;
	// the mode, not the group, decides.
	ok, err := CanManageStudentAbsence(
		t.Context(),
		[]string{"users:read", "users:absence"},
		studentInGroup(42),
		staffCtx(),
		openCareSettings(),
		nil,
	)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestCanManageStudentAbsence_OpenCareWithoutPermissionDenied(t *testing.T) {
	ok, err := CanManageStudentAbsence(
		t.Context(),
		[]string{"users:read", "users:update"},
		groupless(),
		staffCtx(),
		openCareSettings(),
		nil,
	)
	assert.False(t, ok)
	assert.ErrorIs(t, err, ErrAbsencePermissionRequired)
}

func TestCanManageStudentAbsence_OpenCareWithoutReadDenied(t *testing.T) {
	// users:absence is a write scope on top of the children a caller may see.
	// It unlocks no read surface — neither the child's list entry nor the
	// detail page — so on its own it must grant nothing at all rather than a
	// write for a child its holder cannot open.
	ok, err := CanManageStudentAbsence(
		t.Context(),
		[]string{"users:absence"},
		groupless(),
		staffCtx(),
		openCareSettings(),
		nil,
	)
	assert.False(t, ok)
	assert.ErrorIs(t, err, ErrAbsenceReadRequired)
}

func TestCanManageStudentAbsence_OpenCareNonStaffDenied(t *testing.T) {
	// Guest/guardian accounts authenticate against the same portal; holding the
	// permission is not enough without a staff record in this tenant.
	ok, err := CanManageStudentAbsence(
		t.Context(),
		[]string{"users:read", "users:absence"},
		groupless(),
		&stubUserCtx{}, // no staff record
		openCareSettings(),
		nil,
	)
	assert.False(t, ok)
	assert.ErrorIs(t, err, ErrAbsenceStaffRequired)
}

func TestCanManageStudentAbsence_FixedGroupsKeepsSupervisorRestriction(t *testing.T) {
	ok, err := CanManageStudentAbsence(
		t.Context(),
		[]string{"users:update", "users:absence"},
		studentInGroup(7),
		staffCtx(), // supervises nothing
		fixedGroupSettings(),
		nil,
	)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "groups you supervise")
}

func TestCanManageStudentAbsence_FixedGroupsSupervisorAllowed(t *testing.T) {
	userCtx := &stubUserCtx{
		staff:  &users.Staff{},
		groups: []*education.Group{{Model: base.Model{ID: 7}}},
	}
	ok, err := CanManageStudentAbsence(
		t.Context(),
		[]string{"users:update"},
		studentInGroup(7),
		userCtx,
		fixedGroupSettings(),
		nil,
	)
	require.NoError(t, err)
	assert.True(t, ok, "the unchanged supervisor path must not need users:absence")
}

func TestCanManageStudentAbsence_AdminAlwaysAllowed(t *testing.T) {
	ok, err := CanManageStudentAbsence(
		t.Context(),
		[]string{"admin:*"},
		groupless(),
		nil,
		nil,
		nil,
	)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestCanManageStudentAbsence_SettingsFailureFailsClosed(t *testing.T) {
	ok, err := CanManageStudentAbsence(
		t.Context(),
		[]string{"users:absence"},
		groupless(),
		staffCtx(),
		&stubSettings{valueErr: errors.New("db down")},
		nil,
	)
	assert.False(t, ok, "a settings fault must never be read as open care")
	require.Error(t, err)
}

func TestCanManageStudentAbsence_NilSettingsFailsClosed(t *testing.T) {
	ok, _ := CanManageStudentAbsence(
		t.Context(),
		[]string{"users:absence"},
		groupless(),
		staffCtx(),
		nil,
		nil,
	)
	assert.False(t, ok)
}

func TestCanManageStudentAbsence_NilStudentDenied(t *testing.T) {
	ok, err := CanManageStudentAbsence(
		t.Context(),
		[]string{"users:absence"},
		nil,
		staffCtx(),
		openCareSettings(),
		nil,
	)
	assert.False(t, ok)
	require.Error(t, err)
}

// --- AbsenceWritableStudentFilter --------------------------------------

func TestAbsenceWritableStudentFilter_OpenCareCoversEveryStudent(t *testing.T) {
	filter := AbsenceWritableStudentFilter(
		t.Context(),
		[]string{"users:read", "users:absence"},
		staffCtx(),
		openCareSettings(),
		nil,
	)
	assert.True(t, filter(groupless()))
	assert.True(t, filter(studentInGroup(99)))
	assert.False(t, filter(nil))
}

func TestAbsenceWritableStudentFilter_FixedGroupsMatchesSupervisedSet(t *testing.T) {
	userCtx := &stubUserCtx{
		staff:  &users.Staff{},
		groups: []*education.Group{{Model: base.Model{ID: 3}}},
	}
	filter := AbsenceWritableStudentFilter(
		t.Context(),
		[]string{"users:update", "users:absence"},
		userCtx,
		fixedGroupSettings(),
		nil,
	)
	assert.True(t, filter(studentInGroup(3)))
	assert.False(t, filter(studentInGroup(4)))
	assert.False(t, filter(groupless()))
}

func TestAbsenceWritableStudentFilter_OpenCareWithoutPermissionStaysSupervisorScoped(t *testing.T) {
	filter := AbsenceWritableStudentFilter(
		t.Context(),
		[]string{"users:update"},
		staffCtx(),
		openCareSettings(),
		nil,
	)
	assert.False(t, filter(groupless()))
}

func TestAbsenceWritableStudentFilter_OpenCareWithoutReadStaysSupervisorScoped(t *testing.T) {
	// The set form has to agree with CanManageStudentAbsence, including its
	// read requirement — otherwise the queue would list children the decision
	// then refuses.
	filter := AbsenceWritableStudentFilter(
		t.Context(),
		[]string{"users:absence"},
		staffCtx(),
		openCareSettings(),
		nil,
	)
	assert.False(t, filter(groupless()))
}

// --- CanReviewExcusedAbsenceRequests -----------------------------------

func TestCanReviewExcusedAbsenceRequests(t *testing.T) {
	cases := []struct {
		name        string
		permissions []string
		want        bool
	}{
		{"users:update", []string{"users:read", "users:update"}, true},
		{"users:absence", []string{"users:read", "users:absence"}, true},
		{"admin wildcard", []string{"admin:*"}, true},
		{"users:read only", []string{"users:read"}, false},
		{"nothing", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, CanReviewExcusedAbsenceRequests(tc.permissions))
		})
	}
}
