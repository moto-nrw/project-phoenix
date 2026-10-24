package ports

// LifecycleEvent is one realtime notification of the lifecycle. Events are
// refetch triggers, not payloads; the optional fields mirror the event data
// the clients read.
type LifecycleEvent struct {
	Type              string
	ActiveGroupID     string
	InstanceID        *string
	InstanceDate      *string
	InstanceStartTime *string
	RoomID            *string
	RoomName          *string
	ActivityName      *string
	SupervisorIDs     *[]string
	StudentIDs        *[]string
	GroupIDs          *[]string
	Reason            *string
	Source            *string
}

// LifecycleBroadcaster delivers the lifecycle's events; the composition root
// binds the realtime hub.
type LifecycleBroadcaster interface {
	BroadcastToGroup(tenantID int64, topic string, event LifecycleEvent) error
	BroadcastToTenant(tenantID int64, event LifecycleEvent) error
}
