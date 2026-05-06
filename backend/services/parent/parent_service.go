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
}

// ServiceConfig is the dependency-injection bundle.
type ServiceConfig struct {
	ChildRepo           parentModels.ChildRepository
	EnrollablePhaseRepo parentModels.EnrollablePhaseRepository
	DB                  *bun.DB
	Logger              *slog.Logger
}

type service struct {
	childRepo           parentModels.ChildRepository
	enrollablePhaseRepo parentModels.EnrollablePhaseRepository
	db                  *bun.DB
	logger              *slog.Logger
}

// NewService wires a parent-portal service.
func NewService(cfg ServiceConfig) Service {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &service{
		childRepo:           cfg.ChildRepo,
		enrollablePhaseRepo: cfg.EnrollablePhaseRepo,
		db:                  cfg.DB,
		logger:              logger,
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
