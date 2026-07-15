package authorize

// Pure-logic tests for student_access.go. The package's existing tests use
// real DB fixtures; this file mocks the narrow user-context and settings
// interfaces so the branch coverage stays in-package (cross-package callers
// don't credit this file under SonarCloud's new-code metric).

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- stubs --------------------------------------------------------------

type stubUserCtx struct {
	staff        *users.Staff
	staffErr     error
	groups       []*education.Group
	groupsErr    error
	activeGroups []*active.Group
	activeErr    error
	staffCalls   int
	groupsCalls  int
	activeCalls  int
}

func (s *stubUserCtx) GetCurrentStaff(_ context.Context) (*users.Staff, error) {
	s.staffCalls++
	return s.staff, s.staffErr
}
func (s *stubUserCtx) GetMyGroups(_ context.Context) ([]*education.Group, error) {
	s.groupsCalls++
	return s.groups, s.groupsErr
}
func (s *stubUserCtx) GetMyActiveGroups(_ context.Context) ([]*active.Group, error) {
	s.activeCalls++
	return s.activeGroups, s.activeErr
}

type stubSettings struct {
	hasOverride bool
	hasErr      error
	value       string
	valueErr    error
}

func (s *stubSettings) HasTenantOverride(_ context.Context, _ string) (bool, error) {
	return s.hasOverride, s.hasErr
}
func (s *stubSettings) ResolveString(_ context.Context, _ string) (string, error) {
	return s.value, s.valueErr
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

// --- resolveStudentDataScope -------------------------------------------

func TestResolveStudentDataScope_NilSettingsReturnsRestrictiveDefault(t *testing.T) {
	got := resolveStudentDataScope(context.Background(), nil, nil)
	assert.Equal(t, configModel.StudentDataScopeGroupSupervisorsOnly, got,
		"missing settings must NEVER fall through to the permissive value")
}

func TestResolveStudentDataScope_OverrideCheckErrorFallsBack(t *testing.T) {
	got := resolveStudentDataScope(
		context.Background(),
		&stubSettings{hasErr: errors.New("settings unreachable")},
		slog.Default(),
	)
	assert.Equal(t, configModel.StudentDataScopeGroupSupervisorsOnly, got)
}

func TestResolveStudentDataScope_NoOverrideReturnsDefault(t *testing.T) {
	got := resolveStudentDataScope(
		context.Background(),
		&stubSettings{hasOverride: false},
		nil,
	)
	assert.Equal(t, configModel.StudentDataScopeGroupSupervisorsOnly, got)
}

func TestResolveStudentDataScope_ResolveErrorFallsBack(t *testing.T) {
	got := resolveStudentDataScope(
		context.Background(),
		&stubSettings{hasOverride: true, valueErr: errors.New("decode failure")},
		slog.Default(),
	)
	assert.Equal(t, configModel.StudentDataScopeGroupSupervisorsOnly, got)
}

func TestResolveStudentDataScope_EmptyValueFallsBack(t *testing.T) {
	got := resolveStudentDataScope(
		context.Background(),
		&stubSettings{hasOverride: true, value: ""},
		nil,
	)
	assert.Equal(t, configModel.StudentDataScopeGroupSupervisorsOnly, got)
}

func TestResolveStudentDataScope_HappyPath(t *testing.T) {
	got := resolveStudentDataScope(
		context.Background(),
		&stubSettings{hasOverride: true, value: configModel.StudentDataScopeAllStaff},
		nil,
	)
	assert.Equal(t, configModel.StudentDataScopeAllStaff, got)
}

// --- CanReadStudent ----------------------------------------------------

func TestCanReadStudent_NilStudent(t *testing.T) {
	assert.False(t, CanReadStudent(context.Background(), nil, nil, nil, nil, nil))
}

func TestCanReadStudent_AdminAlwaysWins(t *testing.T) {
	// No userCtx, no settings — admin permission alone is enough.
	got := CanReadStudent(
		context.Background(),
		[]string{"admin:*"},
		studentInGroup(1),
		nil,
		nil,
		nil,
	)
	assert.True(t, got)
}

func TestCanReadStudent_AllStaffScopeRequiresStaffRecord(t *testing.T) {
	settings := &stubSettings{hasOverride: true, value: configModel.StudentDataScopeAllStaff}

	// Guest / guardian — has users:read but is NOT a staff member.
	guest := &stubUserCtx{staffErr: errors.New("no staff record")}
	got := CanReadStudent(
		context.Background(),
		[]string{"users:read"},
		studentInGroup(99),
		guest,
		settings,
		nil,
	)
	assert.False(t, got, "all_staff scope must NOT promote non-staff callers")

	// Staff member — same scope, now permitted.
	staffer := &stubUserCtx{staff: &users.Staff{}}
	got = CanReadStudent(
		context.Background(),
		[]string{"users:read"},
		studentInGroup(99),
		staffer,
		settings,
		nil,
	)
	assert.True(t, got)
}

func TestCanReadStudent_NoGroupRequiresAdmin(t *testing.T) {
	uc := &stubUserCtx{staff: &users.Staff{}}
	got := CanReadStudent(
		context.Background(),
		[]string{"users:read"},
		&users.Student{}, // no GroupID
		uc,
		nil,
		nil,
	)
	assert.False(t, got, "students without a group are admin-only under the restrictive default")
}

func TestCanReadStudent_GroupSupervisorPasses(t *testing.T) {
	uc := &stubUserCtx{
		groups: []*education.Group{{Model: base.Model{ID: 42}}},
	}
	got := CanReadStudent(
		context.Background(),
		[]string{"users:read"},
		studentInGroup(42),
		uc,
		nil,
		nil,
	)
	assert.True(t, got)
}

func TestCanReadStudent_GroupsErrorDenies(t *testing.T) {
	uc := &stubUserCtx{groupsErr: errors.New("DB outage")}
	got := CanReadStudent(
		context.Background(),
		[]string{"users:read"},
		studentInGroup(42),
		uc,
		nil,
		nil,
	)
	assert.False(t, got, "errors on supervised-groups lookup must fail closed")
}

func TestCanReadStudent_NonSupervisedGroupDenies(t *testing.T) {
	uc := &stubUserCtx{groups: []*education.Group{{Model: base.Model{ID: 1}}, {Model: base.Model{ID: 2}}}}
	got := CanReadStudent(
		context.Background(),
		[]string{"users:read"},
		studentInGroup(99),
		uc,
		nil,
		nil,
	)
	assert.False(t, got)
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
	assert.Contains(t, err.Error(), "without assigned groups")
	assert.Contains(t, err.Error(), "update", "message must surface the operation verb")
}

func TestCanModifyStudent_GrouplessStudent(t *testing.T) {
	ok, err := CanModifyStudent(
		context.Background(),
		[]string{"users:update"},
		&users.Student{}, // GroupID nil
		&stubUserCtx{staff: &users.Staff{}},
		"delete",
	)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete")
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
	assert.Contains(t, err.Error(), "insufficient permissions")
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
		"update",
	)
	assert.False(t, ok)
	require.Error(t, err)
}

func TestCanModifyStudent_SupervisorPasses(t *testing.T) {
	uc := &stubUserCtx{
		staff:  &users.Staff{},
		groups: []*education.Group{{Model: base.Model{ID: 7}}},
	}
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

func TestCanModifyStudent_NonSupervisorDenies(t *testing.T) {
	uc := &stubUserCtx{
		staff:  &users.Staff{},
		groups: []*education.Group{{Model: base.Model{ID: 1}}},
	}
	ok, err := CanModifyStudent(
		context.Background(),
		[]string{"users:update"},
		studentInGroup(7),
		uc,
		"delete",
	)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "groups you supervise")
	assert.Contains(t, err.Error(), "delete")
}

func TestWritableStudentFilter(t *testing.T) {
	// Admin: every student writable, including a groupless one.
	admin := WritableStudentFilter(context.Background(), []string{"admin:*"}, nil)
	assert.True(t, admin(studentInGroup(7)))
	assert.True(t, admin(&users.Student{}))
	assert.True(t, admin(nil))

	// Nil user context (non-admin): nothing writable.
	none := WritableStudentFilter(context.Background(), []string{"users:update"}, nil)
	assert.False(t, none(studentInGroup(7)))

	// Supervisor of group 7 (education group): only students in group 7.
	sup := WritableStudentFilter(
		context.Background(),
		[]string{"users:update"},
		&stubUserCtx{staff: &users.Staff{}, groups: []*education.Group{{Model: base.Model{ID: 7}}}},
	)
	assert.True(t, sup(studentInGroup(7)))
	assert.False(t, sup(studentInGroup(8)))
	assert.False(t, sup(&users.Student{}), "a groupless student is admin-only")
	assert.False(t, sup(nil))
}

func TestCanUpdateStudent_Wrapper(t *testing.T) {
	// Happy path: supervises group 5.
	uc := &stubUserCtx{staff: &users.Staff{}, groups: []*education.Group{{Model: base.Model{ID: 5}}}}
	ok, err := CanUpdateStudent(context.Background(), []string{"users:update"}, studentInGroup(5), uc)
	assert.True(t, ok)
	require.NoError(t, err)

	// Sad path: does NOT supervise the student's group — wrapper must
	// surface the "update" verb in the error message.
	other := &stubUserCtx{staff: &users.Staff{}, groups: []*education.Group{{Model: base.Model{ID: 99}}}}
	ok, err = CanUpdateStudent(context.Background(), nil, studentInGroup(5), other)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "update")
}

func TestCanDeleteStudent_Wrapper(t *testing.T) {
	uc := &stubUserCtx{staff: &users.Staff{}, groups: []*education.Group{{Model: base.Model{ID: 5}}}}
	ok, err := CanDeleteStudent(context.Background(), []string{"users:update"}, studentInGroup(5), uc)
	assert.True(t, ok)
	require.NoError(t, err)

	other := &stubUserCtx{staff: &users.Staff{}, groups: []*education.Group{{Model: base.Model{ID: 99}}}}
	ok, err = CanDeleteStudent(context.Background(), nil, studentInGroup(5), other)
	assert.False(t, ok)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "delete")
}

// --- IsGroupSupervisor -------------------------------------------------

func TestIsGroupSupervisor_NilUserContext(t *testing.T) {
	assert.False(t, IsGroupSupervisor(context.Background(), 1, nil))
}

func TestIsGroupSupervisor_EducationGroupMatch(t *testing.T) {
	uc := &stubUserCtx{groups: []*education.Group{{Model: base.Model{ID: 11}}, {Model: base.Model{ID: 22}}}}
	assert.True(t, IsGroupSupervisor(context.Background(), 22, uc))
}

func TestIsGroupSupervisor_ActiveGroupTemplateMatch(t *testing.T) {
	// Persistent group lookup errs, but active-group fallback finds the
	// matching template ID — supervisor stands.
	uc := &stubUserCtx{
		groupsErr:    errors.New("DB error on education groups"),
		activeGroups: []*active.Group{{GroupID: ptr(int64(33))}},
	}
	assert.True(t, IsGroupSupervisor(context.Background(), 33, uc))
}

func TestIsGroupSupervisor_SpontaneousActiveGroupIgnored(t *testing.T) {
	// Spontaneous sessions (GroupID == nil) must NOT match a caller-supplied
	// template ID — otherwise any session would grant supervision.
	uc := &stubUserCtx{activeGroups: []*active.Group{{GroupID: nil}}}
	assert.False(t, IsGroupSupervisor(context.Background(), 33, uc))
}

func TestIsGroupSupervisor_NoMatch(t *testing.T) {
	uc := &stubUserCtx{
		groups:       []*education.Group{{Model: base.Model{ID: 1}}},
		activeGroups: []*active.Group{{GroupID: ptr(int64(2))}},
	}
	assert.False(t, IsGroupSupervisor(context.Background(), 99, uc))
}

func TestIsGroupSupervisor_ActiveGroupsErrorIgnored(t *testing.T) {
	// Education-group match wins even when active-group lookup would error.
	uc := &stubUserCtx{
		groups:    []*education.Group{{Model: base.Model{ID: 5}}},
		activeErr: errors.New("active group store offline"),
	}
	assert.True(t, IsGroupSupervisor(context.Background(), 5, uc))
}
