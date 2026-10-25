package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// mappedError keeps Care Plan's message on the wire while answering
// errors.Is for the projection's sentinel, so the HTTP adapter classifies
// the outcome without the client reading a different text.
type MappedError struct {
	err      error
	sentinel error
}

func (e MappedError) Error() string { return e.err.Error() }
func (e MappedError) Unwrap() error { return e.err }
func (e MappedError) Is(target error) bool {
	return target == e.sentinel
}

// arrivalExceptionErrors pairs Care Plan's refusals of a class-wide arrival
// exception with the projection's sentinels.
var arrivalExceptionErrors = []struct {
	owner    error
	sentinel error
}{
	{owner: careplan.ErrClassArrivalExceptionPastDate, sentinel: classday.ErrArrivalExceptionPastDate},
	{owner: careplan.ErrClassArrivalExceptionWeekend, sentinel: classday.ErrArrivalExceptionWeekend},
	{owner: careplan.ErrClassArrivalExceptionClassNotFound, sentinel: classday.ErrArrivalExceptionClassNotFound},
	{owner: careplan.ErrClassArrivalExceptionNotFound, sentinel: classday.ErrArrivalExceptionNotFound},
	{owner: careplan.ErrClassArrivalExceptionNotConfigured, sentinel: ports.ErrClassArrivalExceptionsNotConfigured},
}

func MapArrivalExceptionError(err error) error {
	for _, pair := range arrivalExceptionErrors {
		if errors.Is(err, pair.owner) {
			return MappedError{err: err, sentinel: pair.sentinel}
		}
	}
	return err
}

// classArrivalExceptionBinding reads and writes Care Plan's class-wide
// arrival day exceptions.
type ClassArrivalExceptionBinding struct {
	Store careplan.ClassArrivalExceptions
}

func (b ClassArrivalExceptionBinding) ListClassArrivalExceptions(ctx context.Context, schoolClass string, from, to calendar.Date) ([]ClassArrivalException, error) {
	rows, err := b.Store.ListClassArrivalExceptions(ctx, schoolClass, from, to)
	if err != nil {
		return nil, MapArrivalExceptionError(err)
	}
	out := make([]ClassArrivalException, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			out = append(out, mapClassArrivalException(row))
		}
	}
	return out, nil
}
func (b ClassArrivalExceptionBinding) UpsertClassArrivalException(ctx context.Context, in ClassArrivalExceptionWrite) (ClassArrivalException, error) {
	row, err := b.Store.UpsertClassArrivalException(ctx, careplan.ClassArrivalExceptionInput{
		SchoolClass: in.SchoolClass, Date: in.Date, ArrivalTime: in.ArrivalTime, Reason: in.Reason, Origin: in.Origin,
	}, in.CreatedBy)
	if err != nil {
		return ClassArrivalException{}, MapArrivalExceptionError(err)
	}
	return mapClassArrivalException(row), nil
}
func (b ClassArrivalExceptionBinding) DeleteClassArrivalException(ctx context.Context, schoolClass string, date calendar.Date) error {
	return MapArrivalExceptionError(b.Store.DeleteClassArrivalException(ctx, schoolClass, date))
}
func mapClassArrivalException(row *careplan.ClassArrivalException) ClassArrivalException {
	return ClassArrivalException{
		SchoolClass: row.SchoolClass, Date: row.Date, ArrivalTime: row.ArrivalTime,
		Reason: row.Reason, CreatedAt: row.CreatedAt, Origin: row.Origin,
	}
}

// blockStartBinding answers the "Unterricht fällt aus" preset with the same
// error classification as the exception writes.
type BlockStartBinding struct{ Starts BlockStarts }

func (b BlockStartBinding) EarliestPlannedBlockStartForClass(ctx context.Context, schoolClass string, date calendar.Date) (string, error) {
	start, err := b.Starts.EarliestPlannedBlockStartForClass(ctx, schoolClass, date)
	if err != nil {
		return "", MapArrivalExceptionError(err)
	}
	return start, nil
}

// dayTimeBinding reads Care Plan's effective pickup and arrival times and
// the Abholart of single days.
// The pickup reader is embedded: its GetBulkDayDepartureModesForDate already
// answers the day report's Abholart read. Pickups is the same reader.
type DayTimeBinding struct {
	ClassDayPickupReader
	Pickups  ClassDayPickupReader
	Arrivals ArrivalTimeReader
}

func (b DayTimeBinding) EffectivePickups(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]EffectivePickup, error) {
	effective, err := b.Pickups.GetBulkEffectivePickupTimesForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]EffectivePickup, len(effective))
	for studentID, entry := range effective {
		if entry != nil {
			out[studentID] = EffectivePickup{
				PickupTime: entry.PickupTime, RegularPickupTime: entry.RegularPickupTime,
				IsException: entry.IsException, ChangedAt: entry.ChangedAt,
			}
		}
	}
	return out, nil
}
func (b DayTimeBinding) EffectiveArrivals(ctx context.Context, studentIDs []int64, date calendar.Date) (map[int64]EffectiveArrival, error) {
	effective, err := b.Arrivals.GetBulkEffectiveArrivalTimesForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]EffectiveArrival, len(effective))
	for studentID, entry := range effective {
		if entry != nil {
			out[studentID] = EffectiveArrival{
				ArrivalTime: entry.ArrivalTime, IsException: entry.IsException, ChangedAt: entry.ChangedAt,
			}
		}
	}
	return out, nil
}

// CallerBinding embeds the reader, whose GetMySchoolClasses already answers
// the Caller port; CurrentStaffID below refuses an account without a staff row.
type CallerBinding struct{ CallerReader }

// NewCallerBinding returns no caller port until the root provides its reader.
func NewCallerBinding(reader CallerReader) ports.Caller {
	if reader == nil {
		return nil
	}
	return CallerBinding{CallerReader: reader}
}

// CurrentStaffID resolves the caller's users.staff row (every school-portal account
// has one, EnsureSchoolIdentity). An account without one is refused with the
// sentinel: the entry would be attributed to nobody. Any other failure of the
// lookup is a server error, not a missing record — the caller must not be
// told to fix their account for a database outage.
func (b CallerBinding) CurrentStaffID(ctx context.Context) (int64, error) {
	staffID, found, err := b.CallerReader.CurrentStaffID(ctx)
	switch {
	case err != nil:
		return 0, err
	case !found:
		return 0, classday.ErrStaffRecordRequired
	}
	return staffID, nil
}
