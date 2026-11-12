package careplan

import "context"

// ExceptionBatch changes sets of dated exceptions atomically. Deletions
// address only the supplied IDs; additions keep existing source precedence.
type ExceptionBatch struct {
	Arrivals                      []ArrivalException
	Pickups                       []PickupException
	DeleteArrivals, DeletePickups []int64
}

// ExceptionBatchCommands validates the complete batch before writing in the
// caller's tenant transaction, or opens one when invoked at the top level.
type ExceptionBatchCommands interface {
	ApplyExceptionBatch(context.Context, ExceptionBatch) error
}
