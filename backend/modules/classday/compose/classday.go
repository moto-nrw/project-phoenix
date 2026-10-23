package compose

import (
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

// ClassDayDependencies wires the school-portal capability: the day report
// over Enrollment's day roster and Care Plan's day facts, the supervision
// sheet, and the write seams of moto schule (#2970, #4799). A nil
// ClassArrivalExceptions or TripStatusDays leaves the arrival exception or
// class-trip routes answering "not configured".
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
	BlockStarts            BlockStarts
	Announcer              ArrivalScheduleAnnouncer
	TripStatusDays         ClassTripStatusDays
}

// NewClassDay composes the school-portal capability.
func NewClassDay(deps ClassDayDependencies) classday.ClassDay {
	var caller ports.Caller
	if deps.Caller != nil {
		caller = application.CallerBinding{CallerReader: deps.Caller}
	}
	app := application.ClassDayDependencies{
		Caller: caller, Rosters: deps.Rosters, StatusDays: deps.StatusDays,
		Companions: deps.Companions, Students: deps.Students, EmergencyContacts: deps.EmergencyContacts,
		AccessLog: deps.AccessLog, WriteScope: deps.WriteScope, TripScope: deps.WriteScope, Announcer: deps.Announcer,
		TripStatusDays: deps.TripStatusDays,
	}
	if deps.PickupTimes != nil && deps.ArrivalTimes != nil {
		app.Times = application.DayTimeBinding{ClassDayPickupReader: deps.PickupTimes, Pickups: deps.PickupTimes, Arrivals: deps.ArrivalTimes}
	}
	if deps.CareDays != nil {
		app.CareDays = application.CareDayBinding{Resolver: deps.CareDays}
	}
	if deps.ClassArrivalExceptions != nil {
		app.ClassArrivalExceptions = application.ClassArrivalExceptionBinding{Store: deps.ClassArrivalExceptions}
		app.StudentArrivals = deps.ClassArrivalExceptions
	}
	if deps.BlockStarts != nil {
		app.BlockStarts = application.BlockStartBinding{Starts: deps.BlockStarts}
	}
	return application.NewClassDay(app)
}
