//go:build testhooks

package api

import (
	"context"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// CreateParentPickupExceptionFixture keeps the native owner's validation,
// error mapping and persisted row fields for parent workflow fixtures.
func CreateParentPickupExceptionFixture(ctx context.Context, owner careplan.Capability, row *careplan.PickupException) error {
	if err := row.Validate(); err != nil {
		return err
	}
	rows := owners.NewCarePlanRows(owner)
	value, err := rows.Records.CreatePickupException(ctx, owners.CarePickupExceptionToRows(*row))
	if err != nil {
		return rows.ScheduleError("create", err)
	}
	*row = owners.CarePickupExceptionFromRows(value)
	return nil
}

// CreateParentArrivalExceptionFixture keeps the native owner's validation,
// error mapping and persisted row fields for parent workflow fixtures.
func CreateParentArrivalExceptionFixture(ctx context.Context, owner careplan.Capability, row *careplan.ArrivalException) error {
	if err := row.Validate(); err != nil {
		return err
	}
	rows := owners.NewCarePlanRows(owner)
	value, err := rows.Records.CreateArrivalException(ctx, owners.CareArrivalExceptionToRows(*row))
	if err != nil {
		return rows.ScheduleError("create", err)
	}
	*row = owners.CareArrivalExceptionFromRows(value)
	return nil
}

// CreateParentPickupScheduleFixture keeps the native owner's validation,
// error mapping and persisted row fields for parent workflow fixtures.
func CreateParentPickupScheduleFixture(ctx context.Context, owner careplan.Capability, row *careplan.PickupSchedule) error {
	if err := row.Validate(); err != nil {
		return err
	}
	rows := owners.NewCarePlanRows(owner)
	value, err := rows.Records.CreatePickupSchedule(ctx, owners.CarePickupScheduleToRows(*row))
	if err != nil {
		return rows.ScheduleError("create", err)
	}
	*row = owners.CarePickupScheduleFromRows(value)
	return nil
}

// FindParentPickupExceptionFixture uses the same filtered owner read and
// retained diagnostic as the former schedule fixture adapter.
func FindParentPickupExceptionFixture(ctx context.Context, owner careplan.Capability, studentID int64, date calendar.Date) (*careplan.PickupException, error) {
	rows := owners.NewCarePlanRows(owner)
	values, err := rows.Records.ListPickupExceptions(ctx, timetablePorts.CareScheduleFilter{StudentIDs: []int64{studentID}, Date: timetablePorts.CareDate(date)})
	if err != nil {
		return nil, rows.ScheduleError("find by student id and date", err)
	}
	if len(values) == 0 {
		return nil, nil
	}
	value := owners.CarePickupExceptionFromRows(values[0])
	return &value, nil
}
