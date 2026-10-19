package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

type GuardianRelationship care.GuardianRelationship
type GuardianLinkedChild care.GuardianLinkedChild
type GuardianRelationshipCallbacks struct {
	ByStudent      func(context.Context, int64) ([]*GuardianRelationship, error)
	ByProfile      func(context.Context, int64) ([]*GuardianRelationship, error)
	Lock           func(context.Context, int64, int64) (*GuardianRelationship, error)
	LinkedChildren func(context.Context, []int64) ([]*GuardianLinkedChild, error)
	Permission     func(context.Context, int64, int64, int64, string) (bool, error)
}
type StudentGuardianReads interface{ care.StudentGuardianReads }

func NewStudentGuardianReads(callbacks GuardianRelationshipCallbacks) StudentGuardianReads {
	return studentGuardianReads{callbacks}
}

type studentGuardianReads struct{ callbacks GuardianRelationshipCallbacks }

func nativeGuardianRelationships(rows []*GuardianRelationship, err error) ([]*care.GuardianRelationship, error) {
	if rows == nil {
		return nil, err
	}
	result := make([]*care.GuardianRelationship, len(rows))
	for i, row := range rows {
		result[i] = (*care.GuardianRelationship)(row)
	}
	return result, err
}
func (r studentGuardianReads) FindByStudentID(ctx context.Context, id int64) ([]*care.GuardianRelationship, error) {
	return nativeGuardianRelationships(r.callbacks.ByStudent(ctx, id))
}
func (r studentGuardianReads) FindByGuardianProfileID(ctx context.Context, id int64) ([]*care.GuardianRelationship, error) {
	return nativeGuardianRelationships(r.callbacks.ByProfile(ctx, id))
}
func (r studentGuardianReads) FindByStudentAndGuardianForUpdate(ctx context.Context, student, profile int64) (*care.GuardianRelationship, error) {
	value, err := r.callbacks.Lock(ctx, student, profile)
	return (*care.GuardianRelationship)(value), err
}
func (r studentGuardianReads) ListLinkedChildrenForGuardians(ctx context.Context, ids []int64) ([]*care.GuardianLinkedChild, error) {
	rows, err := r.callbacks.LinkedChildren(ctx, ids)
	if rows == nil {
		return nil, err
	}
	result := make([]*care.GuardianLinkedChild, len(rows))
	for i, row := range rows {
		result[i] = (*care.GuardianLinkedChild)(row)
	}
	return result, err
}
func (r studentGuardianReads) AccountHasStudentPermission(ctx context.Context, account, student, tenant int64, permission string) (bool, error) {
	return r.callbacks.Permission(ctx, account, student, tenant, permission)
}
func WithGuardianRelationshipStudent(value *GuardianRelationship, student *StudentSnapshot) *GuardianRelationship {
	if value != nil {
		value.Student = (*care.PortalStudent)(student)
	}
	return value
}
