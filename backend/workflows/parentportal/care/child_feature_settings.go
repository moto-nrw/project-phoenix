package care

import (
	"context"
	"fmt"

	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	configService "github.com/moto-nrw/project-phoenix/services/config"
)

// childFeatureSettings holds the tenant settings behind ChildFeatureFlags,
// resolved once per request before the child's permissions are applied.
type childFeatureSettings struct {
	sick               bool
	sickApproval       bool
	sickReports        bool
	excusedReports     bool
	excusedApproval    bool
	notes              bool
	pickupChange       bool
	pickupCutoff       careplan.SameDayCutoff
	inviteMode         string
	canRemove          bool
	masterEdit         bool
	masterRequest      bool
	mealPlan           bool
	mealRegistration   bool
	news               bool
	guardianManagement bool
	reasonPolicy       string
}

// childFeatureSettingKeys lists the settings ChildFeatures batch-resolves.
func childFeatureSettingKeys() []string {
	return []string{
		configModels.KeyParentSickNoteEnabled,
		configModels.KeyParentSickReportsEnabled,
		configModels.KeyParentExcusedReportsEnabled,
		configModels.KeyParentSickRequiresApproval,
		configModels.KeyParentExcusedRequiresApproval,
		configModels.KeyParentNotesEnabled,
		configModels.KeyParentPickupChangeEnabled,
		configModels.KeyParentPickupChangeCutoffTime,
		configModels.KeyGuardianParentInviteMode,
		configModels.KeyGuardianParentCanRemove,
		configModels.KeyParentMasterDataEditEnabled,
		configModels.KeyParentMasterDataRequestEnabled,
		configModels.KeyParentNewsEnabled,
		configModels.KeyParentGuardianManagementEnabled,
		configModels.KeyParentRequestReasonPolicy,
	}
}

// childSettingResolver reads tenant settings from a batch snapshot when the
// settings service supports one, and falls back to single-key lookups.
type childSettingResolver struct {
	ctx      context.Context
	settings configService.SettingsService
	tenantID int64
	snapshot *configService.SettingsSnapshot
}

func (s *Service) newChildSettingResolver(ctx context.Context, tenantID int64) (childSettingResolver, error) {
	r := childSettingResolver{ctx: ctx, settings: s.Settings, tenantID: tenantID}
	if batch, ok := s.Settings.(interface {
		ResolveManyForTenant(context.Context, int64, []string) (*configService.SettingsSnapshot, error)
	}); ok {
		snapshot, err := batch.ResolveManyForTenant(ctx, tenantID, childFeatureSettingKeys())
		if err != nil {
			return childSettingResolver{}, fmt.Errorf("parent: resolve child feature settings: %w", err)
		}
		r.snapshot = snapshot
	}
	return r, nil
}

func (r childSettingResolver) resolveBool(key string) (bool, error) {
	if r.snapshot != nil {
		return r.snapshot.Bool(key)
	}
	return r.settings.ResolveBoolForTenant(r.ctx, r.tenantID, key)
}

func (r childSettingResolver) resolveString(key string) (string, error) {
	if r.snapshot != nil {
		return r.snapshot.String(key)
	}
	return r.settings.ResolveStringForTenant(r.ctx, r.tenantID, key)
}

// resolveChildFeatureSettings resolves every setting ChildFeatures needs, in
// a fixed order, for the child's tenant.
func (s *Service) resolveChildFeatureSettings(ctx context.Context, tenantID int64) (childFeatureSettings, error) {
	var out childFeatureSettings
	r, err := s.newChildSettingResolver(ctx, tenantID)
	if err != nil {
		return out, err
	}
	if err := r.resolveAbsenceAndNoteSettings(&out); err != nil {
		return out, err
	}
	if out.pickupChange, out.pickupCutoff, err = s.resolvePickupChangeSettings(r); err != nil {
		return out, err
	}
	if err := r.resolveRelatedAccountAndMasterDataSettings(&out); err != nil {
		return out, err
	}
	if err := s.resolveMealSettings(ctx, tenantID, &out); err != nil {
		return out, err
	}
	if err := r.resolvePortalSettings(&out); err != nil {
		return out, err
	}
	return out, nil
}

func (r childSettingResolver) resolveAbsenceAndNoteSettings(out *childFeatureSettings) error {
	var err error
	if out.sick, err = r.resolveBool(configModels.KeyParentSickNoteEnabled); err != nil {
		return fmt.Errorf("parent: resolve sick-note setting: %w", err)
	}
	if out.sickApproval, err = r.resolveBool(configModels.KeyParentSickRequiresApproval); err != nil {
		return fmt.Errorf("parent: resolve sick-approval setting: %w", err)
	}
	if out.sickReports, err = r.resolveBool(configModels.KeyParentSickReportsEnabled); err != nil {
		return fmt.Errorf("parent: resolve sick reports: %w", err)
	}
	if out.excusedReports, err = r.resolveBool(configModels.KeyParentExcusedReportsEnabled); err != nil {
		return fmt.Errorf("parent: resolve excused reports: %w", err)
	}
	if out.excusedApproval, err = r.resolveBool(configModels.KeyParentExcusedRequiresApproval); err != nil {
		return fmt.Errorf("parent: resolve excused-approval setting: %w", err)
	}
	if out.notes, err = r.resolveBool(configModels.KeyParentNotesEnabled); err != nil {
		return fmt.Errorf("parent: resolve notes setting: %w", err)
	}
	return nil
}

// resolvePickupChangeSettings resolves the one-day pickup change switch and the
// school's same-day cutoff; a switched-off change has no cutoff.
func (s *Service) resolvePickupChangeSettings(r childSettingResolver) (bool, careplan.SameDayCutoff, error) {
	enabled, err := r.resolveBool(configModels.KeyParentPickupChangeEnabled)
	if err != nil {
		return false, careplan.SameDayCutoff{}, fmt.Errorf("parent: resolve pickup-change setting: %w", err)
	}
	pickupCutoffClock, err := r.resolveString(configModels.KeyParentPickupChangeCutoffTime)
	if err != nil {
		return false, careplan.SameDayCutoff{}, fmt.Errorf("parent: resolve pickup-change cutoff: %w", err)
	}
	if !enabled {
		pickupCutoffClock = ""
	}
	cutoff, err := careplan.NewSameDayCutoff(pickupCutoffClock, s.now())
	if err != nil {
		return false, careplan.SameDayCutoff{}, fmt.Errorf("parent: %w", err)
	}
	return enabled, cutoff, nil
}

func (r childSettingResolver) resolveRelatedAccountAndMasterDataSettings(out *childFeatureSettings) error {
	var err error
	if out.inviteMode, err = r.resolveString(configModels.KeyGuardianParentInviteMode); err != nil {
		return fmt.Errorf("parent: resolve invite mode: %w", err)
	}
	if out.canRemove, err = r.resolveBool(configModels.KeyGuardianParentCanRemove); err != nil {
		return fmt.Errorf("parent: resolve remove setting: %w", err)
	}
	if out.masterEdit, err = r.resolveBool(configModels.KeyParentMasterDataEditEnabled); err != nil {
		return fmt.Errorf("parent: resolve master-data edit setting: %w", err)
	}
	if out.masterRequest, err = r.resolveBool(configModels.KeyParentMasterDataRequestEnabled); err != nil {
		return fmt.Errorf("parent: resolve master-data request setting: %w", err)
	}
	return nil
}

func (s *Service) resolveMealSettings(ctx context.Context, tenantID int64, out *childFeatureSettings) error {
	var err error
	if out.mealPlan, err = s.mealPlanAvailableForTenant(ctx, tenantID); err != nil {
		return fmt.Errorf("parent: resolve meal-plan setting: %w", err)
	}
	if out.mealRegistration, err = s.mealRegistrationAvailableForTenant(ctx, tenantID); err != nil {
		return fmt.Errorf("parent: resolve meal-registration setting: %w", err)
	}
	return nil
}

func (r childSettingResolver) resolvePortalSettings(out *childFeatureSettings) error {
	var err error
	if out.news, err = r.resolveBool(configModels.KeyParentNewsEnabled); err != nil {
		return fmt.Errorf("parent: resolve parent-news setting: %w", err)
	}
	if out.guardianManagement, err = r.resolveBool(configModels.KeyParentGuardianManagementEnabled); err != nil {
		return fmt.Errorf("parent: resolve guardian-management setting: %w", err)
	}
	if out.reasonPolicy, err = r.resolveString(configModels.KeyParentRequestReasonPolicy); err != nil {
		return fmt.Errorf("parent: resolve reason policy: %w", err)
	}
	return nil
}
