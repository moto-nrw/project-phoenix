package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

type ReleasePushStore interface {
	ValidReleasePushes(context.Context, int64, []organizationtenancy.ReleasePushCandidate) (map[int64]bool, error)
}
type ReleasePushValidity struct{ store ReleasePushStore }

func NewReleasePushValidity(store ReleasePushStore) *ReleasePushValidity {
	if store == nil {
		panic("organization tenancy: release push store is required")
	}
	return &ReleasePushValidity{store: store}
}
func (p *ReleasePushValidity) ValidReleasePushes(ctx context.Context, organizationID int64, candidates []organizationtenancy.ReleasePushCandidate) (map[int64]bool, error) {
	if organizationID <= 0 {
		return nil, errors.New("organization tenancy: release push requires a Träger")
	}
	if len(candidates) == 0 {
		return map[int64]bool{}, nil
	}
	return p.store.ValidReleasePushes(ctx, organizationID, candidates)
}
