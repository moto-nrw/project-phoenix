package config

import (
	"context"
	"fmt"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
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

// PayrollStatusService reports the payroll configuration state.
type PayrollStatusService interface {
	GetPayrollStatus(ctx context.Context) (*PayrollStatus, error)
}

type payrollStatusService struct {
	settings  SettingsService
	staffRepo userModels.StaffRepository
}

func NewPayrollStatusService(settings SettingsService, staffRepo userModels.StaffRepository) PayrollStatusService {
	return &payrollStatusService{settings: settings, staffRepo: staffRepo}
}

// payrollCategories fixes ID, label, and settings keys per category. The
// order matches the registry sort order and the later export column order.
var payrollCategories = []struct {
	id      string
	label   string
	key     string
	unitKey string
}{
	{"regelarbeit", "Regelarbeit", configModel.KeyPayrollLohnartRegelarbeit, ""},
	{"plus_stunden", "Plus-Stunden", configModel.KeyPayrollLohnartPlusStunden, ""},
	{"auszahlung", "Auszahlung", configModel.KeyPayrollLohnartAuszahlung, ""},
	{"freizeitausgleich", "Freizeitausgleich", configModel.KeyPayrollLohnartFreizeitausgleich, ""},
	{"krank", "Krank", configModel.KeyPayrollLohnartKrank, configModel.KeyPayrollEinheitKrank},
	{"urlaub", "Urlaub", configModel.KeyPayrollLohnartUrlaub, configModel.KeyPayrollEinheitUrlaub},
	{"fortbildung", "Fortbildung", configModel.KeyPayrollLohnartFortbildung, configModel.KeyPayrollEinheitFortbildung},
}

func (s *payrollStatusService) GetPayrollStatus(ctx context.Context) (*PayrollStatus, error) {
	status := &PayrollStatus{TotalCategories: len(payrollCategories)}

	for _, cat := range payrollCategories {
		number, err := s.settings.ResolveString(ctx, cat.key)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", cat.key, err)
		}
		entry := PayrollCategoryStatus{
			ID:         cat.id,
			Label:      cat.label,
			Number:     number,
			SettingKey: cat.key,
		}
		if cat.unitKey != "" {
			unit, err := s.settings.ResolveString(ctx, cat.unitKey)
			if err != nil {
				return nil, fmt.Errorf("resolve %s: %w", cat.unitKey, err)
			}
			entry.Unit = unit
			entry.UnitRequired = true
			entry.UnitSettingKey = cat.unitKey
		}
		if entry.Number != "" && (!entry.UnitRequired || entry.Unit != "") {
			status.ConfiguredCategories++
		}
		status.Categories = append(status.Categories, entry)
	}

	berater, err := s.settings.ResolveString(ctx, configModel.KeyPayrollDatevBeraternummer)
	if err != nil {
		return nil, fmt.Errorf("resolve beraternummer: %w", err)
	}
	mandant, err := s.settings.ResolveString(ctx, configModel.KeyPayrollDatevMandantennummer)
	if err != nil {
		return nil, fmt.Errorf("resolve mandantennummer: %w", err)
	}
	status.Beraternummer = berater
	status.Mandantennummer = mandant
	status.LodasHeaderComplete = berater != "" && mandant != ""

	// Tenant staff counts fit in memory (a school has tens of staff, not
	// thousands); the soft-delete filter comes from the repository.
	staff, err := s.staffRepo.List(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("list staff: %w", err)
	}
	status.StaffTotal = len(staff)
	for _, st := range staff {
		if st.PersonnelNumber == nil || *st.PersonnelNumber == "" {
			status.StaffWithoutPersonnelNumber++
		}
	}

	return status, nil
}
