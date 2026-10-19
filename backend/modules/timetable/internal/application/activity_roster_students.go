package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// ActivityStudents reads and locks the People Directory children the activity
// rosters and the operational day name; the root also serves the lifecycle's
// LifecycleStudents from it.
type ActivityStudents interface {
	FindByID(ctx context.Context, id any) (*timetable.OperationStudent, error)
	FindByIDForUpdate(ctx context.Context, id int64) (*timetable.OperationStudent, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.OperationStudent, error)
	FindByIDsForUpdate(ctx context.Context, ids []int64) (map[int64]*timetable.OperationStudent, error)
	FindReadScopeByIDs(ctx context.Context, ids []int64) (map[int64]*timetable.OperationStudent, error)
}
