package api

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// pickupExceptionDirectory serves the Care Plan pickup exceptions and status
// days in Timetable's projection vocabulary, for the partial-absence preview
// and Student Presence's attendance rules (#4010). Care Plan's retained
// builders keep a private copy in services until #4011 moves them.
type pickupExceptionDirectory struct {
	query carePlanCompose.ExceptionQueries
}

func (d pickupExceptionDirectory) FindPickupException(ctx context.Context, id int64) (*timetableCompose.PickupExceptionProjection, error) {
	value, err := d.query.FindPickupException(ctx, id, false)
	if errors.Is(err, careplan.ErrStudentScheduleNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &timetableCompose.PickupExceptionProjection{ID: value.ID, StudentID: value.StudentID, ExceptionDate: value.ExceptionDate.String(), ExcusedFrom: value.ExcusedFrom, ExcusedAuto: value.ExcusedAuto}, nil
}

func (d pickupExceptionDirectory) ListPickupExceptions(ctx context.Context, filter timetableCompose.PickupExceptionFilter) ([]timetableCompose.PickupExceptionProjection, error) {
	ownerFilter := careplan.StudentScheduleFilter{IDs: filter.IDs, StudentIDs: filter.StudentIDs}
	if filter.Date != "" {
		ownerFilter.Date = careplan.Date(filter.Date)
	}
	if filter.From != "" {
		ownerFilter.From = careplan.Date(filter.From)
	}
	values, err := d.query.ListPickupExceptions(ctx, ownerFilter)
	if err != nil {
		return nil, err
	}
	result := make([]timetableCompose.PickupExceptionProjection, 0, len(values))
	for _, value := range values {
		result = append(result, timetableCompose.PickupExceptionProjection{ID: value.ID, StudentID: value.StudentID, ExceptionDate: value.ExceptionDate.String(), ExcusedFrom: value.ExcusedFrom, ExcusedAuto: value.ExcusedAuto})
	}
	return result, nil
}

func (d pickupExceptionDirectory) FindStudentStatusDay(ctx context.Context, id int64, activeOnly bool) (*timetableCompose.StudentStatusDayProjection, error) {
	value, err := d.query.FindStudentStatusDay(ctx, id, activeOnly)
	if errors.Is(err, careplan.ErrStudentStatusDayNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &timetableCompose.StudentStatusDayProjection{ID: value.ID, StudentID: value.StudentID, Date: value.Date.String(), Status: value.Status}, nil
}

func (d pickupExceptionDirectory) ListStudentStatusDays(ctx context.Context, filter timetableCompose.StudentStatusDayFilter) ([]timetableCompose.StudentStatusDayProjection, error) {
	values, err := d.query.ListStudentStatusDays(ctx, careplan.StudentStatusDayFilter{
		IDs: filter.IDs, StudentIDs: filter.StudentIDs, Date: careplan.Date(filter.Date),
		From: careplan.Date(filter.From), ActiveOnly: filter.ActiveOnly, LatestOnly: filter.LatestOnly,
	})
	if err != nil {
		return nil, err
	}
	result := make([]timetableCompose.StudentStatusDayProjection, 0, len(values))
	for _, value := range values {
		result = append(result, timetableCompose.StudentStatusDayProjection{ID: value.ID, StudentID: value.StudentID, Date: value.Date.String(), Status: value.Status})
	}
	return result, nil
}
