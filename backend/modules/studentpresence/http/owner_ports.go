package studentpresencehttp

// GuardianWake is the consumer-owned port to Communication (#3356): the
// message-independent parent_child_updated fan-out to a child's guardians
// after a staff-side care write, so an open parents-app tab refetches the
// child's care state live (#1725, docs/agents/realtime.md). The root binds
// the Communication parent event emitter.
type GuardianWake interface {
	BroadcastChildUpdateToGuardians(tenantID, studentID int64)
}
