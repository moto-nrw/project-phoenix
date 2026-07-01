package authorize

// Pure-logic tests for ResolveStudentReadScope (student_read_scope.go), the
// list-level counterpart to CanReadStudent. Reuses the stubUserCtx / stubSettings
// doubles defined in student_access_test.go (same package) so the branch coverage
// stays in-package.

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// allStaffSettings resolves gdpr.student_data_scope to "all_staff".
func allStaffSettings() *stubSettings {
	return &stubSettings{hasOverride: true, value: configModel.StudentDataScopeAllStaff}
}

func TestResolveStudentReadScope_AdminSeesAll(t *testing.T) {
	// Admin permission short-circuits before any settings/userCtx lookup.
	all, groups := ResolveStudentReadScope(context.Background(), []string{"admin:*"}, nil, nil, nil)
	assert.True(t, all)
	assert.Nil(t, groups)
}

func TestResolveStudentReadScope_AllStaffScopePromotesStaff(t *testing.T) {
	// gdpr scope all_staff + a real staff record → the caller may read every child.
	uc := &stubUserCtx{staff: &users.Staff{}}
	all, groups := ResolveStudentReadScope(context.Background(), []string{"users:read"}, uc, allStaffSettings(), nil)
	assert.True(t, all)
	assert.Nil(t, groups)
}

func TestResolveStudentReadScope_AllStaffScopeButNotStaffFallsToGroups(t *testing.T) {
	// all_staff scope but the caller has no staff record (e.g. a guardian): they are
	// NOT promoted to tenant-wide read; they fall through to their supervised groups
	// (here none) so they may read nothing.
	uc := &stubUserCtx{staffErr: errors.New("no staff record")}
	all, groups := ResolveStudentReadScope(context.Background(), []string{"users:read"}, uc, allStaffSettings(), nil)
	assert.False(t, all)
	assert.Empty(t, groups, "a non-staff caller under all_staff scope reads only their (zero) groups")
}

func TestResolveStudentReadScope_NilUserContextReadsNothing(t *testing.T) {
	// Restrictive default scope, no user context to resolve groups → read nothing.
	all, groups := ResolveStudentReadScope(context.Background(), []string{"users:read"}, nil, nil, nil)
	assert.False(t, all)
	assert.Nil(t, groups)
}

func TestResolveStudentReadScope_GroupsErrorReadsNothing(t *testing.T) {
	uc := &stubUserCtx{groupsErr: errors.New("DB outage")}
	all, groups := ResolveStudentReadScope(context.Background(), []string{"users:read"}, uc, nil, nil)
	assert.False(t, all)
	assert.Empty(t, groups, "a supervised-groups lookup error must fail closed to no access")
}

func TestResolveStudentReadScope_ReturnsSupervisedGroupIDs(t *testing.T) {
	uc := &stubUserCtx{groups: []*education.Group{
		{Model: base.Model{ID: 7}},
		{Model: base.Model{ID: 11}},
	}}
	all, groups := ResolveStudentReadScope(context.Background(), []string{"users:read"}, uc, nil, nil)
	assert.False(t, all)
	assert.Equal(t, []int64{7, 11}, groups, "a group supervisor reads exactly their groups")
}
