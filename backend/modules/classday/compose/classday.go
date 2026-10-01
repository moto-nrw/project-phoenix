package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
)

// Ports the composition root binds for the school-portal capability.
type (
	DayRoster                = application.DayRoster
	DayRosterRow             = application.DayRosterRow
	DayRosterStudent         = application.DayRosterStudent
	DayRosters               = application.DayRosters
	StatusDayEntry           = application.StatusDayEntry
	StatusDays               = application.StatusDays
	Companions               = application.Companions
	SheetStudent             = application.SheetStudent
	SheetPerson              = application.SheetPerson
	SheetStudents            = application.SheetStudents
	EmergencyContactRow      = application.EmergencyContactRow
	EmergencyContacts        = application.EmergencyContacts
	AccessRecord             = application.AccessRecord
	AccessLog                = application.AccessLog
	BlockStarts              = application.BlockStarts
	ArrivalScheduleAnnouncer = application.ArrivalScheduleAnnouncer
)

// CallerReader is the slice of the user-context service the school portal
// resolves the caller with.
type CallerReader interface {
	GetMySchoolClasses(ctx context.Context) ([]string, error)
	// CurrentStaffID resolves the caller's staff member; found is false,
	// without an error, for a caller who is no staff member.
	CurrentStaffID(ctx context.Context) (staffID int64, found bool, err error)
}

// WriteScopeReader is the slice of the settings service the arrival
// exceptions resolve operations.school_portal_write_scope with.
type WriteScopeReader interface {
	ResolveString(ctx context.Context, key string) (string, error)
}

// ClassDayDependencies wires the school-portal capability: the day report
// over Enrollment's day roster and Care Plan's day facts, the supervision
// sheet, and the one write seam of moto schule (#2970). A nil
// ClassArrivalExceptions leaves the exception routes answering "not
// configured".
type ClassDayDependencies struct {
	Caller                 CallerReader
	Rosters                DayRosters
	StatusDays             StatusDays
	PickupTimes            PickupTimeReader
	ArrivalTimes           ArrivalTimeReader
	CareDays               CareDayResolver
	Companions             Companions
	Students               SheetStudents
	EmergencyContacts      EmergencyContacts
	AccessLog              AccessLog
	ClassArrivalExceptions careplan.ClassArrivalExceptions
	Settings               WriteScopeReader
	BlockStarts            BlockStarts
	Announcer              ArrivalScheduleAnnouncer
}

// NewClassDay composes the school-portal capability.
func NewClassDay(deps ClassDayDependencies) classday.ClassDay {
	var caller ports.Caller
	if deps.Caller != nil {
		caller = callerBinding{context: deps.Caller}
	}
	var writeScope application.ArrivalWriteScope
	if deps.Settings != nil {
		writeScope = writeScopeBinding{settings: deps.Settings}
	}
	app := application.ClassDayDependencies{
		Caller: caller, Rosters: deps.Rosters, StatusDays: deps.StatusDays,
		Companions: deps.Companions, Students: deps.Students, EmergencyContacts: deps.EmergencyContacts,
		AccessLog: deps.AccessLog, WriteScope: writeScope, Announcer: deps.Announcer,
	}
	if deps.PickupTimes != nil && deps.ArrivalTimes != nil {
		app.Times = dayTimeBinding{pickups: deps.PickupTimes, arrivals: deps.ArrivalTimes}
	}
	if deps.CareDays != nil {
		app.CareDays = careDayBinding{resolver: deps.CareDays}
	}
	if deps.ClassArrivalExceptions != nil {
		app.ClassArrivalExceptions = classArrivalExceptionBinding{store: deps.ClassArrivalExceptions}
	}
	if deps.BlockStarts != nil {
		app.BlockStarts = blockStartBinding{starts: deps.BlockStarts}
	}
	return application.NewClassDay(app)
}

// mappedError keeps Care Plan's message on the wire while answering
// errors.Is for the projection's sentinel, so the HTTP adapter classifies
// the outcome without the client reading a different text.
type mappedError struct {
	err      error
	sentinel error
}

func (e mappedError) Error() string { return e.err.Error() }
func (e mappedError) Unwrap() error { return e.err }
func (e mappedError) Is(target error) bool {
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

func mapArrivalExceptionError(err error) error {
	for _, pair := range arrivalExceptionErrors {
		if errors.Is(err, pair.owner) {
			return mappedError{err: err, sentinel: pair.sentinel}
		}
	}
	return err
}

// dayTimeBinding reads Care Plan's effective pickup and arrival times.
type dayTimeBinding struct {
	pickups  PickupTimeReader
	arrivals ArrivalTimeReader
}

func (b dayTimeBinding) EffectivePickups(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]application.EffectivePickup, error) {
	effective, err := b.pickups.GetBulkEffectivePickupTimesForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]application.EffectivePickup, len(effective))
	for studentID, entry := range effective {
		if entry != nil {
			out[studentID] = application.EffectivePickup{
				PickupTime: entry.PickupTime, RegularPickupTime: entry.RegularPickupTime,
				IsException: entry.IsException, ChangedAt: entry.ChangedAt,
			}
		}
	}
	return out, nil
}

func (b dayTimeBinding) EffectiveArrivals(ctx context.Context, studentIDs []int64, date timezone.Date) (map[int64]application.EffectiveArrival, error) {
	effective, err := b.arrivals.GetBulkEffectiveArrivalTimesForDate(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]application.EffectiveArrival, len(effective))
	for studentID, entry := range effective {
		if entry != nil {
			out[studentID] = application.EffectiveArrival{
				ArrivalTime: entry.ArrivalTime, IsException: entry.IsException, ChangedAt: entry.ChangedAt,
			}
		}
	}
	return out, nil
}

// classArrivalExceptionBinding reads and writes Care Plan's class-wide
// arrival day exceptions.
type classArrivalExceptionBinding struct {
	store careplan.ClassArrivalExceptions
}

func (b classArrivalExceptionBinding) ListClassArrivalExceptions(ctx context.Context, schoolClass string, from, to timezone.Date) ([]application.ClassArrivalException, error) {
	rows, err := b.store.ListClassArrivalExceptions(ctx, schoolClass, from, to)
	if err != nil {
		return nil, mapArrivalExceptionError(err)
	}
	out := make([]application.ClassArrivalException, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			out = append(out, classArrivalException(row))
		}
	}
	return out, nil
}

func (b classArrivalExceptionBinding) UpsertClassArrivalException(ctx context.Context, in application.ClassArrivalExceptionWrite) (application.ClassArrivalException, error) {
	row, err := b.store.UpsertClassArrivalException(ctx, careplan.ClassArrivalExceptionInput{
		SchoolClass: in.SchoolClass, Date: in.Date, ArrivalTime: in.ArrivalTime, Reason: in.Reason, Origin: in.Origin,
	}, in.CreatedBy)
	if err != nil {
		return application.ClassArrivalException{}, mapArrivalExceptionError(err)
	}
	return classArrivalException(row), nil
}

func (b classArrivalExceptionBinding) DeleteClassArrivalException(ctx context.Context, schoolClass string, date timezone.Date) error {
	return mapArrivalExceptionError(b.store.DeleteClassArrivalException(ctx, schoolClass, date))
}

func classArrivalException(row *careplan.ClassArrivalException) application.ClassArrivalException {
	return application.ClassArrivalException{
		SchoolClass: row.SchoolClass, Date: row.Date, ArrivalTime: row.ArrivalTime,
		Reason: row.Reason, CreatedAt: row.CreatedAt, Origin: row.Origin,
	}
}

// blockStartBinding answers the "Unterricht fällt aus" preset with the same
// error classification as the exception writes.
type blockStartBinding struct{ starts BlockStarts }

func (b blockStartBinding) EarliestPlannedBlockStartForClass(ctx context.Context, schoolClass string, date timezone.Date) (string, error) {
	start, err := b.starts.EarliestPlannedBlockStartForClass(ctx, schoolClass, date)
	if err != nil {
		return "", mapArrivalExceptionError(err)
	}
	return start, nil
}

type callerBinding struct{ context CallerReader }

func (b callerBinding) AssignedClasses(ctx context.Context) ([]string, error) {
	return b.context.GetMySchoolClasses(ctx)
}

// StaffID resolves the caller's users.staff row (every school-portal account
// has one, EnsureSchoolIdentity). An account without one is refused with the
// sentinel: the entry would be attributed to nobody. Any other failure of the
// lookup is a server error, not a missing record — the caller must not be
// told to fix their account for a database outage.
func (b callerBinding) StaffID(ctx context.Context) (int64, error) {
	staffID, found, err := b.context.CurrentStaffID(ctx)
	switch {
	case err != nil:
		return 0, err
	case !found:
		return 0, classday.ErrStaffRecordRequired
	}
	return staffID, nil
}

// writeScopeBinding applies operations.school_portal_write_scope: only the
// class arrival exception scope opens the school's write.
type writeScopeBinding struct{ settings WriteScopeReader }

func (b writeScopeBinding) SchoolMayWriteClassArrivalExceptions(ctx context.Context) (bool, error) {
	scope, err := b.settings.ResolveString(ctx, configModel.KeySchoolPortalWriteScope)
	if err != nil {
		return false, err
	}
	return scope == configModel.SchoolPortalWriteScopeClassArrivalExceptions, nil
}
