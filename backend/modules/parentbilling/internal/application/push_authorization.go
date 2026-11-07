package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

type DocumentPushStore interface {
	ValidDocumentPushes(context.Context, int64, []parentbilling.DocumentPushCandidate) (map[int64]bool, error)
}

type DocumentPushValidity struct{ store DocumentPushStore }

func NewDocumentPushValidity(store DocumentPushStore) *DocumentPushValidity {
	if store == nil {
		panic("parent billing: document push store is required")
	}
	return &DocumentPushValidity{store: store}
}

func (p *DocumentPushValidity) ValidDocumentPushes(ctx context.Context, organizationID int64, candidates []parentbilling.DocumentPushCandidate) (map[int64]bool, error) {
	if organizationID <= 0 {
		return nil, errors.New("parent billing: document push requires a Träger")
	}
	if len(candidates) == 0 {
		return map[int64]bool{}, nil
	}
	return p.store.ValidDocumentPushes(ctx, organizationID, candidates)
}
