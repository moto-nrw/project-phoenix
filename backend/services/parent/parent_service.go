// Package parent holds the cross-tenant guardian-portal services.
//
// All methods take an account ID rather than reading tenant from
// context because parent-scope JWTs intentionally carry tenant_id=0.
// Per-action tenant context is decided by the picked child on the
// frontend, then validated server-side via the auth.account_tenants
// membership check that already lives in the underlying repos.
package parent

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/uptrace/bun"

	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Service is the public contract consumed by HTTP handlers.
type Service interface {
	// ListChildrenForAccount returns every child linked to any
	// guardian profile owned by the account, across every active
	// tenant mapping. Sorted by school then by name.
	ListChildrenForAccount(ctx context.Context, accountID int64) ([]*parentModels.ChildSummary, error)

	// ListEnrollableForAccount returns every (school, open phase)
	// pair the parent could enroll a new child at, with a flag for
	// schools they're already linked to. Sorted with linked schools
	// first.
	ListEnrollableForAccount(ctx context.Context, accountID int64) ([]*parentModels.EnrollablePhase, error)

	// ListEnrollmentsForAccount returns every enrollment.requests row
	// where guardian_account_id matches the calling account, joined
	// to phase + school + child summaries. Used by the dashboard to
	// surface in-progress / decided submissions without the parent
	// having to dig out the email-link status URL.
	ListEnrollmentsForAccount(ctx context.Context, accountID int64) ([]*parentModels.EnrollmentRequestSummary, error)
}

// ServiceConfig is the dependency-injection bundle.
type ServiceConfig struct {
	ChildRepo             parentModels.ChildRepository
	EnrollablePhaseRepo   parentModels.EnrollablePhaseRepository
	EnrollmentRequestRepo parentModels.EnrollmentRequestRepository
	DB                    *bun.DB
	Logger                *slog.Logger
}

type service struct {
	childRepo             parentModels.ChildRepository
	enrollablePhaseRepo   parentModels.EnrollablePhaseRepository
	enrollmentRequestRepo parentModels.EnrollmentRequestRepository
	db                    *bun.DB
	logger                *slog.Logger
}

// NewService wires a parent-portal service.
func NewService(cfg ServiceConfig) Service {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &service{
		childRepo:             cfg.ChildRepo,
		enrollablePhaseRepo:   cfg.EnrollablePhaseRepo,
		enrollmentRequestRepo: cfg.EnrollmentRequestRepo,
		db:                    cfg.DB,
		logger:                logger,
	}
}

func (s *service) ListChildrenForAccount(ctx context.Context, accountID int64) ([]*parentModels.ChildSummary, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}

	// Wrap in admin tx — the cross-tenant JOIN in the repo can't run
	// under phoenix_tenant or phoenix_auth because RLS on every joined
	// table requires app.current_tenant_id, and the parent-scope JWT
	// has none. Admin role with BYPASSRLS sees every tenant; the
	// account_tenants WHERE clause in the query keeps the result set
	// scoped to the caller's own membership.
	var children []*parentModels.ChildSummary
	err := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		list, listErr := s.childRepo.ListByAccount(adminCtx, accountID)
		if listErr != nil {
			return listErr
		}
		children = list
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("parent: list children: %w", err)
	}

	s.logger.Debug("parent: listed children",
		slog.Int64("account_id", accountID),
		slog.Int("count", len(children)),
	)
	return children, nil
}

// ListEnrollableForAccount returns the (school, open phase) pairs the
// parent can enroll a new child at. Same admin-tx wrap as the children
// query — the join crosses tenant_id boundaries scoped by
// auth.account_tenants membership for the AlreadyLinked flag.
func (s *service) ListEnrollableForAccount(ctx context.Context, accountID int64) ([]*parentModels.EnrollablePhase, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}

	var phases []*parentModels.EnrollablePhase
	err := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		list, listErr := s.enrollablePhaseRepo.ListEnrollable(adminCtx, accountID)
		if listErr != nil {
			return listErr
		}
		phases = list
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("parent: list enrollable: %w", err)
	}

	s.logger.Debug("parent: listed enrollable phases",
		slog.Int64("account_id", accountID),
		slog.Int("count", len(phases)),
	)
	return phases, nil
}

// ListEnrollmentsForAccount returns the parent's enrollment.requests
// rows joined to phase + school + child summaries. Same admin-tx wrap
// as the other parent queries — this crosses tenant_id boundaries.
func (s *service) ListEnrollmentsForAccount(ctx context.Context, accountID int64) ([]*parentModels.EnrollmentRequestSummary, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}
	if s.enrollmentRequestRepo == nil {
		return nil, fmt.Errorf("parent: enrollment request repo not wired")
	}

	var requests []*parentModels.EnrollmentRequestSummary
	err := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		list, listErr := s.enrollmentRequestRepo.ListByAccount(adminCtx, accountID)
		if listErr != nil {
			return listErr
		}
		requests = list
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("parent: list enrollments: %w", err)
	}

	// Redact admin-internal status reasons unless the owning phase opts
	// in via show_status_reason_to_parent. Same gate the public status
	// page and the decision email apply — keeps internal rejection /
	// waitlist notes out of the parent dashboard payload.
	for _, req := range requests {
		if req.ShowStatusReasonToParent {
			continue
		}
		for i := range req.Children {
			req.Children[i].StatusReason = nil
		}
	}

	s.logger.Debug("parent: listed enrollment requests",
		slog.Int64("account_id", accountID),
		slog.Int("count", len(requests)),
	)
	return requests, nil
}
