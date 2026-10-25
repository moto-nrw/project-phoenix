package records

import (
	"fmt"
	"strings"
)

// ModuleKey identifies one additional module of the module catalog (#4812).
// moto Basis is always included and has no key. The value is the suffix of
// the module's setting key ("module.<key>") and the key the frontend uses.
type ModuleKey string

// The additional modules of the module catalog, in catalog order.
const (
	ModuleNFCAnwesenheit      ModuleKey = "nfc_anwesenheit"
	ModuleMittagessen         ModuleKey = "mittagessen"
	ModuleElternApp           ModuleKey = "eltern_app"
	ModuleOnlineAnmeldung     ModuleKey = "online_anmeldung"
	ModuleUnterrichtVormittag ModuleKey = "unterricht_vormittag"
	ModuleDienstplan          ModuleKey = "dienstplan"
	ModuleZeiterfassung       ModuleKey = "zeiterfassung"
	ModulePersonalakte        ModuleKey = "personalakte"
	ModuleTraegerUebersicht   ModuleKey = "traeger_uebersicht"
	ModuleFerienbetreuung     ModuleKey = "ferienbetreuung"
	ModuleVertraege           ModuleKey = "vertraege"
	ModuleElternbeitraege     ModuleKey = "elternbeitraege"
)

// ModuleDefinition describes one additional module: its setting, the id the
// module catalog (collaterals, src/app/modulkatalog/data.ts) gives it, the
// name and summary a person reads, and the modules it requires besides Basis.
type ModuleDefinition struct {
	Key        ModuleKey
	SettingKey string
	CatalogID  string
	Label      string
	Summary    string
	Requires   []ModuleKey
	// Available is false for a module moto does not offer yet. Its switch
	// still records what the school booked.
	Available bool
}

// moduleCatalog is the central module list. Order is catalog order and the
// order of the settings tab.
var moduleCatalog = []ModuleDefinition{
	{Key: ModuleNFCAnwesenheit, SettingKey: KeyModuleNFCAnwesenheit, CatalogID: "nfc", Label: "NFC-Anwesenheit",
		Summary: "Kinder melden sich mit Armband oder Karte an Tablets an und ab, auch beim Raumwechsel.", Available: true},
	{Key: ModuleMittagessen, SettingKey: KeyModuleMittagessen, CatalogID: "mittagessen", Label: "Mittagessen und Catering",
		Summary: "Speiseplan, Essen bestellen und abbestellen, Essenszahlen für die Küche.", Available: true},
	{Key: ModuleElternApp, SettingKey: KeyModuleElternApp, CatalogID: "eltern-app", Label: "Eltern-App",
		Summary: "Eltern melden ihr Kind krank, ändern Abholzeiten und lesen Nachrichten und Elternbriefe.", Available: true},
	{Key: ModuleOnlineAnmeldung, SettingKey: KeyModuleOnlineAnmeldung, CatalogID: "online-anmeldung", Label: "Online-Anmeldung",
		Summary: "Eltern melden ihr Kind online für die Betreuung an. Die Schule vergibt die Plätze.", Available: true},
	{Key: ModuleUnterrichtVormittag, SettingKey: KeyModuleUnterrichtVormittag, CatalogID: "schule", Label: "Unterricht am Vormittag",
		Summary: "Lehrkräfte sehen Krankmeldungen, geben Kinder an die OGS ab und notieren Hausaufgaben.", Available: true},
	{Key: ModuleDienstplan, SettingKey: KeyModuleDienstplan, CatalogID: "dienstplan", Label: "Dienstplan",
		Summary: "Dienste planen, Besetzung prüfen und Krankmeldungen im Team auffangen.", Available: true},
	{Key: ModuleZeiterfassung, SettingKey: KeyModuleZeiterfassung, CatalogID: "zeiterfassung", Label: "Zeiterfassung",
		Summary: "Stempeluhr, Arbeitszeitmodelle, Urlaub und Export für das Lohnbüro.", Available: true},
	{Key: ModulePersonalakte, SettingKey: KeyModulePersonalakte, CatalogID: "personalakte", Label: "Personalakte",
		Summary: "Dokumente, Qualifikationen und Eintrittsdaten der Mitarbeitenden.", Available: true},
	{Key: ModuleTraegerUebersicht, SettingKey: KeyModuleTraegerUebersicht, CatalogID: "traeger-cockpit", Label: "Träger-Übersicht",
		Summary: "Alle Standorte eines Trägers mit Auslastung und Berichten. Gibt es in moto noch nicht."},
	{Key: ModuleFerienbetreuung, SettingKey: KeyModuleFerienbetreuung, CatalogID: "ferienbetreuung", Label: "Ferienbetreuung",
		Summary:  "Eltern buchen Ferienwochen, die Schule führt Ferienlisten. Gibt es in moto noch nicht.",
		Requires: []ModuleKey{ModuleOnlineAnmeldung}},
	{Key: ModuleVertraege, SettingKey: KeyModuleVertraege, CatalogID: "vertraege", Label: "Verträge und Dokumente",
		Summary: "Betreuungsverträge, Vorlagen und Serienbriefe. Gibt es in moto noch nicht."},
	{Key: ModuleElternbeitraege, SettingKey: KeyModuleElternbeitraege, CatalogID: "elternbeitraege", Label: "Elternbeiträge und Abrechnung",
		Summary:  "Rechnungen, Lastschrift und Buchhaltung. Gibt es in moto noch nicht.",
		Requires: []ModuleKey{ModuleVertraege}},
}

// Modules returns the module list in catalog order. The slices are copies.
func Modules() []ModuleDefinition {
	out := make([]ModuleDefinition, len(moduleCatalog))
	for i, module := range moduleCatalog {
		module.Requires = append([]ModuleKey(nil), module.Requires...)
		out[i] = module
	}
	return out
}

// LookupModule returns the definition of a module key.
func LookupModule(key ModuleKey) (ModuleDefinition, bool) {
	for _, module := range Modules() {
		if module.Key == key {
			return module, true
		}
	}
	return ModuleDefinition{}, false
}

// ModuleForSettingKey returns the module whose switch is settingKey.
func ModuleForSettingKey(settingKey string) (ModuleDefinition, bool) {
	for _, module := range Modules() {
		if module.SettingKey == settingKey {
			return module, true
		}
	}
	return ModuleDefinition{}, false
}

// ModuleDependents returns the modules that require key, in catalog order.
func ModuleDependents(key ModuleKey) []ModuleDefinition {
	var dependents []ModuleDefinition
	for _, module := range Modules() {
		for _, required := range module.Requires {
			if required == key {
				dependents = append(dependents, module)
			}
		}
	}
	return dependents
}

// ModuleDependencyError rejects a module switch that breaks a prerequisite:
// booking a module without the modules it requires (Booking true), or
// cancelling a module a booked module still requires (Booking false).
// Conflicts names the missing prerequisites or the booked dependents.
type ModuleDependencyError struct {
	Key       string
	Module    ModuleDefinition
	Booking   bool
	Conflicts []ModuleDefinition
}

func (e *ModuleDependencyError) Error() string {
	if e.Booking {
		return fmt.Sprintf("invalid value for setting %s: module %s requires %s", e.Key, e.Module.Key, moduleKeyList(e.Conflicts))
	}
	return fmt.Sprintf("invalid value for setting %s: module %s is required by %s", e.Key, e.Module.Key, moduleKeyList(e.Conflicts))
}

// ConflictLabels joins the names of the conflicting modules for a message.
func (e *ModuleDependencyError) ConflictLabels() string {
	labels := make([]string, len(e.Conflicts))
	for i, module := range e.Conflicts {
		labels[i] = module.Label
	}
	return strings.Join(labels, ", ")
}

// Unwrap keeps the rejection an invalid value (400) for every caller.
func (e *ModuleDependencyError) Unwrap() error {
	return ErrInvalidValue
}

func moduleKeyList(modules []ModuleDefinition) string {
	keys := make([]string, len(modules))
	for i, module := range modules {
		keys[i] = string(module.Key)
	}
	return strings.Join(keys, ", ")
}

// ModuleNotBookedError reports that a school has not booked a module a
// route, job or portal needs. Module names the first missing module.
type ModuleNotBookedError struct {
	TenantID int64
	Module   ModuleKey
}

func (e *ModuleNotBookedError) Error() string {
	return fmt.Sprintf("module %s is not booked for school %d", e.Module, e.TenantID)
}
