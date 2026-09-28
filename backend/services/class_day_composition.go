package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	classdayCompose "github.com/moto-nrw/project-phoenix/modules/classday/compose"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The bindings below connect the school portal's class-day capability
// (#2970, #2527, #3563) to Enrollment's day roster, Care Plan's status days,
// the People Directory students and guardian contacts, the GDPR access log
// and the realtime hub.

// classDayStatusDayReader reads the active scheduled day statuses.
type classDayStatusDayReader interface {
	FindActiveByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date absencerecords.Date) ([]*absencerecords.StudentStatusDay, error)
}

// classDaySources are the reads and writes the root binds the class-day
// capability to.
type classDaySources struct {
	Caller                 classdayCompose.CallerReader
	Reports                enrollmentOwner.Reports
	StatusDays             classDayStatusDayReader
	PickupTimes            classdayCompose.PickupTimeReader
	ArrivalTimes           classdayCompose.ArrivalTimeReader
	CareDays               classdayCompose.CareDayResolver
	Companions             classdayCompose.Companions
	Students               userModels.StudentRepository
	Persons                userModels.PersonRepository
	StudentGuardians       userModels.StudentGuardianRepository
	AccessLog              auditModels.DataAccessLogRepository
	ClassArrivalExceptions careplan.ClassArrivalExceptions
	Settings               classdayCompose.WriteScopeReader
	BlockStarts            classdayCompose.BlockStarts
	// Broadcaster is optional: without it no arrival schedule change is
	// announced.
	Broadcaster realtime.Broadcaster
	Logger      *slog.Logger
}

// newClassDay composes the school portal's class-day capability.
func newClassDay(sources classDaySources) classday.ClassDay {
	deps := classdayCompose.ClassDayDependencies{
		Caller:                 sources.Caller,
		PickupTimes:            sources.PickupTimes,
		ArrivalTimes:           sources.ArrivalTimes,
		CareDays:               sources.CareDays,
		Companions:             sources.Companions,
		ClassArrivalExceptions: sources.ClassArrivalExceptions,
		Settings:               sources.Settings,
		BlockStarts:            sources.BlockStarts,
		Announcer:              classDayArrivalAnnouncer{broadcaster: sources.Broadcaster, logger: sources.Logger},
	}
	if sources.Reports != nil {
		deps.Rosters = classDayRosters{reports: sources.Reports}
	}
	if sources.StatusDays != nil {
		deps.StatusDays = classDayStatusDays{reader: sources.StatusDays}
	}
	if sources.Students != nil && sources.Persons != nil {
		deps.Students = classDaySheetStudents{students: sources.Students, persons: sources.Persons}
	}
	if sources.StudentGuardians != nil {
		deps.EmergencyContacts = classDayEmergencyContacts{repo: sources.StudentGuardians}
	}
	if sources.AccessLog != nil {
		deps.AccessLog = classDayAccessLog{repo: sources.AccessLog}
	}
	return classdayCompose.NewClassDay(deps)
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
type classDayStatusDays struct{ reader classDayStatusDayReader }

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
	students userModels.StudentRepository
	persons  userModels.PersonRepository
}

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
	repo userModels.StudentGuardianRepository
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
			FirstName: row.FirstName.String, LastName: row.LastName.String,
			RelationshipType: row.RelationshipType.String, PickupNotes: row.PickupNotes.String,
			PhoneNumber: row.PhoneNumber.String, CanPickup: row.CanPickup, IsEmergencyContact: row.IsEmergencyContact,
		})
	}
	return out, nil
}

// classDayAccessLog appends the school portal's reads to
// audit.data_access_log.
type classDayAccessLog struct {
	repo auditModels.DataAccessLogRepository
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
	tenant.RegisterAfterCommit(ctx, func() {
		source := "school"
		event := realtime.NewEvent(realtime.EventArrivalScheduleChanged, "", realtime.EventData{Source: &source})
		if err := a.broadcaster.BroadcastToAll(event); err != nil {
			a.log().Warn("failed to broadcast arrival schedule change",
				"error", err.Error(),
			)
		}
	})
}

func (a classDayArrivalAnnouncer) log() *slog.Logger {
	if a.logger != nil {
		return a.logger
	}
	return slog.Default()
}
