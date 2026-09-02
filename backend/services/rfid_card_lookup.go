package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/services/iot/staffclock"
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
// repository method, so no identity-access import is needed here.
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

func (l rfidCardLookup[C]) FindCard(ctx context.Context, id string) (*staffclock.Card, error) {
	card, err := l.find(ctx, id)
	var missing C
	if err != nil || card == missing {
		return nil, err
	}
	cardID, _ := card.GetID().(string)
	return &staffclock.Card{ID: cardID, Active: card.IsActive()}, nil
}
