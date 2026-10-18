package config

import (
	"context"
	"fmt"

	configModel "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// Payroll configuration status (#1417 Tranche 2b). One source of truth for
// "can a DATEV file be produced yet": the /payroll page renders this today,
// and the LODAS / Lohn und Gehalt writers (follow-up PR) call the same
// method before writing — a file must never be produced from incomplete
// configuration. Which Lohnart categories are REQUIRED is deliberately not
// decided here: an unconfigured category simply exports no line; the status
// reports facts, the writers enforce their own per-format minimums.

type payrollStatusService struct {
	settings       payrollSettings
	countPersonnel func(context.Context) (int, int, error)
}

type payrollSettings interface {
	ResolveString(context.Context, string) (string, error)
}

func NewPayrollStatusService(settings payrollSettings, countPersonnel func(context.Context) (int, int, error)) configModel.PayrollStatusGetter {
	return &payrollStatusService{settings: settings, countPersonnel: countPersonnel}
}

type payrollCategory struct {
	id      string
	label   string
	key     string
	unitKey string
}

// payrollCategories fixes ID, label, and settings keys per category. The
// order matches the registry sort order and the later export column order.
var payrollCategories = []payrollCategory{
	{"regelarbeit", "Regelarbeit", configModel.KeyPayrollLohnartRegelarbeit, ""},
	{"plus_stunden", "Plus-Stunden", configModel.KeyPayrollLohnartPlusStunden, ""},
	{"auszahlung", "Auszahlung", configModel.KeyPayrollLohnartAuszahlung, ""},
	{"freizeitausgleich", "Freizeitausgleich", configModel.KeyPayrollLohnartFreizeitausgleich, ""},
	{"krank", "Krank", configModel.KeyPayrollLohnartKrank, configModel.KeyPayrollEinheitKrank},
	{"urlaub", "Urlaub", configModel.KeyPayrollLohnartUrlaub, configModel.KeyPayrollEinheitUrlaub},
	{"fortbildung", "Fortbildung", configModel.KeyPayrollLohnartFortbildung, configModel.KeyPayrollEinheitFortbildung},
}

func (s *payrollStatusService) GetPayrollStatus(ctx context.Context) (*configModel.PayrollStatus, error) {
	keys := []string{
		configModel.KeyPayrollDatevBeraternummer,
		configModel.KeyPayrollDatevMandantennummer,
	}
	for _, category := range payrollCategories {
		keys = append(keys, category.key)
		if category.unitKey != "" {
			keys = append(keys, category.unitKey)
		}
	}
	var snapshot *configModel.SettingsSnapshot
	if batch, ok := s.settings.(interface {
		ResolveMany(context.Context, []string) (*configModel.SettingsSnapshot, error)
	}); ok {
		var err error
		snapshot, err = batch.ResolveMany(ctx, keys)
		if err != nil {
			return nil, fmt.Errorf("resolve payroll settings: %w", err)
		}
	}

	categories, configuredCategories, err := s.resolvePayrollCategories(ctx, snapshot)
	if err != nil {
		return nil, err
	}

	berater, mandant, err := s.resolveLodasHeader(ctx, snapshot)
	if err != nil {
		return nil, err
	}

	staffTotal, staffWithoutPersonnelNumber, err := s.countStaffPersonnelNumbers(ctx)
	if err != nil {
		return nil, err
	}

	return &configModel.PayrollStatus{
		Categories:                  categories,
		Beraternummer:               berater,
		Mandantennummer:             mandant,
		LodasHeaderComplete:         berater != "" && mandant != "",
		ConfiguredCategories:        configuredCategories,
		TotalCategories:             len(payrollCategories),
		StaffTotal:                  staffTotal,
		StaffWithoutPersonnelNumber: staffWithoutPersonnelNumber,
	}, nil
}

func (s *payrollStatusService) resolvePayrollCategories(ctx context.Context, snapshot *configModel.SettingsSnapshot) ([]configModel.PayrollCategoryStatus, int, error) {
	categories := make([]configModel.PayrollCategoryStatus, 0, len(payrollCategories))
	configuredCategories := 0
	for _, category := range payrollCategories {
		entry, err := s.resolvePayrollCategory(ctx, snapshot, category)
		if err != nil {
			return nil, 0, err
		}
		if entry.Number != "" && (!entry.UnitRequired || entry.Unit != "") {
			configuredCategories++
		}
		categories = append(categories, entry)
	}
	return categories, configuredCategories, nil
}

func (s *payrollStatusService) resolvePayrollCategory(ctx context.Context, snapshot *configModel.SettingsSnapshot, category payrollCategory) (configModel.PayrollCategoryStatus, error) {
	number, err := resolvedPayrollString(ctx, s.settings, snapshot, category.key)
	if err != nil {
		return configModel.PayrollCategoryStatus{}, fmt.Errorf("resolve %s: %w", category.key, err)
	}
	entry := configModel.PayrollCategoryStatus{
		ID:         category.id,
		Label:      category.label,
		Number:     number,
		SettingKey: category.key,
	}
	if category.unitKey == "" {
		return entry, nil
	}

	unit, err := resolvedPayrollString(ctx, s.settings, snapshot, category.unitKey)
	if err != nil {
		return configModel.PayrollCategoryStatus{}, fmt.Errorf("resolve %s: %w", category.unitKey, err)
	}
	entry.Unit = unit
	entry.UnitRequired = true
	entry.UnitSettingKey = category.unitKey
	return entry, nil
}

func (s *payrollStatusService) resolveLodasHeader(ctx context.Context, snapshot *configModel.SettingsSnapshot) (string, string, error) {
	berater, err := resolvedPayrollString(ctx, s.settings, snapshot, configModel.KeyPayrollDatevBeraternummer)
	if err != nil {
		return "", "", fmt.Errorf("resolve beraternummer: %w", err)
	}
	mandant, err := resolvedPayrollString(ctx, s.settings, snapshot, configModel.KeyPayrollDatevMandantennummer)
	if err != nil {
		return "", "", fmt.Errorf("resolve mandantennummer: %w", err)
	}
	return berater, mandant, nil
}

func resolvedPayrollString(ctx context.Context, settings payrollSettings, snapshot *configModel.SettingsSnapshot, key string) (string, error) {
	if snapshot != nil {
		return snapshot.String(key)
	}
	return settings.ResolveString(ctx, key)
}

func (s *payrollStatusService) countStaffPersonnelNumbers(ctx context.Context) (int, int, error) {
	if s.countPersonnel == nil {
		return 0, 0, configModel.ErrRuntimeUnavailable
	}
	staffTotal, withoutPersonnelNumber, err := s.countPersonnel(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("list staff: %w", err)
	}
	return staffTotal, withoutPersonnelNumber, nil
}
