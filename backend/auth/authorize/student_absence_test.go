package authorize

import (
	"context"
	"errors"
	"testing"
)

func TestStudentAbsenceRequiresReadForAbsenceOnlyRole(t *testing.T) {
	t.Parallel()
	student := &testStudent{present: true}
	staff := &testStaffContext{present: true}
	allowed, err := CanManageStudentAbsence(context.Background(), []string{usersAbsence}, student, staff)
	if allowed || !errors.Is(err, ErrAbsenceReadRequired) {
		t.Fatalf("absence-only result = %v, %v", allowed, err)
	}
	allowed, err = CanManageStudentAbsence(context.Background(), []string{usersAbsence, usersRead}, student, staff)
	if !allowed || err != nil {
		t.Fatalf("absence+read result = %v, %v", allowed, err)
	}
}

func TestAbsenceQueuePermissionMatchesWriteGate(t *testing.T) {
	t.Parallel()
	if CanReviewExcusedAbsenceRequests([]string{usersAbsence}) {
		t.Fatal("absence without read must not open review queue")
	}
	if !CanReviewExcusedAbsenceRequests([]string{usersAbsence, usersRead}) || !CanReviewExcusedAbsenceRequests([]string{usersUpdate}) {
		t.Fatal("valid absence authorities must open review queue")
	}
	filter := AbsenceWritableStudentFilter(context.Background(), []string{usersAbsence}, &testStaffContext{present: true})
	if filter(&testStudent{present: true}) {
		t.Fatal("queue filter must enforce the read prerequisite")
	}
}
