package enrollment

import "time"

// OfferingChangeQueueFilters is the consumer's query for its offering-change queue.
// Empty student filters and a nonpositive limit leave the queue unrestricted.
type OfferingChangeQueueFilters struct {
	UrgentOnly    *bool
	UrgentDate    string
	StudentIDs    []int64
	StudentID     int64
	Search        string
	BeforeInstant time.Time
	BeforeID      int64
	Limit         int
}
