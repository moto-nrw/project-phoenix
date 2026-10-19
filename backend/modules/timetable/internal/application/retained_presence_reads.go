package application

import (
	"context"

	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// PresenceReads are the tenant-safe reads that join the Student Presence
// session and attendance onto the Timetable plan. The retained instance and
// participant adapters read through them; the composition binds them to the
// presence projection over the database (#2762, #4084).
type PresenceReads interface {
	CountNonAbsentParticipants(ctx context.Context, instanceIDs []int64) (map[int64]int, error)
	ListParallelPresence(ctx context.Context, excludedInstanceID int64, date string, studentIDs []int64) ([]scheduleModels.ParallelPresence, error)
	ListScheduledInstancesForStudent(ctx context.Context, studentID int64, from, to string) ([]*scheduleModels.ScheduledInstanceRow, error)
	HasPlannedSlotsInRange(ctx context.Context, from, to string) (bool, error)
	ListLegacyInstances(ctx context.Context, filter ports.LegacyInstanceFilter) ([]*scheduleModels.ActivityInstance, error)
	ListLegacyParticipants(ctx context.Context, filter ports.LegacyParticipantFilter) ([]*scheduleModels.InstanceStudent, error)
}
