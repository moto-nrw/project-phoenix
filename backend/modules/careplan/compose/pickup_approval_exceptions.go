package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type PickupApprovalRecords interface {
	ListPickupExceptions(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error)
	CreatePickupException(context.Context, careplan.PickupException) (careplan.PickupException, error)
	UpdatePickupException(context.Context, careplan.PickupException) error
}

func NewPickupApprovalExceptions(records PickupApprovalRecords) (PickupApprovalExceptions, error) {
	if records == nil {
		return nil, errors.New("pickup approval exceptions: records are required")
	}
	return pickupApprovalExceptions{records}, nil
}

type pickupApprovalExceptions struct{ records PickupApprovalRecords }

func (a pickupApprovalExceptions) FindForDate(ctx context.Context, id int64, date calendar.Date) (*careplan.PickupException, error) {
	rows, err := a.records.ListPickupExceptions(ctx, careplan.StudentScheduleFilter{StudentIDs: []int64{id}, Date: careplan.Date(date)})
	if err != nil || len(rows) == 0 {
		return nil, err
	}
	return &rows[0], nil
}

func (a pickupApprovalExceptions) Create(ctx context.Context, value careplan.PickupException) (int64, error) {
	if err := value.Validate(); err != nil {
		return 0, err
	}
	row, err := a.records.CreatePickupException(ctx, value)
	return row.ID, err
}

func (a pickupApprovalExceptions) Update(ctx context.Context, value careplan.PickupException) error {
	if err := value.Validate(); err != nil {
		return err
	}
	return a.records.UpdatePickupException(ctx, value)
}

func (pickupApprovalExceptions) IsUniqueViolation(err error) bool {
	return postgres.IsPickupExceptionConflict(err)
}
