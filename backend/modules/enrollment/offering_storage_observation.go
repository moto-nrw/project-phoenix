package enrollment

import "time"

// OfferingStorageObservation describes an owner call, not a committed workflow.
// A successful owner call can still be rolled back by its enclosing UnitOfWork.
type OfferingStorageObservation struct {
	Operation  string
	Kind       string
	Duration   time.Duration
	InputRows  int64
	OutputRows int64 // -1 when the command does not return an affected-row count.
	Err        error
}

func (m *Module) observeOfferingStorage(operation, kind string, started time.Time, inputRows, outputRows int64, err error) {
	if m.offeringStorageObserver != nil {
		m.offeringStorageObserver(OfferingStorageObservation{Operation: operation, Kind: kind, Duration: time.Since(started), InputRows: inputRows, OutputRows: outputRows, Err: err})
	}
}
