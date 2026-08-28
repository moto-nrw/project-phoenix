package notifications

import (
	"context"
	"errors"
	"fmt"

	authModel "github.com/moto-nrw/project-phoenix/models/auth"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	userModel "github.com/moto-nrw/project-phoenix/models/users"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// ErrUnknownNotificationType is returned when a caller names a type that is not
// in the catalogue. Handlers map it to 400.
var ErrUnknownNotificationType = errors.New("unknown notification type")

// PreferenceState is one row of the profile page: what the type is, whether the
// person agreed, and whether the school currently allows it at all.
type PreferenceState struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Group       string `json:"group"`
	Enabled     bool   `json:"enabled"`
	// Available reports whether the school allows this type right now. A type
	// can be switched on personally while unavailable — see SetForAccount.
	Available bool `json:"available"`
}

// PreferenceOverview is the payload behind the profile page's card.
type PreferenceOverview struct {
	// TenantEnabled mirrors notifications.dispatch_enabled. When false nothing
	// is delivered no matter what the person chose, and the card says so.
	TenantEnabled bool              `json:"tenant_enabled"`
	Types         []PreferenceState `json:"types"`
}

// PreferenceService owns per-account notification consent.
type PreferenceService interface {
	// GetForAccount returns the full catalogue of one portal with the account's
	// stored decisions merged in. Types the account never touched read as off.
	GetForAccount(ctx context.Context, accountID int64, portal string) (*PreferenceOverview, error)

	// SetForAccount records one decision.
	SetForAccount(ctx context.Context, accountID int64, notificationType string, enabled bool) error
	// SetForPortalAccount is SetForAccount for a named portal: the type must be
	// offered there (OfferedInPortal). The school portal (#2208) writes the
	// same row as the staff portal.
	SetForPortalAccount(ctx context.Context, accountID int64, portal, notificationType string, enabled bool) error

	// DisableAllForAccount switches every stored staff-portal decision off.
	DisableAllForAccount(ctx context.Context, accountID int64) error
	// DisableAllForPortalAccount switches off every type the portal offers.
	DisableAllForPortalAccount(ctx context.Context, accountID int64, portal string) error

	// FilterOptedIn narrows a producer's candidate recipients to those who
	// agreed to the type AND whose school still allows it. This is the single
	// enforcement point for "nothing without consent"; producers build the
	// relation-based candidate set and then call this, never the other way
	// round.
	FilterOptedIn(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error)

	// HasAnyOptedIn is a cheap tenant-wide scheduling gate. It only reports
	// stored consent and must never be used as the source of an audience.
	HasAnyOptedIn(ctx context.Context, notificationTypes []string) (bool, error)

	// FilterOptedInByType applies the same consent and tenant-gate rules as
	// FilterOptedIn for multiple types, using one settings read and one
	// preference read.
	FilterOptedInByType(ctx context.Context, notificationTypes []string, accountIDs []int64) (map[string][]int64, error)

	// FilterNotOptedOut is the mirror of FilterOptedIn for channels that already
	// reached people before consent existed: it drops only the accounts that
	// explicitly declined the type, and keeps the ones who never decided. Use it
	// for e-mail, which is addressed to a person the school already writes to;
	// push and in-app hints stay consent-first via FilterOptedIn.
	FilterNotOptedOut(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error)

	// GetForParent and SetForParent are the guardian-portal counterparts. The
	// parents app has no tenant context of its own, so both resolve the
	// account's active school mappings and act across all of them: one set of
	// switches, applied to every school the family belongs to.
	GetForParent(ctx context.Context, accountID int64) (*PreferenceOverview, error)
	SetForParent(ctx context.Context, accountID int64, notificationType string, enabled bool) error
	DisableAllForParent(ctx context.Context, accountID int64) error
}

type preferenceService struct {
	repo           userModel.NotificationPreferenceRepository
	settings       configService.SettingsService
	db             *bun.DB
	accountTenants authModel.AccountTenantRepository
}

// NewPreferenceService builds the consent service. db and accountTenants are
// needed only for the guardian portal, which is cross-tenant.
func NewPreferenceService(
	repo userModel.NotificationPreferenceRepository,
	settings configService.SettingsService,
	db *bun.DB,
	accountTenants authModel.AccountTenantRepository,
) PreferenceService {
	return &preferenceService{
		repo:           repo,
		settings:       settings,
		db:             db,
		accountTenants: accountTenants,
	}
}

func (s *preferenceService) GetForAccount(ctx context.Context, accountID int64, portal string) (*PreferenceOverview, error) {
	if accountID <= 0 {
		return nil, errors.New("account id is required")
	}

	stored, err := s.repo.ListByAccount(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("load notification preferences: %w", err)
	}
	enabledByType := make(map[string]bool, len(stored))
	for _, pref := range stored {
		enabledByType[pref.NotificationType] = pref.Enabled
	}

	tenantEnabled, err := s.resolveBool(ctx, configModel.KeyNotificationsDispatchEnabled)
	if err != nil {
		return nil, err
	}

	defs := TypesForPortal(portal)
	overview := &PreferenceOverview{
		TenantEnabled: tenantEnabled,
		Types:         make([]PreferenceState, 0, len(defs)),
	}

	// Gate values are cached per settings key: several types share one gate,
	// and the page is opened often enough that re-reading would be wasteful.
	gateCache := make(map[string]bool, len(defs))
	for _, def := range defs {
		available := true
		if def.TenantGate != "" {
			cached, seen := gateCache[def.TenantGate]
			if !seen {
				cached, err = s.resolveBool(ctx, def.TenantGate)
				if err != nil {
					return nil, err
				}
				gateCache[def.TenantGate] = cached
			}
			available = cached
		}

		overview.Types = append(overview.Types, PreferenceState{
			Key:         def.Key,
			Label:       def.Label,
			Description: def.Description,
			Group:       def.Group,
			Enabled:     enabledByType[def.Key],
			Available:   available,
		})
	}

	return overview, nil
}

// SetForAccount records one decision.
//
// A type the school currently blocks can still be switched on. The alternative
// would lose a person's choice whenever an admin toggles the school setting off
// and on again; the gate is applied at delivery time instead.
func (s *preferenceService) SetForAccount(ctx context.Context, accountID int64, notificationType string, enabled bool) error {
	return s.SetForPortalAccount(ctx, accountID, PortalStaff, notificationType, enabled)
}

func (s *preferenceService) SetForPortalAccount(ctx context.Context, accountID int64, portal, notificationType string, enabled bool) error {
	if accountID <= 0 {
		return errors.New("account id is required")
	}
	if portal != PortalStaff && portal != PortalSchool {
		return fmt.Errorf("unsupported preference portal %q", portal)
	}
	def, known := GetType(notificationType)
	if !known || !OfferedInPortal(def, portal) {
		return fmt.Errorf("%w: %s", ErrUnknownNotificationType, notificationType)
	}

	pref := &userModel.NotificationPreference{
		AccountID:        accountID,
		NotificationType: notificationType,
		Enabled:          enabled,
	}
	if err := pref.Validate(); err != nil {
		return err
	}
	if err := s.repo.Upsert(ctx, pref); err != nil {
		return fmt.Errorf("save notification preference: %w", err)
	}
	return nil
}

func (s *preferenceService) DisableAllForAccount(ctx context.Context, accountID int64) error {
	return s.DisableAllForPortalAccount(ctx, accountID, PortalStaff)
}

func (s *preferenceService) DisableAllForPortalAccount(ctx context.Context, accountID int64, portal string) error {
	if accountID <= 0 {
		return errors.New("account id is required")
	}
	if portal != PortalStaff && portal != PortalSchool {
		return fmt.Errorf("unsupported preference portal %q", portal)
	}
	if err := s.repo.DisableAllForAccount(ctx, accountID, typeKeysForPortal(portal)); err != nil {
		return fmt.Errorf("disable notification preferences: %w", err)
	}
	return nil
}

func (s *preferenceService) FilterOptedIn(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error) {
	if len(accountIDs) == 0 {
		return nil, nil
	}

	optedInByType, err := s.FilterOptedInByType(ctx, []string{notificationType}, accountIDs)
	if err != nil {
		return nil, err
	}
	return optedInByType[notificationType], nil
}

func (s *preferenceService) HasAnyOptedIn(ctx context.Context, notificationTypes []string) (bool, error) {
	if len(notificationTypes) == 0 {
		return false, nil
	}
	for _, notificationType := range notificationTypes {
		if _, known := GetType(notificationType); !known {
			return false, fmt.Errorf("%w: %s", ErrUnknownNotificationType, notificationType)
		}
	}
	hasAny, err := s.repo.HasAnyOptedIn(ctx, notificationTypes)
	if err != nil {
		return false, fmt.Errorf("check for opted-in accounts: %w", err)
	}
	return hasAny, nil
}

func (s *preferenceService) FilterOptedInByType(
	ctx context.Context,
	notificationTypes []string,
	accountIDs []int64,
) (map[string][]int64, error) {
	filtered := make(map[string][]int64)
	if len(notificationTypes) == 0 || len(accountIDs) == 0 {
		return filtered, nil
	}

	gateKeys := make([]string, 0, len(notificationTypes))
	gateByType := make(map[string]string, len(notificationTypes))
	seenGates := make(map[string]struct{}, len(notificationTypes))
	for _, notificationType := range notificationTypes {
		def, known := GetType(notificationType)
		if !known {
			// Fail closed: an unknown type reaches nobody. A producer naming a
			// type outside the catalogue is a bug.
			return nil, fmt.Errorf("%w: %s", ErrUnknownNotificationType, notificationType)
		}
		gateByType[notificationType] = def.TenantGate
		if def.TenantGate == "" {
			continue
		}
		if _, seen := seenGates[def.TenantGate]; seen {
			continue
		}
		seenGates[def.TenantGate] = struct{}{}
		gateKeys = append(gateKeys, def.TenantGate)
	}

	allowedByGate := make(map[string]bool)
	if len(gateKeys) > 0 {
		if s.settings == nil {
			return nil, errors.New("notification preferences service has no settings service configured")
		}
		var err error
		allowedByGate, err = s.settings.ResolveBools(ctx, gateKeys)
		if err != nil {
			return nil, fmt.Errorf("resolve notification gates: %w", err)
		}
	}

	allowedTypes := make([]string, 0, len(notificationTypes))
	for _, notificationType := range notificationTypes {
		gate := gateByType[notificationType]
		if gate != "" && !allowedByGate[gate] {
			continue
		}
		allowedTypes = append(allowedTypes, notificationType)
	}
	if len(allowedTypes) == 0 {
		return filtered, nil
	}

	optedInByType, err := s.repo.FilterOptedInByType(ctx, allowedTypes, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("filter opted-in accounts: %w", err)
	}
	return optedInByType, nil
}

func (s *preferenceService) FilterNotOptedOut(ctx context.Context, notificationType string, accountIDs []int64) ([]int64, error) {
	if len(accountIDs) == 0 {
		return nil, nil
	}

	def, known := GetType(notificationType)
	if !known {
		// Same reasoning as FilterOptedIn: a producer naming a type outside the
		// catalogue is a bug, and this is not the place to guess an audience.
		return nil, fmt.Errorf("%w: %s", ErrUnknownNotificationType, notificationType)
	}

	// The school-wide gate is applied like in FilterOptedIn. It says "this
	// school does not want this notification at all", which is about the type
	// and not about one delivery channel. TypeParentAnnouncement declares no
	// gate, so today this branch never fires for the e-mail path.
	if def.TenantGate != "" {
		allowed, err := s.resolveBool(ctx, def.TenantGate)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, nil
		}
	}

	remaining, err := s.repo.FilterNotOptedOut(ctx, notificationType, accountIDs)
	if err != nil {
		return nil, fmt.Errorf("filter opted-out accounts: %w", err)
	}
	return remaining, nil
}

// GetForParent merges the guardian's decisions across every active school.
//
// A type reads as enabled only when it is enabled at every active guardian
// mapping. The parents app shows one set of switches and SetForParent applies a
// change to all schools, so a mixed state must read as off rather than claiming
// that a newly added school may deliver when it has no consent row.
func (s *preferenceService) GetForParent(ctx context.Context, accountID int64) (*PreferenceOverview, error) {
	if accountID <= 0 {
		return nil, errors.New("account id is required")
	}

	defs := TypesForPortal(PortalParent)
	overview := &PreferenceOverview{Types: make([]PreferenceState, 0, len(defs))}
	enabledTenantCount := make(map[string]int, len(defs))
	availableTenantCount := make(map[string]int, len(defs))
	tenantCount := 0

	err := s.forEachGuardianTenant(ctx, accountID, func(tenantCtx context.Context, _ int64) error {
		tenantCount++
		stored, err := s.repo.ListByAccount(tenantCtx, accountID)
		if err != nil {
			return fmt.Errorf("load notification preferences: %w", err)
		}
		for _, pref := range stored {
			if pref.Enabled {
				enabledTenantCount[pref.NotificationType]++
			}
		}
		// Any school allowing dispatch is enough for the card to stop saying
		// "your school switched this off".
		tenantEnabled, err := s.resolveBool(tenantCtx, configModel.KeyNotificationsDispatchEnabled)
		if err != nil {
			return err
		}
		if tenantEnabled {
			overview.TenantEnabled = true
		}
		for _, def := range defs {
			if def.TenantGate == "" {
				availableTenantCount[def.Key]++
				continue
			}
			available, err := s.resolveBool(tenantCtx, def.TenantGate)
			if err != nil {
				return err
			}
			if available {
				availableTenantCount[def.Key]++
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	for _, def := range defs {
		overview.Types = append(overview.Types, PreferenceState{
			Key:         def.Key,
			Label:       def.Label,
			Description: def.Description,
			Group:       def.Group,
			Enabled:     tenantCount > 0 && enabledTenantCount[def.Key] == tenantCount,
			Available:   tenantCount > 0 && availableTenantCount[def.Key] == tenantCount,
		})
	}
	return overview, nil
}

// SetForParent applies one decision to every school the guardian belongs to.
func (s *preferenceService) SetForParent(ctx context.Context, accountID int64, notificationType string, enabled bool) error {
	if accountID <= 0 {
		return errors.New("account id is required")
	}
	def, known := GetType(notificationType)
	if !known || def.Portal != PortalParent {
		return fmt.Errorf("%w: %s", ErrUnknownNotificationType, notificationType)
	}

	return s.forEachGuardianTenant(ctx, accountID, func(tenantCtx context.Context, tenantID int64) error {
		pref := &userModel.NotificationPreference{
			AccountID:        accountID,
			NotificationType: notificationType,
			Enabled:          enabled,
		}
		// Set explicitly: the admin transaction carries no tenant in its
		// context, so the row would otherwise land without one.
		pref.SetTenantID(tenantID)
		if err := s.repo.Upsert(tenantCtx, pref); err != nil {
			return fmt.Errorf("save notification preference for tenant %d: %w", tenantID, err)
		}
		return nil
	})
}

// DisableAllForParent switches every stored parent-portal decision off at
// every school.
func (s *preferenceService) DisableAllForParent(ctx context.Context, accountID int64) error {
	if accountID <= 0 {
		return errors.New("account id is required")
	}
	return s.forEachGuardianTenant(ctx, accountID, func(tenantCtx context.Context, tenantID int64) error {
		if err := s.repo.DisableAllForAccount(tenantCtx, accountID, typeKeysForPortal(PortalParent)); err != nil {
			return fmt.Errorf("disable notification preferences for tenant %d: %w", tenantID, err)
		}
		return nil
	})
}

func typeKeysForPortal(portal string) []string {
	defs := TypesForPortal(portal)
	keys := make([]string, 0, len(defs))
	for _, def := range defs {
		keys = append(keys, def.Key)
	}
	return keys
}

// forEachGuardianTenant runs fn once per active school mapping of the account,
// all inside one admin transaction so a multi-school change is atomic. Mirrors
// the push-subscription flow, including the deliberate exclusion of
// pending-enrollment-only schools.
func (s *preferenceService) forEachGuardianTenant(
	ctx context.Context,
	accountID int64,
	fn func(tenantCtx context.Context, tenantID int64) error,
) error {
	if s.db == nil || s.accountTenants == nil {
		return errors.New("notification preferences are not configured for the guardian portal")
	}

	return tenant.WithAdminTx(ctx, s.db, func(txCtx context.Context, _ bun.Tx) error {
		mappings, err := s.accountTenants.FindActiveGuardianByAccountID(txCtx, accountID)
		if err != nil {
			return fmt.Errorf("resolving guardian tenant mappings: %w", err)
		}
		for _, mapping := range mappings {
			if err := fn(tenant.WithTenantID(txCtx, mapping.TenantID), mapping.TenantID); err != nil {
				return err
			}
		}
		return nil
	})
}

// resolveBool surfaces a settings failure rather than guessing. A broken read
// must not silently widen or narrow who gets notified.
func (s *preferenceService) resolveBool(ctx context.Context, key string) (bool, error) {
	if s.settings == nil {
		return false, errors.New("notification preferences service has no settings service configured")
	}
	v, err := s.settings.ResolveBool(ctx, key)
	if err != nil {
		return false, fmt.Errorf("resolve %s: %w", key, err)
	}
	return v, nil
}
