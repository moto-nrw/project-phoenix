package api

import (
	"context"
	"log/slog"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NewTimetableData composes the planner's reads of the Timetable owner
// (#3551) over the retained rows and binds the collaborators it may not name
// itself: Care Plan's baselines, the Student Presence room occupancy and the
// tenant runtime's advisory locks; the retained rows bind the Facilities
// rooms and the Audit Platform's Änderungsprotokoll. arrivalBaselines and
// pickupBaselines are optional; blockMetadata is the block display
// projection the composition root binds; transactional binds the tenant
// runtime's advisory locks a spontaneous start takes.
func NewTimetableData(
	rows timetableCompose.OwnerRows, arrivalBaselines careplan.ArrivalBaselineReader, pickupBaselines careplan.PickupBaselineReader,
	visits timetablePorts.OperationVisits, groups timetablePorts.DataGroups, sessions studentpresence.SessionRecords,
	conflictAcks timetable.ConflictAckCapability, blockMetadata timetablePorts.BlockDisplayMetadata, transactional bool, logger *slog.Logger,
) (timetable.TimetableDataCapability, error) {
	return timetableCompose.NewTimetableData(timetablePorts.TimetableDataDependencies{
		Instances:         rows.Instances.Reads,
		InstanceStaff:     rows.InstanceStaff,
		Participants:      rows.Participants,
		PickupExceptions:  rows.PickupExceptions,
		ArrivalExceptions: rows.ArrivalExceptions,
		ArrivalBaselines:  newArrivalBaselineProjector(arrivalBaselines),
		PickupBaselines:   newPickupBaselineProjector(pickupBaselines),
		Visits:            visits,
		Templates:         rows.Templates,
		Directories:       rows.Directories,
		Groups:            groups,
		BlockMetadata:     blockMetadata,
		Categories:        rows.Categories,
		Rooms:             rows.Rooms,
		RoomOccupancy:     TimetableRoomOccupancy{Sessions: sessions},
		DeviationEvents:   rows.DeviationEvents,
		ConflictAcks:      conflictAcks,
		Locks:             rows.Locks,
		Advisory:          newTenantAdvisoryLocks(transactional),
		Logger:            logger,
	})
}

func newArrivalBaselineProjector(reader careplan.ArrivalBaselineReader) timetablePorts.ArrivalBaselineProjector {
	if reader == nil {
		return nil
	}
	return owners.ArrivalBaselineProjector{Reader: reader}
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

func (o TimetableRoomOccupancy) SpontaneousRoomOccupied(ctx context.Context, roomID int64) (bool, error) {
	occupied, _, err := o.Sessions.CheckRoomConflict(ctx, roomID, 0)
	return occupied, err
}
