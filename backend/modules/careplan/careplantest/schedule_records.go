//go:build testhooks

package careplantest

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

type ScheduleRecords struct {
	StudentArrivalSchedule    ArrivalRecords
	StudentArrivalException   ArrivalExceptionRecords
	StudentPickupSchedule     PickupRecords
	StudentPickupException    PickupExceptionRecords
	CareScheduleChangeRequest RequestRecords
}

func NewScheduleRecords(records careplan.Capability) ScheduleRecords {
	return ScheduleRecords{StudentArrivalSchedule: ArrivalRecords{records}, StudentArrivalException: ArrivalExceptionRecords{records}, StudentPickupSchedule: PickupRecords{records}, StudentPickupException: PickupExceptionRecords{records}, CareScheduleChangeRequest: RequestRecords{records}}
}
func fixtureRecordID(raw any) (int64, error) {
	switch id := raw.(type) {
	case int64:
		return id, nil
	case int:
		return int64(id), nil
	case int32:
		return int64(id), nil
	default:
		return 0, fmt.Errorf("unsupported id type %T", raw)
	}
}
func fixtureRecordPointers[T any](values []T) []*T {
	if values == nil {
		return nil
	}
	rows := make([]*T, len(values))
	for i := range values {
		rows[i] = &values[i]
	}
	return rows
}

type ArrivalRecords struct {
	records careplan.ArrivalSchedulesQuery
}

func (r ArrivalRecords) FindByStudentID(ctx context.Context, id int64) ([]*careplan.ArrivalSchedule, error) {
	values, err := r.records.ListArrivalSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}})
	return fixtureRecordPointers(values), err
}

type ArrivalExceptionRecords struct {
	records careplan.ArrivalSchedulesQuery
}

func (r ArrivalExceptionRecords) FindByStudentID(ctx context.Context, id int64) ([]*careplan.ArrivalException, error) {
	values, err := r.records.ListArrivalExceptions(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}})
	return fixtureRecordPointers(values), err
}

type PickupRecords struct{ records careplan.Capability }

func (r PickupRecords) FindByStudentID(ctx context.Context, id int64) ([]*careplan.PickupSchedule, error) {
	values, err := r.records.ListPickupSchedules(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}})
	return fixtureRecordPointers(values), err
}
func (r PickupRecords) UpsertSchedule(ctx context.Context, row *careplan.PickupSchedule) error {
	if row == nil {
		return fmt.Errorf("schedule cannot be nil")
	}
	value, err := r.records.UpsertPickupSchedule(ctx, *row)
	if err == nil {
		*row = value
	}
	return err
}

type PickupExceptionRecords struct{ records careplan.Capability }

func (r PickupExceptionRecords) Create(ctx context.Context, row *careplan.PickupException) error {
	if row == nil {
		return fmt.Errorf("StudentPickupException cannot be nil or zero value")
	}
	value, err := r.records.CreatePickupException(ctx, *row)
	if err == nil {
		*row = value
	}
	return err
}
func (r PickupExceptionRecords) FindByID(ctx context.Context, raw any) (*careplan.PickupException, error) {
	id, err := fixtureRecordID(raw)
	if err != nil {
		return nil, err
	}
	value, err := r.records.FindPickupException(ctx, id, false)
	if err != nil {
		return nil, err
	}
	return &value, nil
}
func (r PickupExceptionRecords) FindByStudentID(ctx context.Context, id int64) ([]*careplan.PickupException, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}})
}
func (r PickupExceptionRecords) FindByStudentIDAndDate(ctx context.Context, id int64, date careplan.Date) (*careplan.PickupException, error) {
	values, err := r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, Date: date})
	if err != nil || len(values) == 0 {
		return nil, err
	}
	return values[0], nil
}
func (r PickupExceptionRecords) FindByStudentIDAndDateRange(ctx context.Context, id int64, from, to careplan.Date) ([]*careplan.PickupException, error) {
	return r.list(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, From: from, To: to})
}
func (r PickupExceptionRecords) list(ctx context.Context, filter careplan.StudentScheduleFilter) ([]*careplan.PickupException, error) {
	values, err := r.records.ListPickupExceptions(ctx, filter)
	return fixtureRecordPointers(values), err
}

type RequestRecords struct{ records careplan.Capability }

func (r RequestRecords) Create(ctx context.Context, row *careplan.CareScheduleChangeRequest) error {
	if row == nil {
		return fmt.Errorf("CareScheduleChangeRequest cannot be nil or zero value")
	}
	value, err := r.records.CreateCareScheduleRequest(ctx, *row)
	if err == nil {
		*row = value
	}
	return err
}
func (r RequestRecords) FindByID(ctx context.Context, raw any) (*careplan.CareScheduleChangeRequest, error) {
	id, err := fixtureRecordID(raw)
	if err != nil {
		return nil, err
	}
	value, err := r.records.FindCareScheduleRequest(ctx, id, false)
	if err != nil {
		return nil, err
	}
	return &value, nil
}
func (r RequestRecords) Decide(ctx context.Context, id int64, status string, reason *string, reviewedBy *int64, applied bool) error {
	return r.records.DecideCareScheduleRequest(ctx, careplan.CareScheduleRequestDecision{ID: id, Status: status, Reason: reason, ReviewedBy: reviewedBy, Applied: applied})
}
