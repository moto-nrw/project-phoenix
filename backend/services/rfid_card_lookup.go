package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/iot/staffclock"
	"github.com/moto-nrw/project-phoenix/services/users"
)

// rfidCard is what the staff clock needs to know about an identity-access
// card. The root never names that owner's model (#2662): the repository's
// return type satisfies this interface through the base string-id model.
type rfidCard interface {
	GetID() any
	IsActive() bool
}

// rfidCardLookup adapts the identity-access card repository to the narrow
// port the staff clock reads. The type parameter is inferred from the
// repository method, so no identity-access import is needed here. It also
// owns the tag normalization, which belongs to the card's owner and must not
// leak into the workflow.
type rfidCardLookup[C interface {
	comparable
	rfidCard
}] struct {
	find func(context.Context, string) (C, error)
}

func newRFIDCardLookup[C interface {
	comparable
	rfidCard
}](find func(context.Context, string) (C, error)) rfidCardLookup[C] {
	return rfidCardLookup[C]{find: find}
}

func (l rfidCardLookup[C]) FindCard(ctx context.Context, rawTag string) (*staffclock.Card, error) {
	normalized := userModels.NormalizeTagID(rawTag)
	if err := userModels.ValidateTagID(normalized); err != nil {
		return nil, fmt.Errorf("%w: %v", staffclock.ErrInvalidRFIDTag, err)
	}
	card, err := l.find(ctx, normalized)
	var missing C
	if err != nil || card == missing {
		return nil, err
	}
	cardID, _ := card.GetID().(string)
	return &staffclock.Card{ID: cardID, Active: card.IsActive()}, nil
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
