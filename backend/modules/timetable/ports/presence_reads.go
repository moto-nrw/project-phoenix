package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
)

// LegacyInstanceReads list the activity instances with the Student Presence
// session each one runs in, joined in one tenant-safe read.
type LegacyInstanceReads interface {
	ListLegacyInstances(ctx context.Context, filter LegacyInstanceFilter) ([]*scheduleModels.ActivityInstance, error)
}

// InstanceSessions are the Student Presence sessions the blocks run in since
// the presence cutover (#2762).
type InstanceSessions interface {
	studentpresence.ActivitySessionQuery
	studentpresence.ActivitySessionCommand
}

// InstanceRows are the activity instances: the owner plans them, Student
// Presence runs their sessions, and the joined read returns both.
type InstanceRows struct {
	Plans    timetable.ActivityInstanceCapability
	Sessions InstanceSessions
	Reads    LegacyInstanceReads
}

// LegacyInstanceFilter selects activity instances the way the retained
// repositories ask for them. Status uses the legacy vocabulary: planned is a
// plan nobody started, active and completed are session states, cancelled is
// a plan state.
type LegacyInstanceFilter struct {
	IDs                 []int64
	Date                *string
	Dates               []string
	FromDate            *string
	ToDate              *string
	ActivityGroupID     *int64
	ActivityGroupIDs    []int64
	ActiveGroupIDs      []int64
	Status              string
	IsSpontaneous       *bool
	IdempotencyKey      string
	MaterializedPlanned bool
	OrderByDateAndTime  bool
	Limit               int
	Offset              int
}

// LegacyParticipantFilter selects participants with their attendance the way
// the retained repositories ask for them. Status filters the attendance
// status; ExcludeFinished drops participants of cancelled or ended blocks.
type LegacyParticipantFilter struct {
	IDs                        []int64
	InstanceIDs                []int64
	StudentIDs                 []int64
	Status                     *string
	Date                       *string
	FromDate                   *string
	ToDate                     *string
	CurrentTime                *string
	ExcludeFinished            bool
	OrderByCreated             bool
	OrderByInstanceStudent     bool
	OrderByStudentActivityTime bool
	OrderByActivityDateTime    bool
	Limit                      int
	Offset                     int
}
