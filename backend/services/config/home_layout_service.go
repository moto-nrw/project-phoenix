package config

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

// HomeBlockPolicyWritePermission is what an account needs to prescribe the
// school's start page. It is the same permission that writes operational
// settings, because that is what this is: a school-wide display decision.
const HomeBlockPolicyWritePermission = "config:update"

// homeBlocksErrorKey names this surface in the errors the settings adapter
// classifies. It is not a registry key — the start page catalogue is not a
// settings registry entry.
const homeBlocksErrorKey = "home.blocks"

// ErrHomeLayoutUnavailable is returned when the service was built without the
// repository it needs.
var ErrHomeLayoutUnavailable = errors.New("home layout store is not configured")

// HomeLayoutView is everything the start page needs to decide what to render:
// what this person chose, and what their school prescribes.
type HomeLayoutView struct {
	// Overrides are the person's own deviations. Absent means "undecided",
	// which is not the same as hidden.
	Overrides map[string]bool `json:"overrides"`
	// Policies are the school's prescriptions. Absent means BlockOptional.
	Policies map[string]configModel.BlockPolicy `json:"policies"`
	// CanManagePolicies tells the client whether to offer the school-wide
	// dialog at all. It mirrors the permission the write path enforces.
	CanManagePolicies bool `json:"can_manage_policies"`
}

// HomeLayoutService owns the personal composition of the start page and the
// school's prescription for it.
type HomeLayoutService struct {
	repo    configModel.HomeLayoutRepository
	runtime TenantOperationsRuntime
	logger  *slog.Logger
}

// NewHomeLayoutService wires the store and the tenant transaction runtime.
func NewHomeLayoutService(repo configModel.HomeLayoutRepository, runtime TenantOperationsRuntime, logger *slog.Logger) *HomeLayoutService {
	return &HomeLayoutService{repo: repo, runtime: runtime, logger: logger.With("service", "home_layout")}
}

func (s *HomeLayoutService) ready() error {
	if s == nil || s.repo == nil || s.runtime == nil {
		return ErrHomeLayoutUnavailable
	}
	return nil
}

// View reads the person's deviations and the school's prescription together.
// Neither store having a row is the normal state of a school that has never
// touched either dialog, and yields two empty maps.
func (s *HomeLayoutService) View(ctx context.Context, tenantID, accountID int64, permissions []string) (HomeLayoutView, error) {
	if err := s.ready(); err != nil {
		return HomeLayoutView{}, err
	}
	if accountID <= 0 {
		return HomeLayoutView{}, errors.New("account ID is required")
	}

	view := HomeLayoutView{
		Overrides:         map[string]bool{},
		Policies:          map[string]configModel.BlockPolicy{},
		CanManagePolicies: hasPermission(HomeBlockPolicyWritePermission, permissions),
	}

	err := s.runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
		policySet, err := s.repo.FindPolicies(txCtx)
		if err != nil {
			return err
		}
		if policySet != nil && policySet.Policies != nil {
			view.Policies = policySet.Policies
		}

		layout, err := s.repo.FindByAccount(txCtx, accountID)
		if err != nil {
			return err
		}
		if layout != nil && layout.Overrides != nil {
			view.Overrides = layout.Overrides
		}
		return nil
	})
	if err != nil {
		return HomeLayoutView{}, fmt.Errorf("read home layout: %w", err)
	}

	// The school's word beats a stored personal choice, including one stored
	// before the school changed its mind. Applying it on read as well as on
	// write means an admin's change takes effect for everybody immediately,
	// without a migration over other people's rows.
	view.Overrides = applyPolicies(view.Overrides, view.Policies)
	return view, nil
}

// SetOverrides replaces one person's deviations.
//
// Entries the school has settled are dropped rather than rejected: a client
// that still shows a block the admin has just disabled would otherwise get an
// error it cannot explain to the user. What survives is exactly what the person
// is actually allowed to decide.
func (s *HomeLayoutService) SetOverrides(ctx context.Context, tenantID, accountID int64, overrides map[string]bool) error {
	if err := s.ready(); err != nil {
		return err
	}
	if accountID <= 0 {
		return errors.New("account ID is required")
	}
	if err := validateBlockKeys(overrides); err != nil {
		return err
	}

	return s.runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
		policySet, err := s.repo.FindPolicies(txCtx)
		if err != nil {
			return err
		}
		var policies map[string]configModel.BlockPolicy
		if policySet != nil {
			policies = policySet.Policies
		}

		layout := &configModel.HomeLayout{
			TenantID:  tenantID,
			AccountID: accountID,
			Overrides: applyPolicies(overrides, policies),
		}
		if err := s.repo.UpsertForAccount(txCtx, layout); err != nil {
			return err
		}
		s.logger.Info("home_layout_saved",
			slog.Int64("account_id", accountID),
			slog.Int("blocks", len(layout.Overrides)),
		)
		return nil
	})
}

// ResetOverrides restores the recommended start page by dropping the row.
func (s *HomeLayoutService) ResetOverrides(ctx context.Context, tenantID, accountID int64) error {
	if err := s.ready(); err != nil {
		return err
	}
	if accountID <= 0 {
		return errors.New("account ID is required")
	}

	return s.runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
		if err := s.repo.DeleteForAccount(txCtx, accountID); err != nil {
			return err
		}
		s.logger.Info("home_layout_reset", slog.Int64("account_id", accountID))
		return nil
	})
}

// SetPolicies replaces the school's prescription.
//
// Blocks left optional are removed from the map before storing: "the school has
// no opinion" is the default state and storing it would make a later change of
// defaults indistinguishable from a deliberate decision.
func (s *HomeLayoutService) SetPolicies(ctx context.Context, tenantID, accountID int64, permissions []string, policies map[string]configModel.BlockPolicy) error {
	if err := s.ready(); err != nil {
		return err
	}
	if !hasPermission(HomeBlockPolicyWritePermission, permissions) {
		return &PermissionDeniedError{
			Key:                homeBlocksErrorKey,
			RequiredPermission: HomeBlockPolicyWritePermission,
		}
	}

	if len(policies) > configModel.MaxHomeBlockEntries {
		return invalidBlocks(fmt.Sprintf("at most %d start page blocks can be stored", configModel.MaxHomeBlockEntries))
	}
	cleaned := make(map[string]configModel.BlockPolicy, len(policies))
	for key, policy := range policies {
		if err := configModel.ValidateHomeBlockKey(key); err != nil {
			return invalidBlocks(err.Error())
		}
		parsed, err := configModel.ParseBlockPolicy(string(policy))
		if err != nil {
			return invalidBlocks(err.Error())
		}
		if parsed == configModel.BlockOptional {
			continue
		}
		cleaned[key] = parsed
	}

	changedBy := accountID
	return s.runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
		set := &configModel.HomeBlockPolicySet{
			TenantID:  tenantID,
			Policies:  cleaned,
			UpdatedBy: &changedBy,
		}
		if err := s.repo.UpsertPolicies(txCtx, set); err != nil {
			return err
		}
		s.logger.Info("home_block_policies_saved",
			slog.Int64("account_id", accountID),
			slog.Int("prescribed", len(cleaned)),
		)
		return nil
	})
}

func validateBlockKeys(overrides map[string]bool) error {
	if len(overrides) > configModel.MaxHomeBlockEntries {
		return invalidBlocks(fmt.Sprintf("at most %d start page blocks can be stored", configModel.MaxHomeBlockEntries))
	}
	for key := range overrides {
		if err := configModel.ValidateHomeBlockKey(key); err != nil {
			return invalidBlocks(err.Error())
		}
	}
	return nil
}

// invalidBlocks marks a malformed payload as a client error. Without it the
// HTTP adapter classifies every unknown error as internal and answers 500 to
// what is plainly a bad request.
func invalidBlocks(reason string) error {
	return &InvalidValueError{Key: homeBlocksErrorKey, Reason: reason}
}

// applyPolicies drops every personal deviation the school has already settled.
//
// A required block is shown to everybody and a disabled block to nobody, so in
// both cases the person has nothing left to choose and their stored entry is
// noise. Dropping it rather than inverting it means the person's original
// choice comes back untouched if the school later releases the block again —
// which is exactly what someone expects after an admin undoes a decision.
func applyPolicies(overrides map[string]bool, policies map[string]configModel.BlockPolicy) map[string]bool {
	result := make(map[string]bool, len(overrides))
	for key, shown := range overrides {
		switch policies[key] {
		case configModel.BlockRequired, configModel.BlockDisabled:
			continue
		default:
			result[key] = shown
		}
	}
	return result
}
