package application

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/ports"
)

type Service struct {
	store   ports.Store
	people  ports.GuardianDirectory
	observe ports.Observer
}

func NewService(store ports.Store, people ports.GuardianDirectory, observe ports.Observer) *Service {
	if store == nil || people == nil || observe == nil {
		panic("delivery application: store, guardian directory, and observer are required")
	}
	return &Service{store: store, people: people, observe: observe}
}

func (s *Service) Enqueue(ctx context.Context, input domain.EnqueueInput) (result domain.Enqueued, err error) {
	started := time.Now()
	defer func() {
		s.observe(domain.Observation{Operation: "enqueue", Transport: string(input.Transport), Template: input.Template, Duration: time.Since(started), Count: boolCount(err == nil), Err: err})
	}()
	intent := domain.Intent{
		TenantID: input.TenantID, Transport: input.Transport, Template: input.Template,
		Recipient: input.Recipient, Payload: input.Payload, Status: string(domain.StatePending), NextRetryAt: time.Now(),
	}
	if input.IdempotencyKey != "" {
		intent.IdempotencyKey = &input.IdempotencyKey
	}
	if input.Related.Type != "" {
		intent.RelatedEntityType = &input.Related.Type
		intent.RelatedEntityID = &input.Related.ID
	}
	result, err = s.store.Enqueue(ctx, intent)
	if errors.Is(err, domain.ErrIdempotencyConflict) {
		s.observe(domain.Observation{Operation: "idempotency_conflict", Transport: string(input.Transport), Template: input.Template, Count: 1, Err: err})
	} else if result.Duplicate {
		s.observe(domain.Observation{Operation: "duplicate", Transport: string(input.Transport), Template: input.Template, Count: 1})
	}
	return result, err
}

func (s *Service) Cancel(ctx context.Context, tenantID int64, transport domain.Transport, related domain.RelatedEntity, reason string) (count int64, err error) {
	started := time.Now()
	defer func() {
		s.observe(domain.Observation{Operation: "cancel", Transport: string(transport), Duration: time.Since(started), Count: int(count), Err: err})
	}()
	return s.store.Cancel(ctx, tenantID, transport, related.Type, related.ID, reason, time.Now())
}

func (s *Service) Statuses(ctx context.Context, tenantID int64, transport domain.Transport, related domain.RelatedEntity) (statuses []domain.Intent, err error) {
	started := time.Now()
	defer func() {
		s.observe(domain.Observation{Operation: "status_query", Transport: string(transport), Duration: time.Since(started), Count: len(statuses), Err: err})
	}()
	return s.store.Statuses(ctx, tenantID, transport, related.Type, related.ID)
}

func (s *Service) ReplaceEmailDeliveries(ctx context.Context, tenantID int64, related domain.RelatedEntity, rows []domain.EmailDelivery) error {
	return s.store.ReplaceEmailDeliveries(ctx, tenantID, related.Type, related.ID, rows)
}

func (s *Service) DeleteEmailDeliveries(ctx context.Context, tenantID int64, related domain.RelatedEntity) (int64, error) {
	return s.store.DeleteEmailDeliveries(ctx, tenantID, related.Type, related.ID)
}

func (s *Service) EmailDeliveryStatuses(ctx context.Context, tenantID int64, related domain.RelatedEntity) (statuses []domain.EmailDeliveryStatus, err error) {
	started := time.Now()
	defer func() {
		s.observe(domain.Observation{Operation: "delivery_status_query", Transport: string(domain.TransportEmail), Duration: time.Since(started), Count: len(statuses), Err: err})
	}()
	statuses, err = s.store.EmailDeliveryStatuses(ctx, tenantID, related.Type, related.ID)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(statuses))
	for _, status := range statuses {
		if status.GuardianProfileID != nil {
			ids = append(ids, *status.GuardianProfileID)
		}
	}
	displays, err := s.people.Resolve(ctx, ids)
	if err != nil {
		return nil, err
	}
	for index := range statuses {
		if statuses[index].GuardianProfileID == nil {
			continue
		}
		display := displays[*statuses[index].GuardianProfileID]
		statuses[index].FirstName = display.FirstName
		statuses[index].LastName = display.LastName
	}
	sort.SliceStable(statuses, func(i, j int) bool {
		left := strings.ToLower(statuses[i].LastName + "\x00" + statuses[i].FirstName)
		right := strings.ToLower(statuses[j].LastName + "\x00" + statuses[j].FirstName)
		if left == right {
			return statuses[i].DeliveryID < statuses[j].DeliveryID
		}
		return left < right
	})
	return statuses, nil
}

func (s *Service) AttachEmailOutbox(ctx context.Context, tenantID, deliveryID, outboxID int64) error {
	return s.store.AttachEmailOutbox(ctx, tenantID, deliveryID, outboxID)
}

func (s *Service) ClaimFailedEmailDelivery(ctx context.Context, tenantID, deliveryID int64) (bool, error) {
	return s.store.ClaimFailedEmailDelivery(ctx, tenantID, deliveryID)
}

func boolCount(value bool) int {
	if value {
		return 1
	}
	return 0
}
