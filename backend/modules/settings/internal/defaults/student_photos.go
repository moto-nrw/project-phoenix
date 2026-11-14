package defaults

import config "github.com/moto-ogs/project-phoenix/modules/settings/records"

var studentPhotosDefinitions = []config.Definition{
	{
		Key:             config.KeyStudentPhotosEnabled,
		Label:           "Kinderfotos",
		Description:     "Wer Kinder bearbeiten darf, kann in der Datenverwaltung Fotos hinterlegen. Nur mit nachweisbarer Einwilligung der Eltern. Die Fotos erscheinen in Suche, Räumen, Abholplan und beim Kind.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		// Category key is shown verbatim as the section header in the
		// settings UI (schema_builder.Label = catName). German umlaut here
		// matches the visible "Kinder" copy throughout the photo feature.
		Category:  "kinder",
		SortOrder: 50,
	},
	{
		Key:   config.KeyStudentCardActivitiesEnabled,
		Label: "Angebote auf der Kinderkarte",
		// #6487: staff open their group and see at once which offerings a
		// child has today between its arrival and pickup. Off until a
		// school switches it on.
		Description:     "Zeigt auf den Kinderkarten unter „Gruppen“ die Angebote, die ein Kind heute hat. Zum Beispiel: 14:00–15:00 Töpfer-AG.",
		Type:            config.FieldBoolean,
		Default:         false,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "kinder",
		SortOrder:       40,
	},
}
