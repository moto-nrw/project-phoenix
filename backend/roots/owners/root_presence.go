package owners

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// PresenceOwner is Student Presence as the roots compose it (#4010): the
// owner module with its operations logged at debug level and one that logs
// only its failed operations, the session and supervision records over the
// room, activity, device and staff owners, the attendance reads and the
// scheduled day statuses over Care Plan.
type PresenceOwner struct {
	Presence                  *studentpresence.Module
	UnloggedPresence          *studentpresence.Module
	SessionRecords            presenceCompose.SessionRecords
	AttendanceStaff           presenceservice.AttendanceStaff
	AttendanceEducationGroups presenceservice.AttendanceEducationGroups
	AttendanceActivities      presenceservice.AttendanceActivityGroups
	StatusDays                studentpresence.StatusDays
}

// BuildPresenceOwner builds Student Presence, its session records and the
// attendance reads over the owners (#4010).
func BuildPresenceOwner(
	db transactionruntime.Database, logger *slog.Logger, now func() time.Time, devices devicefleet.Capability,
	persons peopledirectory.Capability, rooms facilitiesModule.Query, membership schoolmembership.Capability,
	carePlan careplan.Capability, timetableOwner timetable.Capability, timetableRows TimetableRepositories,
	people PeopleRows, structure StructureRows,
) PresenceOwner {
	return PresenceOwner{
		Presence:                  NewLoggedStudentPresence(db, logger),
		UnloggedPresence:          NewUnloggedStudentPresence(db),
		SessionRecords:            NewSessionRecords(db, now, devices, rooms, timetableOwner, membership, persons),
		AttendanceStaff:           NewAttendanceStaffDirectory(people.Staff),
		AttendanceEducationGroups: NewAttendanceEducationGroups(structure.Groups, people.Students),
		AttendanceActivities:      NewSessionActivities(timetableRows.GroupReads()),
		StatusDays: presenceservice.NewStatusDays(
			NewStudentStatusDayRepository(carePlan), NewManualPartialAbsenceDates(carePlan),
			db.Pool(), carePlan.LockExceptionDay, now,
		),
	}
}
