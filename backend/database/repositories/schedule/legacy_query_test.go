package schedule_test

import (
	"context"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
)

type queryListRepository[T any] interface {
	List(context.Context, *modelBase.QueryOptions) ([]T, error)
}

type arrivalScheduleQueryRepository interface {
	scheduleModels.StudentArrivalScheduleRepository
	queryListRepository[*scheduleModels.StudentArrivalSchedule]
}

type arrivalExceptionQueryRepository interface {
	scheduleModels.StudentArrivalExceptionRepository
	queryListRepository[*scheduleModels.StudentArrivalException]
}

type arrivalNoteQueryRepository interface {
	scheduleModels.StudentArrivalNoteRepository
	queryListRepository[*scheduleModels.StudentArrivalNote]
}

type pickupScheduleQueryRepository interface {
	scheduleModels.StudentPickupScheduleRepository
	queryListRepository[*scheduleModels.StudentPickupSchedule]
}

type pickupExceptionQueryRepository interface {
	scheduleModels.StudentPickupExceptionRepository
	queryListRepository[*scheduleModels.StudentPickupException]
}

type pickupNoteQueryRepository interface {
	scheduleModels.StudentPickupNoteRepository
	queryListRepository[*scheduleModels.StudentPickupNote]
}

type instanceStaffQueryRepository interface {
	scheduleModels.InstanceStaffRepository
	queryListRepository[*scheduleModels.InstanceStaff]
}

type activityExceptionQueryRepository interface {
	scheduleModels.ActivityExceptionRepository
	queryListRepository[*scheduleModels.ActivityException]
}

type dateframeQueryRepository interface {
	scheduleModels.DateframeRepository
	queryListRepository[*scheduleModels.Dateframe]
}

type recurrenceRuleQueryRepository interface {
	scheduleModels.RecurrenceRuleRepository
	queryListRepository[*scheduleModels.RecurrenceRule]
}

type timeframeQueryRepository interface {
	scheduleModels.TimeframeRepository
	queryListRepository[*scheduleModels.Timeframe]
}
