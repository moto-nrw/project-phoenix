package authorize

// Pure-logic tests for ResolveStudentReadScope (student_read_scope.go), the
// list-level counterpart to CanReadStudent. Reuses the stubUserCtx double
// defined in student_access_test.go (same package) so the branch coverage
// stays in-package.

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/moto-nrw/project-phoenix/models/users"
)

func TestResolveStudentReadScope_AdminSeesAll(t *testing.T) {
	// Admin permission short-circuits before any userCtx lookup.
	assert.True(t, ResolveStudentReadScope(context.Background(), []string{"admin:*"}, nil))
}

func TestResolveStudentReadScope_StaffSeesAll(t *testing.T) {
	// #2329: a verified staff record grants tenant-wide read.
	uc := &stubUserCtx{staff: &users.Staff{}}
	assert.True(t, ResolveStudentReadScope(context.Background(), []string{"users:read"}, uc))
}

func TestResolveStudentReadScope_NonStaffReadsNothing(t *testing.T) {
	// A guardian/guest holding users:read has no staff record and is NOT
	// promoted to tenant-wide read.
	uc := &stubUserCtx{staffErr: errors.New("no staff record")}
	assert.False(t, ResolveStudentReadScope(context.Background(), []string{"users:read"}, uc))
}

func TestResolveStudentReadScope_NilUserContextReadsNothing(t *testing.T) {
	assert.False(t, ResolveStudentReadScope(context.Background(), []string{"users:read"}, nil))
}

func TestResolveStudentReadScope_StaffLookupErrorReadsNothing(t *testing.T) {
	uc := &stubUserCtx{staffErr: errors.New("DB outage")}
	assert.False(t, ResolveStudentReadScope(context.Background(), []string{"users:read"}, uc),
		"a staff lookup error must fail closed to no access")
}
