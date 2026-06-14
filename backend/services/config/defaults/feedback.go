package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

func init() {
	config.Register(config.Definition{
		Key:             config.KeyFeedbackEnabled,
		Label:           "Feedback aktiviert",
		Description:     "Ermöglicht das Erfassen und Anzeigen von Feedback. Feedback-Modal beim täglichen Checkout und Feedbackhistorie in der Kinddetailansicht. Aus Datenschutzgründen standardmäßig deaktiviert.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "feedback",
		SortOrder:       20,
	})

	minDays := float64(7)
	maxDays := float64(365)
	config.Register(config.Definition{
		Key:             config.KeyFeedbackDataRetentionDays,
		Label:           "Feedback-Aufbewahrung (Tage)",
		Description:     "Anzahl der Tage, nach denen Feedback-Einträge automatisch gelöscht werden",
		Type:            config.FieldNumber,
		Default:         90,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "gdpr",
		Category:        "feedback",
		SortOrder:       21,
		Validation:      &config.ValidationRules{Min: &minDays, Max: &maxDays},
		DependsOn:       &config.Dependency{Key: config.KeyFeedbackEnabled, Condition: "eq", Value: true},
	})
}
