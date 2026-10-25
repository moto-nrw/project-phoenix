package owners

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	settings "github.com/moto-nrw/project-phoenix/modules/settings"
)

// PortalModuleBooking binds Identity & Access's portal lock (#4895) to the
// module switches of the Settings Platform: the parents portal follows
// Eltern-App, the school portal Unterricht am Vormittag. Every read runs in
// a tenant transaction of its own and never falls back to "booked".
type PortalModuleBooking struct {
	Booking *settings.ModuleBooking
}

// NewPortalModuleBooking binds the lock to the settings service.
func NewPortalModuleBooking(reader settings.ModuleBookingReader) (PortalModuleBooking, error) {
	booking, err := settings.NewModuleBooking(reader)
	if err != nil {
		return PortalModuleBooking{}, err
	}
	return PortalModuleBooking{Booking: booking}, nil
}

// BookedParentPortalSchools keeps the schools that booked Eltern-App.
func (b PortalModuleBooking) BookedParentPortalSchools(ctx context.Context, tenantIDs []int64) ([]int64, error) {
	booked := make([]int64, 0, len(tenantIDs))
	for _, tenantID := range tenantIDs {
		err := b.require(ctx, tenantID, settings.ModuleElternApp)
		if _, notBooked := identityaccess.PortalNotBooked(err); notBooked {
			continue
		}
		if err != nil {
			return nil, err
		}
		booked = append(booked, tenantID)
	}
	return booked, nil
}

// RequireParentPortal refuses a school without Eltern-App.
func (b PortalModuleBooking) RequireParentPortal(ctx context.Context, tenantID int64) error {
	return b.require(ctx, tenantID, settings.ModuleElternApp)
}

// RequireSchoolPortal refuses a school without Unterricht am Vormittag.
func (b PortalModuleBooking) RequireSchoolPortal(ctx context.Context, tenantID int64) error {
	return b.require(ctx, tenantID, settings.ModuleUnterrichtVormittag)
}

func (b PortalModuleBooking) require(ctx context.Context, tenantID int64, module settings.ModuleKey) error {
	if b.Booking == nil {
		return errors.New("portal module booking is not composed")
	}
	err := b.Booking.RequireForTenant(ctx, tenantID, module)
	var notBooked *settings.ModuleNotBookedError
	if !errors.As(err, &notBooked) {
		return err
	}
	refusal := &identityaccess.PortalNotBookedError{Module: string(notBooked.Module)}
	if definition, ok := settings.LookupModule(notBooked.Module); ok {
		refusal.ModuleLabel = definition.Label
	}
	return refusal
}
