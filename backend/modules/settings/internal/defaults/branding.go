package defaults

import (
	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// Branding of the Träger (#4653, ADR 0086 section 4): the first Träger-only
// settings. Documents of the Träger print the colour, the sender lines of the
// letterhead and the footer; the Träger-Portal previews them with the logo.
func init() {
	registerDefinitions(brandingDefinitions)
}

var brandingColorPattern = `^#[0-9A-Fa-f]{6}$`

// brandingColorPresets are the colours a Träger picks without a colour code
// (#5239). Each one keeps white paper text readable: contrast to white at
// least 4.5:1 (WCAG AA), checked in branding_test.go. "Eigene Farbe" in the
// frontend still takes any other colour.
var brandingColorPresets = &config.SelectOptions{Static: []config.SelectOption{
	{Label: "Schwarz", Value: "#111827"},
	{Label: "Dunkelblau", Value: "#1F4E79"},
	{Label: "Blau", Value: "#1D4ED8"},
	{Label: "Petrol", Value: "#0F766E"},
	{Label: "Grün", Value: "#15803D"},
	{Label: "Rot", Value: "#B91C1C"},
	{Label: "Weinrot", Value: "#9D174D"},
	{Label: "Lila", Value: "#7E22CE"},
}}

var brandingDefinitions = []config.Definition{
	{
		Key:             config.KeyBrandingPrimaryColor,
		Levels:          config.LevelTraeger,
		Label:           "Farbe für Briefe und Rechnungen",
		Description:     "In dieser Farbe stehen Linien und Überschriften auf den Briefen des Trägers. Wählen Sie eine Farbe. Passt keine, wählen Sie „Eigene Farbe“.",
		Type:            config.FieldColor,
		Default:         "#111827",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "branding",
		Category:        "briefkopf",
		SortOrder:       10,
		Validation:      &config.ValidationRules{Pattern: &brandingColorPattern},
		Options:         brandingColorPresets,
	},
	{
		Key:             config.KeyBrandingLetterheadText,
		Levels:          config.LevelTraeger,
		Label:           "Absender im Briefkopf",
		Description:     "Name und Anschrift des Trägers, so wie sie oben auf Briefen und Rechnungen stehen. Schreiben Sie jede Angabe in eine eigene Zeile.",
		Type:            config.FieldTextarea,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "branding",
		Category:        "briefkopf",
		SortOrder:       20,
	},
	{
		Key:             config.KeyBrandingFooterText,
		Levels:          config.LevelTraeger,
		Label:           "Fußzeile",
		Description:     "Dieser Text steht unten auf jeder Seite, zum Beispiel Vorstand, Amtsgericht und Steuernummer.",
		Type:            config.FieldTextarea,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "branding",
		Category:        "briefkopf",
		SortOrder:       30,
	},
}
