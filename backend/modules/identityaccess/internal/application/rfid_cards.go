package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
)

type RFIDCards struct {
	service *Service
	store   ports.RFIDCardStore
}

func NewRFIDCards(service *Service, store ports.RFIDCardStore) *RFIDCards {
	return &RFIDCards{service: service, store: store}
}

func (r *RFIDCards) LookupRFIDCard(ctx context.Context, tag string) (card domain.RFIDCard, found bool, err error) {
	err = r.service.run(ctx, r.service.withinSchoolRead, "lookup_rfid_card", func(txCtx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		card, found, queryStats, queryErr = r.store.LookupRFIDCard(txCtx, domain.NormalizeTagID(tag), r.service.tenantOf(txCtx))
		stats.Add(queryStats)
		return queryErr
	})
	return card, found, err
}

func (r *RFIDCards) RegisterRFIDCard(ctx context.Context, tag string) error {
	card := domain.RFIDCard{ID: tag, Active: true}
	if err := card.Validate(); err != nil {
		return err
	}
	return r.service.run(ctx, r.withinSchoolWrite, "register_rfid_card", func(txCtx context.Context, stats *domain.OperationStats) error {
		queryStats, err := r.store.RegisterRFIDCard(txCtx, card, r.service.tenantOf(txCtx))
		stats.Add(queryStats)
		return err
	})
}

func (r *RFIDCards) withinSchoolWrite(ctx context.Context, fn func(context.Context) error) error {
	if r.service.tenantOf(ctx) <= 0 {
		return domain.ErrTenantRequired
	}
	return r.service.tx.RunWrite(ctx, fn)
}
