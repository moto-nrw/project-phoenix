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

var brandingDefinitions = []config.Definition{
	{
		Key:             config.KeyBrandingPrimaryColor,
		Levels:          config.LevelTraeger,
		Label:           "Farbe für Briefe und Rechnungen",
		Description:     "In dieser Farbe stehen Linien und Überschriften auf den Briefen des Trägers. Klicken Sie auf den farbigen Kreis oder geben Sie einen Farbcode ein, zum Beispiel #1F4E79.",
		Type:            config.FieldColor,
		Default:         "#111827",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "branding",
		Category:        "briefkopf",
		SortOrder:       10,
		Validation:      &config.ValidationRules{Pattern: &brandingColorPattern},
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
