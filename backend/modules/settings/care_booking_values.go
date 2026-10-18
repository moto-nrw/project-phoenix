package settings

import (
	"context"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// CareBookingSource is the resolver slice the care booking switches read
// through.
type CareBookingSource interface {
	ResolveBool(ctx context.Context, key string) (bool, error)
}

// CareBookingValues are the two switches the booking-led care flows read.
// Without a resolver the registry defaults apply: the care offering catalog
// is on and the bookings do not decide the care days.
type CareBookingValues struct{ source CareBookingSource }

// NewCareBookingValues binds the switches to the resolver; a nil source
// serves the registry defaults.
func NewCareBookingValues(source CareBookingSource) CareBookingValues {
	return CareBookingValues{source: source}
}

func (v CareBookingValues) CareOfferingsEnabled(ctx context.Context) (bool, error) {
	if v.source == nil {
		return true, nil
	}
	return v.source.ResolveBool(ctx, configModel.KeyEnrollmentCareOfferingsEnabled)
}

func (v CareBookingValues) BookingsAuthoritative(ctx context.Context) (bool, error) {
	if v.source == nil {
		return false, nil
	}
	return v.source.ResolveBool(ctx, configModel.KeyEnrollmentBookingsAuthoritative)
}
