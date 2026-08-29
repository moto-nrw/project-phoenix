package config

import (
	"context"
	"fmt"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
)

// Payroll configuration status (#1417 Tranche 2b). One source of truth for
// "can a DATEV file be produced yet": the /payroll page renders this today,
// and the LODAS / Lohn und Gehalt writers (follow-up PR) call the same
// method before writing — a file must never be produced from incomplete
// configuration. Which Lohnart categories are REQUIRED is deliberately not
// decided here: an unconfigured category simply exports no line; the status
// reports facts, the writers enforce their own per-format minimums.

// PayrollCategoryStatus is one Lohnart category with its configured values.
type PayrollCategoryStatus struct {
	// ID is the stable category identifier (e.g. "regelarbeit").
	ID    string `json:"id"`
	Label string `json:"label"`
	// Number is the tenant's Lohnartnummer, empty when not configured.
	Number string `json:"number"`
	// Unit is "stunden" / "tage" / "" and only meaningful when UnitRequired.
	Unit string `json:"unit,omitempty"`
	// UnitRequired marks the categories that carry day values (sick /
	// vacation / training) and therefore need a unit choice.
	UnitRequired bool `json:"unit_required"`
	// SettingKey / UnitSettingKey let the UI write through the settings API.
	SettingKey     string `json:"setting_key"`
	UnitSettingKey string `json:"unit_setting_key,omitempty"`
}

// PayrollStatus is the complete payroll configuration state of a tenant.
type PayrollStatus struct {
	Categories      []PayrollCategoryStatus `json:"categories"`
	Beraternummer   string                  `json:"beraternummer"`
	Mandantennummer string                  `json:"mandantennummer"`
	// LodasHeaderComplete is true when both LODAS header identifiers are
	// set. Lohn und Gehalt does not need them.
	LodasHeaderComplete bool `json:"lodas_header_complete"`
	// ConfiguredCategories counts categories with a Lohnartnummer whose
	// unit requirement (if any) is also satisfied.
	ConfiguredCategories int `json:"configured_categories"`
	TotalCategories      int `json:"total_categories"`
	StaffTotal           int `json:"staff_total"`
	// StaffWithoutPersonnelNumber counts active staff the payroll system
	// could not match. Count only — names stay behind time_tracking:manage.
	StaffWithoutPersonnelNumber int `json:"staff_without_personnel_number"`
}

// PayrollStatusGetter reports the payroll configuration state.
type PayrollStatusGetter interface {
	GetPayrollStatus(ctx context.Context) (*PayrollStatus, error)
}

type payrollStatusService struct {
	settings       SettingsService
	countPersonnel func(context.Context) (int, int, error)
}

func NewPayrollStatusService(settings SettingsService, countPersonnel func(context.Context) (int, int, error)) PayrollStatusGetter {
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

func (s *payrollStatusService) GetPayrollStatus(ctx context.Context) (*PayrollStatus, error) {
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
	var snapshot *SettingsSnapshot
	if batch, ok := s.settings.(interface {
		ResolveMany(context.Context, []string) (*SettingsSnapshot, error)
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

	return &PayrollStatus{
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

func (s *payrollStatusService) resolvePayrollCategories(ctx context.Context, snapshot *SettingsSnapshot) ([]PayrollCategoryStatus, int, error) {
	categories := make([]PayrollCategoryStatus, 0, len(payrollCategories))
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

func (s *payrollStatusService) resolvePayrollCategory(ctx context.Context, snapshot *SettingsSnapshot, category payrollCategory) (PayrollCategoryStatus, error) {
	number, err := resolvedPayrollString(ctx, s.settings, snapshot, category.key)
	if err != nil {
		return PayrollCategoryStatus{}, fmt.Errorf("resolve %s: %w", category.key, err)
	}
	entry := PayrollCategoryStatus{
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
		return PayrollCategoryStatus{}, fmt.Errorf("resolve %s: %w", category.unitKey, err)
	}
	entry.Unit = unit
	entry.UnitRequired = true
	entry.UnitSettingKey = category.unitKey
	return entry, nil
}

func (s *payrollStatusService) resolveLodasHeader(ctx context.Context, snapshot *SettingsSnapshot) (string, string, error) {
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

func resolvedPayrollString(ctx context.Context, settings SettingsService, snapshot *SettingsSnapshot, key string) (string, error) {
	if snapshot != nil {
		return snapshot.String(key)
	}
	return settings.ResolveString(ctx, key)
}

func (s *payrollStatusService) countStaffPersonnelNumbers(ctx context.Context) (int, int, error) {
	if s.countPersonnel == nil {
		return 0, 0, ErrRuntimeUnavailable
	}
	staffTotal, withoutPersonnelNumber, err := s.countPersonnel(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("list staff: %w", err)
	}
	return staffTotal, withoutPersonnelNumber, nil
}
