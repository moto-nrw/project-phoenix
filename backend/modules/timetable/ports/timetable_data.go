package ports

import (
	"context"
	"log/slog"
	"time"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activitiesModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/activityrecords"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// DataInstances reads blocks from the retained activity instance repository.
type DataInstances interface {
	FindByID(ctx context.Context, id int64) (*scheduleModels.ActivityInstance, error)
	FindByTenantAndDateRange(ctx context.Context, from, to calendar.Date) ([]*scheduleModels.ActivityInstance, error)
}

// DataParticipants reads and patches the participant rows of blocks.
type DataParticipants interface {
	FindByInstanceID(ctx context.Context, instanceID int64) ([]*scheduleModels.InstanceStudent, error)
	FindByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStudent, error)
	FindNotScheduledCandidatesByInstanceIDs(ctx context.Context, instanceIDs []int64) ([]*scheduleModels.InstanceStudent, error)
	FindByInstanceAndStudent(ctx context.Context, instanceID, studentID int64) (*scheduleModels.InstanceStudent, error)
	FindInstancesWithAttendanceByStudentAndDateRange(ctx context.Context, studentID int64, from, to calendar.Date) ([]*scheduleModels.ScheduledInstanceRow, error)
	UpdateAttendanceFields(ctx context.Context, id int64, patch scheduleModels.AttendanceFieldPatch) error
}

// DataPickupExceptions reads the dated pickup exceptions of children.
type DataPickupExceptions interface {
	FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date calendar.Date) ([]*scheduleModels.StudentPickupException, error)
	FindByStudentIDAndDateRange(ctx context.Context, studentID int64, from, to calendar.Date) ([]*scheduleModels.StudentPickupException, error)
}

// DataArrivalExceptions reads the dated arrival exceptions of a child.
type DataArrivalExceptions interface {
	FindByStudentIDAndDateRange(ctx context.Context, studentID int64, from, to calendar.Date) ([]*scheduleModels.StudentArrivalException, error)
}

// PickupBaselineProjector is the consumer-owned port to Care Plan's regular
// pickup plan over an inclusive window (#2414, ADR 0005).
type PickupBaselineProjector interface {
	ProjectPickups(ctx context.Context, studentIDs []int64, from, to calendar.Date) (PickupBaselines, error)
}

// PickupBaselines answers the regular pickup of a student on a date; ok is
// false when no recurring pickup applies.
type PickupBaselines interface {
	RegularPickup(studentID int64, date calendar.Date) (pickup time.Time, ok bool)
}

// DataTemplates reads the Vorlagen list and writes the activity a
// spontaneous start needs, through the retained activity group repository.
type DataTemplates interface {
	ListTemplateRows(ctx context.Context, templateID *int64) ([]activitiesModels.TemplateListRow, error)
	ListTemplateRowsForTemplatePeriod(ctx context.Context, templateID, periodID int64) ([]activitiesModels.TemplateListRow, error)
	ListTemplateRowsForPeriod(ctx context.Context, periodID *int64) ([]activitiesModels.TemplateListRow, error)
	ListTemplateWeekdayRoster(ctx context.Context, templateID, calendarPeriodID *int64) ([]timetable.TemplateWeekdayRosterRow, error)
	ListTemplateCapacityOccurrences(ctx context.Context, periodID *int64, templateIDs []int64) ([]activitiesModels.TemplateCapacityOccurrence, error)
	FindTargetsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]*activityContract.GroupTarget, error)
	FindTargetStudentIDsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64][]int64, error)
	FindByName(ctx context.Context, name string) (*activityContract.Group, error)
	Create(ctx context.Context, group *activityContract.Group) error
}

// DataGroups reads the template a block was materialized from through the
// owner's group capability.
type DataGroups interface {
	FindGroup(ctx context.Context, id int64) (timetable.Group, error)
}

// BlockDisplayMetadata reads the display data for a list of scheduled blocks.
type BlockDisplayMetadata interface {
	ListBlockDisplayMetadata(ctx context.Context, instanceIDs []int64) (map[int64]timetable.BlockDisplayMetadata, error)
}

// DataCategories reads and creates the activity categories.
type DataCategories interface {
	FindByNameIncludingArchivedForShare(ctx context.Context, name string) (*activityContract.Category, error)
	Create(ctx context.Context, category *activityContract.Category) error
}

// RoomOccupancy reports another open Student Presence session in a room.
type RoomOccupancy interface {
	RoomOccupied(ctx context.Context, roomID int64) (bool, error)
}

// DeviationEventReader is the consumer-owned port to the Audit Platform's
// Änderungsprotokoll (#1886).
type DeviationEventReader interface {
	ListDeviationEvents(ctx context.Context, from, to calendar.Date, activityGroupID *int64, startTime *string) ([]timetable.DeviationEvent, error)
}

// AdvisoryLocks takes transaction-scoped advisory locks through the
// Transaction Runtime.
type AdvisoryLocks interface {
	AcquireXactLock(ctx context.Context, key string) error
}

// TimetableDataDependencies wires the planner's reads. Locks, Advisory and
// BlockMetadata are optional: without them attendance patches skip the
// completion lock, spontaneous-start locks are skipped outside a transaction
// only and callers that do not render blocks receive no display metadata.
// Every other collaborator is required.
type TimetableDataDependencies struct {
	Instances         DataInstances
	InstanceStaff     OperationInstanceStaff
	Participants      DataParticipants
	PickupExceptions  DataPickupExceptions
	ArrivalExceptions DataArrivalExceptions
	ArrivalBaselines  ArrivalBaselineProjector
	PickupBaselines   PickupBaselineProjector
	Visits            OperationVisits
	Templates         DataTemplates
	Groups            DataGroups
	BlockMetadata     BlockDisplayMetadata
	Categories        DataCategories
	Rooms             RoomNames
	RoomOccupancy     RoomOccupancy
	DeviationEvents   DeviationEventReader
	ConflictAcks      timetable.ConflictAckCapability
	Locks             AttendanceLocks
	Advisory          AdvisoryLocks
	Logger            *slog.Logger
}
