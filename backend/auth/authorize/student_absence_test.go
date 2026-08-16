package authorize

// Pure-logic tests for student_absence.go (#2232, reworked for #2329). Reuses
// the stubs declared in student_access_test.go (stubUserCtx, studentInGroup,
// ptr).

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func staffCtx() *stubUserCtx {
	return &stubUserCtx{staff: &users.Staff{}}
}

func groupless() *users.Student {
	return &users.Student{}
}

// --- CanManageStudentAbsence -------------------------------------------

func TestCanManageStudentAbsence_StaffGrouplessStudent(t *testing.T) {
	ok, err := CanManageStudentAbsence(
		t.Context(),
		[]string{"users:read", "users:update", "users:absence"},
		groupless(),
		staffCtx(),
	)
	require.NoError(t, err)
	assert.True(t, ok, "staff must be allowed on a child without a group")
}

func TestCanManageStudentAbsence_StaffWithoutSupervisionAllowed(t *testing.T) {
	// #2329: the child's group no longer participates in the decision — any
	// verified staff member of the tenant may write any child's absence.
	ok, err := CanManageStudentAbsence(
		t.Context(),
		[]string{"users:read", "users:absence"},
		studentInGroup(42),
		staffCtx(),
	)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestCanManageStudentAbsence_AbsenceOnlyWithoutReadDenied(t *testing.T) {
	// users:absence is a write scope on top of the children a caller may see.
	// It unlocks no read surface — neither the child's list entry nor the
	// detail page — so on its own it must grant nothing at all rather than a
	// write for a child its holder cannot open.
	ok, err := CanManageStudentAbsence(
		t.Context(),
		[]string{"users:absence"},
		groupless(),
		staffCtx(),
	)
	assert.False(t, ok)
	assert.ErrorIs(t, err, ErrAbsenceReadRequired)
}

func TestCanManageStudentAbsence_UpdateHolderKeepsWorkingWithoutRead(t *testing.T) {
	// The mirror image: users:update is the permission that gated these writes
	// before #2232, and what it requires elsewhere is not this gate's business.
	// A holder of it stays authorized exactly as before.
	ok, err := CanManageStudentAbsence(
		t.Context(),
		[]string{"users:update", "users:absence"},
		studentInGroup(7),
		staffCtx(),
	)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestCanManageStudentAbsence_NonStaffDenied(t *testing.T) {
	// Guest/guardian accounts authenticate against the same portal; holding the
	// permission is not enough without a staff record in this tenant.
	ok, err := CanManageStudentAbsence(
		t.Context(),
		[]string{"users:read", "users:absence"},
		groupless(),
		&stubUserCtx{}, // no staff record
	)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only staff members can update")
}

func TestCanManageStudentAbsence_AdminAlwaysAllowed(t *testing.T) {
	ok, err := CanManageStudentAbsence(
		t.Context(),
		[]string{"admin:*"},
		groupless(),
		nil,
	)
	require.NoError(t, err)
	assert.True(t, ok)
}

func TestCanManageStudentAbsence_NilStudentDenied(t *testing.T) {
	ok, err := CanManageStudentAbsence(
		t.Context(),
		[]string{"users:read", "users:absence"},
		nil,
		staffCtx(),
	)
	assert.False(t, ok)
	require.Error(t, err)
}

// --- AbsenceWritableStudentFilter --------------------------------------

func TestAbsenceWritableStudentFilter_StaffCoversEveryStudent(t *testing.T) {
	filter := AbsenceWritableStudentFilter(
		t.Context(),
		[]string{"users:read", "users:absence"},
		staffCtx(),
	)
	assert.True(t, filter(groupless()))
	assert.True(t, filter(studentInGroup(99)))
	assert.False(t, filter(nil))
}

func TestAbsenceWritableStudentFilter_NonStaffSeesNothing(t *testing.T) {
	filter := AbsenceWritableStudentFilter(
		t.Context(),
		[]string{"users:read", "users:update"},
		&stubUserCtx{}, // no staff record
	)
	assert.False(t, filter(groupless()))
	assert.False(t, filter(studentInGroup(3)))
}

func TestAbsenceWritableStudentFilter_AbsenceOnlyWithoutReadSeesNothing(t *testing.T) {
	// The set form has to agree with CanManageStudentAbsence, including its
	// read requirement — otherwise the queue would list children the decision
	// then refuses.
	filter := AbsenceWritableStudentFilter(
		t.Context(),
		[]string{"users:absence"},
		staffCtx(),
	)
	assert.False(t, filter(groupless()))
	assert.False(t, filter(studentInGroup(3)))
}

func TestAbsenceWritableStudentFilter_AdminCoversEveryStudent(t *testing.T) {
	// The admin wildcard outranks the read prerequisite, exactly as in
	// CanManageStudentAbsence.
	filter := AbsenceWritableStudentFilter(t.Context(), []string{"admin:*"}, nil)
	assert.True(t, filter(groupless()))
	assert.True(t, filter(studentInGroup(3)))
	assert.False(t, filter(nil))
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
		// The coarse queue gate carries the same pair as the per-child gate: a
		// reviewer admitted here without users:read would open a queue that
		// AbsenceWritableStudentFilter answers empty forever.
		{"users:absence without users:read", []string{"users:absence"}, false},
		{"nothing", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, CanReviewExcusedAbsenceRequests(tc.permissions))
		})
	}
}
