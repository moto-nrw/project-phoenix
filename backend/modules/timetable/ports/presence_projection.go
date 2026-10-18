package ports

import (
	"context"
	"time"

	presenceContract "github.com/moto-nrw/project-phoenix/modules/presenceprojection/presencecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type PresenceProjection interface {
	CountNonAbsentParticipants(context.Context, []int64) (map[int64]int, error)
	ListParallelPresence(context.Context, int64, calendar.Date, []int64) ([]presenceContract.ParallelPresence, error)
	ListPartialAbsenceBlocks(context.Context, int64, calendar.Date, time.Time, bool, []int64) ([]presenceContract.PartialAbsenceBlock, error)
	ListScheduledInstancesForStudent(context.Context, int64, calendar.Date, calendar.Date) ([]presenceContract.ScheduledInstance, error)
	HasPlannedSlotsInRange(context.Context, calendar.Date, calendar.Date) (bool, error)
	ListLegacyInstances(context.Context, presenceContract.LegacyInstanceFilter) ([]presenceContract.Instance, error)
	ListLegacyParticipants(context.Context, presenceContract.LegacyParticipantFilter) ([]presenceContract.Participant, error)
}
