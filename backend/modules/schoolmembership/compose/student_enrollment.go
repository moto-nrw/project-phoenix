package compose

import "context"

func (e engine) TransitionStudentStatus(ctx context.Context, id int64, expected, next string) (bool, error) {
	changed, err := e.service.TransitionStudentStatus(ctx, id, expected, next)
	return changed, mapError(err)
}

func (e engine) SetStudentStatus(ctx context.Context, id int64, status string) (bool, error) {
	changed, err := e.service.SetStudentStatus(ctx, id, status)
	return changed, mapError(err)
}

func (e engine) LockStudentClassWrites(ctx context.Context, exclusive bool) error {
	return e.service.LockStudentClassWrites(ctx, exclusive)
}

func (e engine) ResumeStudentCare(ctx context.Context, id int64, from, status, on string) (bool, error) {
	changed, err := e.service.ResumeStudentCare(ctx, id, from, status, on)
	return changed, mapError(err)
}

func (e engine) EndStudentCare(ctx context.Context, ids []int64, until string) (int64, error) {
	return e.service.EndStudentCare(ctx, ids, until)
}

func (e engine) ReactivateStudents(ctx context.Context, ids []int64, status string, enforceChildQuota bool) ([]int64, error) {
	changed, err := e.service.ReactivateStudents(ctx, ids, status, enforceChildQuota)
	return changed, mapError(err)
}

func (e engine) GraduateStudents(ctx context.Context, ids []int64) (int64, error) {
	return e.service.GraduateStudents(ctx, ids)
}

func (e engine) ChangeStudentClass(ctx context.Context, ids []int64, from, to string) (int64, error) {
	return e.service.ChangeStudentClass(ctx, ids, from, to)
}
