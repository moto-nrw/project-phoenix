package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

// Usage analytics (Nutzungsanalyse, #3603). Operator-only: the moto team
// switches the Analyse-Freigabe on after the school or its Träger agreed in
// writing, so a school admin can neither grant nor widen it.
func init() {
	config.Register(config.Definition{
		Key:             config.KeyAnalyticsFreigabe,
		Label:           "Analyse-Freigabe",
		Description:     "Nur einschalten, wenn die Schule oder ihr Träger schriftlich zugestimmt hat. Dann zeichnet moto im OGS-Portal dieser Schule Sitzungen auf. Texte, Eingaben und Bilder sind dabei ausgeblendet. Wer wiederkommt, wird an einer Kennung ohne Namen erkannt. Das Personal sieht dazu einen Hinweis. Eltern-Portal und Schul-Portal werden nie aufgezeichnet.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "system",
		Category:        "nutzungsanalyse",
		SortOrder:       10,
		AccessPolicy:    config.AccessOperatorOnly,
	})

	config.Register(config.Definition{
		Key:             config.KeyAnalyticsRecordingSamplePercent,
		Label:           "Stichprobe der Aufzeichnung (Prozent)",
		Description:     "Wie viele Sitzungen im OGS-Portal mit Analyse-Freigabe aufgezeichnet werden. Ohne Analyse-Freigabe ohne Wirkung.",
		Type:            config.FieldNumber,
		Default:         100,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "system",
		Category:        "nutzungsanalyse",
		SortOrder:       11,
		Validation:      config.Range(1, 100),
		AccessPolicy:    config.AccessOperatorOnly,
		DependsOn:       config.DependsOnEq(config.KeyAnalyticsFreigabe, true),
	})
}
