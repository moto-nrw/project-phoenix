package domain

import "errors"

// Date is a calendar day in the Timetable domain. The module's public
// capability owns string API boundaries and cannot import internal/timezone
// under the architecture policy, so composition converts validated ISO dates
// into this distinct domain value.
type Date string

func (d Date) IsZero() bool   { return d == "" }
func (d Date) String() string { return string(d) }

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
	Date              Date
	Weekday           int
	EffectiveFrom     Date
	PreviousPickup    string
	Pickup            string
}

func (t PickupExtensionTask) IsDay() bool { return !t.Date.IsZero() }

// PickupExtensionBlock is a block with children that overlaps the extra time
// of one task. Member reports whether the child is already on it. For day
// tasks ID is the activity instance, for weekday tasks the template.
type PickupExtensionBlock struct {
	TaskID    int64
	ID        int64
	Title     string
	StartTime string
	EndTime   string
	Member    bool
	// OwnParticipantID is the child's own roster row on the block, when any.
	OwnParticipantID *int64
	// ParticipantIDs are the roster rows of a day block; weekday templates
	// leave it empty.
	ParticipantIDs   []int64
	CalendarPeriodID *int64
	ValidFrom        Date
	ValidUntil       *Date
}

// PickupExtensionInstance is an already planned block of a template on the
// task's weekday that does not list the child yet.
type PickupExtensionInstance struct {
	ID   int64
	Date Date
	// OwnParticipantID is the child's roster row on the block, when any.
	OwnParticipantID *int64
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
