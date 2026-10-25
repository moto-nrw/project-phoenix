package config

import (
	"context"
	"fmt"
	"slices"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// traegerLevelKeys returns the requested keys whose definitions admit a Träger
// value. Only those reach the Träger table, so a read of school-only settings
// costs no extra statement.
func (s *settingsService) traegerLevelKeys(keys []string) []string {
	if s.traegerValues == nil {
		return nil
	}
	var result []string
	for _, key := range keys {
		if definition := s.registry.GetDefinition(key); definition != nil && definition.Levels.AtTraeger() {
			result = append(result, key)
		}
	}
	return result
}

// layeredSnapshot resolves the school's stored overrides above the values of
// its Träger (ADR 0086 section 2), reading the Träger level in the caller's
// transaction. Only the school's own tenant transaction can read it: its
// read-only policy of config.traeger_setting_values admits exactly the
// school's Träger. Any other ambient unit fails instead of resolving without
// the Träger.
func (s *settingsService) layeredSnapshot(ctx context.Context, tenantID int64, keys []string, stored []*config.SettingValue) (*config.SettingsSnapshot, error) {
	var traeger []*config.TraegerSettingValue
	if traegerKeys := s.traegerLevelKeys(keys); len(traegerKeys) > 0 {
		if s.tenantID(ctx) != tenantID {
			return nil, &config.SettingsError{Op: "resolve_many", Err: fmt.Errorf("the Träger level of school %d needs its tenant transaction", tenantID)}
		}
		var err error
		if traeger, err = s.traegerValues.FindForSchool(ctx, tenantID, traegerKeys); err != nil {
			return nil, &config.SettingsError{Op: "resolve_many", Err: err}
		}
	}
	return config.NewLayeredSettingsSnapshot(s.registry, tenantID, keys, stored, traeger)
}

// schoolTraegerOf asks Organisation & Tenancy for the Träger of each school,
// before the batch read opens its administrative transaction. Without a
// Träger-level key it asks nothing.
func (s *settingsService) schoolTraegerOf(ctx context.Context, tenantIDs []int64, keys []string) (map[int64]int64, error) {
	if len(s.traegerLevelKeys(keys)) == 0 {
		return nil, nil
	}
	if s.schoolTraeger == nil {
		return nil, fmt.Errorf("settings: the Träger of the schools is not composed")
	}
	return s.schoolTraeger(ctx, tenantIDs)
}

// traegerValuesForSchools reads the Träger level of several schools inside the
// batch read's administrative transaction, grouped by school.
func (s *settingsService) traegerValuesForSchools(ctx context.Context, schoolTraeger map[int64]int64, keys []string) (map[int64][]*config.TraegerSettingValue, error) {
	traegerKeys := s.traegerLevelKeys(keys)
	if len(traegerKeys) == 0 || len(schoolTraeger) == 0 {
		return nil, nil
	}
	organizationIDs := make([]int64, 0, len(schoolTraeger))
	for _, organizationID := range schoolTraeger {
		if !slices.Contains(organizationIDs, organizationID) {
			organizationIDs = append(organizationIDs, organizationID)
		}
	}
	values, err := s.traegerValues.FindByOrganizations(ctx, organizationIDs, traegerKeys)
	if err != nil {
		return nil, err
	}
	byTraeger := make(map[int64][]*config.TraegerSettingValue, len(organizationIDs))
	for _, value := range values {
		if value != nil {
			byTraeger[value.OrganizationID] = append(byTraeger[value.OrganizationID], value)
		}
	}
	bySchool := make(map[int64][]*config.TraegerSettingValue, len(schoolTraeger))
	for schoolID, organizationID := range schoolTraeger {
		bySchool[schoolID] = byTraeger[organizationID]
	}
	return bySchool, nil
}

// schoolBoundKeys are the keys whose writes take a per-school lock, run a
// check across several fields or manage a school file. ADR 0086 section 3
// keeps them at the school: a Träger value would change them for every
// school without the lock or check of any. The switch cases of
// validateCrossField, the Lock* write helpers, the parent report modes, the
// review-scope projection and the AGB document are the sources.
var schoolBoundKeys = map[string]bool{
	config.KeyOperationalOverviewScope: true, config.KeyAttendanceEditScope: true,
	config.KeyBlockStartScope: true, config.KeyBlockCompleteScope: true,
	config.KeySlotListShortDayCutoff: true, config.KeySlotListLongDayCutoff: true,
	config.KeyEnrollmentCollectGradeLevel: true, config.KeyEnrollmentCollectSchoolClass: true,
	config.KeyEnrollmentGradeLevelMax: true, config.KeyAttendanceVisibleDays: true,
	config.KeyGDPRAttendanceRetentionDays: true, config.KeyMFAMode: true,
	config.KeyParentPickupChangeEnabled: true, config.KeyParentPickupChangeCutoffTime: true,
	config.KeyParentRequestReviewScope: true, config.KeyParentAbsenceReviewScope: true,
	config.KeyParentRequestGroupLeaderReviewEnabled: true, config.KeyGroupMode: true,
	config.KeyEnrollmentLegalAGBDocumentURL: true, config.KeyEnrollmentLegalAGBDisplayMode: true,
}

// ValidateTraegerLevels checks at startup that no Träger-level definition is
// bound to the school by a lock, a check over several fields, a module switch
// or the parent report modes (ADR 0086 section 3). Side-effect handlers are
// refused where they are bound (sideeffects.NewRegistryWith).
func ValidateTraegerLevels(registry *config.Registry) error {
	for key, definition := range registry.AllDefinitions() {
		if !definition.Levels.AtTraeger() {
			continue
		}
		if _, module := config.ModuleForSettingKey(key); module || schoolBoundKeys[key] || isParentReportSettingKey(key) {
			return fmt.Errorf("settings: %s is bound to the school and cannot have a Träger level", key)
		}
	}
	return nil
}
