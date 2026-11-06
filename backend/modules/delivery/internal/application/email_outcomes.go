package application

import (
	"context"
	"errors"
	"strings"
	"time"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
)

const outcomeQueryOperation = "email_outcome_query"

// EmailOutcomes reports the mails of the related entities of one school.
func (s *Service) EmailOutcomes(ctx context.Context, tenantID int64, query deliveryContract.EmailOutcomeQuery) (reports []deliveryContract.EmailOutcomeReport, err error) {
	if tenantID <= 0 {
		return nil, errors.New("delivery: tenant is required")
	}
	query.RelatedType = strings.TrimSpace(query.RelatedType)
	if query.RelatedType == "" {
		return nil, errors.New("delivery: related entity type is required")
	}
	defer s.observeOutcomeQuery(time.Now(), func() (int, error) { return len(reports), err })
	return s.store.EmailOutcomes(ctx, tenantID, query)
}

// LatestEmailOutcomes reports the newest mail of the kinds to each address.
func (s *Service) LatestEmailOutcomes(ctx context.Context, tenantID int64, kinds, addresses []string) (reports map[string]deliveryContract.EmailOutcomeReport, err error) {
	if tenantID <= 0 {
		return nil, errors.New("delivery: tenant is required")
	}
	defer s.observeOutcomeQuery(time.Now(), func() (int, error) { return len(reports), err })
	return s.store.LatestEmailOutcomes(ctx, tenantID, kinds, addresses)
}

// EmailAddressMarks reports the marked addresses among the given ones.
func (s *Service) EmailAddressMarks(ctx context.Context, tenantID int64, addresses []string) (marks map[string]deliveryContract.EmailAddressMark, err error) {
	if tenantID <= 0 {
		return nil, errors.New("delivery: tenant is required")
	}
	defer s.observeOutcomeQuery(time.Now(), func() (int, error) { return len(marks), err })
	return s.store.EmailAddressMarks(ctx, tenantID, addresses)
}

func (s *Service) observeOutcomeQuery(started time.Time, result func() (int, error)) {
	count, err := result()
	s.observe(domain.Observation{Operation: outcomeQueryOperation, Transport: string(domain.TransportEmail), Duration: time.Since(started), Count: count, Err: err})
}
