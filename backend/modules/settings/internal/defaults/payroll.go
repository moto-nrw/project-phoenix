package defaults

import (
	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
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
	registerPayrollWageTypes()
	registerPayrollUnits()
	registerPayrollIdentity()
	registerPayrollDispatch()
}

func registerPayrollWageTypes() {
	for _, l := range lohnarten {
		config.Register(config.Definition{
			Key:             l.key,
			Label:           l.label,
			Description:     "Die Nummer dieser Lohnart in Ihrem DATEV (1 bis 4 Ziffern). Ohne Nummer fehlt diese Lohnart im DATEV-Export.",
			Type:            config.FieldText,
			Default:         "",
			ReadPermission:  payrollReadPermission,
			WritePermission: payrollWritePermission,
			Tab:             "abrechnung",
			Category:        "lohnarten",
			SortOrder:       l.sort,
			Validation:      &config.ValidationRules{Pattern: &lohnartPattern, AllowEmpty: true},
			AccessPolicy:    config.AccessAdminOnly,
		})
	}

}

func registerPayrollUnits() {
	for _, u := range units {
		config.Register(config.Definition{
			Key:             u.key,
			Label:           u.label,
			Description:     "Ob die Lohnart in DATEV Stunden oder Tage erwartet. Das steht in der Lohnabrechnung Ihres Trägers.",
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

}

func registerPayrollIdentity() {
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
		Validation:      &config.ValidationRules{Pattern: &beraterPattern, AllowEmpty: true},
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
		Validation:      &config.ValidationRules{Pattern: &mandantPattern, AllowEmpty: true},
		AccessPolicy:    config.AccessAdminOnly,
	})
}

// registerPayrollDispatch holds the Lohnbüro of the school (#4805): whether
// the month file goes out on its own after the Monatsabschluss, how, in which
// format and, for e-mail, to whom. The switch defaults to off: a school sends
// nothing until somebody chose a way. The e-mail address has no default for
// the same reason a SFTP host has none.
func registerPayrollDispatch() {
	for _, definition := range []config.Definition{
		{
			Key:         config.KeyPayrollDispatchAuto,
			Label:       "Automatisch nach dem Monatsabschluss senden",
			Description: "Nach dem Monatsabschluss schickt moto die Monatsdatei aller Mitarbeitenden an das Lohnbüro.",
			Type:        config.FieldBoolean, Default: false, SortOrder: 10,
		},
		{
			Key:         config.KeyPayrollDispatchChannel,
			Label:       "Versandweg",
			Description: "Per SFTP an die Gegenstelle aus den Einstellungen oder per E-Mail mit einem Link zum Herunterladen.",
			Type:        config.FieldSelect, Default: config.PayrollDispatchChannelSFTP, SortOrder: 20,
			Options: &config.SelectOptions{Static: []config.SelectOption{
				{Label: "SFTP", Value: config.PayrollDispatchChannelSFTP},
				{Label: "E-Mail", Value: config.PayrollDispatchChannelEmail},
			}},
		},
		{
			Key:         config.KeyPayrollDispatchFormat,
			Label:       "Dateiformat",
			Description: "In diesem Format bekommt das Lohnbüro die Monatsdatei.",
			Type:        config.FieldSelect, Default: config.PayrollDispatchFormatCSV, SortOrder: 30,
			Options: &config.SelectOptions{Static: []config.SelectOption{
				{Label: "CSV", Value: config.PayrollDispatchFormatCSV},
				{Label: "Excel", Value: config.PayrollDispatchFormatXLSX},
				{Label: "DATEV LODAS", Value: config.PayrollDispatchFormatDatevLodas},
				{Label: "DATEV Lohn und Gehalt", Value: config.PayrollDispatchFormatDatevLug},
			}},
		},
		{
			Key:         config.KeyPayrollDispatchEmail,
			Label:       "E-Mail-Adresse des Lohnbüros",
			Description: "An diese Adresse geht der Link zur Monatsdatei. Die Datei selbst hängt nicht an der E-Mail.",
			Type:        config.FieldText, Default: "", SortOrder: 40,
			Validation: &config.ValidationRules{Pattern: &payrollEmailPattern, AllowEmpty: true},
			DependsOn:  config.DependsOnEq(config.KeyPayrollDispatchChannel, config.PayrollDispatchChannelEmail),
		},
	} {
		definition.ReadPermission = payrollReadPermission
		definition.WritePermission = payrollWritePermission
		definition.Tab = "abrechnung"
		definition.Category = "lohnbuero"
		definition.AccessPolicy = config.AccessAdminOnly
		config.Register(definition)
	}
}

var payrollEmailPattern = `^[^@\s]{1,64}@[^@\s]+\.[^@\s]{2,}$`
var lohnartPattern = `^\d{1,4}$`
var beraterPattern = `^\d{1,7}$`
var mandantPattern = `^\d{1,5}$`

var unitOptions = &config.SelectOptions{
	Static: []config.SelectOption{
		{Label: "Nicht festgelegt", Value: ""},
		{Label: "Stunden", Value: config.PayrollUnitHours},
		{Label: "Tage", Value: config.PayrollUnitDays},
	},
}

var lohnarten = []struct {
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

var units = []struct {
	key   string
	label string
	sort  int
}{
	{config.KeyPayrollEinheitKrank, "Einheit Krank", 55},
	{config.KeyPayrollEinheitUrlaub, "Einheit Urlaub", 65},
	{config.KeyPayrollEinheitFortbildung, "Einheit Fortbildung", 75},
}
