package domain

import "errors"

var (
	ErrPickupExtensionNotFound  = errors.New("pickup extension task not found")
	ErrPickupExtensionBlockGone = errors.New("pickup extension block is no longer available")
)

// PickupExtensionTask is one stored later-pickup decision (#3261). Exactly
// one of Date (day task) or Weekday/EffectiveFrom (weekday task) is set.
// Times are wall-clock "HH:MM".
type PickupExtensionTask struct {
	ID                int64
	StudentID         int64
	PickupExceptionID *int64
	Date              string
	Weekday           int
	EffectiveFrom     string
	PreviousPickup    string
	Pickup            string
}

func (t PickupExtensionTask) IsDay() bool { return t.Date != "" }

// PickupExtensionBlock is a block with children that overlaps the extra time
// of one task. Member reports whether the child is already on it. For day
// tasks ID is the activity instance, for weekday tasks the template.
type PickupExtensionBlock struct {
	TaskID           int64
	ID               int64
	Title            string
	StartTime        string
	EndTime          string
	Member           bool
	CalendarPeriodID *int64
	ValidFrom        string
}

// PickupExtensionInstance is an already planned block of a template on the
// task's weekday that does not list the child yet.
type PickupExtensionInstance struct {
	ID   int64
	Date string
}

// OpenPickupExtensionBlocks decides which blocks the Leitung can still pick.
// A child already on a block that runs until the new pickup time is looked
// after, so the task is not open. Otherwise every block the child is not on
// is a choice, in time order as given.
func OpenPickupExtensionBlocks(blocks []PickupExtensionBlock, pickup string) []PickupExtensionBlock {
	result := make([]PickupExtensionBlock, 0, len(blocks))
	for _, block := range blocks {
		if block.Member && block.EndTime >= pickup {
			return nil
		}
		if !block.Member {
			result = append(result, block)
		}
	}
	return result
}
