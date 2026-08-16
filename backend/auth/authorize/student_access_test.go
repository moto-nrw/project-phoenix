package authorize

// Pure-logic tests for student_access.go. The package's existing tests use
// real DB fixtures; this file mocks the narrow user-context interface so the
// branch coverage stays in-package (cross-package callers don't credit this
// file under SonarCloud's new-code metric).

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- stubs --------------------------------------------------------------

type stubUserCtx struct {
	staff      *users.Staff
	staffErr   error
	staffCalls int
}

func (s *stubUserCtx) GetCurrentStaff(_ context.Context) (*users.Staff, error) {
	s.staffCalls++
	return s.staff, s.staffErr
}

func ptr[T any](v T) *T { return &v }

func studentInGroup(groupID int64) *users.Student {
	return &users.Student{GroupID: ptr(groupID)}
}

// --- hasAdminPermissions -----------------------------------------------

func TestHasAdminPermissions(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		assert.False(t, HasAdminWildcard(nil))
		assert.False(t, HasAdminWildcard([]string{}))
	})
	t.Run("non-admin scopes only", func(t *testing.T) {
		assert.False(t, HasAdminWildcard([]string{"users:read", "users:update"}))
	})
	t.Run("admin:* grants", func(t *testing.T) {
		assert.True(t, HasAdminWildcard([]string{"users:read", "admin:*"}))
	})
	t.Run("*:* grants", func(t *testing.T) {
		assert.True(t, HasAdminWildcard([]string{"*:*"}))
	})
}

// --- CanReadStudent ----------------------------------------------------

func TestCanReadStudent_NilStudent(t *testing.T) {
	assert.False(t, CanReadStudent(context.Background(), nil, nil, nil))
	assert.False(t, CanReadStudent(context.Background(), []string{"admin:*"}, nil, nil),
		"a missing student is never readable, not even for an admin")
}

func TestCanReadStudent_AdminAlwaysWins(t *testing.T) {
	// No userCtx — the admin permission alone is enough.
	got := CanReadStudent(
		context.Background(),
		[]string{"admin:*"},
		studentInGroup(1),
		nil,
	)
	assert.True(t, got)
}

func TestCanReadStudent_StaffRecordRequired(t *testing.T) {
	// Guest / guardian — has users:read but is NOT a staff member.
	guest := &stubUserCtx{staffErr: errors.New("no staff record")}
	got := CanReadStudent(
		context.Background(),
		[]string{"users:read"},
		studentInGroup(99),
		guest,
	)
	assert.False(t, got, "users:read must NOT promote non-staff callers")

	// Staff member — permitted for any child of the tenant (#2329).
	staffer := &stubUserCtx{staff: &users.Staff{}}
	got = CanReadStudent(
		context.Background(),
		[]string{"users:read"},
		studentInGroup(99),
		staffer,
	)
	assert.True(t, got)
}

func TestCanReadStudent_GrouplessStudentReadableByStaff(t *testing.T) {
	// #2329: a child without a group is an ordinary child, not an admin-only one.
	uc := &stubUserCtx{staff: &users.Staff{}}
	got := CanReadStudent(
		context.Background(),
		[]string{"users:read"},
		&users.Student{}, // no GroupID
		uc,
	)
	assert.True(t, got)
}

func TestCanReadStudent_NilUserContextDenies(t *testing.T) {
	assert.False(t, CanReadStudent(
		context.Background(),
		[]string{"users:read"},
		studentInGroup(42),
		nil,
	), "without a user context the staff record cannot be verified — fail closed")
}

func TestCanReadStudent_StaffLookupErrorDenies(t *testing.T) {
	uc := &stubUserCtx{staffErr: errors.New("DB outage")}
	got := CanReadStudent(
		context.Background(),
		[]string{"users:read"},
		studentInGroup(42),
		uc,
	)
	assert.False(t, got, "errors on the staff lookup must fail closed")
}

// --- CanModifyStudent --------------------------------------------------

func TestCanModifyStudent_AdminWithNoStudent(t *testing.T) {
	ok, err := CanModifyStudent(context.Background(), []string{"admin:*"}, nil, nil, "update")
	assert.True(t, ok)
	require.NoError(t, err)
}

func TestCanModifyStudent_NilStudentDenies(t *testing.T) {
	ok, err := CanModifyStudent(context.Background(), []string{"users:update"}, nil, nil, "update")
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient permissions")
	assert.Contains(t, err.Error(), "update", "message must surface the operation verb")
}

func TestCanModifyStudent_GrouplessStudentWritableByStaff(t *testing.T) {
	// #2329: group membership no longer participates in the decision.
	ok, err := CanModifyStudent(
		context.Background(),
		[]string{"users:update"},
		&users.Student{}, // GroupID nil
		&stubUserCtx{staff: &users.Staff{}},
		"delete",
	)
	assert.True(t, ok)
	require.NoError(t, err)
}

func TestCanModifyStudent_NilUserContextDenies(t *testing.T) {
	ok, err := CanModifyStudent(
		context.Background(),
		[]string{"users:update"},
		studentInGroup(7),
		nil,
		"update",
	)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only staff members can update")
}

func TestCanModifyStudent_StaffLookupErrorDenies(t *testing.T) {
	uc := &stubUserCtx{staffErr: errors.New("DB outage")}
	ok, err := CanModifyStudent(
		context.Background(),
		[]string{"users:update"},
		studentInGroup(7),
		uc,
		"update",
	)
	assert.False(t, ok)
	require.Error(t, err)
}

func TestCanModifyStudent_StaffNilDenies(t *testing.T) {
	uc := &stubUserCtx{staff: nil} // err nil, but no staff record
	ok, err := CanModifyStudent(
		context.Background(),
		[]string{"users:update"},
		studentInGroup(7),
		uc,
		"delete",
	)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only staff members can delete")
}

func TestCanModifyStudent_StaffWithoutSupervisionPasses(t *testing.T) {
	// #2329: any verified staff member of the tenant may write any child; the
	// route-level permission decides WHICH writes they reach.
	uc := &stubUserCtx{staff: &users.Staff{}}
	ok, err := CanModifyStudent(
		context.Background(),
		[]string{"users:update"},
		studentInGroup(7),
		uc,
		"update",
	)
	assert.True(t, ok)
	require.NoError(t, err)
}

func TestWritableStudentFilter(t *testing.T) {
	// Admin: every student writable, including a groupless one.
	admin := WritableStudentFilter(context.Background(), []string{"admin:*"}, nil)
	assert.True(t, admin(studentInGroup(7)))
	assert.True(t, admin(&users.Student{}))
	assert.False(t, admin(nil))

	// Nil user context (non-admin): nothing writable.
	none := WritableStudentFilter(context.Background(), []string{"users:update"}, nil)
	assert.False(t, none(studentInGroup(7)))

	// Verified staff: every child of the tenant, resolved once.
	uc := &stubUserCtx{staff: &users.Staff{}}
	staffFilter := WritableStudentFilter(context.Background(), []string{"users:update"}, uc)
	assert.True(t, staffFilter(studentInGroup(7)))
	assert.True(t, staffFilter(studentInGroup(8)))
	assert.True(t, staffFilter(&users.Student{}))
	assert.False(t, staffFilter(nil))
	assert.Equal(t, 1, uc.staffCalls, "the staff record must be resolved once, not per student")
}

func TestCanUpdateStudent_Wrapper(t *testing.T) {
	uc := &stubUserCtx{staff: &users.Staff{}}
	ok, err := CanUpdateStudent(context.Background(), []string{"users:update"}, studentInGroup(5), uc)
	assert.True(t, ok)
	require.NoError(t, err)

	// Sad path: no staff record — the wrapper must surface the "update" verb.
	guest := &stubUserCtx{}
	ok, err = CanUpdateStudent(context.Background(), nil, studentInGroup(5), guest)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update")
}

func TestCanDeleteStudent_Wrapper(t *testing.T) {
	uc := &stubUserCtx{staff: &users.Staff{}}
	ok, err := CanDeleteStudent(context.Background(), []string{"users:update"}, studentInGroup(5), uc)
	assert.True(t, ok)
	require.NoError(t, err)

	guest := &stubUserCtx{}
	ok, err = CanDeleteStudent(context.Background(), nil, studentInGroup(5), guest)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete")
}
