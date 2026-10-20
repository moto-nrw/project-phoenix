package compose

import (
	"errors"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/adapters/postgres"
	application "github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The session records read the display relations of other owners through
// directories the composition root implements over those owners.
type (
	DirectoryDevice           = ports.DirectoryDevice
	DeviceDirectory           = ports.DeviceDirectory
	DirectoryRoom             = ports.DirectoryRoom
	RoomDirectory             = ports.RoomDirectory
	SessionRoom               = studentpresence.SessionRoom
	SessionActivity           = ports.SessionActivity
	SessionActivityCategory   = ports.SessionActivityCategory
	ActivityDirectory         = ports.ActivityDirectory
	SessionStaff              = ports.SessionStaff
	SessionStaffPerson        = ports.SessionStaffPerson
	SupervisionStaffDirectory = ports.SupervisionStaffDirectory
	RecordErrors              = ports.RecordErrors
	CrossTenantStudent        = ports.CrossTenantStudent
)

// SessionRecordDependencies wire the session and supervision records.
type SessionRecordDependencies struct {
	DB      *bun.DB
	Observe func(Observation)
	// Now is the clock "today" of the supervision reads comes from; nil uses
	// time.Now.
	Now        func() time.Time
	Devices    DeviceDirectory
	Rooms      RoomDirectory
	Activities ActivityDirectory
	// Staff attaches the staff members to the session supervision reads;
	// nil leaves them unset.
	Staff  SupervisionStaffDirectory
	Errors RecordErrors
}

// SessionRecords is the bounded record capability returned by construction.
// The implementation remains private to Student Presence.
type SessionRecords interface {
	studentpresence.SessionRecords
	studentpresence.SupervisionRecords
	RoomDirectory() RoomDirectory
	SupervisionStaff() SupervisionStaffDirectory
}

// NewSessionRecords builds the session and supervision records over the
// owner's store and the composition root's directories.
func NewSessionRecords(deps SessionRecordDependencies) (SessionRecords, error) {
	if deps.DB == nil || deps.Observe == nil || deps.Errors == nil {
		return nil, errors.New("student presence session records: database, observer and error representation are required")
	}
	var clocks []func() time.Time
	now := time.Now
	if deps.Now != nil {
		clocks, now = append(clocks, deps.Now), deps.Now
	}
	records := application.New(postgres.New(databaseRuntime(deps.DB)), transaction{}, deps.Observe, ports.AttendanceRulePorts{}, now)
	return application.NewSessionRecords(
		postgres.NewSessionRepository(records, deps.Errors, postgres.SessionDirectories{Devices: deps.Devices, Rooms: deps.Rooms, Activities: deps.Activities}),
		postgres.NewSupervisionRepository(records, deps.Errors, deps.Staff, calendar.CalendarDateClock(clocks...)), deps.Staff), nil
}

// SessionRepositories binds the native repositories of composed records.
func SessionRepositories(records studentpresence.SessionRecords) (ports.ActiveGroupRepository, ports.GroupSupervisorRepository) {
	return application.SessionRepositories(records)
}
