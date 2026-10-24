package application

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (m *Service) observeOfferingStorage(operation, kind string, started time.Time, inputRows, outputRows int64, err error) {
	if m.offeringStorageObserver != nil {
		m.offeringStorageObserver(enrollment.OfferingStorageObservation{Operation: operation, Kind: kind, Duration: time.Since(started), InputRows: inputRows, OutputRows: outputRows, Err: err})
	}
}
