package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// timetableDataInputs are the retained rows and the owners the planner's
// reads of the Timetable owner read through (#3551). ArrivalBaselines and
// PickupBaselines are optional; Transactional binds the tenant runtime's
// advisory locks a spontaneous start takes.
type timetableDataInputs struct {
	Rows             repositories.TimetableOwnerRows
	ArrivalBaselines careplan.ArrivalBaselineReader
	PickupBaselines  careplan.PickupBaselineReader
	Visits           timetableCompose.OperationVisits
	Groups           timetableCompose.DataGroups
	Sessions         studentpresence.SessionRecords
	ConflictAcks     timetable.ConflictAckCapability
	Transactional    bool
	Logger           *slog.Logger
}

// newTimetableData composes the planner's reads of the Timetable owner and
// binds the collaborators it may not name itself: Care Plan's baselines, the
// Student Presence room occupancy and the tenant runtime's advisory locks;
// the retained rows bind the Facilities rooms and the Audit Platform's
// Änderungsprotokoll.
func newTimetableData(in timetableDataInputs) (timetable.TimetableDataCapability, error) {
	templates, err := in.Rows.DataTemplates()
	if err != nil {
		return nil, err
	}
	return timetableCompose.NewTimetableData(timetableCompose.TimetableDataDependencies{
		Instances:         in.Rows.Instances,
		InstanceStaff:     in.Rows.InstanceStaff,
		Participants:      in.Rows.Participants,
		PickupExceptions:  in.Rows.PickupExceptions,
		ArrivalExceptions: in.Rows.ArrivalExceptions,
		ArrivalBaselines:  newArrivalBaselineProjector(in.ArrivalBaselines),
		PickupBaselines:   newPickupBaselineProjector(in.PickupBaselines),
		Visits:            in.Visits,
		Templates:         templates,
		Groups:            in.Groups,
		Categories:        in.Rows.Categories,
		Rooms:             in.Rows.RoomNames(),
		RoomOccupancy:     timetableRoomOccupancy{sessions: in.Sessions},
		DeviationEvents:   in.Rows.DeviationEventReader(),
		ConflictAcks:      in.ConflictAcks,
		Locks:             in.Rows.AttendanceLocks(),
		Advisory:          newTenantAdvisoryLocks(in.Transactional),
		Logger:            in.Logger,
	})
}

func newArrivalBaselineProjector(reader careplan.ArrivalBaselineReader) timetableCompose.ArrivalBaselineProjector {
	if reader == nil {
		return nil
	}
	return arrivalBaselineProjector{reader: reader}
}

func newPickupBaselineProjector(reader careplan.PickupBaselineReader) timetableCompose.PickupBaselineProjector {
	if reader == nil {
		return nil
	}
	return pickupBaselineProjector{reader: reader}
}

func newTenantAdvisoryLocks(transactional bool) timetableCompose.AdvisoryLocks {
	if !transactional {
		return nil
	}
	return tenantAdvisoryLocks{}
}

// tenantAdvisoryLocks takes transaction-scoped advisory locks through the
// tenant runtime's unit of work; they release at commit or rollback.
type tenantAdvisoryLocks struct{}

func (tenantAdvisoryLocks) AcquireXactLock(ctx context.Context, key string) error {
	return tenant.AcquireLock(ctx, key, false)
}

// pickupBaselineProjector serves the Timetable pickup port from Care Plan's
// baseline projection.
type pickupBaselineProjector struct {
	reader careplan.PickupBaselineReader
}

func (p pickupBaselineProjector) ProjectPickups(ctx context.Context, studentIDs []int64, from, to timezone.Date) (timetableCompose.PickupBaselines, error) {
	projection, err := p.reader.Project(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	return pickupBaselineProjection{projection: projection}, nil
}

type pickupBaselineProjection struct {
	projection *careplan.PickupBaselineProjection
}

func (p pickupBaselineProjection) RegularPickup(studentID int64, date timezone.Date) (time.Time, bool) {
	row := p.projection.ForDate(studentID, date)
	if row == nil {
		return time.Time{}, false
	}
	return row.PickupTime, true
}

// timetableRoomOccupancy asks Student Presence whether another open session
// occupies the room.
type timetableRoomOccupancy struct {
	sessions studentpresence.SessionRecords
}

func (o timetableRoomOccupancy) RoomOccupied(ctx context.Context, roomID int64) (bool, error) {
	occupied, _, err := o.sessions.CheckRoomConflict(ctx, roomID, 0)
	return occupied, err
}
