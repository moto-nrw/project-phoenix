package ports

import (
	"context"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
)

// EndedSessionInstances is the slice of the retained activity instance
// repository the completion reads and writes; its rows carry the Student
// Presence session each block runs in.
type EndedSessionInstances interface {
	ListInstances(ctx context.Context, filter LegacyInstanceFilter) ([]*scheduleModels.ActivityInstance, error)
	CompleteActiveByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, completedAt time.Time) (int64, error)
	UpdateColumns(ctx context.Context, instance *scheduleModels.ActivityInstance, columns ...string) (int64, error)
}

// EndedSessionParticipants is the slice of the retained participant
// repository that finalizes the attendance of the ended blocks.
type EndedSessionParticipants interface {
	FindNotScheduledCandidatesByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStudent, error)
	MarkNotScheduled(ctx context.Context, refs []scheduleModels.StudentInstanceRef) error
	MarkExpectedAbsentByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, updatedAt time.Time, exclusions []scheduleModels.StudentInstanceRef) error
}

// EndedSessionCompletionDependencies wire the completion. CareDays is
// optional: without it no child is spared the absent stamp, which is the
// behaviour that predates #1747 — never a silent skip.
type EndedSessionCompletionDependencies struct {
	Instances    EndedSessionInstances
	Participants EndedSessionParticipants
	CareDays     CareDays
}
