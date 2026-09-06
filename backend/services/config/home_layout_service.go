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
	notify  SettingsChangedNotifier
}

// NewHomeLayoutService wires the store and the tenant transaction runtime.
func NewHomeLayoutService(repo configModel.HomeLayoutRepository, runtime TenantOperationsRuntime, logger *slog.Logger, notifiers ...SettingsChangedNotifier) *HomeLayoutService {
	var notify SettingsChangedNotifier
	if len(notifiers) > 0 {
		notify = notifiers[0]
	}
	return &HomeLayoutService{repo: repo, runtime: runtime, logger: logger.With("service", "home_layout"), notify: notify}
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

	// The client applies Policies when deciding visibility. Keep every stored
	// personal choice here, including one for a block the school currently
	// settles, so it becomes effective again if the school releases that block.
	return view, nil
}

// SetOverrides replaces one person's deviations.
//
// A concurrent school policy wins over an incoming choice without destroying
// the choice already stored for that block. The client sends its full stored
// map so choices for currently unavailable blocks also survive another edit.
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
		// A row lock alone cannot protect the first save, because it has no row
		// yet. This transaction-scoped lock serializes every write for this
		// account, including reset, before it reads the map to replace.
		if err := s.lockAccount(txCtx, tenantID, accountID); err != nil {
			return err
		}
		// The merge below reads the school's prescription. A shared lock keeps
		// concurrent personal saves independent, but conflicts with the
		// exclusive policy-replacement lock so this read cannot go stale before
		// the replacement is stored.
		if err := s.lockPolicies(txCtx, tenantID, true); err != nil {
			return err
		}
		policySet, err := s.repo.FindPolicies(txCtx)
		if err != nil {
			return err
		}
		var policies map[string]configModel.BlockPolicy
		if policySet != nil {
			policies = policySet.Policies
		}
		existing, err := s.repo.FindByAccount(txCtx, accountID)
		if err != nil {
			return err
		}

		layout := &configModel.HomeLayout{
			TenantID:  tenantID,
			AccountID: accountID,
			Overrides: mergeHomeLayoutOverrides(overrides, existing, policies),
		}
		if err := validateBlockKeys(layout.Overrides); err != nil {
			return err
		}
		if err := s.repo.UpsertForAccount(txCtx, layout); err != nil {
			return err
		}
		s.scheduleNotification(txCtx, tenantID, homeLayoutAccountChangeKey(accountID))
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
		if err := s.lockAccount(txCtx, tenantID, accountID); err != nil {
			return err
		}
		if err := s.repo.DeleteForAccount(txCtx, accountID); err != nil {
			return err
		}
		s.scheduleNotification(txCtx, tenantID, homeLayoutAccountChangeKey(accountID))
		s.logger.Info("home_layout_reset", slog.Int64("account_id", accountID))
		return nil
	})
}

func (s *HomeLayoutService) lockAccount(ctx context.Context, tenantID, accountID int64) error {
	if err := s.runtime.AcquireLock(ctx, homeLayoutAccountLockKey(tenantID, accountID), false); err != nil {
		return fmt.Errorf("lock home layout: %w", err)
	}
	return nil
}

func homeLayoutAccountLockKey(tenantID, accountID int64) string {
	return fmt.Sprintf("home-layout:%d:%d", tenantID, accountID)
}

// lockPolicies guards a school's whole policy map. Personal writes hold the
// shared side while merging their overrides; policy replacements hold the
// exclusive side so no merge can read an old prescription.
func (s *HomeLayoutService) lockPolicies(ctx context.Context, tenantID int64, shared bool) error {
	if err := s.runtime.AcquireLock(ctx, homeLayoutPoliciesLockKey(tenantID), shared); err != nil {
		return fmt.Errorf("lock home layout policies: %w", err)
	}
	return nil
}

func homeLayoutPoliciesLockKey(tenantID int64) string {
	return fmt.Sprintf("home-layout:%d:policies", tenantID)
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
		if err := s.lockPolicies(txCtx, tenantID, false); err != nil {
			return err
		}
		set := &configModel.HomeBlockPolicySet{
			TenantID:  tenantID,
			Policies:  cleaned,
			UpdatedBy: &changedBy,
		}
		if err := s.repo.UpsertPolicies(txCtx, set); err != nil {
			return err
		}
		s.scheduleNotification(txCtx, tenantID, homeLayoutPoliciesChangeKey)
		s.logger.Info("home_block_policies_saved",
			slog.Int64("account_id", accountID),
			slog.Int("prescribed", len(cleaned)),
		)
		return nil
	})
}

const homeLayoutPoliciesChangeKey = "home-layout"

func homeLayoutAccountChangeKey(accountID int64) string {
	return fmt.Sprintf("home-layout:%d", accountID)
}

// scheduleNotification broadcasts only after the tenant transaction commits.
// A personal change addresses that account's open start pages; a school-wide
// prescription addresses every start page in the tenant.
func (s *HomeLayoutService) scheduleNotification(ctx context.Context, tenantID int64, key string) {
	if s.notify == nil || tenantID <= 0 {
		return
	}
	s.runtime.AfterCommit(ctx, func() { s.notify(ctx, tenantID, key) })
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

// mergeHomeLayoutOverrides preserves stored choices the school currently
// settles. A stale client cannot create or replace one of those choices, but a
// later policy rollback still reveals the person's original selection.
func mergeHomeLayoutOverrides(overrides map[string]bool, existing *configModel.HomeLayout, policies map[string]configModel.BlockPolicy) map[string]bool {
	result := make(map[string]bool, len(overrides))
	for key, shown := range overrides {
		result[key] = shown
	}
	for key, policy := range policies {
		if policy != configModel.BlockRequired && policy != configModel.BlockDisabled {
			continue
		}
		if existing != nil {
			if shown, ok := existing.Overrides[key]; ok {
				result[key] = shown
				continue
			}
		}
		delete(result, key)
	}
	return result
}
