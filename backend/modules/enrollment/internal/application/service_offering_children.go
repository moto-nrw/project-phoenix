package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (m *Service) ApprovedOfferingChildrenForStudents(ctx context.Context, studentIDs []int64) (rows []enrollment.OfferingChildFacts, err error) {
	started := time.Now()
	defer func() {
		m.observeOfferingStorage("approved_children", "query", started, int64(len(studentIDs)), int64(len(rows)), err)
	}()
	if len(studentIDs) == 0 {
		return []enrollment.OfferingChildFacts{}, nil
	}
	if err = m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.ApprovedOfferingChildrenForStudents(ctx, studentIDs)
}

func (m *Service) OfferingChildren(ctx context.Context, childIDs []int64) (rows []enrollment.OfferingChildFacts, err error) {
	started := time.Now()
	defer func() {
		m.observeOfferingStorage("children", "query", started, int64(len(childIDs)), int64(len(rows)), err)
	}()
	if len(childIDs) == 0 {
		return []enrollment.OfferingChildFacts{}, nil
	}
	if err = m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.OfferingChildren(ctx, childIDs)
}
