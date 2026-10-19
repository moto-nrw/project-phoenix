package parentportal

import (
	"context"
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

type DataRequestSnapshot care.StudentDataChangeRequest
type DataRequestReadCallbacks struct {
	Find            func(context.Context, int64, bool) (*DataRequestSnapshot, error)
	Pending         func(context.Context, int64, string, string) (bool, error)
	List            func(context.Context, int64, []string, int, bool) ([]*DataRequestSnapshot, error)
	NotFoundError   func() error
	NotPendingError func() error
}
type DataRequestReads interface{ care.DataRequestReads }

func NewDataRequestReads(callbacks DataRequestReadCallbacks) DataRequestReads {
	return dataRequestReads{callbacks}
}

type dataRequestReads struct{ callbacks DataRequestReadCallbacks }

func (r dataRequestReads) FindByID(ctx context.Context, id int64) (*care.StudentDataChangeRequest, error) {
	value, err := r.callbacks.Find(ctx, id, false)
	return (*care.StudentDataChangeRequest)(value), err
}
func (r dataRequestReads) FindByIDForUpdate(ctx context.Context, id int64) (*care.StudentDataChangeRequest, error) {
	value, err := r.callbacks.Find(ctx, id, true)
	return (*care.StudentDataChangeRequest)(value), err
}
func (r dataRequestReads) HasPendingForField(ctx context.Context, id int64, target, field string) (bool, error) {
	return r.callbacks.Pending(ctx, id, target, field)
}
func nativeDataRequestRows(rows []*DataRequestSnapshot, err error) ([]*care.StudentDataChangeRequest, error) {
	if rows == nil {
		return nil, err
	}
	result := make([]*care.StudentDataChangeRequest, len(rows))
	for i, row := range rows {
		result[i] = (*care.StudentDataChangeRequest)(row)
	}
	return result, err
}
func (r dataRequestReads) ListByStudent(ctx context.Context, id int64, statuses []string, limit int) ([]*care.StudentDataChangeRequest, error) {
	return nativeDataRequestRows(r.callbacks.List(ctx, id, statuses, limit, false))
}
func (r dataRequestReads) ListParentVisibleByStudent(ctx context.Context, id int64, limit int) ([]*care.StudentDataChangeRequest, error) {
	return nativeDataRequestRows(r.callbacks.List(ctx, id, nil, limit, true))
}
func (r dataRequestReads) NotFoundError() error   { return r.callbacks.NotFoundError() }
func (r dataRequestReads) NotPendingError() error { return r.callbacks.NotPendingError() }

type DataRequestCommandCallbacks struct {
	Create func(context.Context, DataRequestSnapshot) (DataRequestSnapshot, error)
	Update func(context.Context, int64, json.RawMessage) error
}
type DataRequestCommands interface {
	care.StudentDataRequestCommands
}

func NewDataRequestCommands(callbacks DataRequestCommandCallbacks) DataRequestCommands {
	return dataRequestCommands{callbacks}
}

type dataRequestCommands struct{ callbacks DataRequestCommandCallbacks }

func (c dataRequestCommands) CreateStudentDataRequest(ctx context.Context, value care.StudentDataChangeRequest) (care.StudentDataChangeRequest, error) {
	created, err := c.callbacks.Create(ctx, DataRequestSnapshot(value))
	return care.StudentDataChangeRequest(created), err
}
func (c dataRequestCommands) UpdatePendingStudentDataRequest(ctx context.Context, id int64, value json.RawMessage) error {
	return c.callbacks.Update(ctx, id, value)
}
