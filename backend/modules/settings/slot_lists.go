package settings

import "context"

// SlotListSettings supplies the named settings the class-day lists consume.
type SlotListSettings interface {
	TimetableEnabled(context.Context) (bool, error)
	PickupCutoffs(context.Context) (short, long string, err error)
}
