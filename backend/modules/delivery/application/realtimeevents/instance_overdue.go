package realtimeevents

import (
	"strconv"

	"github.com/moto-nrw/project-phoenix/realtime"
)

// OverdueInstance names a planned timetable block whose start lies past the
// school's overdue threshold. StartTime is the wall clock as HH:MM:SS.
type OverdueInstance struct {
	InstanceID int64
	Date       string
	StartTime  string
	RoomID     int64
}

// PublishInstanceOverdue broadcasts instance_overdue to the whole school. A
// planned block has no active group yet, so there is no group topic to route
// it through; the dashboards pick it up tenant-wide.
func PublishInstanceOverdue(broadcaster Publisher, tenantID int64, instance OverdueInstance) error {
	instanceID := strconv.FormatInt(instance.InstanceID, 10)
	date := instance.Date
	startTime := instance.StartTime
	roomID := strconv.FormatInt(instance.RoomID, 10)
	return broadcaster.BroadcastToTenant(tenantID, realtime.NewEvent(realtime.EventInstanceOverdue, "", realtime.EventData{
		InstanceID:        &instanceID,
		InstanceDate:      &date,
		InstanceStartTime: &startTime,
		RoomID:            &roomID,
	}))
}

// PublishInstanceOverdueRefresh asks the live-supervision views of the school
// to refetch because a planned block became overdue.
func PublishInstanceOverdueRefresh(broadcaster Publisher, tenantID, instanceID int64) error {
	id := strconv.FormatInt(instanceID, 10)
	reason := "instance_overdue"
	return broadcaster.BroadcastToTenant(tenantID, realtime.NewEvent(realtime.EventActiveSupervisionChanged, "", realtime.EventData{
		InstanceID: &id,
		Reason:     &reason,
	}))
}
