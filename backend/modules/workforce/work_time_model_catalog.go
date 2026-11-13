package workforce

import "context"

// WorkTimeModelCatalog serves the school's catalog page of work-time
// templates (#6891): how many people a template reaches and whether it is
// still offered for new assignments.
type WorkTimeModelCatalog interface {
	// WorkTimeModelUsage counts the live staff members bound to each template
	// of the caller's school, the same people a template edit rewrites.
	// Unused templates are absent.
	WorkTimeModelUsage(context.Context) (map[int64]int, error)
	// ArchiveWorkTimeModel stops offering a template for new assignments.
	// Staff already bound to it keep it and its Soll.
	ArchiveWorkTimeModel(context.Context, int64) (WorkTimeModel, error)
	// RestoreWorkTimeModel offers an archived template again.
	RestoreWorkTimeModel(context.Context, int64) (WorkTimeModel, error)
}
