package careplanhttp

import (
	context "context"

	calendar "github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// GuardianWake is the consumer-owned port to Communication (#3356): the
// message-independent parent_child_updated fan-out to a child's guardians
// after a staff-side care write, so an open parents-app tab refetches the
// child's care state live (#1725, docs/agents/realtime.md). The root binds
// the Communication parent event emitter.
type GuardianWake interface {
	BroadcastChildUpdateToGuardians(tenantID, studentID int64)
}

// OfferingSourceResyncer re-reconciles Jahrgang-filtered offering-sourced
// Regeltermine from a date on, the same hook a grade transition uses (#2147).
// The root binds Care Plan's booking materialization (#3560).
type OfferingSourceResyncer interface {
	ResyncOfferingSourcedTemplates(ctx context.Context, effectiveFrom calendar.Date) error
}

// TargetCohortChange is one child's Klasse or Gruppe write: what changed and
// the value before it (#6841).
type TargetCohortChange struct {
	StudentID           int64
	SchoolClassChanged  bool
	PreviousSchoolClass string
	GroupChanged        bool
	PreviousGroupID     *int64
}

// FollowCohortChanges moves children who changed Klasse or Gruppe between the
// materialized occurrences of Regeltermine with a Klasse, Jahrgang or Gruppe
// target from the given day on (#6841). The root binds Timetable's cohort
// rosters.
type FollowCohortChanges func(ctx context.Context, changes []TargetCohortChange, from calendar.Date) error
