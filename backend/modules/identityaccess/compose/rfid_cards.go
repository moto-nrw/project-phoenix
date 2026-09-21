package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

func (e engine) LookupRFIDCard(ctx context.Context, tag string) (string, bool, bool, error) {
	card, found, err := e.rfidCards.LookupRFIDCard(ctx, tag)
	return card.ID, card.Active, found, mapError(err)
}

func (e engine) RegisterRFIDCard(ctx context.Context, tag string) error {
	return mapError(e.rfidCards.RegisterRFIDCard(ctx, tag))
}

func (e engine) ValidateRFIDTag(tag string) error {
	card := domain.RFIDCard{ID: domain.NormalizeTagID(tag)}
	return card.Validate()
}
