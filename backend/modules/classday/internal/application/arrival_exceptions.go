package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/classday"
)

// arrivalExceptions is the one write seam of the class-day view (#2970): a
// Lehrkraft sets the class-wide arrival day exception of #2962 for an
// assigned class through "moto schule". The rows are the ones the OGS writes
// in the Kindersuche and Care Plan is the only writer — nothing here touches
// per-child rows (ADR 0005). The seam adds what the school portal needs on
// top: the school's setting, the origin stamp, the "Unterricht fällt aus"
// preset, and the live-view announcement the OGS handler emits itself.
type arrivalExceptions struct {
	schedule    ClassArrivalExceptions
	writeScope  ArrivalWriteScope
	blockStarts BlockStarts
	announcer   ArrivalScheduleAnnouncer
}

// MayWriteArrivalExceptions applies operations.school_portal_write_scope.
func (a arrivalExceptions) MayWriteArrivalExceptions(ctx context.Context) (bool, error) {
	if a.schedule == nil {
		return false, classday.ErrArrivalExceptionsNotConfigured
	}
	if a.writeScope == nil {
		return false, errors.New("class day arrival exceptions: settings not configured")
	}
	may, err := a.writeScope.SchoolMayWriteClassArrivalExceptions(ctx)
	if err != nil {
		return false, fmt.Errorf("class day arrival exceptions: resolve school portal write scope: %w", err)
	}
	return may, nil
}

// ArrivalExceptions returns the exceptions of one class with
// from <= date <= to.
func (a arrivalExceptions) ArrivalExceptions(ctx context.Context, schoolClass string, from, to classday.Date) ([]classday.ArrivalException, error) {
	if a.schedule == nil {
		return nil, classday.ErrArrivalExceptionsNotConfigured
	}
	start, err := parseDate(from)
	if err != nil {
		return nil, err
	}
	end, err := parseDate(to)
	if err != nil {
		return nil, err
	}
	rows, err := a.schedule.ListClassArrivalExceptions(ctx, schoolClass, start, end)
	if err != nil {
		return nil, err
	}
	out := make([]classday.ArrivalException, 0, len(rows))
	for _, row := range rows {
		out = append(out, arrivalException(row))
	}
	return out, nil
}

// SetArrivalException stores the exception of one class and date as entered
// by the school and tells the OGS live views to refetch once it committed.
func (a arrivalExceptions) SetArrivalException(ctx context.Context, in classday.ArrivalExceptionWrite) (*classday.ArrivalException, error) {
	if a.schedule == nil {
		return nil, classday.ErrArrivalExceptionsNotConfigured
	}
	day, err := parseDate(in.Date)
	if err != nil {
		return nil, err
	}
	row, err := a.schedule.UpsertClassArrivalException(ctx, ClassArrivalExceptionWrite{
		SchoolClass: in.SchoolClass,
		Date:        day,
		ArrivalTime: in.ArrivalTime,
		Reason:      in.Reason,
		Origin:      classday.ArrivalExceptionOriginSchool,
		CreatedBy:   in.CreatedBy,
	})
	if err != nil {
		return nil, err
	}
	a.announce(ctx)
	entry := arrivalException(row)
	return &entry, nil
}

// ClearArrivalException deletes the exception of one class and date.
func (a arrivalExceptions) ClearArrivalException(ctx context.Context, schoolClass string, date classday.Date) error {
	if a.schedule == nil {
		return classday.ErrArrivalExceptionsNotConfigured
	}
	day, err := parseDate(date)
	if err != nil {
		return err
	}
	if err := a.schedule.DeleteClassArrivalException(ctx, schoolClass, day); err != nil {
		return err
	}
	a.announce(ctx)
	return nil
}

// EarliestBlockStart returns the "HH:MM" start of the first block of the
// date that addresses the class, "" when there is none.
func (a arrivalExceptions) EarliestBlockStart(ctx context.Context, schoolClass string, date classday.Date) (string, error) {
	if a.schedule == nil {
		return "", classday.ErrArrivalExceptionsNotConfigured
	}
	day, err := parseDate(date)
	if err != nil {
		return "", err
	}
	if a.blockStarts == nil {
		return "", errors.New("class day arrival exceptions: block starts not configured")
	}
	return a.blockStarts.EarliestPlannedBlockStartForClass(ctx, schoolClass, day)
}

// announce tells the OGS live views that a class-wide arrival exception
// changed, so Aufsicht and Meine Gruppe refetch once the write committed.
func (a arrivalExceptions) announce(ctx context.Context) {
	if a.announcer != nil {
		a.announcer.AnnounceArrivalScheduleChange(ctx)
	}
}

func arrivalException(row ClassArrivalException) classday.ArrivalException {
	origin := strings.TrimSpace(row.Origin)
	if origin == "" {
		origin = classday.ArrivalExceptionOriginOGS
	}
	return classday.ArrivalException{
		SchoolClass: row.SchoolClass,
		Date:        classday.Date(row.Date.String()),
		ArrivalTime: row.ArrivalTime.Format("15:04"),
		Reason:      row.Reason,
		CreatedAt:   row.CreatedAt.Format(time.RFC3339),
		Origin:      origin,
	}
}
