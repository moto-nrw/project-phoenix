package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

const (
	payrollReadPermission  = "config:read"
	payrollWritePermission = "config:manage"
)

// Payroll foundation (#1417 Tranche 2b): Lohnart mapping and DATEV client
// identifiers for the upcoming LODAS / Lohn und Gehalt writers.
//
// Every Lohnart default is deliberately the EMPTY STRING. The numbers are
// mandant-specific; no Träger has provided a list yet, and a plausible
// preset would later bill silently wrong. Empty required configuration is
// the desired state, not a defect — the writers fail fast on it.
//
// These settings are maintained on the dedicated /payroll page, not on the
// generic settings page (the "abrechnung" tab is filtered out there).
func init() {
	lohnartPattern := `^\d{1,4}$`
	beraterPattern := `^\d{1,7}$`
	mandantPattern := `^\d{1,5}$`

	unitOptions := &config.SelectOptions{
		Static: []config.SelectOption{
			{Label: "Nicht festgelegt", Value: ""},
			{Label: "Stunden", Value: config.PayrollUnitHours},
			{Label: "Tage", Value: config.PayrollUnitDays},
		},
	}

	lohnarten := []struct {
		key   string
		label string
		sort  int
	}{
		{config.KeyPayrollLohnartRegelarbeit, "Lohnart Regelarbeit", 10},
		{config.KeyPayrollLohnartPlusStunden, "Lohnart Plus-Stunden", 20},
		{config.KeyPayrollLohnartAuszahlung, "Lohnart Auszahlung", 30},
		{config.KeyPayrollLohnartFreizeitausgleich, "Lohnart Freizeitausgleich", 40},
		{config.KeyPayrollLohnartKrank, "Lohnart Krank", 50},
		{config.KeyPayrollLohnartUrlaub, "Lohnart Urlaub", 60},
		{config.KeyPayrollLohnartFortbildung, "Lohnart Fortbildung", 70},
	}
	for _, l := range lohnarten {
		config.Register(config.Definition{
			Key:             l.key,
			Label:           l.label,
			Description:     "Mandantenspezifische DATEV-Lohnartnummer (1 bis 4 Ziffern). Leer bedeutet: noch nicht konfiguriert; ohne Nummer erzeugt der spätere DATEV-Export keine Zeile für diese Kategorie.",
			Type:            config.FieldText,
			Default:         "",
			ReadPermission:  payrollReadPermission,
			WritePermission: payrollWritePermission,
			Tab:             "abrechnung",
			Category:        "lohnarten",
			SortOrder:       l.sort,
			Validation:      &config.ValidationRules{Pattern: &lohnartPattern},
			AccessPolicy:    config.AccessAdminOnly,
		})
	}

	units := []struct {
		key   string
		label string
		sort  int
	}{
		{config.KeyPayrollEinheitKrank, "Einheit Krank", 55},
		{config.KeyPayrollEinheitUrlaub, "Einheit Urlaub", 65},
		{config.KeyPayrollEinheitFortbildung, "Einheit Fortbildung", 75},
	}
	for _, u := range units {
		config.Register(config.Definition{
			Key:             u.key,
			Label:           u.label,
			Description:     "Ob die zugehörige DATEV-Lohnart Stunden oder Tage erwartet. Richtet sich nach der Lohnart-Definition im Lohnsystem des Trägers.",
			Type:            config.FieldSelect,
			Default:         "",
			ReadPermission:  payrollReadPermission,
			WritePermission: payrollWritePermission,
			Tab:             "abrechnung",
			Category:        "lohnarten",
			SortOrder:       u.sort,
			Options:         unitOptions,
			AccessPolicy:    config.AccessAdminOnly,
		})
	}

	config.Register(config.Definition{
		Key:             config.KeyPayrollDatevBeraternummer,
		Label:           "DATEV-Beraternummer",
		Description:     "Beraternummer für den Kopf der LODAS-Datei (1 bis 7 Ziffern). Ohne diesen Wert kann keine LODAS-Datei erzeugt werden; Lohn und Gehalt benötigt ihn nicht.",
		Type:            config.FieldText,
		Default:         "",
		ReadPermission:  payrollReadPermission,
		WritePermission: payrollWritePermission,
		Tab:             "abrechnung",
		Category:        "datev_mandant",
		SortOrder:       10,
		Validation:      &config.ValidationRules{Pattern: &beraterPattern},
		AccessPolicy:    config.AccessAdminOnly,
	})

	config.Register(config.Definition{
		Key:             config.KeyPayrollDatevMandantennummer,
		Label:           "DATEV-Mandantennummer",
		Description:     "Mandantennummer für den Kopf der LODAS-Datei (1 bis 5 Ziffern). Ohne diesen Wert kann keine LODAS-Datei erzeugt werden; Lohn und Gehalt benötigt ihn nicht.",
		Type:            config.FieldText,
		Default:         "",
		ReadPermission:  payrollReadPermission,
		WritePermission: payrollWritePermission,
		Tab:             "abrechnung",
		Category:        "datev_mandant",
		SortOrder:       20,
		Validation:      &config.ValidationRules{Pattern: &mandantPattern},
		AccessPolicy:    config.AccessAdminOnly,
	})
}
