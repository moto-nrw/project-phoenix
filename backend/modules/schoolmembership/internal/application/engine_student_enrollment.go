package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

func (e Engine) TransitionStudentStatus(ctx context.Context, id int64, expected, next string) (bool, error) {
	changed, err := e.service.TransitionStudentStatus(ctx, id, expected, next)
	return changed, MapError(err)
}

func (e Engine) SetStudentStatus(ctx context.Context, id int64, status string) (bool, error) {
	changed, err := e.service.SetStudentStatus(ctx, id, status)
	return changed, MapError(err)
}

func (e Engine) LockStudentClassWrites(ctx context.Context, exclusive bool) error {
	return e.service.LockStudentClassWrites(ctx, exclusive)
}

func (e Engine) ResumeStudentCare(ctx context.Context, id int64, from, status, on string) (bool, error) {
	changed, err := e.service.ResumeStudentCare(ctx, id, from, status, on)
	return changed, MapError(err)
}

func (e Engine) EndStudentCare(ctx context.Context, ids []int64, until string) (int64, error) {
	return e.service.EndStudentCare(ctx, ids, until)
}

func (e Engine) ReactivateStudents(ctx context.Context, ids []int64, status string, enforceChildQuota bool) ([]int64, error) {
	changed, err := e.service.ReactivateStudents(ctx, ids, status, enforceChildQuota)
	return changed, MapError(err)
}

func (e Engine) ChildQuotaUsage(ctx context.Context) (schoolmembership.ChildQuotaUsage, bool, error) {
	usage, limited, err := e.service.ChildQuotaUsage(ctx)
	return schoolmembership.ChildQuotaUsage(usage), limited, MapError(err)
}

func (e Engine) GraduateStudents(ctx context.Context, ids []int64) (int64, error) {
	return e.service.GraduateStudents(ctx, ids)
}

func (e Engine) ChangeStudentClass(ctx context.Context, ids []int64, from, to string) (int64, error) {
	return e.service.ChangeStudentClass(ctx, ids, from, to)
}
