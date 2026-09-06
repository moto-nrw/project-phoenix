package enrollment

import "context"

// ManualPlanningOccurrence is a planned slot without a care-offering source.
// Date is a calendar date in YYYY-MM-DD form.
type ManualPlanningOccurrence struct {
	ActivityGroupID   int64
	ActivityGroupName string
	InstanceID        int64
	Date              string
}

type ManualPlanningReader interface {
	ListManualPlanningOccurrences(context.Context, int64, string, string) ([]ManualPlanningOccurrence, error)
}
