package notifications

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/iot"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

var (
	// ErrWebPushNotConfigured is returned when VAPID keys are missing: devices
	// cannot subscribe while the server cannot sign pushes.
	ErrWebPushNotConfigured = errors.New("web push is not configured on this server")
	// ErrInvalidPushSubscription marks browser-supplied subscription data that
	// failed model validation. Handlers map only this error family to HTTP 400.
	ErrInvalidPushSubscription = errors.New("invalid push subscription")
)

// PushSubscriptionInput is the browser's PushSubscription in wire form.
type PushSubscriptionInput struct {
	Endpoint  string
	P256dh    string
	Auth      string
	UserAgent string
}

// PushSubscriptionService manages Web Push device registrations. Staff
// methods run inside tenant middleware (tenant tx present). Parent subscribe
// spans every active tenant mapping in one admin transaction so all device
// rows commit or roll back together.
type PushSubscriptionService interface {
	// PublicKey returns the VAPID public key the browser subscribes with.
	PublicKey() (string, error)
	// Subscribe registers (or refreshes) a staff device for the current tenant.
	Subscribe(ctx context.Context, accountID int64, input PushSubscriptionInput) error
	// Unsubscribe removes a staff device registration for the current tenant.
	Unsubscribe(ctx context.Context, accountID int64, endpoint string) error
	// SubscribeParent registers a guardian device for every school the account
	// is actively linked to (guardian-scoped events carry one tenant each).
	SubscribeParent(ctx context.Context, accountID int64, input PushSubscriptionInput) error
	// UnsubscribeParent removes a guardian device across all linked schools.
	UnsubscribeParent(ctx context.Context, accountID int64, endpoint string) error
}

type pushSubscriptionService struct {
	db             *bun.DB
	repo           iot.PushSubscriptionRepository
	accountTenants authModels.AccountTenantRepository
	vapid          VAPIDConfig
	logger         *slog.Logger
}

// NewPushSubscriptionService builds the push subscription service.
func NewPushSubscriptionService(
	db *bun.DB,
	repo iot.PushSubscriptionRepository,
	accountTenants authModels.AccountTenantRepository,
	vapid VAPIDConfig,
	logger *slog.Logger,
) PushSubscriptionService {
	return &pushSubscriptionService{db: db, repo: repo, accountTenants: accountTenants, vapid: vapid, logger: logger}
}

func (s *pushSubscriptionService) PublicKey() (string, error) {
	if !s.vapid.Configured() {
		return "", ErrWebPushNotConfigured
	}
	return s.vapid.PublicKey, nil
}

func (s *pushSubscriptionService) buildSubscription(accountID int64, portal string, input PushSubscriptionInput) (*iot.PushSubscription, error) {
	sub := &iot.PushSubscription{
		AccountID: accountID,
		Portal:    portal,
		Endpoint:  input.Endpoint,
		P256dh:    input.P256dh,
		Auth:      input.Auth,
		UserAgent: input.UserAgent,
	}
	if err := sub.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPushSubscription, err)
	}
	return sub, nil
}

func (s *pushSubscriptionService) Subscribe(ctx context.Context, accountID int64, input PushSubscriptionInput) error {
	if !s.vapid.Configured() {
		return ErrWebPushNotConfigured
	}
	sub, err := s.buildSubscription(accountID, iot.PushPortalStaff, input)
	if err != nil {
		return err
	}
	return s.repo.Upsert(ctx, sub)
}

func (s *pushSubscriptionService) Unsubscribe(ctx context.Context, accountID int64, endpoint string) error {
	return s.repo.DeleteByEndpoint(ctx, accountID, endpoint)
}

func (s *pushSubscriptionService) SubscribeParent(ctx context.Context, accountID int64, input PushSubscriptionInput) error {
	if !s.vapid.Configured() {
		return ErrWebPushNotConfigured
	}
	prototype, err := s.buildSubscription(accountID, iot.PushPortalParent, input)
	if err != nil {
		return err
	}
	mappings, err := s.accountTenants.FindActiveByAccountID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("resolving guardian tenant mappings: %w", err)
	}
	if len(mappings) == 0 {
		return errors.New("account has no active school mapping")
	}

	// One admin transaction keeps all tenant rows atomic. The account ID comes
	// from the authenticated parent token and mappings limit writes to active
	// schools; each row still carries its explicit tenant ID.
	return tenant.WithAdminTx(ctx, s.db, func(txCtx context.Context, _ bun.Tx) error {
		for _, mapping := range mappings {
			sub := *prototype
			sub.TenantID = mapping.TenantID
			if err := s.repo.Upsert(txCtx, &sub); err != nil {
				return fmt.Errorf("registering push subscription for tenant %d: %w", mapping.TenantID, err)
			}
		}
		return nil
	})
}

func (s *pushSubscriptionService) UnsubscribeParent(ctx context.Context, accountID int64, endpoint string) error {
	mappings, err := s.accountTenants.FindActiveByAccountID(ctx, accountID)
	if err != nil {
		return fmt.Errorf("resolving guardian tenant mappings: %w", err)
	}
	for _, mapping := range mappings {
		err = tenant.WithTenantTx(ctx, s.db, mapping.TenantID, func(txCtx context.Context, _ bun.Tx) error {
			return s.repo.DeleteByEndpoint(txCtx, accountID, endpoint)
		})
		if err != nil {
			return fmt.Errorf("removing push subscription for tenant %d: %w", mapping.TenantID, err)
		}
	}
	return nil
}
