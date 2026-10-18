package api

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewTimetableData composes the planner's reads of the Timetable owner and
// binds the collaborators it may not name itself: Care Plan's baselines, the
// Student Presence room occupancy and the tenant runtime's advisory locks;
// the retained rows bind the Facilities rooms and the Audit Platform's
// Änderungsprotokoll.
func NewTimetableData(in TimetableDataInputs) (timetable.TimetableDataCapability, error) {
	templates, err := in.Rows.DataTemplates()
	if err != nil {
		return nil, err
	}
	return timetableCompose.NewTimetableData(timetablePorts.TimetableDataDependencies{
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
		BlockMetadata:     in.BlockMetadata,
		Categories:        in.Rows.Categories,
		Rooms:             in.Rows.Rooms,
		RoomOccupancy:     TimetableRoomOccupancy{Sessions: in.Sessions},
		DeviationEvents:   in.Rows.DeviationEvents,
		ConflictAcks:      in.ConflictAcks,
		Locks:             in.Rows.Locks,
		Advisory:          newTenantAdvisoryLocks(in.Transactional),
		Logger:            in.Logger,
	})
}

func newArrivalBaselineProjector(reader careplan.ArrivalBaselineReader) timetablePorts.ArrivalBaselineProjector {
	if reader == nil {
		return nil
	}
	return ArrivalBaselineProjector{Reader: reader}
}

func newPickupBaselineProjector(reader careplan.PickupBaselineReader) timetablePorts.PickupBaselineProjector {
	if reader == nil {
		return nil
	}
	return PickupBaselineProjector{Reader: reader}
}

func newTenantAdvisoryLocks(transactional bool) timetablePorts.AdvisoryLocks {
	if !transactional {
		return nil
	}
	return timetableAdvisoryLocks{}
}

// PickupBaselineProjector serves the Timetable pickup port from Care Plan's
// baseline projection.
type PickupBaselineProjector struct {
	Reader careplan.PickupBaselineReader
}

func (p PickupBaselineProjector) ProjectPickups(ctx context.Context, studentIDs []int64, from, to timezone.Date) (timetablePorts.PickupBaselines, error) {
	projection, err := p.Reader.Project(ctx, studentIDs, from, to)
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

// TimetableRoomOccupancy asks Student Presence whether another open session
// occupies the room.
type TimetableRoomOccupancy struct {
	Sessions studentpresence.SessionRecords
}

func (o TimetableRoomOccupancy) RoomOccupied(ctx context.Context, roomID int64) (bool, error) {
	occupied, _, err := o.Sessions.CheckRoomConflict(ctx, roomID, 0)
	return occupied, err
}
