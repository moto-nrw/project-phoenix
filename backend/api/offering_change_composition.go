package api

import (
	"context"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
)

// Care Plan's offering-change review (#3561) reads and writes its own
// requests, Enrollment's requests, children, phases and bookings, People
// Directory students, the settings, the Timetable planning and the
// Communication side through its own ports. The adapters below bind them over
// the owners' capabilities with the queries and error shapes the retained
// repositories had; they decide nothing.

var _ carePlanCompose.RequestLedger = (*owners.ParentRequestLedger)(nil)

type BookingSettingsReads interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
}
