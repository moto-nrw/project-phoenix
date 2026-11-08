package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

type DocumentPushStore interface {
	ValidContractPushes(context.Context, int64, []carecontract.DocumentPushCandidate) (map[int64]bool, error)
}
type DocumentPushValidity struct{ store DocumentPushStore }

func NewDocumentPushValidity(store DocumentPushStore) *DocumentPushValidity {
	if store == nil {
		panic("care contract: document push store is required")
	}
	return &DocumentPushValidity{store: store}
}
func (p *DocumentPushValidity) ValidContractPushes(ctx context.Context, organizationID int64, candidates []carecontract.DocumentPushCandidate) (map[int64]bool, error) {
	if organizationID <= 0 {
		return nil, errors.New("care contract: document push requires a Träger")
	}
	if len(candidates) == 0 {
		return map[int64]bool{}, nil
	}
	return p.store.ValidContractPushes(ctx, organizationID, candidates)
}
