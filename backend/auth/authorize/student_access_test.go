package authorize

import (
	"context"
	"errors"
	"testing"
)

type testStudent struct{ present bool }

func (s *testStudent) IsAuthorizationStudent() bool { return s != nil && s.present }

type testStaffContext struct {
	present bool
	err     error
	calls   int
}

func (s *testStaffContext) HasCurrentStaff(context.Context) (bool, error) {
	s.calls++
	return s.present, s.err
}

func TestStudentAccessFailsClosed(t *testing.T) {
	t.Parallel()
	student := &testStudent{present: true}
	staff := &testStaffContext{present: true}
	if !CanReadStudent(context.Background(), []string{"users:read"}, student, staff) {
		t.Fatal("verified staff must read an existing student")
	}
	if CanReadStudent(context.Background(), []string{"users:read"}, (*testStudent)(nil), staff) {
		t.Fatal("missing student must deny")
	}
	broken := &testStaffContext{err: errors.New("db")}
	if CanReadStudent(context.Background(), []string{"users:read"}, student, broken) {
		t.Fatal("staff lookup error must deny")
	}
	if ok, err := CanModifyStudent(context.Background(), nil, student, &testStaffContext{}, "update"); ok || err == nil {
		t.Fatal("non-staff modification must return a denial reason")
	}
}

func TestStudentAdminAndWritableFilter(t *testing.T) {
	t.Parallel()
	student := &testStudent{present: true}
	if ok, err := CanModifyStudent(context.Background(), []string{"admin:*"}, (*testStudent)(nil), nil, "update"); !ok || err != nil {
		t.Fatalf("admin wildcard must bypass row lookup: %v, %v", ok, err)
	}
	staff := &testStaffContext{present: true}
	writable := WritableStudentFilter(context.Background(), nil, staff)
	if !writable(student) || writable((*testStudent)(nil)) || staff.calls != 1 {
		t.Fatal("writable filter must resolve staff once and reject missing students")
	}
}
