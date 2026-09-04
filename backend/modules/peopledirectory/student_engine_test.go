package peopledirectory_test

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// The student half of the recording engine: every call is counted and its
// normalized arguments are kept so the module's validation can be asserted
// without a database.
type studentCall struct {
	ids     []int64
	classes []string
	from    string
	to      string
	status  string
}

func (e *recordingEngine) ListStudentsByIDs(_ context.Context, ids []int64) ([]peopledirectory.Student, error) {
	e.calls++
	e.student = studentCall{ids: ids}
	return nil, nil
}

func (e *recordingEngine) ListStudentNamesByIDs(_ context.Context, ids []int64) ([]peopledirectory.StudentName, error) {
	e.calls++
	e.student = studentCall{ids: ids}
	return nil, nil
}

func (e *recordingEngine) ListStudentsAcrossTenantsByIDs(_ context.Context, ids []int64) ([]peopledirectory.Student, error) {
	e.calls++
	e.student = studentCall{ids: ids}
	e.across = true
	return nil, nil
}

func (e *recordingEngine) ListStudentsByClasses(_ context.Context, classes []string) ([]peopledirectory.Student, error) {
	e.calls++
	e.student = studentCall{classes: classes}
	return nil, nil
}

func (e *recordingEngine) ListEnrolledStudents(context.Context) ([]peopledirectory.Student, error) {
	e.calls++
	return nil, nil
}

func (e *recordingEngine) ListSchoolClasses(context.Context) ([]string, error) {
	e.calls++
	return nil, nil
}

func (e *recordingEngine) LockStudent(_ context.Context, id int64) error {
	e.calls++
	e.student = studentCall{ids: []int64{id}}
	return nil
}

func (e *recordingEngine) PromoteStudents(_ context.Context, ids []int64, from, to string) (int64, error) {
	e.calls++
	e.student = studentCall{ids: ids, from: from, to: to}
	return int64(len(ids)), nil
}

func (e *recordingEngine) RevertStudentClass(_ context.Context, id int64, from, to string) (int64, error) {
	e.calls++
	e.student = studentCall{ids: []int64{id}, from: from, to: to}
	return 1, nil
}

func (e *recordingEngine) GraduateStudentsByClasses(_ context.Context, classes []string) (int64, error) {
	e.calls++
	e.student = studentCall{classes: classes}
	return int64(len(classes)), nil
}

func (e *recordingEngine) GraduateStudents(_ context.Context, ids []int64) (int64, error) {
	e.calls++
	e.student = studentCall{ids: ids}
	return int64(len(ids)), nil
}

func (e *recordingEngine) ReactivateStudents(_ context.Context, ids []int64, status string) ([]int64, error) {
	e.calls++
	e.student = studentCall{ids: ids, status: status}
	return ids, nil
}
