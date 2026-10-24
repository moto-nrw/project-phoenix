package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	classdayCompose "github.com/moto-nrw/project-phoenix/modules/classday/compose"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	configModels "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/realtime"

	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The bindings below connect the school portal's class-day capability
// (#2970, #2527, #3563) to Enrollment's day roster, Care Plan's status days,
// the People Directory students and guardian contacts, the GDPR access log
// and the realtime hub.

// NewClassDay composes the school portal's class-day capability. deps
// carries the caller, the pickup and arrival times, the care days, the
// companions, the class arrival exceptions and the block starts; the
// remaining ports are bound here over the reports, the status days, the
// student, person and contact rows, the access log, the write-scope and
// morning attendance settings and the broadcaster. deps also carries
// Student Presence's morning attendance (#4790). Without a broadcaster no arrival schedule change is
// announced.
func NewClassDay(
	deps classdayCompose.ClassDayDependencies, reports enrollmentOwner.Reports, statusDays classDayStatusDayReader,
	students classDayStudentRows, persons classDayPersonRows, studentGuardians classDayContactRows,
	accessLog owners.AccessLogLedger, settings classDayWriteScopeSettings, broadcaster realtime.Broadcaster, logger *slog.Logger,
) classday.ClassDay {
	deps.WriteScope = newClassDayWriteScope(settings)
	if settings != nil {
		// operations.school_portal_morning_attendance (#4790): on unless the
		// school switched it off.
		deps.Scope = func(ctx context.Context) (bool, error) {
			return settings.ResolveBool(ctx, configModels.KeySchoolPortalMorningAttendance)
		}
	}
	deps.Announcer = classDayArrivalAnnouncer{broadcaster: broadcaster, logger: logger}
	if reports != nil {
		deps.Rosters = classDayRosters{reports: reports}
	}
	if statusDays != nil {
		deps.StatusDays = classDayStatusDays{reader: statusDays}
	}
	if students != nil && persons != nil {
		deps.Students = classDaySheetStudents{students: students, persons: persons}
	}
	if studentGuardians != nil {
		deps.EmergencyContacts = classDayEmergencyContacts{repo: studentGuardians}
	}
	if accessLog != nil {
		deps.AccessLog = classDayAccessLog{repo: accessLog}
	}
	return classdayCompose.NewClassDay(deps)
}

// classDayTripRecorder is Student Presence's status-day write for a class
// trip moto schule reports (#4799).
type classDayTripRecorder interface {
	RecordClassTrip(ctx context.Context, studentIDs []int64, dates []timezone.Date, reason string, inClass func(schoolClass string) bool) error
}

// NewClassDayTripStatusDays binds the class-trip status-day write. The class
// check on the locked child compares the normalized class names, like the
// caller's class assignment does, and the write's refusals answer the
// projection's sentinels.
func NewClassDayTripStatusDays(recorder classDayTripRecorder) classdayCompose.ClassTripStatusDays {
	if recorder == nil {
		return nil
	}
	return classDayTripStatusDays{recorder: recorder}
}

type classDayTripStatusDays struct{ recorder classDayTripRecorder }

func (b classDayTripStatusDays) RecordClassTrip(ctx context.Context, schoolClass string, studentIDs []int64, dates []timezone.Date, reason string) error {
	want := schoolstructure.NormalizeClass(schoolClass)
	err := b.recorder.RecordClassTrip(ctx, studentIDs, dates, reason, func(class string) bool {
		return schoolstructure.NormalizeClass(class) == want
	})
	var conflict *studentpresence.StudentStatusDayConflictError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &conflict), errors.Is(err, studentpresence.ErrStudentStatusDayPartialAbsenceConflict):
		return fmt.Errorf("%w: %w", classday.ErrClassTripStatusConflict, err)
	case errors.Is(err, studentpresence.ErrStudentStatusDayReassigned):
		return fmt.Errorf("%w: %w", classday.ErrClassTripStudentNotInClass, err)
	}
	return err
}

// classDayRosters reads Enrollment's class roster of a day.
type classDayRosters struct{ reports enrollmentOwner.Reports }

func (b classDayRosters) ClassRosterDay(ctx context.Context, schoolClass string, date timezone.Date) (*classdayCompose.DayRoster, error) {
	roster, err := b.reports.ClassRosterDay(ctx, schoolClass, date)
	if err != nil || roster == nil {
		return nil, err
	}
	out := &classdayCompose.DayRoster{
		PhaseNames: roster.PhaseNames,
		Rows:       make([]classdayCompose.DayRosterRow, 0, len(roster.Rows)),
		Students:   make([]classdayCompose.DayRosterStudent, 0, len(roster.Students)),
	}
	for _, row := range roster.Rows {
		out.Rows = append(out.Rows, classdayCompose.DayRosterRow{
			StudentID: row.StudentID, FirstName: row.FirstName, LastName: row.LastName,
			ListEntry: row.ListEntry, ListEntryID: row.ListEntryID, GroupName: row.GroupName, Registered: row.Registered,
			OfferingsByDay: row.OfferingsByDay, ArrivalByDay: row.ArrivalByDay, PickupByDay: row.PickupByDay,
		})
	}
	for _, student := range roster.Students {
		modes := make([]peopledirectory.DepartureMode, 0, len(student.DepartureModes))
		for _, mode := range student.DepartureModes {
			modes = append(modes, peopledirectory.DepartureMode(mode))
		}
		out.Students = append(out.Students, classdayCompose.DayRosterStudent{ID: student.ID, DepartureModes: modes})
	}
	return out, nil
}

// classDayStatusDays reads the active scheduled day statuses.
type classDayStatusDays struct {
	reader classDayStatusDayReader
}

// classDayStatusDayReader reads Care Plan's active scheduled day statuses.
type classDayStatusDayReader interface {
	FindActiveByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date absencerecords.Date) ([]*absencerecords.StudentStatusDay, error)
}

func (b classDayStatusDays) ActiveStatusDays(ctx context.Context, studentIDs []int64, date timezone.Date) ([]classdayCompose.StatusDayEntry, error) {
	entries, err := b.reader.FindActiveByStudentIDsAndDate(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	out := make([]classdayCompose.StatusDayEntry, 0, len(entries))
	for _, entry := range entries {
		if entry != nil {
			out = append(out, classdayCompose.StatusDayEntry{StudentID: entry.StudentID, Status: entry.Status, ReportedAt: entry.ReportedAt})
		}
	}
	return out, nil
}

// classDaySheetStudents reads the child of a supervision sheet.
type classDaySheetStudents struct {
	students classDayStudentRows
	persons  classDayPersonRows
}

// classDayStudentRows, classDayPersonRows and classDayContactRows are the
// People Directory rows the class day reads.
type (
	classDayStudentRows interface {
		FindByID(ctx context.Context, id any) (*peopledirectory.StudentState, error)
	}
	classDayPersonRows interface {
		FindByIDs(ctx context.Context, ids []int64) (map[int64]*peopledirectory.DirectoryPersonSnapshot, error)
	}
	classDayContactRows interface {
		ListEmergencyContactRows(ctx context.Context, studentIDs []int64) ([]peopledirectory.GuardianEmergencyContactRow, error)
	}
)

func (b classDaySheetStudents) FindSheetStudent(ctx context.Context, studentID int64) (*classdayCompose.SheetStudent, error) {
	student, err := b.students.FindByID(ctx, studentID)
	if err != nil || student == nil {
		return nil, err
	}
	return &classdayCompose.SheetStudent{
		ID: student.ID, PersonID: student.PersonID, SchoolClass: student.SchoolClass,
		AllowedDepartureModes: student.AllowedDepartureModes, DepartureDays: student.DepartureDays,
	}, nil
}

func (b classDaySheetStudents) PersonsByID(ctx context.Context, personIDs []int64) (map[int64]classdayCompose.SheetPerson, error) {
	persons, err := b.persons.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]classdayCompose.SheetPerson, len(persons))
	for _, person := range persons {
		if person != nil {
			out[person.ID] = classdayCompose.SheetPerson{FirstName: person.FirstName, LastName: person.LastName}
		}
	}
	return out, nil
}

// classDayEmergencyContacts reads the guardian contact rows of a child.
type classDayEmergencyContacts struct {
	repo classDayContactRows
}

func (b classDayEmergencyContacts) EmergencyContactRows(ctx context.Context, studentIDs []int64) ([]classdayCompose.EmergencyContactRow, error) {
	rows, err := b.repo.ListEmergencyContactRows(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	out := make([]classdayCompose.EmergencyContactRow, 0, len(rows))
	for _, row := range rows {
		out = append(out, classdayCompose.EmergencyContactRow{
			StudentID: row.StudentID, GuardianProfileID: row.GuardianProfileID,
			FirstName: owners.StringValue(row.FirstName), LastName: owners.StringValue(row.LastName),
			RelationshipType: owners.StringValue(row.RelationshipType), PickupNotes: owners.StringValue(row.PickupNotes),
			PhoneNumber: owners.StringValue(row.PhoneNumber), CanPickup: row.CanPickup, IsEmergencyContact: row.IsEmergencyContact,
		})
	}
	return out, nil
}

// classDayAccessLog appends the school portal's reads to
// audit.data_access_log.
type classDayAccessLog struct {
	repo owners.AccessLogLedger
}

func (l classDayAccessLog) ClassDayViewSeenSince(ctx context.Context, actorAccountID int64, schoolClass, date string, since time.Time) (bool, error) {
	return l.repo.ExistsSince(ctx, actorAccountID, auditModels.ResourceTypeClassDayView,
		map[string]string{"school_class": schoolClass, "date": date}, since)
}

func (l classDayAccessLog) RecordClassDayView(ctx context.Context, entry classdayCompose.AccessRecord) error {
	return l.repo.Create(ctx, classDayAccessRow(auditModels.ResourceTypeClassDayView, entry))
}

func (l classDayAccessLog) RecordSupervisionSheet(ctx context.Context, entry classdayCompose.AccessRecord) error {
	return l.repo.Create(ctx, classDayAccessRow(auditModels.ResourceTypeSupervisionStudentSheet, entry))
}

func classDayAccessRow(resourceType string, entry classdayCompose.AccessRecord) *auditModels.DataAccessLog {
	return &auditModels.DataAccessLog{
		ActorAccountID: entry.ActorAccountID,
		ActorRole:      entry.ActorRole,
		ResourceType:   resourceType,
		StudentID:      entry.StudentID,
		RangeStart:     entry.RangeStart,
		RangeEnd:       entry.RangeEnd,
		AccessedAt:     entry.AccessedAt,
		Metadata:       entry.Metadata,
	}
}

// classDayArrivalAnnouncer emits the arrival-schedule event the OGS handler
// emits after its own writes, so Aufsicht and Meine Gruppe refetch. A
// class-wide change concerns every child of the class, so no student ID
// travels with it.
type classDayArrivalAnnouncer struct {
	broadcaster realtime.Broadcaster
	logger      *slog.Logger
}

func (a classDayArrivalAnnouncer) AnnounceArrivalScheduleChange(ctx context.Context) {
	if a.broadcaster == nil {
		return
	}
	deliveryCompose.QueueArrivalScheduleChanged(ctx, a.broadcaster, a.logger)
}
