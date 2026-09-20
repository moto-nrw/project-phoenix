package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/services/iot/staffclock"
	"github.com/moto-nrw/project-phoenix/services/users"
)

// rfidCardLookup maps the owner's validated card facts to the staff-clock port.
type rfidCardLookup struct{ cards identityaccess.RFIDCards }

func (l rfidCardLookup) FindCard(ctx context.Context, rawTag string) (*staffclock.Card, error) {
	if err := l.cards.ValidateRFIDTag(rawTag); err != nil {
		return nil, fmt.Errorf("%w: %v", staffclock.ErrInvalidRFIDTag, err)
	}
	id, active, found, err := l.cards.LookupRFIDCard(ctx, rawTag)
	if err != nil || !found {
		return nil, err
	}
	return &staffclock.Card{ID: id, Active: active}, nil
}

// staffClockStaffLookup resolves the staff member behind a normalized tag
// through the retained person service. It answers the two stable kiosk
// outcomes the workflow classifies on and lets every other failure through.
type staffClockStaffLookup struct {
	people users.PersonService
}

// StaffClockStaffLookup binds the staff resolution of the kiosk to the person
// service.
func StaffClockStaffLookup(people users.PersonService) staffclock.StaffLookup {
	if people == nil {
		panic("staff clock staff lookup: person service is required")
	}
	return staffClockStaffLookup{people: people}
}

func (l staffClockStaffLookup) ResolveStaffByTag(ctx context.Context, tag string) (staffclock.StaffIdentity, error) {
	person, err := l.people.FindByTagID(ctx, tag)
	if err != nil {
		if errors.Is(err, users.ErrPersonNotFound) {
			return staffclock.StaffIdentity{}, staffclock.ErrRFIDTagNotFound
		}
		return staffclock.StaffIdentity{}, fmt.Errorf("look up person by RFID tag: %w", err)
	}
	// An active card that is not linked to anybody resolves to no person. The
	// kiosk gets the same stable "unknown tag" answer it gets for a card the
	// system has never seen.
	if person == nil {
		return staffclock.StaffIdentity{}, staffclock.ErrRFIDTagNotFound
	}
	staff, err := l.people.GetStaffByPersonID(ctx, person.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return staffclock.StaffIdentity{}, staffclock.ErrRFIDTagNotStaff
		}
		return staffclock.StaffIdentity{}, err
	}
	if staff == nil {
		return staffclock.StaffIdentity{}, staffclock.ErrRFIDTagNotStaff
	}
	return staffclock.StaffIdentity{StaffID: staff.ID, FullName: person.GetFullName()}, nil
}
