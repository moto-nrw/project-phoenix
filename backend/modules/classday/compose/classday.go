package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/application"
)

// Ports the composition root binds for the school-portal capability.
type (
	DayRoster                     = application.DayRoster
	DayRosterRow                  = application.DayRosterRow
	DayRosterStudent              = application.DayRosterStudent
	DayRosters                    = application.DayRosters
	StatusDayEntry                = application.StatusDayEntry
	StatusDays                    = application.StatusDays
	Companions                    = application.Companions
	SheetStudent                  = application.SheetStudent
	SheetPerson                   = application.SheetPerson
	SheetStudents                 = application.SheetStudents
	EmergencyContactRow           = application.EmergencyContactRow
	EmergencyContacts             = application.EmergencyContacts
	AccessRecord                  = application.AccessRecord
	AccessLog                     = application.AccessLog
	ArrivalScheduleAnnouncer      = application.ArrivalScheduleAnnouncer
	MorningAttendanceDependencies = application.MorningAttendanceDependencies
	DayHintDependencies           = application.DayHintDependencies
	HomeworkDependencies          = application.HomeworkDependencies
	HomeworkEntry                 = application.HomeworkEntry
	HomeworkFeedbackEntry         = application.HomeworkFeedbackEntry
	ClassHandoverDependencies     = application.ClassHandoverDependencies
)

// ClassDayDependencies wire the school-portal reads and write seams; a nil seam is unconfigured.
type ClassDayDependencies struct {
	Caller                 CallerReader
	Rosters                DayRosters
	StatusDays             StatusDays
	PickupTimes            ClassDayPickupReader
	ArrivalTimes           ArrivalTimeReader
	CareDays               CareDayResolver
	Companions             Companions
	Students               SheetStudents
	EmergencyContacts      EmergencyContacts
	AccessLog              AccessLog
	ClassArrivalExceptions careplan.ArrivalScheduleService
	WriteScope             SchoolWriteScope
	BlockStarts            application.BlockStarts
	Announcer              ArrivalScheduleAnnouncer
	TripStatusDays         ClassTripStatusDays
	MorningAttendanceDependencies
	DayHintDependencies
	HomeworkDependencies
	ClassHandoverDependencies
}

// NewClassDay composes the school-portal capability.
func NewClassDay(deps ClassDayDependencies) classday.ClassDay {
	app := application.ClassDayDependencies{
		Caller: application.NewCallerBinding(deps.Caller), Rosters: deps.Rosters, StatusDays: deps.StatusDays,
		Companions: deps.Companions, Students: deps.Students, EmergencyContacts: deps.EmergencyContacts,
		AccessLog: deps.AccessLog, WriteScope: deps.WriteScope, TripScope: deps.WriteScope, Announcer: deps.Announcer,
		StudentArrivals: deps.ClassArrivalExceptions, TripStatusDays: deps.TripStatusDays, HomeworkDependencies: deps.HomeworkDependencies,
		MorningAttendanceDependencies: deps.MorningAttendanceDependencies, ClassHandoverDependencies: deps.ClassHandoverDependencies,
		DayHintDependencies: deps.DayHintDependencies, HintScope: deps.WriteScope,
	}
	if deps.PickupTimes != nil && deps.ArrivalTimes != nil {
		app.Times = application.DayTimeBinding{ClassDayPickupReader: deps.PickupTimes, Pickups: deps.PickupTimes, Arrivals: deps.ArrivalTimes}
	}
	if deps.CareDays != nil {
		app.CareDays = application.CareDayBinding{Resolver: deps.CareDays}
	}
	if deps.ClassArrivalExceptions != nil {
		app.ClassArrivalExceptions = application.ClassArrivalExceptionBinding{Store: deps.ClassArrivalExceptions}
	}
	if deps.BlockStarts != nil {
		app.BlockStarts = application.BlockStartBinding{Starts: deps.BlockStarts}
	}
	return application.NewClassDay(app)
}
