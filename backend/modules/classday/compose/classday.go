package compose

import (
	"context"
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/usercontext"
)

// DayReportReader is the slice of the enrollment report service the school
// portal reads.
type DayReportReader interface {
	ClassDay(ctx context.Context, schoolClass string, date timezone.Date, actorAccountID int64, actorRole string) (*enrollment.ClassDayReport, error)
}

// CallerReader is the slice of the user-context service the school portal
// resolves the caller with.
type CallerReader interface {
	GetMySchoolClasses(ctx context.Context) ([]string, error)
	GetCurrentStaff(ctx context.Context) (*userModel.Staff, error)
}

// ClassDayDependencies wires the school-portal capability. ArrivalExceptions
// is the one write seam of moto schule (#2970); nil leaves its routes
// answering "not configured".
type ClassDayDependencies struct {
	Reports           DayReportReader
	Caller            CallerReader
	ArrivalExceptions enrollment.ClassDayArrivalExceptionService
}

// NewClassDay composes the school-portal capability over the retained
// enrollment report, the arrival-exception service and the identity chain.
func NewClassDay(deps ClassDayDependencies) classday.ClassDay {
	var exceptions ports.ArrivalExceptions
	if deps.ArrivalExceptions != nil {
		exceptions = arrivalExceptionBinding{service: deps.ArrivalExceptions}
	}
	var caller ports.Caller
	if deps.Caller != nil {
		caller = callerBinding{context: deps.Caller}
	}
	var reports ports.DayReports
	if deps.Reports != nil {
		reports = reportBinding{reports: deps.Reports}
	}
	return application.NewClassDay(application.ClassDayDependencies{
		Caller: caller, Reports: reports, ArrivalExceptions: exceptions,
	})
}

// mappedError keeps the retained service's message on the wire while
// answering errors.Is for the projection's sentinel, so the HTTP adapter
// classifies the outcome without the client reading a different text.
type mappedError struct {
	err      error
	sentinel error
}

func (e mappedError) Error() string { return e.err.Error() }
func (e mappedError) Unwrap() error { return e.err }
func (e mappedError) Is(target error) bool {
	return target == e.sentinel
}

func mapError(err error, pairs ...errorPair) error {
	for _, pair := range pairs {
		if errors.Is(err, pair.legacy) {
			return mappedError{err: err, sentinel: pair.sentinel}
		}
	}
	return err
}

type errorPair struct {
	legacy   error
	sentinel error
}

var arrivalExceptionErrors = []errorPair{
	{legacy: enrollment.ErrClassDayArrivalExceptionPastDate, sentinel: classday.ErrArrivalExceptionPastDate},
	{legacy: enrollment.ErrClassDayArrivalExceptionWeekend, sentinel: classday.ErrArrivalExceptionWeekend},
	{legacy: enrollment.ErrClassDayArrivalExceptionClassNotFound, sentinel: classday.ErrArrivalExceptionClassNotFound},
	{legacy: enrollment.ErrClassDayArrivalExceptionNotFound, sentinel: classday.ErrArrivalExceptionNotFound},
}

type reportBinding struct{ reports DayReportReader }

func (b reportBinding) ClassDay(ctx context.Context, schoolClass string, date timezone.Date, actor classday.Actor) (*classday.DayReport, error) {
	report, err := b.reports.ClassDay(ctx, schoolClass, date, actor.AccountID, actor.Roles)
	if err != nil {
		return nil, mapError(err, errorPair{legacy: enrollment.ErrReportInvalidFilter, sentinel: classday.ErrInvalidReportFilter})
	}
	return DayReportFromEnrollment(report), nil
}

// DayReportFromEnrollment projects the enrollment report onto the public
// contract field by field; the JSON shapes are identical (see the parity
// test).
func DayReportFromEnrollment(report *enrollment.ClassDayReport) *classday.DayReport {
	if report == nil {
		return nil
	}
	out := &classday.DayReport{
		SchoolClass:     report.SchoolClass,
		Date:            classday.Date(report.Date.String()),
		Weekday:         report.Weekday,
		SchoolDay:       report.SchoolDay,
		PhaseName:       report.PhaseName,
		EnrollmentKnown: report.EnrollmentKnown,
		Totals: classday.DayTotals{
			Students: report.Totals.Students, Staying: report.Totals.Staying, Leaving: report.Totals.Leaving,
			Absent: report.Totals.Absent, ListEntries: report.Totals.ListEntries,
		},
		Rows: make([]classday.DayRow, 0, len(report.Rows)),
	}
	for _, row := range report.Rows {
		out.Rows = append(out.Rows, classday.DayRow{
			StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName,
			ListEntry: row.ListEntry, ListEntryID: row.ListEntryID, GroupName: row.GroupName,
			Registered: row.Registered, StaysToday: row.StaysToday, Offerings: row.Offerings,
			Arrival: row.Arrival, Pickup: row.Pickup, Departure: row.Departure, Status: row.Status,
			PickupChanged: row.PickupChanged, PickupRegular: row.PickupRegular, ReportedAt: row.ReportedAt,
		})
	}
	if report.ClassArrivalException != nil {
		out.ClassArrivalException = &classday.DayArrivalException{
			ArrivalTime: report.ClassArrivalException.ArrivalTime,
			Reason:      report.ClassArrivalException.Reason,
			Origin:      report.ClassArrivalException.Origin,
		}
	}
	return out
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
	staff, err := b.context.GetCurrentStaff(ctx)
	switch {
	case errors.Is(err, usercontext.ErrUserNotLinkedToStaff), errors.Is(err, usercontext.ErrUserNotLinkedToPerson):
		return 0, classday.ErrStaffRecordRequired
	case err != nil:
		return 0, err
	case staff == nil:
		return 0, classday.ErrStaffRecordRequired
	}
	return staff.ID, nil
}

type arrivalExceptionBinding struct {
	service enrollment.ClassDayArrivalExceptionService
}

func (b arrivalExceptionBinding) SchoolMayWrite(ctx context.Context) (bool, error) {
	return b.service.SchoolMayWrite(ctx)
}

func (b arrivalExceptionBinding) ListForClass(ctx context.Context, schoolClass string, from, to timezone.Date) ([]classday.ArrivalException, error) {
	entries, err := b.service.List(ctx, schoolClass, from, to)
	if err != nil {
		return nil, mapError(err, arrivalExceptionErrors...)
	}
	out := make([]classday.ArrivalException, 0, len(entries))
	for _, entry := range entries {
		out = append(out, arrivalExceptionFromEnrollment(entry))
	}
	return out, nil
}

func (b arrivalExceptionBinding) Set(ctx context.Context, in classday.ArrivalExceptionWrite) (*classday.ArrivalException, error) {
	date, err := timezone.ParseDate(strings.TrimSpace(string(in.Date)))
	if err != nil {
		return nil, err
	}
	entry, err := b.service.Set(ctx, enrollment.ClassDayArrivalExceptionWrite{
		SchoolClass: in.SchoolClass, Date: date, ArrivalTime: in.ArrivalTime, Reason: in.Reason, CreatedBy: in.CreatedBy,
	})
	if err != nil {
		return nil, mapError(err, arrivalExceptionErrors...)
	}
	if entry == nil {
		return nil, nil
	}
	mapped := arrivalExceptionFromEnrollment(*entry)
	return &mapped, nil
}

func (b arrivalExceptionBinding) Clear(ctx context.Context, schoolClass string, date timezone.Date) error {
	return mapError(b.service.Remove(ctx, schoolClass, date), arrivalExceptionErrors...)
}

func (b arrivalExceptionBinding) EarliestBlockStart(ctx context.Context, schoolClass string, date timezone.Date) (string, error) {
	start, err := b.service.EarliestBlockStart(ctx, schoolClass, date)
	if err != nil {
		return "", mapError(err, arrivalExceptionErrors...)
	}
	return start, nil
}

func arrivalExceptionFromEnrollment(entry enrollment.ClassDayArrivalExceptionEntry) classday.ArrivalException {
	return classday.ArrivalException{
		SchoolClass: entry.SchoolClass, Date: classday.Date(entry.Date), ArrivalTime: entry.ArrivalTime,
		Reason: entry.Reason, CreatedAt: entry.CreatedAt, Origin: entry.Origin,
	}
}
